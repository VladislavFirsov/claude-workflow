package orchestration

import (
	"context"
	"fmt"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// orchestrator implements contracts.Orchestrator with the main execution loop.
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

	// queued tracks tasks already in queue to prevent duplicates
	queued map[contracts.TaskID]struct{}
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
		queued:         make(map[contracts.TaskID]struct{}),
	}
}

// Run executes all tasks in the run according to the dependency graph.
func (o *orchestrator) Run(ctx context.Context, run *contracts.Run) error {
	// Init
	if err := o.init(run); err != nil {
		return err
	}

	// Initial ready queue
	if _, err := o.enqueueReady(run); err != nil {
		run.State = contracts.RunFailed
		return err
	}

	lastProgress := 0 // completed task count at last check

	// Execute loop
	for {
		select {
		case <-ctx.Done():
			run.State = contracts.RunAborted
			return ctx.Err()
		default:
		}

		taskID, ok := o.queue.Dequeue()
		if !ok {
			// Check terminal
			if o.allTerminal(run) {
				if o.hasFailures(run) {
					run.State = contracts.RunFailed
				} else {
					run.State = contracts.RunCompleted
				}
				return nil
			}

			// Try to refill
			added, err := o.enqueueReady(run)
			if err != nil {
				run.State = contracts.RunFailed
				return err
			}

			// Deadlock detection: no tasks added and no progress
			currentCompleted := o.countTerminal(run)
			if added == 0 && currentCompleted == lastProgress {
				run.State = contracts.RunFailed
				return contracts.ErrDeadlock
			}
			lastProgress = currentCompleted
			continue
		}

		if err := o.executeTask(ctx, run, taskID); err != nil {
			run.State = contracts.RunFailed
			return err
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

// enqueueReady adds ready tasks to the queue, skipping already queued tasks.
func (o *orchestrator) enqueueReady(run *contracts.Run) (int, error) {
	ready, err := o.scheduler.NextReady(run)
	if err != nil {
		return 0, err
	}
	added := 0
	for _, taskID := range ready {
		if _, exists := o.queued[taskID]; exists {
			continue // already in queue, skip
		}
		o.queued[taskID] = struct{}{}
		o.queue.Enqueue(taskID)
		added++
	}
	return added, nil
}

// getTask safely retrieves a task from the run.
func (o *orchestrator) getTask(run *contracts.Run, taskID contracts.TaskID) (*contracts.Task, error) {
	if run.Tasks == nil {
		return nil, contracts.ErrTaskNotFound
	}
	task, ok := run.Tasks[taskID]
	if !ok || task == nil {
		return nil, contracts.ErrTaskNotFound
	}
	return task, nil
}

// executeTask executes a single task with full error handling.
func (o *orchestrator) executeTask(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) error {
	// Remove from queued set (task is being processed)
	delete(o.queued, taskID)

	task, err := o.getTask(run, taskID)
	if err != nil {
		return err
	}

	// Build context
	bundle, err := o.contextBuilder.Build(run, taskID)
	if err != nil {
		return fmt.Errorf("context build failed: %w", err)
	}

	compacted, err := o.compactor.Compact(bundle, run.Policy.ContextPolicy)
	if err != nil {
		return fmt.Errorf("context compact failed: %w", err)
	}

	// Estimate cost
	tokens, err := o.tokenEstimator.Estimate(task.Inputs, compacted)
	if err != nil {
		return fmt.Errorf("token estimation failed: %w", err)
	}

	cost, err := o.costCalc.Estimate(tokens, task.Model)
	if err != nil {
		return fmt.Errorf("cost estimation failed: %w", err)
	}

	// Budget check (pre-execution)
	if err := o.budgetEnforcer.Allow(run, cost); err != nil {
		return err // ErrBudgetExceeded
	}

	// Mark as running before execution
	task.State = contracts.TaskRunning

	// Execute
	result, err := o.executor.Execute(ctx, run, taskID)
	if err != nil {
		task.State = contracts.TaskFailed
		return err
	}

	// Validate usage (executor must report non-zero usage on success)
	if result.Usage.Tokens == 0 {
		return fmt.Errorf("executor returned zero usage for task %s", taskID)
	}

	// Record usage (post-execution)
	if err := o.budgetEnforcer.Record(run, result.Usage.Cost); err != nil {
		return fmt.Errorf("budget record failed: %w", err)
	}
	o.usageTracker.Add(run, result.Usage) // Add() returns no error per interface

	// Route to dependents
	if run.DAG != nil && run.DAG.Nodes != nil {
		if node, ok := run.DAG.Nodes[taskID]; ok && node != nil {
			for _, depID := range node.Next {
				if err := o.router.Route(run, taskID, depID, result); err != nil {
					return fmt.Errorf("routing failed: %w", err)
				}
			}
		}
	}

	// Mark complete
	return o.scheduler.MarkComplete(run, taskID, result)
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

// countTerminal counts tasks in terminal states.
func (o *orchestrator) countTerminal(run *contracts.Run) int {
	count := 0
	for _, task := range run.Tasks {
		if isTerminal(task.State) {
			count++
		}
	}
	return count
}
