package cost

import (
	"sync"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestUsageTracker_Add_UpdatesRunUsage(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	usage := contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   1.50,
			Currency: "USD",
		},
	}

	ut.Add(run, usage)

	// Verify run.Usage.Tokens is updated
	if run.Usage.Tokens != 1000 {
		t.Errorf("run.Usage.Tokens = %d, want 1000", run.Usage.Tokens)
	}

	// Cost is NOT updated by UsageTracker (BudgetEnforcer.Record handles that)
	if run.Usage.Cost.Amount != 0 {
		t.Errorf("run.Usage.Cost.Amount = %v, want 0 (not updated by UsageTracker)", run.Usage.Cost.Amount)
	}
}

func TestUsageTracker_Add_TokensAccumulation(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// First add
	ut.Add(run, contracts.Usage{Tokens: 500})

	// Second add
	ut.Add(run, contracts.Usage{Tokens: 300})

	// Third add
	ut.Add(run, contracts.Usage{Tokens: 200})

	if run.Usage.Tokens != 1000 {
		t.Errorf("run.Usage.Tokens = %d, want 1000", run.Usage.Tokens)
	}
}

func TestUsageTracker_Snapshot_ReturnsRunUsage(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{
		ID: "run-1",
		Usage: contracts.Usage{
			Tokens: 500,
			Cost: contracts.Cost{
				Amount:   0.75,
				Currency: "USD",
			},
		},
	}

	snapshot := ut.Snapshot(run)

	if snapshot.Tokens != 500 {
		t.Errorf("Snapshot().Tokens = %d, want 500", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 0.75 {
		t.Errorf("Snapshot().Cost.Amount = %v, want 0.75", snapshot.Cost.Amount)
	}
	if snapshot.Cost.Currency != "USD" {
		t.Errorf("Snapshot().Cost.Currency = %s, want USD", snapshot.Cost.Currency)
	}
}

func TestUsageTracker_Snapshot_ZeroUsage(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	snapshot := ut.Snapshot(run)

	if snapshot.Tokens != 0 {
		t.Errorf("Snapshot().Tokens = %d, want 0", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 0 {
		t.Errorf("Snapshot().Cost.Amount = %v, want 0", snapshot.Cost.Amount)
	}
}

func TestUsageTracker_Add_NilRun(t *testing.T) {
	ut := NewUsageTracker()

	// Should not panic when adding to nil run
	ut.Add(nil, contracts.Usage{Tokens: 1000})

	// Should not panic when snapshotting nil run
	snapshot := ut.Snapshot(nil)
	if snapshot.Tokens != 0 {
		t.Errorf("Snapshot(nil).Tokens = %d, want 0", snapshot.Tokens)
	}
}

func TestUsageTracker_Concurrent_Add(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}
	var wg sync.WaitGroup

	// 100 concurrent adds
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ut.Add(run, contracts.Usage{Tokens: 10})
		}()
	}
	wg.Wait()

	if run.Usage.Tokens != 1000 {
		t.Errorf("run.Usage.Tokens = %d, want 1000", run.Usage.Tokens)
	}
}

func TestUsageTracker_Concurrent_AddAndSnapshot(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}
	var wg sync.WaitGroup

	// Mix of add and snapshot operations
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			ut.Add(run, contracts.Usage{Tokens: 10})
		}()

		go func() {
			defer wg.Done()
			ut.Snapshot(run)
		}()
	}
	wg.Wait()

	if run.Usage.Tokens != 500 {
		t.Errorf("run.Usage.Tokens = %d, want 500", run.Usage.Tokens)
	}
}

func TestUsageTracker_MultipleRuns_Independent(t *testing.T) {
	ut := NewUsageTracker()

	run1 := &contracts.Run{ID: "run-1"}
	run2 := &contracts.Run{ID: "run-2"}

	ut.Add(run1, contracts.Usage{Tokens: 1000})
	ut.Add(run2, contracts.Usage{Tokens: 2000})

	if run1.Usage.Tokens != 1000 {
		t.Errorf("run1.Usage.Tokens = %d, want 1000", run1.Usage.Tokens)
	}
	if run2.Usage.Tokens != 2000 {
		t.Errorf("run2.Usage.Tokens = %d, want 2000", run2.Usage.Tokens)
	}
}

func TestUsageTracker_LargeTokenCounts(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	largeToken := contracts.TokenCount(1_000_000_000_000) // 1 trillion

	ut.Add(run, contracts.Usage{Tokens: largeToken / 2})
	ut.Add(run, contracts.Usage{Tokens: largeToken / 2})

	if run.Usage.Tokens != largeToken {
		t.Errorf("run.Usage.Tokens = %d, want %d", run.Usage.Tokens, largeToken)
	}
}

func TestUsageTracker_DoesNotUpdateCost(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// Set initial cost (simulating BudgetEnforcer.Record)
	run.Usage.Cost = contracts.Cost{Amount: 5.0, Currency: "USD"}

	// Add usage with different cost
	ut.Add(run, contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   1.0, // This should be ignored
			Currency: "EUR",
		},
	})

	// Tokens should be updated
	if run.Usage.Tokens != 1000 {
		t.Errorf("run.Usage.Tokens = %d, want 1000", run.Usage.Tokens)
	}

	// Cost should remain unchanged (not overwritten by Add)
	if run.Usage.Cost.Amount != 5.0 {
		t.Errorf("run.Usage.Cost.Amount = %v, want 5.0 (unchanged)", run.Usage.Cost.Amount)
	}
	if run.Usage.Cost.Currency != "USD" {
		t.Errorf("run.Usage.Cost.Currency = %s, want USD (unchanged)", run.Usage.Cost.Currency)
	}
}
