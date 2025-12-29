package orchestration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// TaskExecutorFunc is the function type for actual task execution.
// In production, this would call an LLM API.
type TaskExecutorFunc func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error)

// parallelExecutor implements contracts.ParallelExecutor with bounded concurrency.
// CRITICAL: This component handles concurrent task execution.
// Race conditions here cause incorrect results or deadlocks.
//
// Thread-safety: Uses semaphore for concurrency control and mutex for state updates.
type parallelExecutor struct {
	mu       sync.Mutex
	sem      chan struct{}            // semaphore for bounded concurrency
	executor TaskExecutorFunc         // actual task execution function
	running  map[contracts.TaskID]bool // tracks currently running tasks
}

// NewParallelExecutor creates a new ParallelExecutor with specified max parallelism.
// If maxParallelism <= 0, defaults to 1.
// If executor is nil, uses a no-op executor that returns empty result.
func NewParallelExecutor(maxParallelism int, executor TaskExecutorFunc) contracts.ParallelExecutor {
	if maxParallelism <= 0 {
		maxParallelism = 1
	}
	if executor == nil {
		executor = defaultExecutor
	}
	return &parallelExecutor{
		sem:      make(chan struct{}, maxParallelism),
		executor: executor,
		running:  make(map[contracts.TaskID]bool),
	}
}

// NewParallelExecutorFromPolicy creates a ParallelExecutor using run policy settings.
func NewParallelExecutorFromPolicy(policy contracts.RunPolicy, executor TaskExecutorFunc) contracts.ParallelExecutor {
	return NewParallelExecutor(policy.MaxParallelism, executor)
}

// defaultExecutor is a no-op executor for testing.
func defaultExecutor(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
	return &contracts.TaskResult{
		Output: fmt.Sprintf("executed: %s", task.ID),
		Usage: contracts.Usage{
			Tokens: 100, // Non-zero for invariant check
			Cost:   contracts.Cost{Amount: 0.001, Currency: "USD"},
		},
	}, nil
}

// Execute runs a task and returns its result.
// Blocks until a concurrency slot is available.
// ctx is used for cancellation; if run.Policy.TimeoutMs > 0, a timeout is also applied.
//
// IMPORTANT: This executor is "pure" - it does NOT mutate task.State or task.Outputs.
// State management is the responsibility of Orchestrator and Scheduler.
//
// Returns error if:
// - ctx is nil or run is nil (ErrInvalidInput)
// - task not found (ErrTaskNotFound)
// - task already being executed by this executor (ErrTaskNotReady)
// - execution timeout (ErrTaskTimeout)
// - execution failed (ErrTaskFailed)
func (p *parallelExecutor) Execute(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
	if ctx == nil || run == nil {
		return nil, contracts.ErrInvalidInput
	}

	// Validate task exists and track execution
	task, err := p.validateAndTrack(run, taskID)
	if err != nil {
		return nil, err
	}
	defer p.untrack(taskID)

	// Acquire semaphore slot with ctx check (blocks if at capacity)
	select {
	case p.sem <- struct{}{}:
		defer func() { <-p.sem }()
	case <-ctx.Done():
		return nil, fmt.Errorf("task %s: semaphore acquire cancelled: %w", taskID, contracts.ErrTaskCancelled)
	}

	// Apply timeout from policy if specified
	execCtx := ctx
	if run.Policy.TimeoutMs > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, time.Duration(run.Policy.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// Execute the task
	resultCh := make(chan *contracts.TaskResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := p.executor(execCtx, task)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- result
		}
	}()

	// Wait for result or timeout/cancellation
	select {
	case result := <-resultCh:
		return result, nil

	case err := <-errCh:
		return nil, fmt.Errorf("task %s failed: %w: %v", taskID, contracts.ErrTaskFailed, err)

	case <-execCtx.Done():
		if execCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("task %s timed out: %w", taskID, contracts.ErrTaskTimeout)
		}
		return nil, fmt.Errorf("task %s cancelled: %w", taskID, contracts.ErrTaskCancelled)
	}
}

// validateAndTrack validates task exists and tracks it as being executed.
// Does NOT mutate task state - that's Orchestrator's responsibility.
func (p *parallelExecutor) validateAndTrack(run *contracts.Run, taskID contracts.TaskID) (*contracts.Task, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check run state
	if run.State != contracts.RunRunning {
		return nil, fmt.Errorf("run %s is not running: %w", run.ID, contracts.ErrTaskNotReady)
	}

	// Check task exists
	if run.Tasks == nil {
		return nil, contracts.ErrTaskNotFound
	}
	task, exists := run.Tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task %s not found: %w", taskID, contracts.ErrTaskNotFound)
	}

	// Defensive check: reject terminal tasks
	// NOTE: TaskRunning is NOT blocked here because Orchestrator sets it before calling Execute.
	// The running map handles duplicate prevention within the same executor instance.
	// Edge case (different executor instance with stale TaskRunning) is accepted for v1.
	if task.State == contracts.TaskCompleted ||
		task.State == contracts.TaskFailed ||
		task.State == contracts.TaskSkipped {
		return nil, fmt.Errorf("task %s is in terminal state %s: %w",
			taskID, task.State, contracts.ErrTaskNotReady)
	}

	// Check not already being executed by this executor
	if p.running[taskID] {
		return nil, fmt.Errorf("task %s is already being executed: %w", taskID, contracts.ErrTaskNotReady)
	}

	// Track as running (internally only, don't mutate task.State)
	p.running[taskID] = true

	return task, nil
}

// untrack removes task from internal tracking.
func (p *parallelExecutor) untrack(taskID contracts.TaskID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.running, taskID)
}
