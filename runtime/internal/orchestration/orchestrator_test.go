package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// Mock implementations for testing

type mockScheduler struct {
	nextReadyFn     func(run *contracts.Run) ([]contracts.TaskID, error)
	markCompleteFn  func(run *contracts.Run, taskID contracts.TaskID, result *contracts.TaskResult) error
}

func (m *mockScheduler) NextReady(run *contracts.Run) ([]contracts.TaskID, error) {
	if m.nextReadyFn != nil {
		return m.nextReadyFn(run)
	}
	return nil, nil
}

func (m *mockScheduler) MarkComplete(run *contracts.Run, taskID contracts.TaskID, result *contracts.TaskResult) error {
	if m.markCompleteFn != nil {
		return m.markCompleteFn(run, taskID, result)
	}
	if task, ok := run.Tasks[taskID]; ok {
		task.State = contracts.TaskCompleted
	}
	return nil
}

type mockDependencyResolver struct {
	validateFn func(dag *contracts.DAG) error
}

func (m *mockDependencyResolver) BuildDAG(tasks []contracts.Task) (*contracts.DAG, error) {
	return nil, nil
}

func (m *mockDependencyResolver) Validate(dag *contracts.DAG) error {
	if m.validateFn != nil {
		return m.validateFn(dag)
	}
	return nil
}

type mockQueueManager struct {
	queue []contracts.TaskID
}

func (m *mockQueueManager) Enqueue(taskID contracts.TaskID) {
	m.queue = append(m.queue, taskID)
}

func (m *mockQueueManager) Dequeue() (contracts.TaskID, bool) {
	if len(m.queue) == 0 {
		return "", false
	}
	taskID := m.queue[0]
	m.queue = m.queue[1:]
	return taskID, true
}

func (m *mockQueueManager) Len() int {
	return len(m.queue)
}

type mockParallelExecutor struct {
	executeFn func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error)
}

func (m *mockParallelExecutor) Execute(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
	if m.executeFn != nil {
		return m.executeFn(ctx, run, taskID)
	}
	return &contracts.TaskResult{
		Output: "executed",
		Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.01, Currency: "USD"}},
	}, nil
}

type mockContextBuilder struct {
	buildFn func(run *contracts.Run, taskID contracts.TaskID) (*contracts.ContextBundle, error)
}

func (m *mockContextBuilder) Build(run *contracts.Run, taskID contracts.TaskID) (*contracts.ContextBundle, error) {
	if m.buildFn != nil {
		return m.buildFn(run, taskID)
	}
	return &contracts.ContextBundle{}, nil
}

type mockContextCompactor struct {
	compactFn func(bundle *contracts.ContextBundle, policy contracts.ContextPolicy) (*contracts.ContextBundle, error)
}

func (m *mockContextCompactor) Compact(bundle *contracts.ContextBundle, policy contracts.ContextPolicy) (*contracts.ContextBundle, error) {
	if m.compactFn != nil {
		return m.compactFn(bundle, policy)
	}
	return bundle, nil
}

type mockTokenEstimator struct {
	estimateFn func(input *contracts.TaskInput, ctx *contracts.ContextBundle) (contracts.TokenCount, error)
}

func (m *mockTokenEstimator) Estimate(input *contracts.TaskInput, ctx *contracts.ContextBundle) (contracts.TokenCount, error) {
	if m.estimateFn != nil {
		return m.estimateFn(input, ctx)
	}
	return 100, nil
}

type mockCostCalculator struct {
	estimateFn func(tokens contracts.TokenCount, model contracts.ModelID) (contracts.Cost, error)
}

func (m *mockCostCalculator) Estimate(tokens contracts.TokenCount, model contracts.ModelID) (contracts.Cost, error) {
	if m.estimateFn != nil {
		return m.estimateFn(tokens, model)
	}
	return contracts.Cost{Amount: 0.01, Currency: "USD"}, nil
}

type mockBudgetEnforcer struct {
	allowFn  func(run *contracts.Run, estimate contracts.Cost) error
	recordFn func(run *contracts.Run, actual contracts.Cost) error
}

func (m *mockBudgetEnforcer) Allow(run *contracts.Run, estimate contracts.Cost) error {
	if m.allowFn != nil {
		return m.allowFn(run, estimate)
	}
	return nil
}

