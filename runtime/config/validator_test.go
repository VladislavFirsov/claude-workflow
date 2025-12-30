package config

import (
	"errors"
	"testing"
)

func TestValidator_NilConfig(t *testing.T) {
	v := NewValidator()
	err := v.Validate(nil)
	if !errors.Is(err, ErrConfigEmpty) {
		t.Fatalf("expected ErrConfigEmpty, got %v", err)
	}
}

func TestValidator_WorkflowNameEmpty(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:  "",
			Steps: []Step{{ID: "a", Role: "spec-analyst"}},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrWorkflowNameEmpty) {
		t.Fatalf("expected ErrWorkflowNameEmpty, got %v", err)
	}
}

func TestValidator_NoSteps(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:  "test",
			Steps: []Step{},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrNoSteps) {
		t.Fatalf("expected ErrNoSteps, got %v", err)
	}
}

func TestValidator_StepIDEmpty(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "", Role: "spec-analyst"},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrStepIDEmpty) {
		t.Fatalf("expected ErrStepIDEmpty, got %v", err)
	}
}

func TestValidator_DuplicateStepID(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst"},
				{ID: "a", Role: "spec-architect"},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrStepIDDuplicate) {
		t.Fatalf("expected ErrStepIDDuplicate, got %v", err)
	}
}

func TestValidator_StepRoleEmpty(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: ""},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrStepRoleEmpty) {
		t.Fatalf("expected ErrStepRoleEmpty, got %v", err)
	}
}

func TestValidator_DependencyNotFound(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst", DependsOn: []string{"nonexistent"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrDependencyNotFound) {
		t.Fatalf("expected ErrDependencyNotFound, got %v", err)
	}
}

