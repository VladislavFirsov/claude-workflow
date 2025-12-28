package orchestration

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
	"github.com/anthropics/claude-workflow/runtime/internal/cost"
)

func TestNewOrchestratorWithDefaults(t *testing.T) {
	policy := contracts.RunPolicy{
		MaxParallelism: 2,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "USD"},
	}

	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		return &contracts.TaskResult{
			Output: "test",
			Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.01, Currency: "USD"}},
		}, nil
	}

	orch := NewOrchestratorWithDefaults(policy, executor)

	if orch == nil {
		t.Fatal("expected non-nil orchestrator")
	}
}

func TestNewOrchestratorWithDefaults_NilExecutor(t *testing.T) {
	policy := contracts.RunPolicy{
		MaxParallelism: 1,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "USD"},
	}

	// nil executor should be accepted (uses defaultExecutor internally)
	orch := NewOrchestratorWithDefaults(policy, nil)

	if orch == nil {
		t.Fatal("expected non-nil orchestrator with nil executor")
	}
}

func TestNewOrchestratorWithOptions_CustomCatalog(t *testing.T) {
	policy := contracts.RunPolicy{
		MaxParallelism: 1,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "USD"},
	}

	// Create custom catalog with a test model
	customModels := []contracts.ModelInfo{
		{
			ID:              "test-model",
			Provider:        "test",
			MaxContext:      100000,
			InputCostPer1M:  1.0,
			OutputCostPer1M: 2.0,
			DefaultRole:     contracts.RoleFast,
			SupportsTools:   true,
		},
	}
	customCatalog := cost.NewModelCatalogWithModels(customModels, map[contracts.ModelRole]contracts.ModelID{
		contracts.RoleFast: "test-model",
	})

	opts := FactoryOptions{
		ModelCatalog: customCatalog,
		Currency:     "EUR",
	}

	orch := NewOrchestratorWithOptions(policy, nil, opts)

	if orch == nil {
		t.Fatal("expected non-nil orchestrator with custom options")
	}
}

func TestNewOrchestratorWithOptions_CustomCurrencyOnly(t *testing.T) {
	policy := contracts.RunPolicy{
		MaxParallelism: 1,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "EUR"},
	}

	opts := FactoryOptions{
		Currency: "EUR", // Custom currency, default catalog
	}

	orch := NewOrchestratorWithOptions(policy, nil, opts)

	if orch == nil {
		t.Fatal("expected non-nil orchestrator with custom currency")
	}
}

// TestFactory_SingleTaskE2E verifies that factory-created orchestrator
// can execute a minimal single-task run successfully.
func TestFactory_SingleTaskE2E(t *testing.T) {
	policy := contracts.RunPolicy{
		MaxParallelism: 1,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "USD"},
	}

	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		return &contracts.TaskResult{
			Output: "ok:" + string(task.ID),
			Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.000075, Currency: "USD"}},
		}, nil
	}

	orch := NewOrchestratorWithDefaults(policy, executor)

	// Build DAG with single task
	resolver := NewDependencyResolver()
	dag, err := resolver.BuildDAG([]contracts.Task{{ID: "A"}})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	// Create run with proper Task.Inputs and valid Model
	run := &contracts.Run{
		ID:    "run-factory-test",
		State: contracts.RunPending,
		DAG:   dag,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"A": {
				ID:    "A",
				State: contracts.TaskPending,
				Model: "claude-3-haiku-20240307", // Valid model from catalog
				Inputs: &contracts.TaskInput{
					Prompt: strings.Repeat("x", 400), // ~100 tokens
				},
			},
		},
		Policy: policy,
		Memory: make(map[string]string),
	}

	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if run.State != contracts.RunCompleted {
		t.Errorf("expected RunCompleted, got %v", run.State)
	}

	if run.Tasks["A"].State != contracts.TaskCompleted {
		t.Errorf("expected task A completed, got %v", run.Tasks["A"].State)
	}

	if run.Tasks["A"].Outputs == nil || run.Tasks["A"].Outputs.Output != "ok:A" {
		t.Error("expected task A output to be 'ok:A'")
	}

	// Verify usage was tracked
	if run.Usage.Tokens != 100 {
		t.Errorf("expected 100 tokens, got %d", run.Usage.Tokens)
	}
}

// TestFactory_MultiTaskE2E verifies factory orchestrator handles dependencies correctly.
func TestFactory_MultiTaskE2E(t *testing.T) {
	policy := contracts.RunPolicy{
		MaxParallelism: 2,
		BudgetLimit:    contracts.Cost{Amount: 1.0, Currency: "USD"},
	}

	executed := make([]contracts.TaskID, 0)
	var mu sync.Mutex
	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		mu.Lock()
		executed = append(executed, task.ID)
		mu.Unlock()
		return &contracts.TaskResult{
			Output: "ok:" + string(task.ID),
			Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.000075, Currency: "USD"}},
		}, nil
	}

	orch := NewOrchestratorWithDefaults(policy, executor)

	// Build DAG: A -> B
	resolver := NewDependencyResolver()
	dag, err := resolver.BuildDAG([]contracts.Task{
		{ID: "A"},
		{ID: "B", Deps: []contracts.TaskID{"A"}},
	})
	if err != nil {
		t.Fatalf("BuildDAG failed: %v", err)
	}

	run := &contracts.Run{
		ID:    "run-factory-multi",
		State: contracts.RunPending,
		DAG:   dag,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"A": {
				ID:     "A",
				State:  contracts.TaskPending,
				Model:  "claude-3-haiku-20240307",
				Inputs: &contracts.TaskInput{Prompt: strings.Repeat("a", 400)},
			},
			"B": {
				ID:     "B",
				State:  contracts.TaskPending,
				Model:  "claude-3-haiku-20240307",
				Deps:   []contracts.TaskID{"A"},
				Inputs: &contracts.TaskInput{Prompt: strings.Repeat("b", 400)},
			},
		},
		Policy: policy,
		Memory: make(map[string]string),
	}

	err = orch.Run(context.Background(), run)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if run.State != contracts.RunCompleted {
		t.Errorf("expected RunCompleted, got %v", run.State)
	}

	// Verify execution order: A before B
	mu.Lock()
	if len(executed) != 2 {
		mu.Unlock()
		t.Fatalf("expected 2 tasks executed, got %d", len(executed))
	}
	if executed[0] != "A" || executed[1] != "B" {
		mu.Unlock()
		t.Errorf("unexpected execution order: %v", executed)
	}
	mu.Unlock()

	// Verify context routing: B should have A's output in Inputs.Inputs
	if run.Tasks["B"].Inputs.Inputs == nil {
		t.Fatal("expected B to have routed inputs")
	}
	if run.Tasks["B"].Inputs.Inputs["A"] != "ok:A" {
		t.Errorf("expected B to receive 'ok:A', got '%s'", run.Tasks["B"].Inputs.Inputs["A"])
	}
}
