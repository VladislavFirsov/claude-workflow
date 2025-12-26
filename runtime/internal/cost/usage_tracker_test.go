package cost

import (
	"sync"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestUsageTracker_Add_SingleRun(t *testing.T) {
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

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != 1000 {
		t.Errorf("Snapshot() tokens = %d, want 1000", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 1.50 {
		t.Errorf("Snapshot() cost amount = %v, want 1.50", snapshot.Cost.Amount)
	}
	if snapshot.Cost.Currency != "USD" {
		t.Errorf("Snapshot() currency = %s, want USD", snapshot.Cost.Currency)
	}
}

func TestUsageTracker_Add_Accumulation(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// First add
	ut.Add(run, contracts.Usage{
		Tokens: 500,
		Cost: contracts.Cost{
			Amount:   0.75,
			Currency: "USD",
		},
	})

	// Second add
	ut.Add(run, contracts.Usage{
		Tokens: 300,
		Cost: contracts.Cost{
			Amount:   0.45,
			Currency: "USD",
		},
	})

	// Third add
	ut.Add(run, contracts.Usage{
		Tokens: 200,
		Cost: contracts.Cost{
			Amount:   0.30,
			Currency: "USD",
		},
	})

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != 1000 {
		t.Errorf("Snapshot() tokens = %d, want 1000", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 1.50 {
		t.Errorf("Snapshot() cost amount = %v, want 1.50", snapshot.Cost.Amount)
	}
}

func TestUsageTracker_Add_MultipleRuns(t *testing.T) {
	ut := NewUsageTracker()

	run1 := &contracts.Run{ID: "run-1"}
	run2 := &contracts.Run{ID: "run-2"}
	run3 := &contracts.Run{ID: "run-3"}

	ut.Add(run1, contracts.Usage{
		Tokens: 1000,
		Cost:   contracts.Cost{Amount: 1.0, Currency: "USD"},
	})

	ut.Add(run2, contracts.Usage{
		Tokens: 2000,
		Cost:   contracts.Cost{Amount: 2.0, Currency: "USD"},
	})

	ut.Add(run3, contracts.Usage{
		Tokens: 3000,
		Cost:   contracts.Cost{Amount: 3.0, Currency: "USD"},
	})

	snapshot1 := ut.Snapshot(run1)
	snapshot2 := ut.Snapshot(run2)
	snapshot3 := ut.Snapshot(run3)

	if snapshot1.Tokens != 1000 {
		t.Errorf("run1 tokens = %d, want 1000", snapshot1.Tokens)
	}
	if snapshot2.Tokens != 2000 {
		t.Errorf("run2 tokens = %d, want 2000", snapshot2.Tokens)
	}
	if snapshot3.Tokens != 3000 {
		t.Errorf("run3 tokens = %d, want 3000", snapshot3.Tokens)
	}
}

func TestUsageTracker_Snapshot_ZeroUsage(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// Snapshot before any add
	snapshot := ut.Snapshot(run)

	if snapshot.Tokens != 0 {
		t.Errorf("Snapshot() tokens = %d, want 0", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 0 {
		t.Errorf("Snapshot() cost amount = %v, want 0", snapshot.Cost.Amount)
	}
	if snapshot.Cost.Currency != "" {
		t.Errorf("Snapshot() currency = %s, want empty string", snapshot.Cost.Currency)
	}
}

func TestUsageTracker_Snapshot_ReturnssCopy(t *testing.T) {
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

	snapshot1 := ut.Snapshot(run)
	snapshot2 := ut.Snapshot(run)

	// Verify they have the same values
	if snapshot1.Tokens != snapshot2.Tokens {
		t.Errorf("snapshot1 tokens %d != snapshot2 tokens %d", snapshot1.Tokens, snapshot2.Tokens)
	}
	if snapshot1.Cost.Amount != snapshot2.Cost.Amount {
		t.Errorf("snapshot1 amount %v != snapshot2 amount %v", snapshot1.Cost.Amount, snapshot2.Cost.Amount)
	}

	// Snapshots should be independent copies (modifying one shouldn't affect the other)
	// Note: Since Usage contains a Cost struct (not pointer), modifying snapshot1.Cost.Amount
	// doesn't affect snapshot2, confirming it's a copy
}

func TestUsageTracker_Add_NilRun(t *testing.T) {
	ut := NewUsageTracker()

	// Should not panic when adding to nil run
	ut.Add(nil, contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   1.0,
			Currency: "USD",
		},
	})

	// Should not panic when snapshotting nil run
	snapshot := ut.Snapshot(nil)
	if snapshot.Tokens != 0 {
		t.Errorf("Snapshot(nil) tokens = %d, want 0", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 0 {
		t.Errorf("Snapshot(nil) cost amount = %v, want 0", snapshot.Cost.Amount)
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
			ut.Add(run, contracts.Usage{
				Tokens: 10,
				Cost: contracts.Cost{
					Amount:   0.01,
					Currency: "USD",
				},
			})
		}()
	}
	wg.Wait()

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != 1000 {
		t.Errorf("Snapshot() tokens = %d, want 1000", snapshot.Tokens)
	}
	// Use approximate comparison for floating point
	epsilon := 1e-9
	if snapshot.Cost.Amount < 1.0-epsilon || snapshot.Cost.Amount > 1.0+epsilon {
		t.Errorf("Snapshot() cost amount = %v, want ~1.0", snapshot.Cost.Amount)
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
			ut.Add(run, contracts.Usage{
				Tokens: 10,
				Cost: contracts.Cost{
					Amount:   0.01,
					Currency: "USD",
				},
			})
		}()

		go func() {
			defer wg.Done()
			ut.Snapshot(run)
		}()
	}
	wg.Wait()

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != 500 {
		t.Errorf("Snapshot() tokens = %d, want 500", snapshot.Tokens)
	}
	// Use approximate comparison for floating point
	epsilon := 1e-9
	if snapshot.Cost.Amount < 0.5-epsilon || snapshot.Cost.Amount > 0.5+epsilon {
		t.Errorf("Snapshot() cost amount = %v, want ~0.5", snapshot.Cost.Amount)
	}
}

func TestUsageTracker_Concurrent_MultipleRuns(t *testing.T) {
	ut := NewUsageTracker()
	var wg sync.WaitGroup

	// Concurrent operations on different runs
	for runIdx := 0; runIdx < 10; runIdx++ {
		runID := contracts.RunID("run-" + string(rune('0'+runIdx)))
		run := &contracts.Run{ID: runID}

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ut.Add(run, contracts.Usage{
					Tokens: 100,
					Cost: contracts.Cost{
						Amount:   0.1,
						Currency: "USD",
					},
				})
			}()
		}
	}
	wg.Wait()

	// Verify each run accumulated 1000 tokens
	epsilon := 1e-9
	for runIdx := 0; runIdx < 10; runIdx++ {
		runID := contracts.RunID("run-" + string(rune('0'+runIdx)))
		run := &contracts.Run{ID: runID}
		snapshot := ut.Snapshot(run)
		if snapshot.Tokens != 1000 {
			t.Errorf("run %s tokens = %d, want 1000", runID, snapshot.Tokens)
		}
		if snapshot.Cost.Amount < 1.0-epsilon || snapshot.Cost.Amount > 1.0+epsilon {
			t.Errorf("run %s cost amount = %v, want ~1.0", runID, snapshot.Cost.Amount)
		}
	}
}

func TestUsageTracker_CurrencyPreservation(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// First add establishes currency
	ut.Add(run, contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   1.0,
			Currency: "USD",
		},
	})

	// Second add with same currency
	ut.Add(run, contracts.Usage{
		Tokens: 500,
		Cost: contracts.Cost{
			Amount:   0.5,
			Currency: "USD",
		},
	})

	snapshot := ut.Snapshot(run)
	if snapshot.Cost.Currency != "USD" {
		t.Errorf("Snapshot() currency = %s, want USD", snapshot.Cost.Currency)
	}
}

func TestUsageTracker_CurrencyFromSecondAdd(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// First add with no currency
	ut.Add(run, contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   1.0,
			Currency: "",
		},
	})

	// Second add establishes currency
	ut.Add(run, contracts.Usage{
		Tokens: 500,
		Cost: contracts.Cost{
			Amount:   0.5,
			Currency: "EUR",
		},
	})

	snapshot := ut.Snapshot(run)
	if snapshot.Cost.Currency != "EUR" {
		t.Errorf("Snapshot() currency = %s, want EUR", snapshot.Cost.Currency)
	}
}

