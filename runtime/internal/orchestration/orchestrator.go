package orchestration

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
	"github.com/anthropics/claude-workflow/runtime/internal/audit"
)

// orchestrator implements contracts.Orchestrator with batched execution loop.
// Key design: parallel executor I/O, sequential deterministic merge.
type orchestrator struct {
	scheduler      contracts.Scheduler
	depResolver    contracts.DependencyResolver
	queue          contracts.QueueManager
	executor       contracts.ParallelExecutor
	contextBuilder contracts.ContextBuilder
	compactor      contracts.ContextCompactor
	tokenEstimator contracts.TokenEstimator
	costCalc       contracts.CostCalculator
	budgetEnforcer contracts.BudgetEnforcer
	usageTracker   contracts.UsageTracker
	router         contracts.ContextRouter

	// onProgress is called after each successful batch merge (optional).
	onProgress func(*contracts.Run)

	// runStart tracks when the run started for duration calculation.
	runStart time.Time
}

// OrchestratorDeps contains all dependencies needed by the orchestrator.
type OrchestratorDeps struct {
	Scheduler      contracts.Scheduler
	DepResolver    contracts.DependencyResolver
	Queue          contracts.QueueManager
	Executor       contracts.ParallelExecutor
	ContextBuilder contracts.ContextBuilder
	Compactor      contracts.ContextCompactor
	TokenEstimator contracts.TokenEstimator
	CostCalc       contracts.CostCalculator
	BudgetEnforcer contracts.BudgetEnforcer
	UsageTracker   contracts.UsageTracker
	Router         contracts.ContextRouter
}

// NewOrchestrator creates a new Orchestrator with the given dependencies.
func NewOrchestrator(deps OrchestratorDeps) contracts.Orchestrator {
	return &orchestrator{
		scheduler:      deps.Scheduler,
		depResolver:    deps.DepResolver,
		queue:          deps.Queue,
		executor:       deps.Executor,
		contextBuilder: deps.ContextBuilder,
		compactor:      deps.Compactor,
		tokenEstimator: deps.TokenEstimator,
		costCalc:       deps.CostCalc,
		budgetEnforcer: deps.BudgetEnforcer,
		usageTracker:   deps.UsageTracker,
		router:         deps.Router,
	}
}

// NewOrchestratorWithCallback creates an Orchestrator with progress callback.
// The callback is called after each successful batch merge.
func NewOrchestratorWithCallback(deps OrchestratorDeps, onProgress func(*contracts.Run)) contracts.Orchestrator {
	o := NewOrchestrator(deps).(*orchestrator)
	o.onProgress = onProgress
	return o
}

// deniedResult contains info about a task denied in pre-check.
type deniedResult struct {
	taskID    contracts.TaskID
	errorCode string
	errorMsg  string
	err       error // sentinel error for proper HTTP mapping
}

// batchResult contains the result of executing a single task in a batch.
type batchResult struct {
	taskID    contracts.TaskID
	result    *contracts.TaskResult
	err       error
	startTime time.Time // for duration calculation in audit logs
}

