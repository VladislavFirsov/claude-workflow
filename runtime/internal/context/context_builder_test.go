package context

import (
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestNewContextBuilder(t *testing.T) {
	cb := NewContextBuilder()
	if cb == nil {
		t.Fatal("NewContextBuilder() returned nil")
	}

	// Verify it implements the interface
	var _ contracts.ContextBuilder = cb
}

func TestBuild_Success_SingleDependency(t *testing.T) {
	cb := NewContextBuilder()

	// Create a run with two tasks: task1 (dependency) and task2 (dependent)
	run := &contracts.Run{
		ID:   contracts.RunID("run1"),
		Tasks: make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{
			"key1": "value1",
		},
	}

	task1ID := contracts.TaskID("task1")
	task2ID := contracts.TaskID("task2")

	// Task 1 is completed with output
	run.Tasks[task1ID] = &contracts.Task{
		ID:    task1ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "task1 output",
		},
	}

	// Task 2 depends on task1
	run.Tasks[task2ID] = &contracts.Task{
		ID:   task2ID,
		Deps: []contracts.TaskID{task1ID},
	}

	bundle, err := cb.Build(run, task2ID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if bundle == nil {
		t.Fatal("Build() returned nil bundle")
	}

	// Check messages
	if len(bundle.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(bundle.Messages))
	}
	if bundle.Messages[0] != "task1 output" {
		t.Fatalf("Messages[0] = %q, want %q", bundle.Messages[0], "task1 output")
	}

	// Check memory
	if len(bundle.Memory) != 1 {
		t.Fatalf("Memory length = %d, want 1", len(bundle.Memory))
	}
	if bundle.Memory["key1"] != "value1" {
		t.Fatalf("Memory[key1] = %q, want %q", bundle.Memory["key1"], "value1")
	}

	// Check tools
	if len(bundle.Tools) != 0 {
		t.Fatalf("Tools length = %d, want 0", len(bundle.Tools))
	}
}

func TestBuild_Success_MultipleDependencies(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:    contracts.RunID("run1"),
		Tasks: make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{
			"memory_key": "memory_value",
		},
	}

	task1ID := contracts.TaskID("task1")
	task2ID := contracts.TaskID("task2")
	task3ID := contracts.TaskID("task3")

	// Task 1 completed
	run.Tasks[task1ID] = &contracts.Task{
		ID:    task1ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "output from task1",
		},
	}

	// Task 2 completed
	run.Tasks[task2ID] = &contracts.Task{
		ID:    task2ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "output from task2",
		},
	}

	// Task 3 depends on task1 and task2
	run.Tasks[task3ID] = &contracts.Task{
		ID:   task3ID,
		Deps: []contracts.TaskID{task1ID, task2ID},
	}

	bundle, err := cb.Build(run, task3ID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Check messages (should have both dependency outputs)
	if len(bundle.Messages) != 2 {
		t.Fatalf("Messages length = %d, want 2", len(bundle.Messages))
	}

	// Messages should contain both outputs
	hasTask1 := false
	hasTask2 := false
	for _, msg := range bundle.Messages {
		if msg == "output from task1" {
			hasTask1 = true
		}
		if msg == "output from task2" {
			hasTask2 = true
		}
	}
	if !hasTask1 || !hasTask2 {
		t.Fatalf("Messages do not contain both dependency outputs")
	}
}

func TestBuild_Success_NilMemory(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: nil, // No memory
	}

	taskID := contracts.TaskID("task1")
	run.Tasks[taskID] = &contracts.Task{
		ID:   taskID,
		Deps: []contracts.TaskID{},
	}

	bundle, err := cb.Build(run, taskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Memory should be an empty map, not nil
	if bundle.Memory == nil {
		t.Fatal("Memory is nil, want empty map")
	}
	if len(bundle.Memory) != 0 {
		t.Fatalf("Memory length = %d, want 0", len(bundle.Memory))
	}
}

func TestBuild_Success_EmptyMemory(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: make(map[string]string), // Empty memory
	}

	taskID := contracts.TaskID("task1")
	run.Tasks[taskID] = &contracts.Task{
		ID:   taskID,
		Deps: []contracts.TaskID{},
	}

	bundle, err := cb.Build(run, taskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if len(bundle.Memory) != 0 {
		t.Fatalf("Memory length = %d, want 0", len(bundle.Memory))
	}
}

func TestBuild_Success_NoDependencies(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:    contracts.RunID("run1"),
		Tasks: make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{
			"key": "value",
		},
	}

	taskID := contracts.TaskID("task1")
	run.Tasks[taskID] = &contracts.Task{
		ID:   taskID,
		Deps: []contracts.TaskID{}, // No dependencies
	}

	bundle, err := cb.Build(run, taskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if len(bundle.Messages) != 0 {
		t.Fatalf("Messages length = %d, want 0", len(bundle.Messages))
	}
	if len(bundle.Memory) != 1 {
		t.Fatalf("Memory length = %d, want 1", len(bundle.Memory))
	}
}