func (m *mockBudgetEnforcer) Record(run *contracts.Run, actual contracts.Cost) error {
	if m.recordFn != nil {
		return m.recordFn(run, actual)
	}
	return nil
}

type mockUsageTracker struct {
	addFn func(run *contracts.Run, usage contracts.Usage)
}

func (m *mockUsageTracker) Add(run *contracts.Run, usage contracts.Usage) {
	if m.addFn != nil {
		m.addFn(run, usage)
	}
}

func (m *mockUsageTracker) Snapshot(run *contracts.Run) contracts.Usage {
	return run.Usage
}

type mockContextRouter struct {
	routeFn func(run *contracts.Run, from contracts.TaskID, to contracts.TaskID, output *contracts.TaskResult) error
}

func (m *mockContextRouter) Route(run *contracts.Run, from contracts.TaskID, to contracts.TaskID, output *contracts.TaskResult) error {
	if m.routeFn != nil {
		return m.routeFn(run, from, to, output)
	}
	return nil
}

// Helper to create a default deps structure with mocks
func defaultDeps() OrchestratorDeps {
	return OrchestratorDeps{
		Scheduler:      &mockScheduler{},
		DepResolver:    &mockDependencyResolver{},
		Queue:          &mockQueueManager{},
		Executor:       &mockParallelExecutor{},
		ContextBuilder: &mockContextBuilder{},
		Compactor:      &mockContextCompactor{},
		TokenEstimator: &mockTokenEstimator{},
		CostCalc:       &mockCostCalculator{},
		BudgetEnforcer: &mockBudgetEnforcer{},
		UsageTracker:   &mockUsageTracker{},
		Router:         &mockContextRouter{},
	}
}

func TestOrchestrator_NilRun(t *testing.T) {
	orch := NewOrchestrator(defaultDeps())
	err := orch.Run(context.Background(), nil)
	if !errors.Is(err, contracts.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestOrchestrator_NilDAG(t *testing.T) {
	orch := NewOrchestrator(defaultDeps())
	run := &contracts.Run{ID: "run-1", DAG: nil}
	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput, got %v", err)
	}
}

func TestOrchestrator_DAGValidationFails(t *testing.T) {
	deps := defaultDeps()
	deps.DepResolver = &mockDependencyResolver{
		validateFn: func(dag *contracts.DAG) error {
			return contracts.ErrDAGCycle
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:  "run-1",
		DAG: &contracts.DAG{},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrDAGCycle) {
		t.Errorf("expected ErrDAGCycle, got %v", err)
	}
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed, got %v", run.State)
	}
}

func TestOrchestrator_SingleTask(t *testing.T) {
	deps := defaultDeps()

	executed := false
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			// Return task-1 only once
			for _, task := range run.Tasks {
				if task.State == contracts.TaskPending {
					return []contracts.TaskID{task.ID}, nil
				}
			}
			return nil, nil
		},
	}
	deps.Executor = &mockParallelExecutor{
		executeFn: func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
			executed = true
			return &contracts.TaskResult{
				Output: "done",
				Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.01, Currency: "USD"}},
			}, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		State: contracts.RunPending,
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending, Model: "claude-3-haiku"},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !executed {
		t.Error("task was not executed")
	}
	if run.State != contracts.RunCompleted {
		t.Errorf("expected RunCompleted, got %v", run.State)
	}
}

func TestOrchestrator_BudgetExceeded(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.BudgetEnforcer = &mockBudgetEnforcer{
		allowFn: func(run *contracts.Run, estimate contracts.Cost) error {
			return contracts.ErrBudgetExceeded
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got %v", err)
	}
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed, got %v", run.State)
	}
}

func TestOrchestrator_ContextCancelled(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return nil, nil // No tasks ready
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := orch.Run(ctx, run)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if run.State != contracts.RunAborted {
		t.Errorf("expected RunAborted, got %v", run.State)
	}
}