// Run executes all tasks in the run according to the dependency graph.
// Uses batched execution: parallel executor I/O, sequential deterministic merge.
// Fail-fast: any task failure terminates the run immediately.
func (o *orchestrator) Run(ctx context.Context, run *contracts.Run) error {
	o.runStart = time.Now()
	batchNum := 0

	// Init
	if err := o.init(run); err != nil {
		return err
	}

	// Main batched execution loop
	for {
		batchNum++
		select {
		case <-ctx.Done():
			run.State = contracts.RunAborted
			audit.Log("event=run_aborted run_id=%s duration_ms=%d reason=context_cancelled",
				run.ID, time.Since(o.runStart).Milliseconds())
			return ctx.Err()
		default:
		}

		// 1. Get ready tasks (sorted by TaskID for determinism)
		ready, err := o.scheduler.NextReady(run)
		if err != nil {
			run.State = contracts.RunFailed
			audit.Log("event=run_failed run_id=%s duration_ms=%d error_code=scheduler_error error_msg=%s",
				run.ID, time.Since(o.runStart).Milliseconds(), err.Error())
			return err
		}

		// 2. Check termination (all tasks terminal)
		if len(ready) == 0 {
			if o.allTerminal(run) {
				// Check if any task failed - if so, run is failed
				if o.hasFailures(run) {
					run.State = contracts.RunFailed
					// Note: individual task failures already logged, no separate run_failed here
				} else {
					run.State = contracts.RunCompleted
					audit.Log("event=run_completed run_id=%s duration_ms=%d total_tokens=%d total_cost=%.4f%s state=completed",
						run.ID, time.Since(o.runStart).Milliseconds(), run.Usage.Tokens,
						run.Usage.Cost.Amount, run.Usage.Cost.Currency)
				}
				return nil
			}
			// Unreachable if fail-fast works correctly
			run.State = contracts.RunFailed
			audit.Log("event=run_failed run_id=%s duration_ms=%d error_code=deadlock",
				run.ID, time.Since(o.runStart).Milliseconds())
			return contracts.ErrDeadlock
		}

		// 3. Pre-check budget SEQUENTIALLY (deterministic)
		allowed, deniedResults := o.preCheckBudget(run, ready)

		// 4. Handle denied tasks with fail-fast
		if len(deniedResults) > 0 {
			// Mark ALL denied tasks as failed for auditability
			for _, dr := range deniedResults {
				task, exists := run.Tasks[dr.taskID]
				if exists {
					task.State = contracts.TaskFailed
					task.Error = &contracts.TaskError{
						Code:    dr.errorCode,
						Message: dr.errorMsg,
					}
				}
			}
			// Return error for first denied task (with sentinel wrapped)
			dr := deniedResults[0]
			run.State = contracts.RunFailed
			audit.Log("event=run_failed run_id=%s duration_ms=%d error_code=%s task_id=%s",
				run.ID, time.Since(o.runStart).Milliseconds(), dr.errorCode, dr.taskID)
			return fmt.Errorf("task %s: %s: %w", dr.taskID, dr.errorMsg, dr.err)
		}

		// 5. Log batch started
		taskIDStrs := make([]string, len(allowed))
		for i, tid := range allowed {
			taskIDStrs[i] = string(tid)
		}
		audit.Log("event=batch_started run_id=%s batch=%d task_count=%d tasks=%s",
			run.ID, batchNum, len(allowed), strings.Join(taskIDStrs, ","))
		batchStart := time.Now()

		// 6. Execute allowed batch (parallel executor calls, NO mutations except TaskRunning)
		results := o.executeBatch(ctx, run, allowed)

		// 7. Deterministic merge (sequential, sorted by TaskID)
		// Returns error on first failure (fail-fast)
		if err := o.mergeBatchResults(run, results); err != nil {
			run.State = contracts.RunFailed
			audit.Log("event=run_failed run_id=%s duration_ms=%d error_code=merge_failed error_msg=%s",
				run.ID, time.Since(o.runStart).Milliseconds(), err.Error())
			return err
		}

		// 8. Log batch completed
		audit.Log("event=batch_completed run_id=%s batch=%d duration_ms=%d tasks_completed=%d",
			run.ID, batchNum, time.Since(batchStart).Milliseconds(), len(allowed))

		// 9. Call progress callback if set
		if o.onProgress != nil {
			o.onProgress(run)
		}
	}
}

// init validates the run and marks it as running.
func (o *orchestrator) init(run *contracts.Run) error {
	if run == nil || run.DAG == nil {
		return contracts.ErrInvalidInput
	}
	if err := o.depResolver.Validate(run.DAG); err != nil {
		run.State = contracts.RunFailed
		audit.Log("event=run_failed run_id=%s duration_ms=%d error_code=dag_validation error_msg=%s",
			run.ID, time.Since(o.runStart).Milliseconds(), err.Error())
		return err
	}
	run.State = contracts.RunRunning
	audit.Log("event=run_started run_id=%s policy_timeout_ms=%d policy_parallelism=%d policy_budget=%.2f%s",
		run.ID, run.Policy.TimeoutMs, run.Policy.MaxParallelism,
		run.Policy.BudgetLimit.Amount, run.Policy.BudgetLimit.Currency)
	return nil
}

