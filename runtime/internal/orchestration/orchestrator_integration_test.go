package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
	ctxpkg "github.com/anthropics/claude-workflow/runtime/internal/context"
	"github.com/anthropics/claude-workflow/runtime/internal/cost"
)

// =============================================================================
// Stub Executor
// =============================================================================

// stubExecutor implements TaskExecutorFunc for integration tests.
// It returns deterministic results and tracks execution order.
type stubExecutor struct {
	mu       sync.Mutex
	executed []contracts.TaskID
	failFor  map[contracts.TaskID]error
}

func newStubExecutor() *stubExecutor {
	return &stubExecutor{
		executed: make([]contracts.TaskID, 0),
		failFor:  make(map[contracts.TaskID]error),
	}
}

func (s *stubExecutor) Execute(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
	s.mu.Lock()
	s.executed = append(s.executed, task.ID)
	s.mu.Unlock()

	if err, ok := s.failFor[task.ID]; ok {
		return nil, err
	}

	return &contracts.TaskResult{
		Output: fmt.Sprintf("ok:%s", task.ID),
		Usage: contracts.Usage{
			Tokens: 100,
			Cost:   contracts.Cost{Amount: 0.000075, Currency: "USD"},
		},
	}, nil
}

func (s *stubExecutor) ExecutedTasks() []contracts.TaskID {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]contracts.TaskID, len(s.executed))
	copy(result, s.executed)
	return result
}

// =============================================================================
// Helper Functions
// =============================================================================

// createRealDeps creates OrchestratorDeps with real components and stub executor
func createRealDeps(policy contracts.RunPolicy, execFn TaskExecutorFunc) OrchestratorDeps {
	return OrchestratorDeps{
		Scheduler:      NewScheduler(),
		DepResolver:    NewDependencyResolver(),
		Queue:          NewQueueManager(),
		Executor:       NewParallelExecutorFromPolicy(policy, execFn),
		ContextBuilder: ctxpkg.NewContextBuilder(),
		Compactor:      ctxpkg.NewContextCompactor(),
		TokenEstimator: cost.NewTokenEstimator(),
		CostCalc:       cost.NewCostCalculator(),
		BudgetEnforcer: cost.NewBudgetEnforcer(),
		UsageTracker:   cost.NewUsageTracker(),
		Router:         ctxpkg.NewContextRouter(),
	}
}

// buildLinearDAG creates a linear DAG: A -> B -> C through real resolver
func buildLinearDAG(ids []contracts.TaskID) (*contracts.DAG, error) {
	resolver := NewDependencyResolver()
	tasks := make([]contracts.Task, len(ids))
	for i, id := range ids {
		if i == 0 {
			tasks[i] = contracts.Task{ID: id}
		} else {
			tasks[i] = contracts.Task{ID: id, Deps: []contracts.TaskID{ids[i-1]}}
		}
	}
	return resolver.BuildDAG(tasks)
}

// buildFanInDAG creates: A -> C, B -> C (parallel A,B then C)
func buildFanInDAG() (*contracts.DAG, error) {
	resolver := NewDependencyResolver()
	tasks := []contracts.Task{
		{ID: "A"},
		{ID: "B"},
		{ID: "C", Deps: []contracts.TaskID{"A", "B"}},
	}
	return resolver.BuildDAG(tasks)
}

// buildDiamondDAG creates: A -> B, A -> C, B -> D, C -> D
func buildDiamondDAG() (*contracts.DAG, error) {
	resolver := NewDependencyResolver()
	tasks := []contracts.Task{
		{ID: "A"},
		{ID: "B", Deps: []contracts.TaskID{"A"}},
		{ID: "C", Deps: []contracts.TaskID{"A"}},
		{ID: "D", Deps: []contracts.TaskID{"B", "C"}},
	}
	return resolver.BuildDAG(tasks)
}

// createTasksFromDAG creates Task map synchronized with DAG
func createTasksFromDAG(dag *contracts.DAG, inputChars int) map[contracts.TaskID]*contracts.Task {
	tasks := make(map[contracts.TaskID]*contracts.Task)
	for id, node := range dag.Nodes {
		tasks[id] = &contracts.Task{
			ID:    id,
			State: contracts.TaskPending,
			Model: "claude-3-haiku-20240307",
			Deps:  node.Deps,
			Inputs: &contracts.TaskInput{
				Prompt: strings.Repeat("x", inputChars),
			},
		}
	}
	return tasks
}