func TestOrchestrator_TaskNotFound(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"nonexistent-task"}, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{},
		Tasks: map[contracts.TaskID]*contracts.Task{},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrTaskNotFound) {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestOrchestrator_ExecutorZeroUsage(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.Executor = &mockParallelExecutor{
		executeFn: func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
			return &contracts.TaskResult{
				Output: "done",
				Usage:  contracts.Usage{Tokens: 0}, // Zero usage - should fail
			}, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err == nil {
		t.Error("expected error for zero usage, got nil")
	}
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed, got %v", run.State)
	}
}

func TestOrchestrator_DeadlockDetection(t *testing.T) {
	deps := defaultDeps()
	callCount := 0
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			callCount++
			// Always return empty - simulates deadlock
			return nil, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending}, // Never becomes ready
		},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrDeadlock) {
		t.Errorf("expected ErrDeadlock, got %v", err)
	}
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed, got %v", run.State)
	}
}

func TestOrchestrator_MultipleTasks(t *testing.T) {
	deps := defaultDeps()

	executedTasks := make(map[contracts.TaskID]bool)
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			var ready []contracts.TaskID
			for id, task := range run.Tasks {
				if task.State == contracts.TaskPending {
					ready = append(ready, id)
				}
			}
			return ready, nil
		},
	}
	deps.Executor = &mockParallelExecutor{
		executeFn: func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
			executedTasks[taskID] = true
			return &contracts.TaskResult{
				Output: "done",
				Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.01, Currency: "USD"}},
			}, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:  "run-1",
		DAG: &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{
			"task-1": {ID: "task-1"},
			"task-2": {ID: "task-2"},
			"task-3": {ID: "task-3"},
		}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
			"task-2": {ID: "task-2", State: contracts.TaskPending},
			"task-3": {ID: "task-3", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(executedTasks) != 3 {
		t.Errorf("expected 3 tasks executed, got %d", len(executedTasks))
	}
	if run.State != contracts.RunCompleted {
		t.Errorf("expected RunCompleted, got %v", run.State)
	}
}

func TestOrchestrator_TaskRunningState(t *testing.T) {
	deps := defaultDeps()

	var taskStateDuringExecution contracts.TaskState
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			for id, task := range run.Tasks {
				if task.State == contracts.TaskPending {
					return []contracts.TaskID{id}, nil
				}
			}
			return nil, nil
		},
	}
	deps.Executor = &mockParallelExecutor{
		executeFn: func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
			// Capture task state during execution
			taskStateDuringExecution = run.Tasks[taskID].State
			return &contracts.TaskResult{
				Output: "done",
				Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.01, Currency: "USD"}},
			}, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if taskStateDuringExecution != contracts.TaskRunning {
		t.Errorf("expected TaskRunning during execution, got %v", taskStateDuringExecution)
	}
}

func TestOrchestrator_RoutesToDependents(t *testing.T) {
	deps := defaultDeps()

	routeCalls := make(map[string]bool)
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			for id, task := range run.Tasks {
				if task.State == contracts.TaskPending {
					return []contracts.TaskID{id}, nil
				}
			}
			return nil, nil
		},
	}
	deps.Router = &mockContextRouter{
		routeFn: func(run *contracts.Run, from contracts.TaskID, to contracts.TaskID, output *contracts.TaskResult) error {
			routeCalls[string(from)+"->"+string(to)] = true
			return nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID: "run-1",
		DAG: &contracts.DAG{
			Nodes: map[contracts.TaskID]*contracts.DAGNode{
				"task-1": {ID: "task-1", Next: []contracts.TaskID{"task-2", "task-3"}},
				"task-2": {ID: "task-2"},
				"task-3": {ID: "task-3"},
			},
		},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
			"task-2": {ID: "task-2", State: contracts.TaskPending},
			"task-3": {ID: "task-3", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !routeCalls["task-1->task-2"] {
		t.Error("expected route from task-1 to task-2")
	}
	if !routeCalls["task-1->task-3"] {
		t.Error("expected route from task-1 to task-3")
	}
}

func TestOrchestrator_SkippedTasksAreTerminal(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return nil, nil // No tasks ready
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskSkipped},
			"task-2": {ID: "task-2", State: contracts.TaskCompleted},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.State != contracts.RunCompleted {
		t.Errorf("expected RunCompleted with skipped tasks, got %v", run.State)
	}
}

func TestOrchestrator_FailedTasksMarkRunFailed(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return nil, nil // No tasks ready
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskFailed},
			"task-2": {ID: "task-2", State: contracts.TaskCompleted},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed with failed tasks, got %v", run.State)
	}
}

