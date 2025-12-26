package cost

import (
	"errors"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestCostCalculator_Estimate(t *testing.T) {
	calc := NewCostCalculator()

	tests := []struct {
		name      string
		tokens    contracts.TokenCount
		model     contracts.ModelID
		wantCost  float64
		wantErr   error
	}{
		{
			name:     "zero tokens",
			tokens:   0,
			model:    "claude-3-haiku-20240307",
			wantCost: 0.0,
		},
		{
			name:     "haiku 1M tokens",
			tokens:   1_000_000,
			model:    "claude-3-haiku-20240307",
			wantCost: 0.75, // (0.25 + 1.25) / 2 = 0.75
		},
		{
			name:     "sonnet 4.5 1M tokens",
			tokens:   1_000_000,
			model:    "claude-sonnet-4-5-20250929",
			wantCost: 9.0, // (3 + 15) / 2 = 9
		},
		{
			name:     "opus 4.5 1M tokens",
			tokens:   1_000_000,
			model:    "claude-opus-4-5-20251101",
			wantCost: 45.0, // (15 + 75) / 2 = 45
		},
		{
			name:     "haiku 100K tokens",
			tokens:   100_000,
			model:    "claude-3-haiku-20240307",
			wantCost: 0.075, // 0.75 / 10
		},
		{
			name:     "unknown model",
			tokens:   1000,
			model:    "unknown-model",
			wantErr:  contracts.ErrModelUnknown,
		},
		{
			name:     "claude-3-5-sonnet",
			tokens:   1_000_000,
			model:    "claude-3-5-sonnet-20240620",
			wantCost: 9.0, // (3 + 15) / 2 = 9
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calc.Estimate(tt.tokens, tt.model)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("Estimate() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Estimate() unexpected error = %v", err)
			}

			if got.Amount != tt.wantCost {
				t.Errorf("Estimate() amount = %v, want %v", got.Amount, tt.wantCost)
			}

			if got.Currency != "USD" {
				t.Errorf("Estimate() currency = %v, want USD", got.Currency)
			}
		})
	}
}

func TestCostCalculator_EstimateByRole(t *testing.T) {
	calc := NewCostCalculator().(*costCalculator)

	tests := []struct {
		name     string
		tokens   contracts.TokenCount
		role     contracts.ModelRole
		wantCost float64
		wantErr  error
	}{
		{
			name:     "flagship role",
			tokens:   1_000_000,
			role:     contracts.RoleFlagship,
			wantCost: 45.0, // opus 4.5: (15 + 75) / 2
		},
		{
			name:     "balanced role",
			tokens:   1_000_000,
			role:     contracts.RoleBalanced,
			wantCost: 9.0, // sonnet 4.5: (3 + 15) / 2
		},
		{
			name:     "fast role",
			tokens:   1_000_000,
			role:     contracts.RoleFast,
			wantCost: 0.75, // haiku 3: (0.25 + 1.25) / 2
		},
		{
			name:    "unknown role",
			tokens:  1000,
			role:    contracts.ModelRole("unknown"),
			wantErr: contracts.ErrModelUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := calc.EstimateByRole(tt.tokens, tt.role)

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("EstimateByRole() error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("EstimateByRole() unexpected error = %v", err)
			}

			if got.Amount != tt.wantCost {
				t.Errorf("EstimateByRole() amount = %v, want %v", got.Amount, tt.wantCost)
			}
		})
	}
}

func TestCostCalculator_CustomCatalog(t *testing.T) {
	customModels := []contracts.ModelInfo{
		{
			ID:              "custom-model",
			Provider:        "custom",
			InputCostPer1M:  10.0,
			OutputCostPer1M: 20.0,
			DefaultRole:     contracts.RoleFlagship,
		},
	}
	customMappings := map[contracts.ModelRole]contracts.ModelID{
		contracts.RoleFlagship: "custom-model",
	}

	catalog := NewModelCatalogWithModels(customModels, customMappings)
	calc := NewCostCalculatorWithCatalog(catalog, "EUR")

	got, err := calc.Estimate(1_000_000, "custom-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// (10 + 20) / 2 = 15
	if got.Amount != 15.0 {
		t.Errorf("amount = %v, want 15.0", got.Amount)
	}

	if got.Currency != "EUR" {
		t.Errorf("currency = %v, want EUR", got.Currency)
	}

	// Default model should not exist
	_, err = calc.Estimate(1000, "claude-opus-4")
	if !errors.Is(err, contracts.ErrModelUnknown) {
		t.Errorf("expected ErrModelUnknown for model not in custom catalog")
	}
}

func TestCostCalculator_DefaultsOnNil(t *testing.T) {
	calc := NewCostCalculatorWithCatalog(nil, "")

	// Should use default catalog
	got, err := calc.Estimate(1_000_000, "claude-3-haiku-20240307")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// (0.25 + 1.25) / 2 = 0.75
	if got.Amount != 0.75 {
		t.Errorf("amount = %v, want 0.75", got.Amount)
	}

	if got.Currency != "USD" {
		t.Errorf("currency = %v, want USD", got.Currency)
	}
}
