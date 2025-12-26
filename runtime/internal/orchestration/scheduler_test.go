package orchestration

import (
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestScheduler_NextReady(t *testing.T) {
	scheduler := NewScheduler()

	tests := []struct {
		name      string
		run       *contracts.Run
		wantTasks []contracts.TaskID
		wantErr   error
	}{
		{
			name:    "nil run returns error",
			run:     nil,
			wantErr: contracts.ErrInvalidInput,
		},
		{
			name: "run not in running state returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunPending,
			},
			wantErr: contracts.ErrRunCompleted,
		},
		{
			name: "run with nil DAG returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG:   nil,
			},
			wantErr: contracts.ErrDAGInvalid,
		},
		{
			name: "run with nil Tasks returns empty",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{},
				},
				Tasks: nil,
			},
			wantTasks: []contracts.TaskID{},
		},
		{
			name: "run with empty Tasks returns empty",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{},
			},
			wantTasks: []contracts.TaskID{},
		},
		{
			name: "single ready task",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Pending: 0},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskPending},
				},
			},
			wantTasks: []contracts.TaskID{"task-1"},
		},
		{
			name: "multiple ready tasks sorted by ID",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-c": {ID: "task-c", Pending: 0},
						"task-a": {ID: "task-a", Pending: 0},
						"task-b": {ID: "task-b", Pending: 0},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-c": {ID: "task-c", State: contracts.TaskPending},
					"task-a": {ID: "task-a", State: contracts.TaskPending},
					"task-b": {ID: "task-b", State: contracts.TaskReady},
				},
			},
			wantTasks: []contracts.TaskID{"task-a", "task-b", "task-c"},
		},
		{
			name: "task with pending deps not returned",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Pending: 0},
						"task-2": {ID: "task-2", Pending: 1}, // has pending dep
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskPending},
					"task-2": {ID: "task-2", State: contracts.TaskPending},
				},
			},
			wantTasks: []contracts.TaskID{"task-1"},
		},
		{
			name: "completed task not returned",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Pending: 0},
						"task-2": {ID: "task-2", Pending: 0},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskCompleted},
					"task-2": {ID: "task-2", State: contracts.TaskPending},
				},
			},
			wantTasks: []contracts.TaskID{"task-2"},
		},
		{
			name: "failed task not returned",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Pending: 0},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskFailed},
				},
			},
			wantTasks: []contracts.TaskID{},
		},
		{
			name: "running task not returned",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Pending: 0},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskRunning},
				},
			},
			wantTasks: []contracts.TaskID{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := scheduler.NextReady(tt.run)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("NextReady() expected error containing %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("NextReady() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("NextReady() unexpected error = %v", err)
			}

			if len(got) != len(tt.wantTasks) {
				t.Fatalf("NextReady() = %v, want %v", got, tt.wantTasks)
			}

			for i, taskID := range got {
				if taskID != tt.wantTasks[i] {
					t.Errorf("NextReady()[%d] = %v, want %v", i, taskID, tt.wantTasks[i])
				}
			}
		})
	}
}