// createRun creates a Run with the given DAG, tasks, and budget
func createRun(id contracts.RunID, dag *contracts.DAG, tasks map[contracts.TaskID]*contracts.Task, policy contracts.RunPolicy) *contracts.Run {
	return &contracts.Run{
		ID:     id,
		State:  contracts.RunPending,
		DAG:    dag,
		Tasks:  tasks,
		Policy: policy,
		Memory: make(map[string]string),
	}
}

// defaultPolicy returns a policy with generous budget
func defaultPolicy() contracts.RunPolicy {
	return contracts.RunPolicy{
		MaxParallelism: 2,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "USD"},
	}
}

// =============================================================================
// Assertions
// =============================================================================

func assertRunCompleted(t *testing.T, run *contracts.Run) {
	t.Helper()
	if run.State != contracts.RunCompleted {
		t.Errorf("expected RunCompleted, got %v", run.State)
	}
}

func assertRunFailed(t *testing.T, run *contracts.Run) {
	t.Helper()
	if run.State != contracts.RunFailed {
		t.Errorf("expected RunFailed, got %v", run.State)
	}
}

func assertRunAborted(t *testing.T, run *contracts.Run) {
	t.Helper()
	if run.State != contracts.RunAborted {
		t.Errorf("expected RunAborted, got %v", run.State)
	}
}

func assertAllTasksCompleted(t *testing.T, run *contracts.Run) {
	t.Helper()
	for id, task := range run.Tasks {
		if task.State != contracts.TaskCompleted {
			t.Errorf("task %s: expected TaskCompleted, got %v", id, task.State)
		}
	}
}

func assertTaskCompleted(t *testing.T, run *contracts.Run, taskID contracts.TaskID) {
	t.Helper()
	task, ok := run.Tasks[taskID]
	if !ok {
		t.Fatalf("task %s not found", taskID)
	}
	if task.State != contracts.TaskCompleted {
		t.Errorf("task %s: expected TaskCompleted, got %v", taskID, task.State)
	}
}

func assertTaskFailed(t *testing.T, run *contracts.Run, taskID contracts.TaskID) {
	t.Helper()
	task, ok := run.Tasks[taskID]
	if !ok {
		t.Fatalf("task %s not found", taskID)
	}
	if task.State != contracts.TaskFailed {
		t.Errorf("task %s: expected TaskFailed, got %v", taskID, task.State)
	}
}

func assertContextRouted(t *testing.T, task *contracts.Task, fromID contracts.TaskID, expectedValue string) {
	t.Helper()
	if task.Inputs == nil {
		t.Fatalf("task %s has nil Inputs", task.ID)
	}
	if task.Inputs.Inputs == nil {
		t.Fatalf("task %s has nil Inputs.Inputs", task.ID)
	}
	got, ok := task.Inputs.Inputs[string(fromID)]
	if !ok {
		t.Errorf("task %s missing routed input from %s", task.ID, fromID)
		return
	}
	if got != expectedValue {
		t.Errorf("expected routed value %q, got %q", expectedValue, got)
	}
}

func assertTotalTokens(t *testing.T, run *contracts.Run, expected contracts.TokenCount) {
	t.Helper()
	if run.Usage.Tokens != expected {
		t.Errorf("expected %d tokens, got %d", expected, run.Usage.Tokens)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

// TestIntegration_LinearDAG_ABC tests the basic linear DAG: A -> B -> C
func TestIntegration_LinearDAG_ABC(t *testing.T) {
	// Build DAG through real resolver
	dag, err := buildLinearDAG([]contracts.TaskID{"A", "B", "C"})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	// Create tasks with 400 char input (~100 tokens each)
	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-linear", dag, tasks, policy)

	// Stub executor
	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	assertRunCompleted(t, run)
	assertAllTasksCompleted(t, run)

	// Verify execution order (must be A, B, C due to dependencies)
	executed := stub.ExecutedTasks()
	if len(executed) != 3 {
		t.Fatalf("expected 3 tasks executed, got %d", len(executed))
	}
	if executed[0] != "A" || executed[1] != "B" || executed[2] != "C" {
		t.Errorf("unexpected execution order: %v", executed)
	}

	// Verify context routing: B received A's output, C received B's output
	assertContextRouted(t, run.Tasks["B"], "A", "ok:A")
	assertContextRouted(t, run.Tasks["C"], "B", "ok:B")

	// Verify usage: 3 tasks * 100 tokens = 300 (from executor)
	assertTotalTokens(t, run, 300)
}

// TestIntegration_FanInDAG tests parallel tasks converging: A,B -> C
func TestIntegration_FanInDAG(t *testing.T) {
	dag, err := buildFanInDAG()
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-fanin", dag, tasks, policy)

	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	assertRunCompleted(t, run)
	assertAllTasksCompleted(t, run)

	// Verify C executed last (after both A and B)
	executed := stub.ExecutedTasks()
	if len(executed) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(executed))
	}
	if executed[2] != "C" {
		t.Errorf("expected C to execute last, got %s", executed[2])
	}

	// C should have received outputs from both A and B
	assertContextRouted(t, run.Tasks["C"], "A", "ok:A")
	assertContextRouted(t, run.Tasks["C"], "B", "ok:B")

	assertTotalTokens(t, run, 300)
}

