package contracts

import "errors"

// Sentinel errors for the runtime layer.
var (
	// Budget errors
	ErrBudgetExceeded = errors.New("budget exceeded")
	ErrBudgetNotSet   = errors.New("budget not set")

	// Task errors
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskNotReady   = errors.New("task not ready for execution")
	ErrTaskFailed     = errors.New("task execution failed")
	ErrTaskTimeout    = errors.New("task execution timeout")
	ErrTaskCancelled  = errors.New("task cancelled")

	// Run errors
	ErrRunNotFound    = errors.New("run not found")
	ErrRunCompleted   = errors.New("run already completed")
	ErrRunAborted     = errors.New("run aborted")

	// DAG errors
	ErrDAGCycle       = errors.New("cycle detected in task dependencies")
	ErrDAGInvalid     = errors.New("invalid DAG structure")
	ErrDepNotFound    = errors.New("dependency task not found")

	// Context errors
	ErrContextTooLarge = errors.New("context exceeds maximum token limit")
	ErrContextEmpty    = errors.New("context bundle is empty")

	// Estimation errors
	ErrEstimationFailed = errors.New("token estimation failed")
	ErrModelUnknown     = errors.New("unknown model for cost calculation")

	// Input validation errors
	ErrInvalidInput = errors.New("invalid input: nil or malformed")
)
