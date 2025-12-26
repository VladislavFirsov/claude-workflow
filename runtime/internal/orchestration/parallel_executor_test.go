package orchestration

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestParallelExecutor_Execute(t *testing.T) {
	tests := []struct {
		name    string
		run     *contracts.Run
		taskID  contracts.TaskID
		wantErr error
	}{
		{
			name:    "nil run returns error",
			run:     nil,
			taskID:  "task-1",
			wantErr: contracts.ErrInvalidInput,
		},
		{
			name: "run not running returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunPending,
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotReady,
		},
		{
			name: "nil tasks returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: nil,
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotFound,
		},
		{
			name: "task not found returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: map[contracts.TaskID]*contracts.Task{},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotFound,
		},
		{
			name: "completed task returns error (defensive check)",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskCompleted},
				},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotReady,
		},
		{
			name: "failed task returns error (defensive check)",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskFailed},
				},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotReady,
		},
		{
			name: "skipped task returns error (defensive check)",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskSkipped},
				},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotReady,
		},
		{
			name: "pending task executes successfully",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskPending},
				},
			},
			taskID:  "task-1",
			wantErr: nil,
		},
		{
			name: "ready task executes successfully",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskReady},
				},
			},
			taskID:  "task-1",
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := NewParallelExecutor(2, nil)
			result, err := executor.Execute(context.Background(), tt.run, tt.taskID)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Execute() expected error containing %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Execute() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			if result == nil {
				t.Fatal("Execute() result is nil")
			}

			// Note: ParallelExecutor is now "pure" - it does NOT mutate task.State
			// State management is handled by Orchestrator + Scheduler
		})
	}
}

func TestParallelExecutor_ExecutorFunc(t *testing.T) {
	called := false
	customExecutor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		called = true
		return &contracts.TaskResult{Output: "custom output"}, nil
	}

	executor := NewParallelExecutor(1, customExecutor)

	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	result, err := executor.Execute(context.Background(), run, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Error("custom executor was not called")
	}

	if result.Output != "custom output" {
		t.Errorf("output = %q, want %q", result.Output, "custom output")
	}
}

func TestParallelExecutor_ExecutorError(t *testing.T) {
	failingExecutor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		return nil, errors.New("execution failed")
	}

	executor := NewParallelExecutor(1, failingExecutor)

	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	_, err := executor.Execute(context.Background(), run, "task-1")
	if !errors.Is(err, contracts.ErrTaskFailed) {
		t.Errorf("expected ErrTaskFailed, got %v", err)
	}

	// Note: ParallelExecutor is now "pure" - it does NOT mutate task.State
	// Orchestrator is responsible for setting TaskFailed on error
}

func TestParallelExecutor_Timeout(t *testing.T) {
	slowExecutor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		select {
		case <-time.After(1 * time.Second):
			return &contracts.TaskResult{Output: "done"}, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	executor := NewParallelExecutor(1, slowExecutor)

	run := &contracts.Run{
		ID:     "run-1",
		State:  contracts.RunRunning,
		Policy: contracts.RunPolicy{TimeoutMs: 50}, // 50ms timeout
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	_, err := executor.Execute(context.Background(), run, "task-1")
	if !errors.Is(err, contracts.ErrTaskTimeout) {
		t.Errorf("expected ErrTaskTimeout, got %v", err)
	}

	// Note: ParallelExecutor is now "pure" - it does NOT mutate task.State
	// Orchestrator is responsible for setting TaskFailed on timeout
}

func TestParallelExecutor_BoundedConcurrency(t *testing.T) {
	maxParallelism := 2
	var concurrent int32
	var maxConcurrent int32

	slowExecutor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		current := atomic.AddInt32(&concurrent, 1)
		defer atomic.AddInt32(&concurrent, -1)

		// Track max concurrent
		for {
			old := atomic.LoadInt32(&maxConcurrent)
			if current <= old || atomic.CompareAndSwapInt32(&maxConcurrent, old, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		return &contracts.TaskResult{Output: string(task.ID)}, nil
	}

	executor := NewParallelExecutor(maxParallelism, slowExecutor)

	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
			"task-2": {ID: "task-2", State: contracts.TaskPending},
			"task-3": {ID: "task-3", State: contracts.TaskPending},
			"task-4": {ID: "task-4", State: contracts.TaskPending},
		},
	}

	var wg sync.WaitGroup
	for _, taskID := range []contracts.TaskID{"task-1", "task-2", "task-3", "task-4"} {
		wg.Add(1)
		go func(id contracts.TaskID) {
			defer wg.Done()
			executor.Execute(context.Background(), run, id)
		}(taskID)
	}
	wg.Wait()

	if maxConcurrent > int32(maxParallelism) {
		t.Errorf("max concurrent = %d, exceeded limit of %d", maxConcurrent, maxParallelism)
	}
}

func TestParallelExecutor_PreventsDuplicateExecution(t *testing.T) {
	blockingExecutor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		time.Sleep(100 * time.Millisecond)
		return &contracts.TaskResult{Output: "done"}, nil
	}

	executor := NewParallelExecutor(2, blockingExecutor)

	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)

	// Try to execute the same task twice concurrently
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := executor.Execute(context.Background(), run, "task-1")
			results <- err
		}()
		time.Sleep(10 * time.Millisecond) // Small delay to ensure ordering
	}

	wg.Wait()
	close(results)

	// One should succeed, one should fail
	var successCount, failCount int
	for err := range results {
		if err == nil {
			successCount++
		} else if errors.Is(err, contracts.ErrTaskNotReady) {
			failCount++
		}
	}

	if successCount != 1 || failCount != 1 {
		t.Errorf("expected 1 success and 1 failure, got %d successes and %d failures", successCount, failCount)
	}
}

func TestParallelExecutor_DefaultMaxParallelism(t *testing.T) {
	// Zero or negative should default to 1
	executor := NewParallelExecutor(0, nil)

	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	result, err := executor.Execute(context.Background(), run, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Error("result is nil")
	}
}

func TestParallelExecutor_FromPolicy(t *testing.T) {
	policy := contracts.RunPolicy{MaxParallelism: 5}
	executor := NewParallelExecutorFromPolicy(policy, nil)

	// Verify it works
	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	_, err := executor.Execute(context.Background(), run, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParallelExecutor_ResultReturned(t *testing.T) {
	executor := NewParallelExecutor(1, func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		return &contracts.TaskResult{
			Output: "result output",
			Outputs: map[string]string{
				"key1": "value1",
			},
			Usage: contracts.Usage{Tokens: 100},
		}, nil
	})

	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	result, err := executor.Execute(context.Background(), run, "task-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check result is returned correctly
	if result.Output != "result output" {
		t.Errorf("result.Output = %q, want %q", result.Output, "result output")
	}
	if result.Outputs["key1"] != "value1" {
		t.Errorf("result.Outputs[key1] = %q, want %q", result.Outputs["key1"], "value1")
	}

	// Note: ParallelExecutor is now "pure" - it does NOT set task.Outputs
	// Scheduler.MarkComplete is responsible for that
}
