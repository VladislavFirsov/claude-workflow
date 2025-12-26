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
	}, nil
}

// Execute runs a task and returns its result.
// Blocks until a concurrency slot is available.
// Returns error if:
// - run is nil (ErrInvalidInput)
// - task not found (ErrTaskNotFound)
// - task not ready (ErrTaskNotReady)
// - task already running (ErrTaskNotReady)
// - execution timeout (ErrTaskTimeout)
// - execution failed (ErrTaskFailed)
func (p *parallelExecutor) Execute(run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
	if run == nil {
		return nil, contracts.ErrInvalidInput
	}

	// Validate task exists
	task, err := p.validateAndMarkRunning(run, taskID)
	if err != nil {
		return nil, err
	}

	// Acquire semaphore slot (blocks if at capacity)
	p.sem <- struct{}{}
	defer func() { <-p.sem }()

	// Create context with timeout if specified
	ctx := context.Background()
	if run.Policy.TimeoutMs > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(run.Policy.TimeoutMs)*time.Millisecond)
		defer cancel()
	}

	// Execute the task
	resultCh := make(chan *contracts.TaskResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := p.executor(ctx, task)
		if err != nil {
			errCh <- err
		} else {
			resultCh <- result
		}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultCh:
		p.markCompleted(run, taskID, result)
		return result, nil

	case err := <-errCh:
		p.markFailed(run, taskID, err)
		return nil, fmt.Errorf("task %s failed: %w: %v", taskID, contracts.ErrTaskFailed, err)

	case <-ctx.Done():
		p.markFailed(run, taskID, ctx.Err())
		return nil, fmt.Errorf("task %s timed out: %w", taskID, contracts.ErrTaskTimeout)
	}
}

// validateAndMarkRunning validates task state and marks it as running.
func (p *parallelExecutor) validateAndMarkRunning(run *contracts.Run, taskID contracts.TaskID) (*contracts.Task, error) {
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

	// Check task is ready to execute
	if task.State != contracts.TaskPending && task.State != contracts.TaskReady {
		return nil, fmt.Errorf("task %s is in state %s: %w", taskID, task.State, contracts.ErrTaskNotReady)
	}

	// Check not already running (tracked internally)
	if p.running[taskID] {
		return nil, fmt.Errorf("task %s is already running: %w", taskID, contracts.ErrTaskNotReady)
	}

	// Mark as running
	task.State = contracts.TaskRunning
	p.running[taskID] = true

	return task, nil
}

// markCompleted marks task as completed and stores result.
func (p *parallelExecutor) markCompleted(run *contracts.Run, taskID contracts.TaskID, result *contracts.TaskResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if task, exists := run.Tasks[taskID]; exists {
		task.State = contracts.TaskCompleted
		task.Outputs = result
	}
	delete(p.running, taskID)
}

// markFailed marks task as failed and stores error.
func (p *parallelExecutor) markFailed(run *contracts.Run, taskID contracts.TaskID, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if task, exists := run.Tasks[taskID]; exists {
		task.State = contracts.TaskFailed
		task.Error = &contracts.TaskError{
			Code:    "EXECUTION_FAILED",
			Message: err.Error(),
		}
	}
	delete(p.running, taskID)
}