func TestValidator_CycleDetected_SelfReference(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst", DependsOn: []string{"a"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestValidator_CycleDetected_TwoNodes(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst", DependsOn: []string{"b"}},
				{ID: "b", Role: "spec-architect", DependsOn: []string{"a"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestValidator_CycleDetected_ThreeNodes(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst", DependsOn: []string{"c"}},
				{ID: "b", Role: "spec-architect", DependsOn: []string{"a"}},
				{ID: "c", Role: "spec-developer", DependsOn: []string{"b"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

func TestValidator_RequiredRoleMissing(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "test",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst"},
				{ID: "b", Role: "spec-architect"},
				// Missing spec-developer and spec-validator
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrRequiredRoleMissing) {
		t.Fatalf("expected ErrRequiredRoleMissing, got %v", err)
	}
}

func TestValidator_ValidConfig_LinearChain(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "default-spec-flow",
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst", Outputs: []string{"requirements.md"}},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_ValidConfig_DAGDiamond(t *testing.T) {
	v := NewValidator()
	// Diamond pattern: a -> (b, c) -> d
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "dag-flow",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst"},
				{ID: "b", Role: "spec-architect", DependsOn: []string{"a"}},
				{ID: "c", Role: "spec-developer", DependsOn: []string{"a"}},
				{ID: "d", Role: "spec-validator", DependsOn: []string{"b", "c"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for DAG diamond, got %v", err)
	}
}

func TestValidator_ValidConfig_NoDependencies(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "parallel-flow",
			Steps: []Step{
				{ID: "a", Role: "spec-analyst"},
				{ID: "b", Role: "spec-architect"},
				{ID: "c", Role: "spec-developer"},
				{ID: "d", Role: "spec-validator"},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for parallel steps, got %v", err)
	}
}

func TestValidator_ValidConfig_WithOutputs(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "output-flow",
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst", Outputs: []string{"requirements.md", "user-stories.md"}},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}, Outputs: []string{"architecture.md", "api-spec.md"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}, Outputs: []string{"src/", "tests/"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}, Outputs: []string{"validation-report.md"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_CustomWorkflow_WithoutRequiredRoles(t *testing.T) {
	// Custom workflow with different roles - type=custom skips required role check
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "custom-flow",
			Type: WorkflowTypeCustom,
			Steps: []Step{
				{ID: "fetch", Role: "data-fetcher"},
				{ID: "process", Role: "data-processor", DependsOn: []string{"fetch"}},
				{ID: "store", Role: "data-writer", DependsOn: []string{"process"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error for custom workflow, got %v", err)
	}
}

func TestValidator_CustomWorkflow_StillValidatesStructure(t *testing.T) {
	// Custom workflow still validates cycles
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "custom-cycle",
			Type: WorkflowTypeCustom,
			Steps: []Step{
				{ID: "a", Role: "custom-role", DependsOn: []string{"b"}},
				{ID: "b", Role: "custom-role", DependsOn: []string{"a"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrCycleDetected) {
		t.Fatalf("expected ErrCycleDetected, got %v", err)
	}
}

// ============ spec-default validation tests ============

func TestValidator_SpecDefault_ValidCanonical(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "default-spec-flow",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_SpecDefault_ValidWithOptional(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "spec-flow-with-tester",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "testing", Role: "spec-tester", DependsOn: []string{"validation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_SpecDefault_MissingRequiredRole(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "missing-role",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				// Missing spec-developer and spec-validator
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrRequiredRoleMissing) {
		t.Fatalf("expected ErrRequiredRoleMissing, got %v", err)
	}
}

func TestValidator_SpecDefault_DuplicateRequiredRole(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "duplicate-role",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis1", Role: "spec-analyst"},
				{ID: "analysis2", Role: "spec-analyst", DependsOn: []string{"analysis1"}},
				{ID: "arch", Role: "spec-architect", DependsOn: []string{"analysis2"}},
				{ID: "dev", Role: "spec-developer", DependsOn: []string{"arch"}},
				{ID: "val", Role: "spec-validator", DependsOn: []string{"dev"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrRequiredRoleDuplicate) {
		t.Fatalf("expected ErrRequiredRoleDuplicate, got %v", err)
	}
}

func TestValidator_SpecDefault_WrongOrder(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "wrong-order",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "arch", Role: "spec-architect"},
				{ID: "analysis", Role: "spec-analyst", DependsOn: []string{"arch"}},
				{ID: "dev", Role: "spec-developer", DependsOn: []string{"analysis"}},
				{ID: "val", Role: "spec-validator", DependsOn: []string{"dev"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrRequiredRoleOrder) {
		t.Fatalf("expected ErrRequiredRoleOrder, got %v", err)
	}
}

func TestValidator_SpecDefault_InvalidDependencyChain(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "broken-chain",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect"}, // Missing depends_on analysis
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrInvalidDependencyChain) {
		t.Fatalf("expected ErrInvalidDependencyChain, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalInMiddle(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "optional-in-middle",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "review", Role: "spec-reviewer", DependsOn: []string{"analysis"}}, // Wrong placement
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "dev", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "val", Role: "spec-validator", DependsOn: []string{"dev"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrOptionalRolePlacement) {
		t.Fatalf("expected ErrOptionalRolePlacement, got %v", err)
	}
}

func TestValidator_SpecDefault_UnknownRole(t *testing.T) {
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "unknown-role",
			Type: WorkflowTypeSpecDefault,
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "custom", Role: "my-custom-agent", DependsOn: []string{"validation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestValidator_EmptyType_RequiredRolesPresent(t *testing.T) {
	// type="" with required roles present - should pass
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "empty-type-valid",
			// Type is empty (default)
			Steps: []Step{
				{ID: "a", Role: "spec-analyst"},
				{ID: "b", Role: "spec-architect"},
				{ID: "c", Role: "spec-developer"},
				{ID: "d", Role: "spec-validator"},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_EmptyType_MissingRequiredRole(t *testing.T) {
	// type="" with missing required role - should fail
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "empty-type-missing",
			// Type is empty (default)
			Steps: []Step{
				{ID: "a", Role: "spec-analyst"},
				{ID: "b", Role: "spec-architect"},
				// Missing spec-developer and spec-validator
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrRequiredRoleMissing) {
		t.Fatalf("expected ErrRequiredRoleMissing, got %v", err)
	}
}

// ============ optional_roles and optional_enabled tests ============

func TestValidator_SpecDefault_CustomOptionalRoles(t *testing.T) {
	// Custom optional_roles allows new roles
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:          "custom-optional",
			Type:          WorkflowTypeSpecDefault,
			OptionalRoles: []string{"spec-qa", "spec-security"},
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "qa", Role: "spec-qa", DependsOn: []string{"validation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_SpecDefault_CustomOptionalRoles_RejectsDefault(t *testing.T) {
	// Custom optional_roles replaces default - spec-tester no longer allowed
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:          "custom-optional-reject",
			Type:          WorkflowTypeSpecDefault,
			OptionalRoles: []string{"spec-qa"}, // Only spec-qa allowed
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "testing", Role: "spec-tester", DependsOn: []string{"validation"}}, // Not in optional_roles
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalEnabledNotInOptionalRoles(t *testing.T) {
	// optional_enabled contains role not in optional_roles
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:            "invalid-optional-enabled",
			Type:            WorkflowTypeSpecDefault,
			OptionalRoles:   []string{"spec-qa"},
			OptionalEnabled: []string{"spec-security"}, // Not in optional_roles
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrOptionalNotAllowed) {
		t.Fatalf("expected ErrOptionalNotAllowed, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalEnabledSubset(t *testing.T) {
	// optional_enabled is subset of optional_roles - only enabled roles allowed
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:            "optional-enabled-subset",
			Type:            WorkflowTypeSpecDefault,
			OptionalRoles:   []string{"spec-reviewer", "spec-tester", "spec-qa"},
			OptionalEnabled: []string{"spec-reviewer"}, // Only reviewer enabled
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "review", Role: "spec-reviewer", DependsOn: []string{"validation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalEnabledRejectsNonEnabled(t *testing.T) {
	// optional_enabled restricts which optional roles can be used
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:            "optional-enabled-rejects",
			Type:            WorkflowTypeSpecDefault,
			OptionalRoles:   []string{"spec-reviewer", "spec-tester"},
			OptionalEnabled: []string{"spec-reviewer"}, // Only reviewer enabled
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "testing", Role: "spec-tester", DependsOn: []string{"validation"}}, // Not enabled
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalEnabledWithDefaultRoles(t *testing.T) {
	// optional_enabled with default optional_roles (empty optional_roles)
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:            "optional-enabled-default",
			Type:            WorkflowTypeSpecDefault,
			OptionalEnabled: []string{"spec-reviewer"}, // Enable only reviewer from defaults
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "review", Role: "spec-reviewer", DependsOn: []string{"validation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalEnabledRejectsNonEnabledDefault(t *testing.T) {
	// optional_enabled with default optional_roles - rejects non-enabled
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:            "optional-enabled-default-reject",
			Type:            WorkflowTypeSpecDefault,
			OptionalEnabled: []string{"spec-reviewer"}, // Enable only reviewer
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
				{ID: "testing", Role: "spec-tester", DependsOn: []string{"validation"}}, // Not enabled
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrUnknownRole) {
		t.Fatalf("expected ErrUnknownRole, got %v", err)
	}
}

func TestValidator_SpecDefault_OptionalRoleMustDependOnValidator(t *testing.T) {
	// Even with custom optional_roles, placement rule applies
	v := NewValidator()
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name:          "optional-placement",
			Type:          WorkflowTypeSpecDefault,
			OptionalRoles: []string{"spec-qa"},
			Steps: []Step{
				{ID: "analysis", Role: "spec-analyst"},
				{ID: "architecture", Role: "spec-architect", DependsOn: []string{"analysis"}},
				{ID: "qa", Role: "spec-qa", DependsOn: []string{"architecture"}}, // Wrong - should depend on validator
				{ID: "implementation", Role: "spec-developer", DependsOn: []string{"architecture"}},
				{ID: "validation", Role: "spec-validator", DependsOn: []string{"implementation"}},
			},
		},
	}
	err := v.Validate(cfg)
	if !errors.Is(err, ErrOptionalRolePlacement) {
		t.Fatalf("expected ErrOptionalRolePlacement, got %v", err)
	}
}
