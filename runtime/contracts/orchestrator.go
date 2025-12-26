package contracts

import "context"

// Orchestrator coordinates the execution of a run's tasks according to their DAG.
type Orchestrator interface {
	// Run executes all tasks in the run according to the dependency graph.
	// It manages the full lifecycle: validation, scheduling, execution, and completion.
	//
	// Returns nil on successful completion (run.State == RunCompleted).
	// Returns error on:
	// - ErrInvalidInput: run or DAG is nil
	// - ErrDAGInvalid: DAG validation failed
	// - ErrBudgetExceeded: budget limit reached
	// - ErrDeadlock: no progress possible
	// - context.Canceled/DeadlineExceeded: context cancelled or timed out
	// - other errors from task execution
	//
	// On error, run.State is set to RunFailed or RunAborted.
	Run(ctx context.Context, run *Run) error
}
