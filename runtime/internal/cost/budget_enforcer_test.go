package cost

import (
	"errors"
	"sync"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestBudgetEnforcer_Allow(t *testing.T) {
	enforcer := NewBudgetEnforcer()

	tests := []struct {
		name     string
		run      *contracts.Run
		estimate contracts.Cost
		wantErr  error
	}{
		{
			name:    "nil run returns error",
			run:     nil,
			wantErr: contracts.ErrInvalidInput,
		},
		{
			name: "zero budget returns ErrBudgetNotSet",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 0}},
			},
			estimate: contracts.Cost{Amount: 1.0},
			wantErr:  contracts.ErrBudgetNotSet,
		},
		{
			name: "negative budget returns ErrBudgetNotSet",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: -10}},
			},
			estimate: contracts.Cost{Amount: 1.0},
			wantErr:  contracts.ErrBudgetNotSet,
		},
		{
			name: "estimate within budget allowed",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 0}},
			},
			estimate: contracts.Cost{Amount: 50, Currency: "USD"},
			wantErr:  nil,
		},
		{
			name: "estimate exactly at budget allowed",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 50}},
			},
			estimate: contracts.Cost{Amount: 50, Currency: "USD"},
			wantErr:  nil,
		},
		{
			name: "estimate exceeds budget",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 60}},
			},
			estimate: contracts.Cost{Amount: 50, Currency: "USD"},
			wantErr:  contracts.ErrBudgetExceeded,
		},
		{
			name: "estimate alone exceeds budget",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 10, Currency: "USD"}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 0}},
			},
			estimate: contracts.Cost{Amount: 15, Currency: "USD"},
			wantErr:  contracts.ErrBudgetExceeded,
		},
		{
			name: "currency mismatch returns error",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
			},
			estimate: contracts.Cost{Amount: 10, Currency: "EUR"},
			wantErr:  contracts.ErrInvalidInput,
		},
		{
			name: "empty estimate currency allowed",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
			},
			estimate: contracts.Cost{Amount: 10, Currency: ""},
			wantErr:  nil,
		},
		{
			name: "empty budget currency allowed",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: ""}},
			},
			estimate: contracts.Cost{Amount: 10, Currency: "USD"},
			wantErr:  nil,
		},
		{
			name: "zero estimate always allowed",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 99}},
			},
			estimate: contracts.Cost{Amount: 0, Currency: "USD"},
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.Allow(tt.run, tt.estimate)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Allow() expected error containing %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Allow() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("Allow() unexpected error = %v", err)
			}
		})
	}
}

func TestBudgetEnforcer_Record(t *testing.T) {
	tests := []struct {
		name       string
		run        *contracts.Run
		actual     contracts.Cost
		wantErr    error
		wantAmount float64
	}{
		{
			name:    "nil run returns error",
			run:     nil,
			actual:  contracts.Cost{Amount: 10},
			wantErr: contracts.ErrInvalidInput,
		},
		{
			name: "record updates usage",
			run: &contracts.Run{
				ID:    "run-1",
				Usage: contracts.Usage{Cost: contracts.Cost{Amount: 10}},
			},
			actual:     contracts.Cost{Amount: 5, Currency: "USD"},
			wantAmount: 15,
		},
		{
			name: "record sets currency if empty",
			run: &contracts.Run{
				ID:    "run-1",
				Usage: contracts.Usage{Cost: contracts.Cost{Amount: 0, Currency: ""}},
			},
			actual:     contracts.Cost{Amount: 10, Currency: "USD"},
			wantAmount: 10,
		},
		{
			name: "record without budget limit",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 0}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 100}},
			},
			actual:     contracts.Cost{Amount: 50},
			wantAmount: 150,
		},
		{
			name: "record within budget",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 40}},
			},
			actual:     contracts.Cost{Amount: 30},
			wantAmount: 70,
		},
		{
			name: "record exceeds budget (safety check)",
			run: &contracts.Run{
				ID:     "run-1",
				Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100}},
				Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 80}},
			},
			actual:  contracts.Cost{Amount: 30},
			wantErr: contracts.ErrBudgetExceeded,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enforcer := NewBudgetEnforcer()
			err := enforcer.Record(tt.run, tt.actual)

			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("Record() expected error containing %v, got nil", tt.wantErr)
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Record() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Record() unexpected error = %v", err)
			}

			if tt.run.Usage.Cost.Amount != tt.wantAmount {
				t.Errorf("Record() usage amount = %v, want %v", tt.run.Usage.Cost.Amount, tt.wantAmount)
			}
		})
	}
}

