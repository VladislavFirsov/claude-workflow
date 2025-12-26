package cost

import (
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// usageTracker implements contracts.UsageTracker to track token usage for a run.
// Writes directly to run.Usage for synchronization with BudgetEnforcer.
// Thread-safe for concurrent access using sync.Mutex.
type usageTracker struct {
	mu sync.Mutex
}

// NewUsageTracker creates a new UsageTracker.
func NewUsageTracker() contracts.UsageTracker {
	return &usageTracker{}
}

// Add adds usage tokens to the run's total.
// Only updates Tokens - Cost is updated by BudgetEnforcer.Record() to avoid double-counting.
// If run is nil, it gracefully returns without panicking.
func (ut *usageTracker) Add(run *contracts.Run, usage contracts.Usage) {
	if run == nil {
		return
	}

	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Only update Tokens - Cost is updated by BudgetEnforcer.Record()
	run.Usage.Tokens += usage.Tokens
}

// Snapshot returns the current usage for the run.
// If run is nil, it returns a zero-value Usage struct.
func (ut *usageTracker) Snapshot(run *contracts.Run) contracts.Usage {
	if run == nil {
		return contracts.Usage{}
	}

	ut.mu.Lock()
	defer ut.mu.Unlock()

	return run.Usage
}