func TestScheduler_MarkComplete(t *testing.T) {
	tests := []struct {
		name    string
		run     *contracts.Run
		taskID  contracts.TaskID
		result  *contracts.TaskResult
		wantErr error
		verify  func(t *testing.T, run *contracts.Run)
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
				State: contracts.RunCompleted,
			},
			taskID:  "task-1",
			wantErr: contracts.ErrRunCompleted,
		},
		{
			name: "nil DAG returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG:   nil,
			},
			taskID:  "task-1",
			wantErr: contracts.ErrDAGInvalid,
		},
		{
			name: "nil Tasks returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG:   &contracts.DAG{},
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
				DAG:   &contracts.DAG{},
				Tasks: map[contracts.TaskID]*contracts.Task{},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotFound,
		},
		{
			name: "already completed task returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG:   &contracts.DAG{},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskCompleted},
				},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotReady,
		},
		{
			name: "failed task returns error",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG:   &contracts.DAG{},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskFailed},
				},
			},
			taskID:  "task-1",
			wantErr: contracts.ErrTaskNotReady,
		},
		{
			name: "successful completion updates state",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Next: []contracts.TaskID{}},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskRunning},
				},
			},
			taskID: "task-1",
			result: &contracts.TaskResult{Output: "done"},
			verify: func(t *testing.T, run *contracts.Run) {
				task := run.Tasks["task-1"]
				if task.State != contracts.TaskCompleted {
					t.Errorf("task state = %v, want %v", task.State, contracts.TaskCompleted)
				}
				if task.Outputs == nil || task.Outputs.Output != "done" {
					t.Errorf("task outputs not set correctly")
				}
			},
		},
		{
			name: "completion decrements dependent pending counts",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1", Next: []contracts.TaskID{"task-2", "task-3"}},
						"task-2": {ID: "task-2", Pending: 1},
						"task-3": {ID: "task-3", Pending: 2},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskRunning},
					"task-2": {ID: "task-2", State: contracts.TaskPending},
					"task-3": {ID: "task-3", State: contracts.TaskPending},
				},
			},
			taskID: "task-1",
			result: &contracts.TaskResult{Output: "done"},
			verify: func(t *testing.T, run *contracts.Run) {
				node2 := run.DAG.Nodes["task-2"]
				if node2.Pending != 0 {
					t.Errorf("task-2 pending = %d, want 0", node2.Pending)
				}
				node3 := run.DAG.Nodes["task-3"]
				if node3.Pending != 1 {
					t.Errorf("task-3 pending = %d, want 1", node3.Pending)
				}
			},
		},
		{
			name: "pending task can be completed",
			run: &contracts.Run{
				ID:    "run-1",
				State: contracts.RunRunning,
				DAG: &contracts.DAG{
					Nodes: map[contracts.TaskID]*contracts.DAGNode{
						"task-1": {ID: "task-1"},
					},
				},
				Tasks: map[contracts.TaskID]*contracts.Task{
					"task-1": {ID: "task-1", State: contracts.TaskPending},
				},
			},
			taskID: "task-1",
			result: &contracts.TaskResult{Output: "done"},
			verify: func(t *testing.T, run *contracts.Run) {
				if run.Tasks["task-1"].State != contracts.TaskCompleted {
					t.Errorf("task not marked completed")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheduler := NewScheduler()
			err := scheduler.MarkComplete(tt.run, tt.taskID, tt.result)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("MarkComplete() expected error containing %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("MarkComplete() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("MarkComplete() unexpected error = %v", err)
			}

			if tt.verify != nil {
				tt.verify(t, tt.run)
			}
		})
	}
}