// preCheckBudget checks budget SEQUENTIALLY for determinism.
// Returns (allowed, denied) — denied contains detailed error codes.
// Budget is "reserved" for allowed tasks to prevent over-commitment in batch.
func (o *orchestrator) preCheckBudget(
	run *contracts.Run,
	taskIDs []contracts.TaskID,
) (allowed []contracts.TaskID, denied []deniedResult) {
	// Track reserved cost for this batch to prevent over-commitment
	var reservedCost contracts.Cost

	for _, tid := range taskIDs {
		// Guard: validate task exists
		task, exists := run.Tasks[tid]
		if !exists {
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "task_not_found",
				errorMsg:  fmt.Sprintf("task %s not found in run", tid),
				err:       contracts.ErrTaskNotFound,
			})
			continue
		}

		// Build context for estimation
		bundle, err := o.contextBuilder.Build(run, tid)
		if err != nil {
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "context_build_failed",
				errorMsg:  fmt.Sprintf("failed to build context: %v", err),
				err:       err,
			})
			continue
		}

		// Compact context
		compacted, err := o.compactor.Compact(bundle, run.Policy.ContextPolicy)
		if err != nil {
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "context_compact_failed",
				errorMsg:  fmt.Sprintf("failed to compact context: %v", err),
				err:       err,
			})
			continue
		}

		// Estimate tokens
		tokens, err := o.tokenEstimator.Estimate(task.Inputs, compacted)
		if err != nil {
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "token_estimation_failed",
				errorMsg:  fmt.Sprintf("failed to estimate tokens: %v", err),
				err:       err,
			})
			continue
		}

		// Estimate cost
		cost, err := o.costCalc.Estimate(tokens, task.Model)
		if err != nil {
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "model_unknown",
				errorMsg:  fmt.Sprintf("failed to estimate cost for model %s: %v", task.Model, err),
				err:       err,
			})
			continue
		}

		// Pre-check budget INCLUDING already reserved cost for this batch
		// This prevents over-commitment when multiple tasks pass Allow() individually
		totalEstimate := contracts.Cost{
			Amount:   cost.Amount + reservedCost.Amount,
			Currency: cost.Currency,
		}
		if err := o.budgetEnforcer.Allow(run, totalEstimate); err != nil {
			audit.Log("event=budget_precheck_failed run_id=%s task_id=%s estimated_cost=%.4f%s reason=budget_exceeded",
				run.ID, tid, cost.Amount, cost.Currency)
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "budget_exceeded",
				errorMsg:  fmt.Sprintf("budget pre-check failed: %v", err),
				err:       contracts.ErrBudgetExceeded,
			})
			continue
		}

		// Budget precheck passed
		audit.Log("event=budget_precheck_ok run_id=%s task_id=%s estimated_tokens=%d estimated_cost=%.4f%s",
			run.ID, tid, tokens, cost.Amount, cost.Currency)

		// Reserve this cost for subsequent checks in this batch
		reservedCost.Amount += cost.Amount
		if reservedCost.Currency == "" {
			reservedCost.Currency = cost.Currency
		}

		allowed = append(allowed, tid)
	}
	return allowed, denied
}

// executeBatch executes tasks in parallel (executor I/O only).
// Each goroutine sets task.State = TaskRunning (safe: each touches different task).
// Returns results slice with same indices as input taskIDs.
func (o *orchestrator) executeBatch(
	ctx context.Context,
	run *contracts.Run,
	taskIDs []contracts.TaskID,
) []batchResult {
	results := make([]batchResult, len(taskIDs))
	var wg sync.WaitGroup

	for i, taskID := range taskIDs {
		wg.Add(1)
		go func(idx int, tid contracts.TaskID) {
			defer wg.Done()

			// Validate task exists
			task, exists := run.Tasks[tid]
			if !exists {
				results[idx] = batchResult{
					taskID:    tid,
					err:       fmt.Errorf("task %s not found", tid),
					startTime: time.Now(),
				}
				return
			}

			// Log task started (after existence check to avoid panic)
			taskStart := time.Now()
			audit.Log("event=task_started run_id=%s task_id=%s model=%s",
				run.ID, tid, task.Model)

			// Mark as running (safe: each goroutine touches different task)
			task.State = contracts.TaskRunning

			// Execute via ParallelExecutor (respects ctx, semaphore)
			result, err := o.executor.Execute(ctx, run, tid)
			results[idx] = batchResult{taskID: tid, result: result, err: err, startTime: taskStart}
		}(i, taskID)
	}

	wg.Wait()
	return results
}

