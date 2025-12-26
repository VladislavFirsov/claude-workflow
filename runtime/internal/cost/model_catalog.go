package cost

import (
	"fmt"
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// DefaultModels contains the default model catalog.
// Model IDs from: https://docs.litellm.ai/docs/providers/anthropic
// Can be overridden via configuration.
var DefaultModels = []contracts.ModelInfo{
	// Claude 4.5 models (latest generation, Dec 2025)
	{
		ID:              "claude-opus-4-5-20251101",
		Provider:        "anthropic",
		MaxContext:      200000,
		InputCostPer1M:  15.0,
		OutputCostPer1M: 75.0,
		DefaultRole:     contracts.RoleFlagship,
		SupportsTools:   true,
	},
	{
		ID:              "claude-sonnet-4-5-20250929",
		Provider:        "anthropic",
		MaxContext:      200000,
		InputCostPer1M:  3.0,
		OutputCostPer1M: 15.0,
		DefaultRole:     contracts.RoleBalanced,
		SupportsTools:   true,
	},

	// Claude 4 models (May 2025)
	{
		ID:              "claude-opus-4-20250514",
		Provider:        "anthropic",
		MaxContext:      200000,
		InputCostPer1M:  15.0,
		OutputCostPer1M: 75.0,
		DefaultRole:     contracts.RoleFlagship,
		SupportsTools:   true,
	},
	{
		ID:              "claude-sonnet-4-20250514",
		Provider:        "anthropic",
		MaxContext:      200000,
		InputCostPer1M:  3.0,
		OutputCostPer1M: 15.0,
		DefaultRole:     contracts.RoleBalanced,
		SupportsTools:   true,
	},

	// Claude 3.5 models
	{
		ID:              "claude-3-5-sonnet-20240620",
		Provider:        "anthropic",
		MaxContext:      200000,
		InputCostPer1M:  3.0,
		OutputCostPer1M: 15.0,
		DefaultRole:     contracts.RoleBalanced,
		SupportsTools:   true,
	},

	// Claude 3 models (fast/cheap)
	{
		ID:              "claude-3-haiku-20240307",
		Provider:        "anthropic",
		MaxContext:      200000,
		InputCostPer1M:  0.25,
		OutputCostPer1M: 1.25,
		DefaultRole:     contracts.RoleFast,
		SupportsTools:   true,
	},
}

// DefaultRoleMappings maps roles to default model IDs.
var DefaultRoleMappings = map[contracts.ModelRole]contracts.ModelID{
	contracts.RoleFlagship: "claude-opus-4-5-20251101",
	contracts.RoleBalanced: "claude-sonnet-4-5-20250929",
	contracts.RoleFast:     "claude-3-haiku-20240307",
}

// modelCatalog implements contracts.ModelCatalog.
type modelCatalog struct {
	mu           sync.RWMutex
	models       map[contracts.ModelID]contracts.ModelInfo
	roleMappings map[contracts.ModelRole]contracts.ModelID
}

// NewModelCatalog creates a new ModelCatalog with default models.
func NewModelCatalog() contracts.ModelCatalog {
	return NewModelCatalogWithModels(DefaultModels, DefaultRoleMappings)
}

// NewModelCatalogWithModels creates a ModelCatalog with custom models.
func NewModelCatalogWithModels(models []contracts.ModelInfo, roleMappings map[contracts.ModelRole]contracts.ModelID) contracts.ModelCatalog {
	c := &modelCatalog{
		models:       make(map[contracts.ModelID]contracts.ModelInfo),
		roleMappings: make(map[contracts.ModelRole]contracts.ModelID),
	}

	for _, m := range models {
		c.models[m.ID] = m
	}

	for role, id := range roleMappings {
		c.roleMappings[role] = id
	}

	return c
}

// Get returns model info by ID.
func (c *modelCatalog) Get(id contracts.ModelID) (contracts.ModelInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	info, ok := c.models[id]
	return info, ok
}

// GetByRole returns the default model for a given role.
func (c *modelCatalog) GetByRole(role contracts.ModelRole) (contracts.ModelInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	id, ok := c.roleMappings[role]
	if !ok {
		return contracts.ModelInfo{}, false
	}

	info, ok := c.models[id]
	return info, ok
}

// List returns all available models.
func (c *modelCatalog) List() []contracts.ModelInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]contracts.ModelInfo, 0, len(c.models))
	for _, m := range c.models {
		result = append(result, m)
	}
	return result
}

// SetRoleMapping sets which model ID to use for a role.
func (c *modelCatalog) SetRoleMapping(role contracts.ModelRole, modelID contracts.ModelID) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, ok := c.models[modelID]; !ok {
		return fmt.Errorf("model %s not found: %w", modelID, contracts.ErrModelUnknown)
	}

	c.roleMappings[role] = modelID
	return nil
}
