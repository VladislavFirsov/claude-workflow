package config

import "fmt"

// ValidatorOptions configures optional validation behavior.
type ValidatorOptions struct {
	// RequireDefaultRoles enables validation that all default spec workflow roles
	// (spec-analyst, spec-architect, spec-developer, spec-validator) are present.
	// Default: true for backwards compatibility with default spec workflow.
	RequireDefaultRoles bool
}

// DefaultValidatorOptions returns options suitable for the default spec workflow.
func DefaultValidatorOptions() ValidatorOptions {
	return ValidatorOptions{
		RequireDefaultRoles: true,
	}
}

// Validator validates workflow configurations.
type Validator struct {
	opts ValidatorOptions
}

// NewValidator creates a new configuration validator with default options.
// Validates that all required spec workflow roles are present.
func NewValidator() *Validator {
	return &Validator{opts: DefaultValidatorOptions()}
}

// NewValidatorWithOptions creates a validator with custom options.
// Use RequireDefaultRoles=false for custom workflows without spec-* roles.
func NewValidatorWithOptions(opts ValidatorOptions) *Validator {
	return &Validator{opts: opts}
}

// Validate performs comprehensive validation of a WorkflowConfig.
// Returns nil if valid, or an error describing the first validation failure.
func (v *Validator) Validate(cfg *WorkflowConfig) error {
	if cfg == nil {
		return ErrConfigEmpty
	}

	// 1. Validate workflow.name is not empty
	if cfg.Workflow.Name == "" {
		return ErrWorkflowNameEmpty
	}

	// 2. Validate steps is not empty
	if len(cfg.Workflow.Steps) == 0 {
		return ErrNoSteps
	}

	// 3. Validate each step has id and role, collect id set
	stepIDs := make(map[string]bool)
	roleSet := make(map[Role]bool)

	for i, step := range cfg.Workflow.Steps {
		if step.ID == "" {
			return fmt.Errorf("step[%d]: %w", i, ErrStepIDEmpty)
		}

		if stepIDs[step.ID] {
			return fmt.Errorf("step.id=%s: %w", step.ID, ErrStepIDDuplicate)
		}
		stepIDs[step.ID] = true

		if step.Role == "" {
			return fmt.Errorf("step[%d] id=%s: %w", i, step.ID, ErrStepRoleEmpty)
		}

		roleSet[Role(step.Role)] = true
	}

	// 4. Validate depends_on references existing ids
	for _, step := range cfg.Workflow.Steps {
		for _, depID := range step.DependsOn {
			if !stepIDs[depID] {
				return fmt.Errorf("step.id=%s depends_on=%s: %w",
					step.ID, depID, ErrDependencyNotFound)
			}
		}
	}

	// 5. Validate no cycles (DFS with color marking)
	if err := v.detectCycle(cfg.Workflow.Steps); err != nil {
		return err
	}

	// 6. Validate required roles are present (if enabled)
	if v.opts.RequireDefaultRoles {
		for _, requiredRole := range RequiredRoles() {
			if !roleSet[requiredRole] {
				return fmt.Errorf("role=%s: %w", requiredRole, ErrRequiredRoleMissing)
			}
		}
	}

	return nil
}

// detectCycle uses DFS with color marking to detect cycles in dependencies.
// Builds a separate graph from DependsOn (not using runtime DAG).
// Colors: 0=white (unvisited), 1=gray (visiting), 2=black (visited)
func (v *Validator) detectCycle(steps []Step) error {
	// Build adjacency list from DependsOn: depID -> []stepID (forward edges)
	// Edge: depID -> stepID means stepID depends on depID
	adjacency := make(map[string][]string)
	for _, step := range steps {
		if _, exists := adjacency[step.ID]; !exists {
			adjacency[step.ID] = []string{}
		}
	}
	for _, step := range steps {
		for _, depID := range step.DependsOn {
			adjacency[depID] = append(adjacency[depID], step.ID)
		}
	}

	colors := make(map[string]int)
	for _, step := range steps {
		colors[step.ID] = 0 // white
	}

	for _, step := range steps {
		if colors[step.ID] == 0 {
			if v.hasCycle(step.ID, colors, adjacency) {
				return fmt.Errorf("starting from step.id=%s: %w", step.ID, ErrCycleDetected)
			}
		}
	}

	return nil
}

// hasCycle performs DFS to detect cycles.
func (v *Validator) hasCycle(node string, colors map[string]int, adj map[string][]string) bool {
	colors[node] = 1 // gray (visiting)

	for _, next := range adj[node] {
		if colors[next] == 1 { // back edge to gray node
			return true
		}
		if colors[next] == 0 { // white (unvisited)
			if v.hasCycle(next, colors, adj) {
				return true
			}
		}
		// black (visited) - skip
	}

	colors[node] = 2 // black (visited)
	return false
}