// TestIntegration_DiamondDAG tests: A -> B,C -> D
func TestIntegration_DiamondDAG(t *testing.T) {
	dag, err := buildDiamondDAG()
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-diamond", dag, tasks, policy)

	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	assertRunCompleted(t, run)
	assertAllTasksCompleted(t, run)

	// Verify execution order constraints
	executed := stub.ExecutedTasks()
	if len(executed) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(executed))
	}

	// A must be first
	if executed[0] != "A" {
		t.Errorf("expected A to execute first, got %s", executed[0])
	}

	// D must be last
	if executed[3] != "D" {
		t.Errorf("expected D to execute last, got %s", executed[3])
	}

	// D should have received outputs from both B and C
	assertContextRouted(t, run.Tasks["D"], "B", "ok:B")
	assertContextRouted(t, run.Tasks["D"], "C", "ok:C")

	assertTotalTokens(t, run, 400)
}

// TestIntegration_SingleTask tests minimal DAG with one task
func TestIntegration_SingleTask(t *testing.T) {
	dag, err := buildLinearDAG([]contracts.TaskID{"A"})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-single", dag, tasks, policy)

	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	assertRunCompleted(t, run)
	assertAllTasksCompleted(t, run)
	assertTotalTokens(t, run, 100)
}

// TestIntegration_EmptyDAG tests empty DAG (no tasks)
func TestIntegration_EmptyDAG(t *testing.T) {
	resolver := NewDependencyResolver()
	dag, err := resolver.BuildDAG([]contracts.Task{})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := make(map[contracts.TaskID]*contracts.Task) // Empty tasks
	policy := defaultPolicy()
	run := createRun("run-empty", dag, tasks, policy)

	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	assertRunCompleted(t, run)
	assertTotalTokens(t, run, 0)

	executed := stub.ExecutedTasks()
	if len(executed) != 0 {
		t.Errorf("expected 0 tasks executed, got %d", len(executed))
	}
}

// TestIntegration_ContextRouting verifies context is properly routed between tasks
func TestIntegration_ContextRouting(t *testing.T) {
	dag, err := buildLinearDAG([]contracts.TaskID{"A", "B", "C"})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-routing", dag, tasks, policy)

	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	assertRunCompleted(t, run)

	// Verify context routing: ContextRouter writes to Inputs.Inputs[fromID]
	assertContextRouted(t, run.Tasks["B"], "A", "ok:A")
	assertContextRouted(t, run.Tasks["C"], "B", "ok:B")

	// Verify task outputs are stored (by Scheduler.MarkComplete)
	if run.Tasks["A"].Outputs == nil || run.Tasks["A"].Outputs.Output != "ok:A" {
		t.Errorf("task A outputs not stored correctly")
	}
	if run.Tasks["B"].Outputs == nil || run.Tasks["B"].Outputs.Output != "ok:B" {
		t.Errorf("task B outputs not stored correctly")
	}
	if run.Tasks["C"].Outputs == nil || run.Tasks["C"].Outputs.Output != "ok:C" {
		t.Errorf("task C outputs not stored correctly")
	}
}