// mergeBatchResults applies batch results SEQUENTIALLY with fail-fast.
// Results are sorted by TaskID for determinism before applying side-effects.
// Returns error on first failure.
func (o *orchestrator) mergeBatchResults(run *contracts.Run, results []batchResult) error {
	// 1. Sort by TaskID for determinism
	sort.Slice(results, func(i, j int) bool {
		return string(results[i].taskID) < string(results[j].taskID)
	})

	// 2. Apply side-effects sequentially
	for _, r := range results {
		task, exists := run.Tasks[r.taskID]
		if !exists {
			return fmt.Errorf("task %s not found during merge", r.taskID)
		}

		if r.err != nil {
			// Mark task failed with error
			task.State = contracts.TaskFailed
			task.Error = &contracts.TaskError{
				Code:    "execution_failed",
				Message: r.err.Error(),
			}
			durationMs := time.Since(r.startTime).Milliseconds()
			audit.Log("event=task_failed run_id=%s task_id=%s duration_ms=%d error_code=execution_failed error_msg=%s",
				run.ID, r.taskID, durationMs, r.err.Error())
			// FAIL-FAST: return immediately
			return fmt.Errorf("task %s execution failed: %w", r.taskID, r.err)
		}

		// Validate result
		if r.result == nil || r.result.Usage.Tokens == 0 {
			task.State = contracts.TaskFailed
			task.Error = &contracts.TaskError{
				Code:    "invalid_result",
				Message: "executor returned nil or zero usage",
			}
			durationMs := time.Since(r.startTime).Milliseconds()
			audit.Log("event=task_failed run_id=%s task_id=%s duration_ms=%d error_code=invalid_result error_msg=executor returned nil or zero usage",
				run.ID, r.taskID, durationMs)
			return fmt.Errorf("task %s: invalid result", r.taskID)
		}

		// Record budget (may fail if over budget post-execution)
		if err := o.budgetEnforcer.Record(run, r.result.Usage.Cost); err != nil {
			task.State = contracts.TaskFailed
			task.Error = &contracts.TaskError{
				Code:    "budget_exceeded",
				Message: err.Error(),
			}
			audit.Log("event=budget_record_failed run_id=%s task_id=%s actual_cost=%.4f%s reason=exceeded",
				run.ID, r.taskID, r.result.Usage.Cost.Amount, r.result.Usage.Cost.Currency)
			return fmt.Errorf("task %s budget exceeded: %w", r.taskID, err)
		}

		// Budget record succeeded
		audit.Log("event=budget_record_ok run_id=%s task_id=%s actual_cost=%.4f%s",
			run.ID, r.taskID, r.result.Usage.Cost.Amount, r.result.Usage.Cost.Currency)

		// Track usage
		o.usageTracker.Add(run, r.result.Usage)

		// Scheduler.MarkComplete: sets task.State = Completed, task.Outputs = result
		// This is the ONLY place where task state becomes Completed
		if err := o.scheduler.MarkComplete(run, r.taskID, r.result); err != nil {
			task.State = contracts.TaskFailed
			task.Error = &contracts.TaskError{
				Code:    "scheduler_error",
				Message: err.Error(),
			}
			durationMs := time.Since(r.startTime).Milliseconds()
			audit.Log("event=task_failed run_id=%s task_id=%s duration_ms=%d error_code=scheduler_error error_msg=%s",
				run.ID, r.taskID, durationMs, err.Error())
			return fmt.Errorf("task %s scheduler error: %w", r.taskID, err)
		}

		// Task completed successfully - log after all finalization steps
		durationMs := time.Since(r.startTime).Milliseconds()
		audit.Log("event=task_completed run_id=%s task_id=%s duration_ms=%d tokens=%d cost=%.4f%s",
			run.ID, r.taskID, durationMs, r.result.Usage.Tokens,
			r.result.Usage.Cost.Amount, r.result.Usage.Cost.Currency)

		// Route to dependents: iterate DAG.Nodes[taskID].Next
		// Routing errors are FATAL — inconsistent context state
		node, nodeExists := run.DAG.Nodes[r.taskID]
		if !nodeExists {
			task.State = contracts.TaskFailed
			task.Error = &contracts.TaskError{
				Code:    "dag_inconsistent",
				Message: fmt.Sprintf("DAG node for task %s not found", r.taskID),
			}
			return fmt.Errorf("task %s: DAG node not found", r.taskID)
		}
		for _, depID := range node.Next {
			if err := o.router.Route(run, r.taskID, depID, r.result); err != nil {
				// Mark the dependent task as failed (not the completed one)
				depTask, depExists := run.Tasks[depID]
				if depExists {
					depTask.State = contracts.TaskFailed
					depTask.Error = &contracts.TaskError{
						Code:    "routing_failed",
						Message: fmt.Sprintf("failed to route from %s: %v", r.taskID, err),
					}
				}
				return fmt.Errorf("routing from %s to %s failed: %w", r.taskID, depID, err)
			}
		}
	}

	return nil
}

// isTerminal checks if a task state is terminal (no further processing needed).
func isTerminal(state contracts.TaskState) bool {
	return state == contracts.TaskCompleted ||
		state == contracts.TaskSkipped ||
		state == contracts.TaskFailed
}

// allTerminal checks if all tasks have reached a terminal state.
func (o *orchestrator) allTerminal(run *contracts.Run) bool {
	for _, task := range run.Tasks {
		if !isTerminal(task.State) {
			return false
		}
	}
	return true
}

// hasFailures checks if any task has failed.
func (o *orchestrator) hasFailures(run *contracts.Run) bool {
	for _, task := range run.Tasks {
		if task.State == contracts.TaskFailed {
			return true
		}
	}
	return false
}