func TestBuild_Success_DependencyNotCompleted(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	task1ID := contracts.TaskID("task1")
	task2ID := contracts.TaskID("task2")

	// Task 1 is not completed (no Outputs)
	run.Tasks[task1ID] = &contracts.Task{
		ID:      task1ID,
		Outputs: nil, // Not completed
	}

	// Task 2 depends on task1
	run.Tasks[task2ID] = &contracts.Task{
		ID:   task2ID,
		Deps: []contracts.TaskID{task1ID},
	}

	bundle, err := cb.Build(run, task2ID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Messages should be empty since dependency is not completed
	if len(bundle.Messages) != 0 {
		t.Fatalf("Messages length = %d, want 0 (incomplete dependency)", len(bundle.Messages))
	}
}

func TestBuild_Success_DependencyEmptyOutput(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	task1ID := contracts.TaskID("task1")
	task2ID := contracts.TaskID("task2")

	// Task 1 is completed but has empty output
	run.Tasks[task1ID] = &contracts.Task{
		ID:    task1ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "", // Empty output
		},
	}

	// Task 2 depends on task1
	run.Tasks[task2ID] = &contracts.Task{
		ID:   task2ID,
		Deps: []contracts.TaskID{task1ID},
	}

	bundle, err := cb.Build(run, task2ID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Messages should be empty since output is empty
	if len(bundle.Messages) != 0 {
		t.Fatalf("Messages length = %d, want 0 (empty output)", len(bundle.Messages))
	}
}

func TestBuild_Success_MissingDependency(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	task1ID := contracts.TaskID("task1")
	task2ID := contracts.TaskID("task2")
	missingID := contracts.TaskID("missing")

	// Task 1 is present and completed
	run.Tasks[task1ID] = &contracts.Task{
		ID:    task1ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "task1 output",
		},
	}

	// Task 2 depends on task1 and a missing task
	run.Tasks[task2ID] = &contracts.Task{
		ID:   task2ID,
		Deps: []contracts.TaskID{task1ID, missingID},
	}

	bundle, err := cb.Build(run, task2ID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Should only have task1 output, missing dependency should be skipped
	if len(bundle.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(bundle.Messages))
	}
	if bundle.Messages[0] != "task1 output" {
		t.Fatalf("Messages[0] = %q, want %q", bundle.Messages[0], "task1 output")
	}
}

func TestBuild_Error_NilRun(t *testing.T) {
	cb := NewContextBuilder()

	bundle, err := cb.Build(nil, contracts.TaskID("task1"))

	if err == nil {
		t.Fatal("Build() error = nil, want error")
	}
	if bundle != nil {
		t.Fatal("Build() returned non-nil bundle for nil run")
	}
	if err != contracts.ErrInvalidInput {
		t.Fatalf("Build() error = %v, want ErrInvalidInput", err)
	}
}

func TestBuild_Error_TaskNotFound(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	bundle, err := cb.Build(run, contracts.TaskID("nonexistent"))

	if err == nil {
		t.Fatal("Build() error = nil, want error")
	}
	if bundle != nil {
		t.Fatal("Build() returned non-nil bundle for non-existent task")
	}
	if err != contracts.ErrTaskNotFound {
		t.Fatalf("Build() error = %v, want ErrTaskNotFound", err)
	}
}

