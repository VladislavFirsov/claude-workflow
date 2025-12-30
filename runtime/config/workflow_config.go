// Package config provides static workflow configuration loading and validation.
package config

// WorkflowConfig represents the root configuration structure.
type WorkflowConfig struct {
	Workflow Workflow `json:"workflow"`
}

// WorkflowType defines the type of workflow for validation purposes.
type WorkflowType string

const (
	// WorkflowTypeSpecDefault enables strict canonical validation.
	WorkflowTypeSpecDefault WorkflowType = "spec-default"
	// WorkflowTypeCustom disables required role checking.
	WorkflowTypeCustom WorkflowType = "custom"
)

// Workflow defines a named workflow with a list of steps.
type Workflow struct {
	Name  string       `json:"name"`
	Type  WorkflowType `json:"type,omitempty"`
	Steps []Step       `json:"steps"`
}

// Step defines a single step in the workflow.
type Step struct {
	ID        string   `json:"id"`
	Role      string   `json:"role"`
	DependsOn []string `json:"depends_on,omitempty"`
	Outputs   []string `json:"outputs,omitempty"`
}

// Role represents an agent role identifier.
type Role string

// Required roles for the default spec workflow (in canonical order).
const (
	RoleSpecAnalyst   Role = "spec-analyst"
	RoleSpecArchitect Role = "spec-architect"
	RoleSpecDeveloper Role = "spec-developer"
	RoleSpecValidator Role = "spec-validator"
)

// Optional roles for the spec-default workflow.
const (
	RoleSpecTester   Role = "spec-tester"
	RoleSpecReviewer Role = "spec-reviewer"
)

// RequiredRoles returns the list of roles that must be present in a valid workflow.
// The order is canonical for spec-default validation.
func RequiredRoles() []Role {
	return []Role{
		RoleSpecAnalyst,
		RoleSpecArchitect,
		RoleSpecDeveloper,
		RoleSpecValidator,
	}
}

// OptionalRoles returns the list of optional roles allowed in spec-default workflow.
func OptionalRoles() []Role {
	return []Role{
		RoleSpecTester,
		RoleSpecReviewer,
	}
}