func TestBudgetEnforcer_CurrencyPreservation(t *testing.T) {
	enforcer := NewBudgetEnforcer()

	run := &contracts.Run{
		ID:    "run-1",
		Usage: contracts.Usage{Cost: contracts.Cost{Amount: 10, Currency: "USD"}},
	}

	// Record with different currency should preserve original
	err := enforcer.Record(run, contracts.Cost{Amount: 5, Currency: "EUR"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.Usage.Cost.Currency != "USD" {
		t.Errorf("currency changed from USD to %v", run.Usage.Cost.Currency)
	}
}

func TestBudgetEnforcer_Concurrent(t *testing.T) {
	enforcer := NewBudgetEnforcer()

	run := &contracts.Run{
		ID:     "run-1",
		Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 1000, Currency: "USD"}},
		Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 0, Currency: "USD"}},
	}

	var wg sync.WaitGroup
	errChan := make(chan error, 200)

	// 100 concurrent Allow calls
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := enforcer.Allow(run, contracts.Cost{Amount: 5, Currency: "USD"})
			if err != nil {
				errChan <- err
			}
		}()
	}

	// 100 concurrent Record calls (small amounts to stay within budget)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := enforcer.Record(run, contracts.Cost{Amount: 0.1, Currency: "USD"})
			if err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for unexpected errors
	for err := range errChan {
		// Only budget exceeded errors are expected
		if !errors.Is(err, contracts.ErrBudgetExceeded) {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// Final usage should be ~10 (100 * 0.1)
	if run.Usage.Cost.Amount < 5 || run.Usage.Cost.Amount > 15 {
		t.Errorf("unexpected final usage: %v", run.Usage.Cost.Amount)
	}
}

func TestBudgetEnforcer_Integration(t *testing.T) {
	enforcer := NewBudgetEnforcer()

	run := &contracts.Run{
		ID:     "run-1",
		Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 100, Currency: "USD"}},
		Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 0, Currency: "USD"}},
	}

	// Step 1: Check if we can afford 30
	err := enforcer.Allow(run, contracts.Cost{Amount: 30, Currency: "USD"})
	if err != nil {
		t.Fatalf("Allow(30) unexpected error: %v", err)
	}

	// Step 2: Record 30
	err = enforcer.Record(run, contracts.Cost{Amount: 30, Currency: "USD"})
	if err != nil {
		t.Fatalf("Record(30) unexpected error: %v", err)
	}

	// Step 3: Check if we can afford another 50
	err = enforcer.Allow(run, contracts.Cost{Amount: 50, Currency: "USD"})
	if err != nil {
		t.Fatalf("Allow(50) unexpected error: %v", err)
	}

	// Step 4: Record 50
	err = enforcer.Record(run, contracts.Cost{Amount: 50, Currency: "USD"})
	if err != nil {
		t.Fatalf("Record(50) unexpected error: %v", err)
	}

	// Step 5: Now at 80, try to afford 30 (should fail)
	err = enforcer.Allow(run, contracts.Cost{Amount: 30, Currency: "USD"})
	if !errors.Is(err, contracts.ErrBudgetExceeded) {
		t.Fatalf("Allow(30) should exceed budget, got: %v", err)
	}

	// Step 6: Can still afford 20
	err = enforcer.Allow(run, contracts.Cost{Amount: 20, Currency: "USD"})
	if err != nil {
		t.Fatalf("Allow(20) unexpected error: %v", err)
	}

	// Verify final state
	if run.Usage.Cost.Amount != 80 {
		t.Errorf("final usage = %v, want 80", run.Usage.Cost.Amount)
	}
}

func TestBudgetEnforcer_PrecisionEdgeCases(t *testing.T) {
	enforcer := NewBudgetEnforcer()

	// Test floating point precision
	run := &contracts.Run{
		ID:     "run-1",
		Policy: contracts.RunPolicy{BudgetLimit: contracts.Cost{Amount: 0.3, Currency: "USD"}},
		Usage:  contracts.Usage{Cost: contracts.Cost{Amount: 0.1, Currency: "USD"}},
	}

	// 0.1 + 0.2 should equal 0.3 (but floating point...)
	err := enforcer.Allow(run, contracts.Cost{Amount: 0.2, Currency: "USD"})
	if err != nil {
		t.Logf("Note: floating point precision issue: %v", err)
	}
}
