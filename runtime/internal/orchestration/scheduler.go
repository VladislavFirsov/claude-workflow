package orchestration

import (
	"fmt"
	"sort"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// scheduler implements contracts.Scheduler using DAG-based task scheduling.
// It determines which tasks are ready to execute based on dependency completion
// and uses topological order with TaskID as tie-breaker for determinism.
//
// Thread-safety: The scheduler assumes the caller holds appropriate locks.
// All operations on Run and DAG must be externally synchronized.
type scheduler struct{}

// NewScheduler creates a new Scheduler.
func NewScheduler() contracts.Scheduler {
	return &scheduler{}
}

// NextReady returns task IDs that are ready to execute (all deps satisfied).
// Returns empty slice if no tasks are ready.
// Returns error if run is in invalid state.
func (s *scheduler) NextReady(run *contracts.Run) ([]contracts.TaskID, error) {
	// Invariant: run must not be nil
	if run == nil {
		return nil, contracts.ErrInvalidInput
	}

	// Invariant: run must be in Running state
	if run.State != contracts.RunRunning {
		return nil, fmt.Errorf("run %s is not running (state: %s): %w", run.ID, run.State, contracts.ErrRunCompleted)
	}

	// Edge case: nil DAG
	if run.DAG == nil {
		return nil, fmt.Errorf("run %s has no DAG: %w", run.ID, contracts.ErrDAGInvalid)
	}

	// Edge case: nil or empty Tasks
	if run.Tasks == nil || len(run.Tasks) == 0 {
		return []contracts.TaskID{}, nil
	}

	// Edge case: nil DAG.Nodes
	if run.DAG.Nodes == nil {
		return nil, fmt.Errorf("run %s has nil DAG nodes: %w", run.ID, contracts.ErrDAGInvalid)
	}

	var ready []contracts.TaskID

	// Find all tasks where Pending == 0 and state is Pending or Ready
	for taskID, node := range run.DAG.Nodes {
		if node.Pending != 0 {
			continue
		}

		task, exists := run.Tasks[taskID]
		if !exists {
			// DAG node exists but task doesn't - inconsistent state, skip
			continue
		}

		// Only return tasks that are Pending or Ready (not Running, Completed, Failed, Skipped)
		if task.State == contracts.TaskPending || task.State == contracts.TaskReady {
			ready = append(ready, taskID)
		}
	}

	// Sort by TaskID for deterministic ordering
	sort.Slice(ready, func(i, j int) bool {
		return string(ready[i]) < string(ready[j])
	})

	return ready, nil
}

// MarkComplete marks a task as completed and updates the run state.
// Updates Pending counts for dependent tasks.
// Returns error if task not found or already completed.
func (s *scheduler) MarkComplete(run *contracts.Run, taskID contracts.TaskID, result *contracts.TaskResult) error {
	// Invariant: run must not be nil
	if run == nil {
		return contracts.ErrInvalidInput
	}

	// Invariant: run must be in Running state
	if run.State != contracts.RunRunning {
		return fmt.Errorf("run %s is not running (state: %s): %w", run.ID, run.State, contracts.ErrRunCompleted)
	}

	// Validate DAG exists
	if run.DAG == nil {
		return fmt.Errorf("run %s has no DAG: %w", run.ID, contracts.ErrDAGInvalid)
	}

	// Validate Tasks map exists
	if run.Tasks == nil {
		return fmt.Errorf("run %s has no tasks: %w", run.ID, contracts.ErrTaskNotFound)
	}

	// Find the task
	task, exists := run.Tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found in run %s: %w", taskID, run.ID, contracts.ErrTaskNotFound)
	}

	// Check if task is already completed (idempotency decision: error)
	if task.State == contracts.TaskCompleted {
		return fmt.Errorf("task %s already completed: %w", taskID, contracts.ErrTaskNotReady)
	}

	// Check if task is in a terminal state (Failed, Skipped)
	if task.State == contracts.TaskFailed || task.State == contracts.TaskSkipped {
		return fmt.Errorf("task %s is in terminal state %s: %w", taskID, task.State, contracts.ErrTaskNotReady)
	}

	// Update task state
	task.State = contracts.TaskCompleted
	task.Outputs = result

	// Update Pending counts for dependent tasks
	if run.DAG.Nodes != nil {
		node, exists := run.DAG.Nodes[taskID]
		if exists && node.Next != nil {
			for _, nextID := range node.Next {
				nextNode, nextExists := run.DAG.Nodes[nextID]
				if nextExists && nextNode.Pending > 0 {
					nextNode.Pending--
				}
			}
		}
	}

	return nil
}
