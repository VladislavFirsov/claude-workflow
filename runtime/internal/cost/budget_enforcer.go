package cost

import (
	"fmt"
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// budgetEnforcer implements contracts.BudgetEnforcer.
// CRITICAL: This component enforces budget limits. Errors here mean client money loss.
//
// Thread-safety: Uses mutex for concurrent access to run state.
// The enforcer tracks usage per run to prevent budget overruns.
type budgetEnforcer struct {
	mu sync.Mutex
}

// NewBudgetEnforcer creates a new BudgetEnforcer.
func NewBudgetEnforcer() contracts.BudgetEnforcer {
	return &budgetEnforcer{}
}

// Allow checks if the estimated cost is within budget.
// Returns error if:
// - run is nil (ErrInvalidInput)
// - budget not set (ErrBudgetNotSet)
// - estimate would exceed budget (ErrBudgetExceeded)
// - currency mismatch between estimate and budget
func (b *budgetEnforcer) Allow(run *contracts.Run, estimate contracts.Cost) error {
	if run == nil {
		return contracts.ErrInvalidInput
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if budget is set
	budget := run.Policy.BudgetLimit
	if budget.Amount <= 0 {
		return contracts.ErrBudgetNotSet
	}

	// Validate currency matches
	if estimate.Currency != "" && budget.Currency != "" && estimate.Currency != budget.Currency {
		return fmt.Errorf("currency mismatch: estimate %s, budget %s: %w",
			estimate.Currency, budget.Currency, contracts.ErrInvalidInput)
	}

	// Calculate projected total: current usage + estimate
	currentUsage := run.Usage.Cost.Amount
	projectedTotal := currentUsage + estimate.Amount

	// Check if projected total exceeds budget
	if projectedTotal > budget.Amount {
		return fmt.Errorf("projected cost %.4f exceeds budget %.4f (current: %.4f, estimate: %.4f): %w",
			projectedTotal, budget.Amount, currentUsage, estimate.Amount, contracts.ErrBudgetExceeded)
	}

	return nil
}

// Record records actual cost and updates the run usage.
// Returns error if:
// - run is nil (ErrInvalidInput)
// - recording would exceed budget (ErrBudgetExceeded) - safety check
//
// Note: Record updates run.Usage.Cost in place.
func (b *budgetEnforcer) Record(run *contracts.Run, actual contracts.Cost) error {
	if run == nil {
		return contracts.ErrInvalidInput
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Safety check: don't allow recording if it would exceed budget
	// This catches cases where Allow was bypassed or estimate was wrong
	budget := run.Policy.BudgetLimit
	if budget.Amount > 0 {
		projectedTotal := run.Usage.Cost.Amount + actual.Amount
		if projectedTotal > budget.Amount {
			return fmt.Errorf("recording cost %.4f would exceed budget %.4f (current: %.4f): %w",
				actual.Amount, budget.Amount, run.Usage.Cost.Amount, contracts.ErrBudgetExceeded)
		}
	}

	// Update usage
	run.Usage.Cost.Amount += actual.Amount
	if run.Usage.Cost.Currency == "" && actual.Currency != "" {
		run.Usage.Cost.Currency = actual.Currency
	}

	return nil
}
