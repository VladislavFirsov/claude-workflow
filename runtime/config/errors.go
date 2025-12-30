package config

import "errors"

// Sentinel errors for workflow configuration validation.
var (
	// ErrConfigEmpty is returned when the config data is empty (zero bytes).
	ErrConfigEmpty = errors.New("workflow configuration is empty")

	// ErrWorkflowNameEmpty is returned when workflow.name is empty.
	ErrWorkflowNameEmpty = errors.New("workflow.name is required")

	// ErrNoSteps is returned when workflow.steps is empty.
	ErrNoSteps = errors.New("workflow.steps must not be empty")

	// ErrStepIDEmpty is returned when a step has an empty id.
	ErrStepIDEmpty = errors.New("step.id is required")

	// ErrStepIDDuplicate is returned when two steps have the same id.
	ErrStepIDDuplicate = errors.New("duplicate step.id")

	// ErrStepRoleEmpty is returned when a step has an empty role.
	ErrStepRoleEmpty = errors.New("step.role is required")

	// ErrDependencyNotFound is returned when depends_on references a non-existent id.
	ErrDependencyNotFound = errors.New("depends_on references unknown step id")

	// ErrCycleDetected is returned when a cycle is detected in step dependencies.
	ErrCycleDetected = errors.New("cycle detected in step dependencies")

	// ErrRequiredRoleMissing is returned when a required role is not present.
	ErrRequiredRoleMissing = errors.New("required role is missing")

	// ErrRequiredRoleOrder is returned when required roles are not in canonical order.
	ErrRequiredRoleOrder = errors.New("required roles must be in canonical order")

	// ErrRequiredRoleDuplicate is returned when a required role appears more than once.
	ErrRequiredRoleDuplicate = errors.New("required role appears more than once")

	// ErrOptionalRolePlacement is returned when optional role depends on non-validator step.
	ErrOptionalRolePlacement = errors.New("optional role must depend on spec-validator")

	// ErrUnknownRole is returned when a role is neither required nor optional for spec-default.
	ErrUnknownRole = errors.New("unknown role for spec-default workflow")

	// ErrInvalidDependencyChain is returned when required steps don't form proper chain.
	ErrInvalidDependencyChain = errors.New("required step must depend on previous required step")

	// ErrOptionalNotAllowed is returned when optional_enabled contains a role not in optional_roles.
	ErrOptionalNotAllowed = errors.New("optional_enabled contains role not in optional_roles")
)