func TestOrchestrator_NoDuplicateQueueing(t *testing.T) {
	deps := defaultDeps()

	callCount := 0
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			callCount++
			// Always return same task
			return []contracts.TaskID{"task-1"}, nil
		},
	}

	executionCount := 0
	deps.Executor = &mockParallelExecutor{
		executeFn: func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
			executionCount++
			return &contracts.TaskResult{
				Output: "done",
				Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.01, Currency: "USD"}},
			}, nil
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if executionCount != 1 {
		t.Errorf("expected 1 execution, got %d", executionCount)
	}
}

func TestOrchestrator_ContextBuildError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.ContextBuilder = &mockContextBuilder{
		buildFn: func(run *contracts.Run, taskID contracts.TaskID) (*contracts.ContextBundle, error) {
			return nil, errors.New("context build failed")
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err == nil {
		t.Error("expected error, got nil")
	}
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed, got %v", run.State)
	}
}

func TestOrchestrator_CompactError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.Compactor = &mockContextCompactor{
		compactFn: func(bundle *contracts.ContextBundle, policy contracts.ContextPolicy) (*contracts.ContextBundle, error) {
			return nil, contracts.ErrContextTooLarge
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrContextTooLarge) {
		t.Errorf("expected ErrContextTooLarge, got %v", err)
	}
}

func TestOrchestrator_TokenEstimationError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.TokenEstimator = &mockTokenEstimator{
		estimateFn: func(input *contracts.TaskInput, ctx *contracts.ContextBundle) (contracts.TokenCount, error) {
			return 0, contracts.ErrEstimationFailed
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrEstimationFailed) {
		t.Errorf("expected ErrEstimationFailed, got %v", err)
	}
}

func TestOrchestrator_CostCalculationError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.CostCalc = &mockCostCalculator{
		estimateFn: func(tokens contracts.TokenCount, model contracts.ModelID) (contracts.Cost, error) {
			return contracts.Cost{}, contracts.ErrModelUnknown
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrModelUnknown) {
		t.Errorf("expected ErrModelUnknown, got %v", err)
	}
}

func TestOrchestrator_ExecutorError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.Executor = &mockParallelExecutor{
		executeFn: func(ctx context.Context, run *contracts.Run, taskID contracts.TaskID) (*contracts.TaskResult, error) {
			return nil, contracts.ErrTaskFailed
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if !errors.Is(err, contracts.ErrTaskFailed) {
		t.Errorf("expected ErrTaskFailed, got %v", err)
	}

	task := run.Tasks["task-1"]
	if task.State != contracts.TaskFailed {
		t.Errorf("expected TaskFailed, got %v", task.State)
	}
}

func TestOrchestrator_BudgetRecordError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			return []contracts.TaskID{"task-1"}, nil
		},
	}
	deps.BudgetEnforcer = &mockBudgetEnforcer{
		recordFn: func(run *contracts.Run, actual contracts.Cost) error {
			return errors.New("budget record failed")
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID:    "run-1",
		DAG:   &contracts.DAG{Nodes: map[contracts.TaskID]*contracts.DAGNode{"task-1": {ID: "task-1"}}},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestOrchestrator_RouteError(t *testing.T) {
	deps := defaultDeps()
	deps.Scheduler = &mockScheduler{
		nextReadyFn: func(run *contracts.Run) ([]contracts.TaskID, error) {
			for id, task := range run.Tasks {
				if task.State == contracts.TaskPending {
					return []contracts.TaskID{id}, nil
				}
			}
			return nil, nil
		},
	}
	deps.Router = &mockContextRouter{
		routeFn: func(run *contracts.Run, from contracts.TaskID, to contracts.TaskID, output *contracts.TaskResult) error {
			return errors.New("routing failed")
		},
	}

	orch := NewOrchestrator(deps)
	run := &contracts.Run{
		ID: "run-1",
		DAG: &contracts.DAG{
			Nodes: map[contracts.TaskID]*contracts.DAGNode{
				"task-1": {ID: "task-1", Next: []contracts.TaskID{"task-2"}},
				"task-2": {ID: "task-2"},
			},
		},
		Tasks: map[contracts.TaskID]*contracts.Task{
			"task-1": {ID: "task-1", State: contracts.TaskPending},
			"task-2": {ID: "task-2", State: contracts.TaskPending},
		},
	}

	err := orch.Run(context.Background(), run)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