// TestIntegration_BudgetExceeded tests budget enforcement with deterministic token counts
func TestIntegration_BudgetExceeded(t *testing.T) {
	dag, err := buildLinearDAG([]contracts.TaskID{"A", "B", "C"})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400) // 400 chars = 100 base tokens

	// Token calculation:
	// A: 400 chars = 100 tokens -> cost = 100 * 0.75 / 1M = 0.000075
	// B: 400 + 4 (Inputs["A"]) + 4 (ctx.Messages) = 408 chars = 102 tokens -> cost = 0.0000765
	// C: 400 + 4 (Inputs["B"]) + 4 (ctx.Messages) = 408 chars = 102 tokens -> cost = 0.0000765
	//
	// Budget: 0.0001521 allows A + B, but C fails pre-check (with epsilon for float safety)
	policy := contracts.RunPolicy{
		MaxParallelism: 1,
		BudgetLimit:    contracts.Cost{Amount: 0.0001521, Currency: "USD"},
	}

	run := createRun("run-budget", dag, tasks, policy)

	stub := newStubExecutor()
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if err == nil {
		t.Error("expected error, got nil")
	}

	assertRunFailed(t, run)
	assertTaskCompleted(t, run, "A")
	assertTaskCompleted(t, run, "B")

	// C should have budget_exceeded error code
	taskC := run.Tasks["C"]
	if taskC.State != contracts.TaskFailed {
		t.Errorf("expected task C failed, got %v", taskC.State)
	}
	if taskC.Error == nil || taskC.Error.Code != "budget_exceeded" {
		t.Errorf("expected task C error with code budget_exceeded, got %+v", taskC.Error)
	}
}

// TestIntegration_TaskFailure tests run failure when a task fails
func TestIntegration_TaskFailure(t *testing.T) {
	dag, err := buildLinearDAG([]contracts.TaskID{"A", "B", "C"})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-failure", dag, tasks, policy)

	stub := newStubExecutor()
	stub.failFor["B"] = errors.New("task B failed")
	deps := createRealDeps(policy, stub.Execute)

	orch := NewOrchestrator(deps)
	err = orch.Run(context.Background(), run)

	if !errors.Is(err, contracts.ErrTaskFailed) {
		t.Errorf("expected ErrTaskFailed, got %v", err)
	}

	assertRunFailed(t, run)
	assertTaskCompleted(t, run, "A")
	assertTaskFailed(t, run, "B")

	// C should not be executed (dependency B failed)
	if run.Tasks["C"].State == contracts.TaskCompleted {
		t.Error("expected task C not to execute after B failed")
	}
}

// TestIntegration_ContextCancellation tests run behavior on context cancellation.
// Depending on timing, cancellation may surface as ErrTaskCancelled (ctx.Done path)
// or ErrTaskFailed (executor returns ctx.Err and is wrapped). Run should be Failed.
// RunAborted is only set when context is cancelled BEFORE task execution starts.
func TestIntegration_ContextCancellation(t *testing.T) {
	dag, err := buildLinearDAG([]contracts.TaskID{"A", "B", "C"})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	tasks := createTasksFromDAG(dag, 400)
	policy := defaultPolicy()
	run := createRun("run-cancel", dag, tasks, policy)

	// Create stub that delays execution
	stub := &stubExecutor{
		executed: make([]contracts.TaskID, 0),
		failFor:  make(map[contracts.TaskID]error),
	}
	slowExecute := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		stub.mu.Lock()
		stub.executed = append(stub.executed, task.ID)
		stub.mu.Unlock()

		// Delay to allow cancellation
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		return &contracts.TaskResult{
			Output: fmt.Sprintf("ok:%s", task.ID),
			Usage: contracts.Usage{
				Tokens: 100,
				Cost:   contracts.Cost{Amount: 0.000075, Currency: "USD"},
			},
		}, nil
	}

	deps := createRealDeps(policy, slowExecute)

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after short delay (during task execution)
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	orch := NewOrchestrator(deps)
	err = orch.Run(ctx, run)

	// Cancellation during execution can surface as ErrTaskCancelled or ErrTaskFailed.
	if !errors.Is(err, contracts.ErrTaskCancelled) && !errors.Is(err, contracts.ErrTaskFailed) {
		t.Errorf("expected ErrTaskCancelled or ErrTaskFailed, got %v", err)
	}

	// Run state is Failed (not Aborted) because cancellation happened during task
	assertRunFailed(t, run)
}
