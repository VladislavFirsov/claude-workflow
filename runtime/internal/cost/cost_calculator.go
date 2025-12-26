package cost

import (
	"github.com/anthropics/claude-workflow/runtime/contracts"
)

const defaultCurrency = contracts.Currency("USD")

// costCalculator implements contracts.CostCalculator using ModelCatalog.
type costCalculator struct {
	catalog  contracts.ModelCatalog
	currency contracts.Currency
}

// NewCostCalculator creates a new CostCalculator with default catalog.
func NewCostCalculator() contracts.CostCalculator {
	return &costCalculator{
		catalog:  NewModelCatalog(),
		currency: defaultCurrency,
	}
}

// NewCostCalculatorWithCatalog creates a CostCalculator with custom catalog.
func NewCostCalculatorWithCatalog(catalog contracts.ModelCatalog, currency contracts.Currency) contracts.CostCalculator {
	if catalog == nil {
		catalog = NewModelCatalog()
	}
	if currency == "" {
		currency = defaultCurrency
	}
	return &costCalculator{
		catalog:  catalog,
		currency: currency,
	}
}

// Estimate returns the estimated cost for the given tokens and model.
func (c *costCalculator) Estimate(tokens contracts.TokenCount, model contracts.ModelID) (contracts.Cost, error) {
	info, ok := c.catalog.Get(model)
	if !ok {
		return contracts.Cost{}, contracts.ErrModelUnknown
	}

	// Use average cost (input + output) / 2
	pricePerMillion := info.AverageCostPer1M()
	amount := float64(tokens) * pricePerMillion / 1_000_000

	return contracts.Cost{
		Amount:   amount,
		Currency: c.currency,
	}, nil
}

// EstimateByRole estimates cost using the model assigned to a role.
func (c *costCalculator) EstimateByRole(tokens contracts.TokenCount, role contracts.ModelRole) (contracts.Cost, error) {
	info, ok := c.catalog.GetByRole(role)
	if !ok {
		return contracts.Cost{}, contracts.ErrModelUnknown
	}

	pricePerMillion := info.AverageCostPer1M()
	amount := float64(tokens) * pricePerMillion / 1_000_000

	return contracts.Cost{
		Amount:   amount,
		Currency: c.currency,
	}, nil
}
