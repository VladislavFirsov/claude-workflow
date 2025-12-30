package config

import "fmt"

// Validator validates workflow configurations.
type Validator struct{}

// NewValidator creates a new configuration validator.
func NewValidator() *Validator {
	return &Validator{}
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

	// 6. Type-based validation dispatch
	switch cfg.Workflow.Type {
	case WorkflowTypeSpecDefault:
		// Strict canonical validation
		return v.validateSpecDefault(cfg.Workflow.Steps, roleSet)
	case WorkflowTypeCustom:
		// Skip required role checking entirely
		return nil
	default:
		// type == "" (empty): current behavior - required roles must be present
		return v.validateRequiredRolesPresent(roleSet)
	}
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

// validateRequiredRolesPresent checks that all required roles are present (no order).
// Used for type == "" (empty) backwards compatibility.
func (v *Validator) validateRequiredRolesPresent(roleSet map[Role]bool) error {
	for _, requiredRole := range RequiredRoles() {
		if !roleSet[requiredRole] {
			return fmt.Errorf("role=%s: %w", requiredRole, ErrRequiredRoleMissing)
		}
	}
	return nil
}

// validateSpecDefault performs strict canonical validation for spec-default workflow.
func (v *Validator) validateSpecDefault(steps []Step, roleSet map[Role]bool) error {
	requiredRoles := RequiredRoles()
	optionalRoles := OptionalRoles()

	// Build lookup sets
	requiredSet := make(map[Role]bool)
	for _, r := range requiredRoles {
		requiredSet[r] = true
	}
	optionalSet := make(map[Role]bool)
	for _, r := range optionalRoles {
		optionalSet[r] = true
	}

	// 1. Check all roles are either required or optional
	for _, step := range steps {
		role := Role(step.Role)
		if !requiredSet[role] && !optionalSet[role] {
			return fmt.Errorf("step.id=%s role=%s: %w", step.ID, step.Role, ErrUnknownRole)
		}
	}

	// 2. Check required roles are present exactly once
	roleCounts := make(map[Role]int)
	for _, step := range steps {
		role := Role(step.Role)
		if requiredSet[role] {
			roleCounts[role]++
		}
	}
	for _, reqRole := range requiredRoles {
		count := roleCounts[reqRole]
		if count == 0 {
			return fmt.Errorf("role=%s: %w", reqRole, ErrRequiredRoleMissing)
		}
		if count > 1 {
			return fmt.Errorf("role=%s: %w", reqRole, ErrRequiredRoleDuplicate)
		}
	}

	// 3. Check required roles are in canonical order
	// Find steps with required roles and check their order matches
	requiredSteps := make([]Step, 0, len(requiredRoles))
	for _, step := range steps {
		role := Role(step.Role)
		if requiredSet[role] {
			requiredSteps = append(requiredSteps, step)
		}
	}

	// Check order matches canonical order
	for i, step := range requiredSteps {
		expectedRole := requiredRoles[i]
		actualRole := Role(step.Role)
		if actualRole != expectedRole {
			return fmt.Errorf("step.id=%s: expected role=%s at position %d, got %s: %w",
				step.ID, expectedRole, i, actualRole, ErrRequiredRoleOrder)
		}
	}

	// 4. Check dependency chain for required steps
	// Each required step (except first) must depend on the previous required step
	stepByRole := make(map[Role]Step)
	for _, step := range steps {
		role := Role(step.Role)
		if requiredSet[role] {
			stepByRole[role] = step
		}
	}

	for i := 1; i < len(requiredRoles); i++ {
		currentRole := requiredRoles[i]
		prevRole := requiredRoles[i-1]
		currentStep := stepByRole[currentRole]
		prevStep := stepByRole[prevRole]

		// Check that current step depends on previous step
		dependsOnPrev := false
		for _, depID := range currentStep.DependsOn {
			if depID == prevStep.ID {
				dependsOnPrev = true
				break
			}
		}
		if !dependsOnPrev {
			return fmt.Errorf("step.id=%s (role=%s) must depend on step.id=%s (role=%s): %w",
				currentStep.ID, currentRole, prevStep.ID, prevRole, ErrInvalidDependencyChain)
		}
	}

	// 5. Check optional roles depend only on spec-validator
	validatorStep := stepByRole[RoleSpecValidator]
	for _, step := range steps {
		role := Role(step.Role)
		if optionalSet[role] {
			// Optional step must depend on validator
			dependsOnValidator := false
			for _, depID := range step.DependsOn {
				if depID == validatorStep.ID {
					dependsOnValidator = true
					break
				}
			}
			if !dependsOnValidator {
				return fmt.Errorf("step.id=%s (role=%s) must depend on %s: %w",
					step.ID, step.Role, validatorStep.ID, ErrOptionalRolePlacement)
			}
		}
	}

	return nil
}
