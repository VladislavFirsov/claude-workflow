package cost

import (
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// usageTracker implements contracts.UsageTracker to track token and cost usage for a run.
// Thread-safe for concurrent access using sync.Mutex.
type usageTracker struct {
	mu sync.Mutex
	// Map from RunID to accumulated Usage
	usage map[contracts.RunID]contracts.Usage
}

// NewUsageTracker creates a new UsageTracker.
func NewUsageTracker() contracts.UsageTracker {
	return &usageTracker{
		usage: make(map[contracts.RunID]contracts.Usage),
	}
}

// Add adds usage to the run's total.
// If run is nil, it gracefully returns without panicking.
func (ut *usageTracker) Add(run *contracts.Run, usage contracts.Usage) {
	if run == nil {
		return
	}

	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Get the current usage for this run
	current := ut.usage[run.ID]

	// Add tokens
	current.Tokens += usage.Tokens

	// Add cost amount (keeping the currency from either existing or new usage)
	if current.Cost.Currency == "" {
		current.Cost.Currency = usage.Cost.Currency
	}
	current.Cost.Amount += usage.Cost.Amount

	// Store the updated usage
	ut.usage[run.ID] = current
}

// Snapshot returns the current usage for the run as a copy.
// If run is nil, it returns a zero-value Usage struct.
func (ut *usageTracker) Snapshot(run *contracts.Run) contracts.Usage {
	if run == nil {
		return contracts.Usage{}
	}

	ut.mu.Lock()
	defer ut.mu.Unlock()

	// Return a copy of the usage
	return ut.usage[run.ID]
}