func TestBuild_Success_MultipleMemoryKeys(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:    contracts.RunID("run1"),
		Tasks: make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	taskID := contracts.TaskID("task1")
	run.Tasks[taskID] = &contracts.Task{
		ID:   taskID,
		Deps: []contracts.TaskID{},
	}

	bundle, err := cb.Build(run, taskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if len(bundle.Memory) != 3 {
		t.Fatalf("Memory length = %d, want 3", len(bundle.Memory))
	}

	if bundle.Memory["key1"] != "value1" {
		t.Fatalf("Memory[key1] = %q, want %q", bundle.Memory["key1"], "value1")
	}
	if bundle.Memory["key2"] != "value2" {
		t.Fatalf("Memory[key2] = %q, want %q", bundle.Memory["key2"], "value2")
	}
	if bundle.Memory["key3"] != "value3" {
		t.Fatalf("Memory[key3] = %q, want %q", bundle.Memory["key3"], "value3")
	}
}

func TestBuild_Success_MemoryIsolation(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:    contracts.RunID("run1"),
		Tasks: make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{
			"original_key": "original_value",
		},
	}

	taskID := contracts.TaskID("task1")
	run.Tasks[taskID] = &contracts.Task{
		ID:   taskID,
		Deps: []contracts.TaskID{},
	}

	bundle, err := cb.Build(run, taskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Modify the bundle memory
	bundle.Memory["new_key"] = "new_value"

	// Original run memory should not be affected
	if _, exists := run.Memory["new_key"]; exists {
		t.Fatal("Bundle memory modification affected original run memory")
	}

	// Original key should still be there
	if run.Memory["original_key"] != "original_value" {
		t.Fatal("Original run memory was corrupted")
	}
}

func TestBuild_Success_LargeNumberOfDependencies(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	// Create 10 dependency tasks
	var deps []contracts.TaskID
	for i := 0; i < 10; i++ {
		taskID := contracts.TaskID("dep" + string(rune(48+i)))
		deps = append(deps, taskID)
		run.Tasks[taskID] = &contracts.Task{
			ID:    taskID,
			State: contracts.TaskCompleted,
			Outputs: &contracts.TaskResult{
				Output: "output " + string(rune(48+i)),
			},
		}
	}

	// Create a task that depends on all of them
	mainTaskID := contracts.TaskID("main")
	run.Tasks[mainTaskID] = &contracts.Task{
		ID:   mainTaskID,
		Deps: deps,
	}

	bundle, err := cb.Build(run, mainTaskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	if len(bundle.Messages) != 10 {
		t.Fatalf("Messages length = %d, want 10", len(bundle.Messages))
	}
}

func TestBuild_Success_MixedCompletedAndIncompleted(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	task1ID := contracts.TaskID("task1")
	task2ID := contracts.TaskID("task2")

	// Task 1: completed with output
	run.Tasks[task1ID] = &contracts.Task{
		ID:    task1ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "output1",
		},
	}

	// Task 2: completed but empty output
	run.Tasks[task2ID] = &contracts.Task{
		ID:    task2ID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "",
		},
	}

	// Task 3: not completed (Pending state)
	notCompletedID := contracts.TaskID("notcompleted")
	run.Tasks[notCompletedID] = &contracts.Task{
		ID:      notCompletedID,
		State:   contracts.TaskPending,
		Outputs: nil,
	}

	// Main task depends on all
	mainTaskID := contracts.TaskID("main")
	run.Tasks[mainTaskID] = &contracts.Task{
		ID:   mainTaskID,
		Deps: []contracts.TaskID{task1ID, task2ID, notCompletedID},
	}

	bundle, err := cb.Build(run, mainTaskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Should only have task1 output (task2 has empty output, notcompleted is not completed)
	if len(bundle.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(bundle.Messages))
	}
	if bundle.Messages[0] != "output1" {
		t.Fatalf("Messages[0] = %q, want %q", bundle.Messages[0], "output1")
	}
}

func TestBuild_FailedDependencyNotIncluded(t *testing.T) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:     contracts.RunID("run1"),
		Tasks:  make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{},
	}

	// Failed task with output (should NOT be included)
	failedID := contracts.TaskID("failed")
	run.Tasks[failedID] = &contracts.Task{
		ID:    failedID,
		State: contracts.TaskFailed,
		Outputs: &contracts.TaskResult{
			Output: "failed output",
		},
	}

	// Completed task with output (should be included)
	completedID := contracts.TaskID("completed")
	run.Tasks[completedID] = &contracts.Task{
		ID:    completedID,
		State: contracts.TaskCompleted,
		Outputs: &contracts.TaskResult{
			Output: "completed output",
		},
	}

	// Running task with output (should NOT be included)
	runningID := contracts.TaskID("running")
	run.Tasks[runningID] = &contracts.Task{
		ID:    runningID,
		State: contracts.TaskRunning,
		Outputs: &contracts.TaskResult{
			Output: "running output",
		},
	}

	// Main task depends on all
	mainTaskID := contracts.TaskID("main")
	run.Tasks[mainTaskID] = &contracts.Task{
		ID:   mainTaskID,
		Deps: []contracts.TaskID{failedID, completedID, runningID},
	}

	bundle, err := cb.Build(run, mainTaskID)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	// Should only have completed output
	if len(bundle.Messages) != 1 {
		t.Fatalf("Messages length = %d, want 1", len(bundle.Messages))
	}
	if bundle.Messages[0] != "completed output" {
		t.Fatalf("Messages[0] = %q, want %q", bundle.Messages[0], "completed output")
	}
}

func BenchmarkBuild(b *testing.B) {
	cb := NewContextBuilder()

	run := &contracts.Run{
		ID:    contracts.RunID("run1"),
		Tasks: make(map[contracts.TaskID]*contracts.Task),
		Memory: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
	}

	// Create 5 dependency tasks
	var deps []contracts.TaskID
	for i := 0; i < 5; i++ {
		taskID := contracts.TaskID("dep" + string(rune(48+i)))
		deps = append(deps, taskID)
		run.Tasks[taskID] = &contracts.Task{
			ID:    taskID,
			State: contracts.TaskCompleted,
			Outputs: &contracts.TaskResult{
				Output: "output",
			},
		}
	}

	mainTaskID := contracts.TaskID("main")
	run.Tasks[mainTaskID] = &contracts.Task{
		ID:   mainTaskID,
		Deps: deps,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cb.Build(run, mainTaskID)
	}
}
