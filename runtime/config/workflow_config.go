// Package config provides static workflow configuration loading and validation.
package config

// WorkflowConfig represents the root configuration structure.
type WorkflowConfig struct {
	Workflow Workflow `json:"workflow"`
}

// Workflow defines a named workflow with a list of steps.
type Workflow struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
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

// Required roles for the default spec workflow.
const (
	RoleSpecAnalyst   Role = "spec-analyst"
	RoleSpecArchitect Role = "spec-architect"
	RoleSpecDeveloper Role = "spec-developer"
	RoleSpecValidator Role = "spec-validator"
)

// RequiredRoles returns the list of roles that must be present in a valid workflow.
func RequiredRoles() []Role {
	return []Role{
		RoleSpecAnalyst,
		RoleSpecArchitect,
		RoleSpecDeveloper,
		RoleSpecValidator,
	}
}
