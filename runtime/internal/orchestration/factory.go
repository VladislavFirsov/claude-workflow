package orchestration

import (
	"github.com/anthropics/claude-workflow/runtime/contracts"
	ctxpkg "github.com/anthropics/claude-workflow/runtime/internal/context"
	"github.com/anthropics/claude-workflow/runtime/internal/cost"
)

// FactoryOptions provides optional customization for orchestrator assembly.
type FactoryOptions struct {
	// ModelCatalog overrides the default model catalog for cost calculation.
	// If nil, uses the default catalog with standard Anthropic models.
	ModelCatalog contracts.ModelCatalog

	// Currency overrides the default currency (USD) for cost calculation.
	// If empty, defaults to USD.
	Currency contracts.Currency
}

// NewOrchestratorWithDefaults creates an orchestrator with all default components.
// This is the simplest way to create a fully functional orchestrator.
//
// Parameters:
//   - policy: RunPolicy containing MaxParallelism, BudgetLimit, etc.
//   - executor: Function that executes tasks (calls LLM API). If nil, uses no-op executor.
//
// The orchestrator is assembled with:
//   - Scheduler, DependencyResolver, QueueManager (orchestration)
//   - ContextBuilder, ContextCompactor, ContextRouter (context management)
//   - TokenEstimator, CostCalculator, BudgetEnforcer, UsageTracker (cost control)
//   - ParallelExecutor configured from policy.MaxParallelism
func NewOrchestratorWithDefaults(
	policy contracts.RunPolicy,
	executor TaskExecutorFunc,
) contracts.Orchestrator {
	return NewOrchestratorWithOptions(policy, executor, FactoryOptions{})
}

// NewOrchestratorWithOptions creates an orchestrator with custom options.
// Use this when you need to customize the ModelCatalog or Currency.
//
// Parameters:
//   - policy: RunPolicy containing MaxParallelism, BudgetLimit, etc.
//   - executor: Function that executes tasks. If nil, uses no-op executor.
//   - opts: Optional customization for ModelCatalog and Currency.
func NewOrchestratorWithOptions(
	policy contracts.RunPolicy,
	executor TaskExecutorFunc,
	opts FactoryOptions,
) contracts.Orchestrator {
	// Create cost calculator with custom options if provided
	var costCalc contracts.CostCalculator
	if opts.ModelCatalog != nil || opts.Currency != "" {
		costCalc = cost.NewCostCalculatorWithCatalog(opts.ModelCatalog, opts.Currency)
	} else {
		costCalc = cost.NewCostCalculator()
	}

	deps := OrchestratorDeps{
		Scheduler:      NewScheduler(),
		DepResolver:    NewDependencyResolver(),
		Queue:          NewQueueManager(),
		Executor:       NewParallelExecutorFromPolicy(policy, executor),
		ContextBuilder: ctxpkg.NewContextBuilder(),
		Compactor:      ctxpkg.NewContextCompactor(),
		TokenEstimator: cost.NewTokenEstimator(),
		CostCalc:       costCalc,
		BudgetEnforcer: cost.NewBudgetEnforcer(),
		UsageTracker:   cost.NewUsageTracker(),
		Router:         ctxpkg.NewContextRouter(),
	}

	return NewOrchestrator(deps)
}
