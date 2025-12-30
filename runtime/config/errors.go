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
)
