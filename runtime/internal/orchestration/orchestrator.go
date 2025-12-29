package orchestration

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
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
	taskID contracts.TaskID
	result *contracts.TaskResult
	err    error
}

// Run executes all tasks in the run according to the dependency graph.
// Uses batched execution: parallel executor I/O, sequential deterministic merge.
// Fail-fast: any task failure terminates the run immediately.
func (o *orchestrator) Run(ctx context.Context, run *contracts.Run) error {
	// Init
	if err := o.init(run); err != nil {
		return err
	}

	// Main batched execution loop
	for {
		select {
		case <-ctx.Done():
			run.State = contracts.RunAborted
			return ctx.Err()
		default:
		}

		// 1. Get ready tasks (sorted by TaskID for determinism)
		ready, err := o.scheduler.NextReady(run)
		if err != nil {
			run.State = contracts.RunFailed
			return err
		}

		// 2. Check termination (all tasks terminal)
		if len(ready) == 0 {
			if o.allTerminal(run) {
				// Check if any task failed - if so, run is failed
				if o.hasFailures(run) {
					run.State = contracts.RunFailed
				} else {
					run.State = contracts.RunCompleted
				}
				return nil
			}
			// Unreachable if fail-fast works correctly
			run.State = contracts.RunFailed
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
			return fmt.Errorf("task %s: %s: %w", dr.taskID, dr.errorMsg, dr.err)
		}

		// 5. Execute allowed batch (parallel executor calls, NO mutations except TaskRunning)
		results := o.executeBatch(ctx, run, allowed)

		// 6. Deterministic merge (sequential, sorted by TaskID)
		// Returns error on first failure (fail-fast)
		if err := o.mergeBatchResults(run, results); err != nil {
			run.State = contracts.RunFailed
			return err
		}

		// 7. Call progress callback if set
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
		return err
	}
	run.State = contracts.RunRunning
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
			denied = append(denied, deniedResult{
				taskID:    tid,
				errorCode: "budget_exceeded",
				errorMsg:  fmt.Sprintf("budget pre-check failed: %v", err),
				err:       contracts.ErrBudgetExceeded,
			})
			continue
		}

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
					taskID: tid,
					err:    fmt.Errorf("task %s not found", tid),
				}
				return
			}

			// Mark as running (safe: each goroutine touches different task)
			task.State = contracts.TaskRunning

			// Execute via ParallelExecutor (respects ctx, semaphore)
			result, err := o.executor.Execute(ctx, run, tid)
			results[idx] = batchResult{taskID: tid, result: result, err: err}
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
			return fmt.Errorf("task %s: invalid result", r.taskID)
		}

		// Record budget (may fail if over budget post-execution)
		if err := o.budgetEnforcer.Record(run, r.result.Usage.Cost); err != nil {
			task.State = contracts.TaskFailed
			task.Error = &contracts.TaskError{
				Code:    "budget_exceeded",
				Message: err.Error(),
			}
			return fmt.Errorf("task %s budget exceeded: %w", r.taskID, err)
		}

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
			return fmt.Errorf("task %s scheduler error: %w", r.taskID, err)
		}

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
