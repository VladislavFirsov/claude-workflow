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
	// Custom workflow with different roles - RequireDefaultRoles=false
	v := NewValidatorWithOptions(ValidatorOptions{RequireDefaultRoles: false})
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "custom-flow",
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
	// Custom workflow still validates cycles even with RequireDefaultRoles=false
	v := NewValidatorWithOptions(ValidatorOptions{RequireDefaultRoles: false})
	cfg := &WorkflowConfig{
		Workflow: Workflow{
			Name: "custom-cycle",
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