func TestScheduler_Integration(t *testing.T) {
	scheduler := NewScheduler()

	// Build a simple DAG: task-1 → task-2 → task-3
	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		DAG: &contracts.DAG{
			Nodes: map[contracts.TaskID]*contracts.DAGNode{
				"task-1": {ID: "task-1", Pending: 0, Next: []contracts.TaskID{"task-2"}},
				"task-2": {ID: "task-2", Pending: 1, Next: []contracts.TaskID{"task-3"}},
				"task-3": {ID: "task-3", Pending: 1, Next: []contracts.TaskID{}},
			},
		},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
			"task-2": {ID: "task-2", State: contracts.TaskPending},
			"task-3": {ID: "task-3", State: contracts.TaskPending},
		},
	}

	// Step 1: Only task-1 should be ready
	ready, err := scheduler.NextReady(run)
	if err != nil {
		t.Fatalf("Step 1: NextReady() error = %v", err)
	}
	if len(ready) != 1 || ready[0] != "task-1" {
		t.Fatalf("Step 1: NextReady() = %v, want [task-1]", ready)
	}

	// Step 2: Complete task-1
	err = scheduler.MarkComplete(run, "task-1", &contracts.TaskResult{Output: "result-1"})
	if err != nil {
		t.Fatalf("Step 2: MarkComplete() error = %v", err)
	}

	// Step 3: Now task-2 should be ready
	ready, err = scheduler.NextReady(run)
	if err != nil {
		t.Fatalf("Step 3: NextReady() error = %v", err)
	}
	if len(ready) != 1 || ready[0] != "task-2" {
		t.Fatalf("Step 3: NextReady() = %v, want [task-2]", ready)
	}

	// Step 4: Complete task-2
	err = scheduler.MarkComplete(run, "task-2", &contracts.TaskResult{Output: "result-2"})
	if err != nil {
		t.Fatalf("Step 4: MarkComplete() error = %v", err)
	}

	// Step 5: Now task-3 should be ready
	ready, err = scheduler.NextReady(run)
	if err != nil {
		t.Fatalf("Step 5: NextReady() error = %v", err)
	}
	if len(ready) != 1 || ready[0] != "task-3" {
		t.Fatalf("Step 5: NextReady() = %v, want [task-3]", ready)
	}

	// Step 6: Complete task-3
	err = scheduler.MarkComplete(run, "task-3", &contracts.TaskResult{Output: "result-3"})
	if err != nil {
		t.Fatalf("Step 6: MarkComplete() error = %v", err)
	}

	// Step 7: No more tasks should be ready
	ready, err = scheduler.NextReady(run)
	if err != nil {
		t.Fatalf("Step 7: NextReady() error = %v", err)
	}
	if len(ready) != 0 {
		t.Fatalf("Step 7: NextReady() = %v, want []", ready)
	}
}

func TestScheduler_ParallelTasks(t *testing.T) {
	scheduler := NewScheduler()

	// DAG with parallel execution: task-1 → [task-2a, task-2b] → task-3
	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunRunning,
		DAG: &contracts.DAG{
			Nodes: map[contracts.TaskID]*contracts.DAGNode{
				"task-1":  {ID: "task-1", Pending: 0, Next: []contracts.TaskID{"task-2a", "task-2b"}},
				"task-2a": {ID: "task-2a", Pending: 1, Next: []contracts.TaskID{"task-3"}},
				"task-2b": {ID: "task-2b", Pending: 1, Next: []contracts.TaskID{"task-3"}},
				"task-3":  {ID: "task-3", Pending: 2, Next: []contracts.TaskID{}},
			},
		},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1":  {ID: "task-1", State: contracts.TaskPending},
			"task-2a": {ID: "task-2a", State: contracts.TaskPending},
			"task-2b": {ID: "task-2b", State: contracts.TaskPending},
			"task-3":  {ID: "task-3", State: contracts.TaskPending},
		},
	}

	// Complete task-1
	_ = scheduler.MarkComplete(run, "task-1", &contracts.TaskResult{})

	// Both task-2a and task-2b should be ready (sorted)
	ready, _ := scheduler.NextReady(run)
	if len(ready) != 2 {
		t.Fatalf("expected 2 ready tasks, got %d", len(ready))
	}
	if ready[0] != "task-2a" || ready[1] != "task-2b" {
		t.Errorf("ready = %v, want [task-2a, task-2b]", ready)
	}

	// Complete task-2a, task-3 still has pending=1
	_ = scheduler.MarkComplete(run, "task-2a", &contracts.TaskResult{})
	ready, _ = scheduler.NextReady(run)
	if len(ready) != 1 || ready[0] != "task-2b" {
		t.Errorf("ready = %v, want [task-2b]", ready)
	}

	// Complete task-2b, now task-3 is ready
	_ = scheduler.MarkComplete(run, "task-2b", &contracts.TaskResult{})
	ready, _ = scheduler.NextReady(run)
	if len(ready) != 1 || ready[0] != "task-3" {
		t.Errorf("ready = %v, want [task-3]", ready)
	}
}
