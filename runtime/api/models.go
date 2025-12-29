// Package api provides the HTTP API layer for the runtime sidecar.
package api

import (
	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// ============================================================================
// Request DTOs
// ============================================================================

// StartRunRequest is the request body for POST /api/v1/runs.
type StartRunRequest struct {
	ID     string    `json:"id,omitempty"`
	Policy PolicyDTO `json:"policy"`
	Tasks  []TaskDTO `json:"tasks"`
}

// PolicyDTO represents execution constraints for a run.
type PolicyDTO struct {
	TimeoutMs      int64             `json:"timeout_ms"`
	MaxParallelism int               `json:"max_parallelism"`
	BudgetLimit    CostDTO           `json:"budget_limit"`
	ContextPolicy  *ContextPolicyDTO `json:"context_policy,omitempty"`
}

// ContextPolicyDTO represents context management settings.
type ContextPolicyDTO struct {
	MaxTokens  int64  `json:"max_tokens,omitempty"`
	Strategy   string `json:"strategy,omitempty"`
	KeepLastN  int    `json:"keep_last_n,omitempty"`
	TruncateTo int64  `json:"truncate_to,omitempty"`
}

// TaskDTO represents a task in the request.
type TaskDTO struct {
	ID       string            `json:"id"`
	Prompt   string            `json:"prompt"`
	Model    string            `json:"model"`
	Inputs   map[string]string `json:"inputs,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
	Deps     []string          `json:"deps,omitempty"`
}

// CostDTO represents a monetary cost.
type CostDTO struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// ============================================================================
// Response DTOs
// ============================================================================

// RunResponse is the response body for run-related endpoints.
type RunResponse struct {
	ID        string                   `json:"id"`
	State     string                   `json:"state"`
	Tasks     map[string]TaskStatusDTO `json:"tasks,omitempty"`
	Usage     *UsageDTO                `json:"usage,omitempty"`
	Error     *ErrorDTO                `json:"error,omitempty"`
	CreatedAt int64                    `json:"created_at"`
	UpdatedAt int64                    `json:"updated_at,omitempty"`
}

// TaskStatusDTO represents the status of a single task.
type TaskStatusDTO struct {
	State  string    `json:"state"`
	Output string    `json:"output,omitempty"`
	Error  *ErrorDTO `json:"error,omitempty"`
}

// UsageDTO represents token and cost usage.
type UsageDTO struct {
	Tokens int64    `json:"tokens"`
	Cost   *CostDTO `json:"cost,omitempty"`
}

// ErrorDTO represents an error in the response.
type ErrorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ============================================================================
// Converters: Request DTO → contracts
// ============================================================================

// ToRunPolicy converts PolicyDTO to contracts.RunPolicy.
func (p *PolicyDTO) ToRunPolicy() contracts.RunPolicy {
	policy := contracts.RunPolicy{
		TimeoutMs:      p.TimeoutMs,
		MaxParallelism: p.MaxParallelism,
		BudgetLimit: contracts.Cost{
			Amount:   p.BudgetLimit.Amount,
			Currency: contracts.Currency(p.BudgetLimit.Currency),
		},
	}
	if p.ContextPolicy != nil {
		policy.ContextPolicy = contracts.ContextPolicy{
			MaxTokens:  contracts.TokenCount(p.ContextPolicy.MaxTokens),
			Strategy:   p.ContextPolicy.Strategy,
			KeepLastN:  p.ContextPolicy.KeepLastN,
			TruncateTo: contracts.TokenCount(p.ContextPolicy.TruncateTo),
		}
	}
	return policy
}

// ToTask converts TaskDTO to contracts.Task.
func (t *TaskDTO) ToTask() *contracts.Task {
	task := &contracts.Task{
		ID:    contracts.TaskID(t.ID),
		State: contracts.TaskPending,
		Model: contracts.ModelID(t.Model),
		Inputs: &contracts.TaskInput{
			Prompt:   t.Prompt,
			Inputs:   t.Inputs,
			Metadata: t.Metadata,
		},
	}
	if len(t.Deps) > 0 {
		task.Deps = make([]contracts.TaskID, len(t.Deps))
		for i, dep := range t.Deps {
			task.Deps[i] = contracts.TaskID(dep)
		}
	}
	return task
}

// ============================================================================
// Converters: contracts → Response DTO
// ============================================================================

// RunToResponse converts a contracts.Run to RunResponse.
// The apiState parameter allows overriding the state (e.g., "aborting").
func RunToResponse(run *contracts.Run, apiState string, createdAt, updatedAt int64) *RunResponse {
	state := apiState
	if state == "" {
		state = run.State.String()
	}

	resp := &RunResponse{
		ID:        string(run.ID),
		State:     state,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	// Add task statuses
	if len(run.Tasks) > 0 {
		resp.Tasks = make(map[string]TaskStatusDTO, len(run.Tasks))
		for id, task := range run.Tasks {
			taskDTO := TaskStatusDTO{
				State: task.State.String(),
			}
			if task.Outputs != nil {
				taskDTO.Output = task.Outputs.Output
			}
			if task.Error != nil {
				taskDTO.Error = &ErrorDTO{
					Code:    task.Error.Code,
					Message: task.Error.Message,
				}
			}
			resp.Tasks[string(id)] = taskDTO
		}
	}

	// Add usage
	if run.Usage.Tokens > 0 || run.Usage.Cost.Amount > 0 {
		resp.Usage = &UsageDTO{
			Tokens: int64(run.Usage.Tokens),
			Cost: &CostDTO{
				Amount:   run.Usage.Cost.Amount,
				Currency: string(run.Usage.Cost.Currency),
			},
		}
	}

	return resp
}

// ErrorToResponse converts an error to ErrorDTO with appropriate code.
func ErrorToResponse(err error, code string) *ErrorDTO {
	return &ErrorDTO{
		Code:    code,
		Message: err.Error(),
	}
}

// SnapshotToResponse converts a RunSnapshot to RunResponse.
// This is the thread-safe way to build API responses.
func SnapshotToResponse(snap *RunSnapshot) *RunResponse {
	resp := &RunResponse{
		ID:        string(snap.ID),
		State:     snap.APIState,
		CreatedAt: snap.CreatedAt,
		UpdatedAt: snap.UpdatedAt,
	}

	// Add task statuses
	if len(snap.Tasks) > 0 {
		resp.Tasks = make(map[string]TaskStatusDTO, len(snap.Tasks))
		for id, task := range snap.Tasks {
			taskDTO := TaskStatusDTO{
				State:  task.State.String(),
				Output: task.Output,
			}
			if task.Error != nil {
				taskDTO.Error = &ErrorDTO{
					Code:    task.Error.Code,
					Message: task.Error.Message,
				}
			}
			resp.Tasks[string(id)] = taskDTO
		}
	}

	// Add usage
	if snap.Usage.Tokens > 0 || snap.Usage.Cost.Amount > 0 {
		resp.Usage = &UsageDTO{
			Tokens: int64(snap.Usage.Tokens),
			Cost: &CostDTO{
				Amount:   snap.Usage.Cost.Amount,
				Currency: string(snap.Usage.Cost.Currency),
			},
		}
	}

	// Add error if present
	if snap.Error != nil {
		httpErr := MapError(snap.Error)
		resp.Error = &ErrorDTO{
			Code:    string(httpErr.Code),
			Message: snap.Error.Error(),
		}
	}

	return resp
}
