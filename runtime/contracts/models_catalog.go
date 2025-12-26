package contracts

// ModelRole represents the intended use case for a model.
type ModelRole string

const (
	// RoleFlagship - maximum quality, for TIER-3 critical tasks.
	RoleFlagship ModelRole = "flagship"
	// RoleBalanced - good quality/cost ratio, for TIER-2 tasks.
	RoleBalanced ModelRole = "balanced"
	// RoleFast - cheap and fast, for TIER-1 and auxiliary tasks.
	RoleFast ModelRole = "fast"
)

// ModelInfo contains metadata about a model.
type ModelInfo struct {
	ID            ModelID   `json:"id"`
	Provider      string    `json:"provider"`
	MaxContext    int       `json:"max_context"`
	InputCostPer1M  float64 `json:"input_cost_per_1m"`  // USD per 1M tokens
	OutputCostPer1M float64 `json:"output_cost_per_1m"` // USD per 1M tokens
	DefaultRole   ModelRole `json:"default_role"`
	SupportsTools bool      `json:"supports_tools"`
}

// AverageCostPer1M returns the average cost per 1M tokens (input + output / 2).
func (m ModelInfo) AverageCostPer1M() float64 {
	return (m.InputCostPer1M + m.OutputCostPer1M) / 2
}

// ModelCatalog provides model information and role-based selection.
type ModelCatalog interface {
	// Get returns model info by ID.
	Get(id ModelID) (ModelInfo, bool)

	// GetByRole returns the default model for a given role.
	GetByRole(role ModelRole) (ModelInfo, bool)

	// List returns all available models.
	List() []ModelInfo

	// SetRoleMapping sets which model ID to use for a role.
	SetRoleMapping(role ModelRole, modelID ModelID) error
}