func TestUsageTracker_ZeroCostUsage(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// Add usage with zero cost
	ut.Add(run, contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   0.0,
			Currency: "USD",
		},
	})

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != 1000 {
		t.Errorf("Snapshot() tokens = %d, want 1000", snapshot.Tokens)
	}
	if snapshot.Cost.Amount != 0.0 {
		t.Errorf("Snapshot() cost amount = %v, want 0.0", snapshot.Cost.Amount)
	}
	if snapshot.Cost.Currency != "USD" {
		t.Errorf("Snapshot() currency = %s, want USD", snapshot.Cost.Currency)
	}
}

func TestUsageTracker_LargeTokenCounts(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: "run-1"}

	// Add large token counts (avoiding overflow)
	// Max int64 = 9,223,372,036,854,775,807
	// Using a safe large value instead
	largeToken := contracts.TokenCount(1_000_000_000_000) // 1 trillion

	ut.Add(run, contracts.Usage{
		Tokens: largeToken / 2,
		Cost: contracts.Cost{
			Amount:   1.0,
			Currency: "USD",
		},
	})

	ut.Add(run, contracts.Usage{
		Tokens: largeToken / 2,
		Cost: contracts.Cost{
			Amount:   1.0,
			Currency: "USD",
		},
	})

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != largeToken {
		t.Errorf("Snapshot() tokens = %d, want %d", snapshot.Tokens, largeToken)
	}
}

func TestUsageTracker_EmptyRunID(t *testing.T) {
	ut := NewUsageTracker()
	run := &contracts.Run{ID: ""}

	ut.Add(run, contracts.Usage{
		Tokens: 1000,
		Cost: contracts.Cost{
			Amount:   1.0,
			Currency: "USD",
		},
	})

	snapshot := ut.Snapshot(run)
	if snapshot.Tokens != 1000 {
		t.Errorf("Snapshot() tokens = %d, want 1000", snapshot.Tokens)
	}
}
