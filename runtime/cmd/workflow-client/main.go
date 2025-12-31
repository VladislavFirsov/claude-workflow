// Package main provides a CLI client for the runtime sidecar.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/anthropics/claude-workflow/runtime/config"
)

// roleToModel maps workflow roles to Claude model IDs.
var roleToModel = map[string]string{
	"spec-analyst":   "claude-sonnet-4-20250514",
	"spec-architect": "claude-sonnet-4-20250514",
	"spec-developer": "claude-sonnet-4-20250514",
	"spec-validator": "claude-sonnet-4-20250514",
	"spec-tester":    "claude-sonnet-4-20250514",
	"spec-reviewer":  "claude-sonnet-4-20250514",
}

const defaultModel = "claude-sonnet-4-20250514"

// Default policy values.
const (
	defaultTimeoutMs      int64   = 300000 // 5 minutes
	defaultMaxParallelism int     = 1      // sequential
	defaultBudgetAmount   float64 = 10.0   // $10 USD
	defaultBudgetCurrency string  = "USD"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "submit":
		submitCmd(os.Args[2:])
	case "submit-config":
		submitConfigCmd(os.Args[2:])
	case "status":
		statusCmd(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  workflow-client submit --file <path> --addr <url>
  workflow-client submit-config --file <workflow.json> [--addr <url>] [--run-id <id>]
  workflow-client status --id <run-id> --addr <url>
`)
}

// submitCmd: POST /api/v1/runs
func submitCmd(args []string) {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	file := fs.String("file", "", "JSON file path (StartRunRequest)")
	addr := fs.String("addr", "http://localhost:8080", "Sidecar address")
	fs.Parse(args)

	if *file == "" {
		fmt.Fprintln(os.Stderr, "error: --file is required")
		os.Exit(1)
	}

	// Read JSON file
	data, err := os.ReadFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// POST request
	resp, err := http.Post(*addr+"/api/v1/runs", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		printAPIError(body, resp.StatusCode)
		os.Exit(1)
	}

	// Parse response
	var run runResponse
	if err := json.Unmarshal(body, &run); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s state=%s\n", run.ID, run.State)
}

// submitConfigCmd: convert WorkflowConfig → StartRunRequest and POST /api/v1/runs
func submitConfigCmd(args []string) {
	fs := flag.NewFlagSet("submit-config", flag.ExitOnError)
	file := fs.String("file", "", "Workflow config JSON file path")
	addr := fs.String("addr", "http://localhost:8080", "Sidecar address")
	runID := fs.String("run-id", "", "Override run ID (default: workflow.name)")
	fs.Parse(args)

	if *file == "" {
		fmt.Fprintln(os.Stderr, "error: --file is required")
		os.Exit(1)
	}

	// Load and validate workflow config
	loader := config.NewLoader()
	cfg, err := loader.LoadFromFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Determine run ID
	id := *runID
	if id == "" {
		id = cfg.Workflow.Name
	}

	// Convert to StartRunRequest
	req := convertWorkflowConfig(cfg, id)

	// Marshal to JSON
	data, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// POST request
	resp, err := http.Post(*addr+"/api/v1/runs", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		printAPIError(body, resp.StatusCode)
		os.Exit(1)
	}

	// Parse response
	var run runResponse
	if err := json.Unmarshal(body, &run); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s state=%s\n", run.ID, run.State)
}

// convertWorkflowConfig converts a WorkflowConfig to StartRunRequest.
func convertWorkflowConfig(cfg *config.WorkflowConfig, runID string) *startRunRequest {
	tasks := make([]taskDTO, 0, len(cfg.Workflow.Steps))

	for _, step := range cfg.Workflow.Steps {
		model := getModelForRole(cfg, step.Role)

		// Build metadata
		metadata := map[string]string{
			"role": step.Role,
		}
		if len(step.Outputs) > 0 {
			outputsJSON, _ := json.Marshal(step.Outputs)
			metadata["outputs"] = string(outputsJSON)
		}

		task := taskDTO{
			ID:       step.ID,
			Prompt:   fmt.Sprintf("Execute %s step: %s", step.Role, step.ID),
			Model:    model,
			Deps:     step.DependsOn,
			Metadata: metadata,
		}
		tasks = append(tasks, task)
	}

	// Build policy with defaults
	policy := policyDTO{
		TimeoutMs:      defaultTimeoutMs,
		MaxParallelism: defaultMaxParallelism,
		BudgetLimit: costDTO{
			Amount:   defaultBudgetAmount,
			Currency: defaultBudgetCurrency,
		},
	}

	// Override from config if specified
	if cfg.Workflow.Policy != nil {
		if cfg.Workflow.Policy.TimeoutMs > 0 {
			policy.TimeoutMs = cfg.Workflow.Policy.TimeoutMs
		}
		if cfg.Workflow.Policy.MaxParallelism > 0 {
			policy.MaxParallelism = cfg.Workflow.Policy.MaxParallelism
		}
		if cfg.Workflow.Policy.BudgetLimit != nil {
			if cfg.Workflow.Policy.BudgetLimit.Amount > 0 {
				policy.BudgetLimit.Amount = cfg.Workflow.Policy.BudgetLimit.Amount
			}
			if cfg.Workflow.Policy.BudgetLimit.Currency != "" {
				policy.BudgetLimit.Currency = cfg.Workflow.Policy.BudgetLimit.Currency
			}
		}
	}

	return &startRunRequest{
		ID:     runID,
		Policy: policy,
		Tasks:  tasks,
	}
}

// getModelForRole resolves model for a role with fallback chain:
// 1. cfg.Workflow.Models[role] (config override)
// 2. roleToModel[role] (CLI default)
// 3. defaultModel + warning
func getModelForRole(cfg *config.WorkflowConfig, role string) string {
	// 1. Check config models
	if cfg.Workflow.Models != nil {
		if model, ok := cfg.Workflow.Models[role]; ok {
			return model
		}
	}
	// 2. Check CLI fallback
	if model, ok := roleToModel[role]; ok {
		return model
	}
	// 3. Default + warning
	fmt.Fprintf(os.Stderr, "warning: unknown role %q, using default model\n", role)
	return defaultModel
}

// statusCmd: GET /api/v1/runs/{id}
func statusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	id := fs.String("id", "", "Run ID")
	addr := fs.String("addr", "http://localhost:8080", "Sidecar address")
	fs.Parse(args)

	if *id == "" {
		fmt.Fprintln(os.Stderr, "error: --id is required")
		os.Exit(1)
	}

	// GET request
	resp, err := http.Get(*addr + "/api/v1/runs/" + *id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		printAPIError(body, resp.StatusCode)
		os.Exit(1)
	}

	// Parse response
	var run runResponse
	if err := json.Unmarshal(body, &run); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing response: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("run_id=%s state=%s\n", run.ID, run.State)

	// Print tasks summary (with error codes for failed tasks)
	if len(run.Tasks) > 0 {
		var parts []string
		// Sort for deterministic output
		taskIDs := make([]string, 0, len(run.Tasks))
		for id := range run.Tasks {
			taskIDs = append(taskIDs, id)
		}
		sort.Strings(taskIDs)

		for _, id := range taskIDs {
			task := run.Tasks[id]
			if task.State == "failed" && task.Error != nil {
				parts = append(parts, fmt.Sprintf("%s=failed(%s)", id, task.Error.Code))
			} else {
				parts = append(parts, fmt.Sprintf("%s=%s", id, task.State))
			}
		}
		fmt.Printf("tasks: %s\n", strings.Join(parts, ", "))
	}

	// Print run-level error if present
	if run.Error != nil {
		fmt.Printf("error: [%s] %s\n", run.Error.Code, run.Error.Message)
	}
}

func printAPIError(body []byte, statusCode int) {
	// API returns flat ErrorDTO: {"code":"...","message":"..."}
	var errResp errorDTO
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		fmt.Fprintf(os.Stderr, "error: [%s] %s\n", errResp.Code, errResp.Message)
	} else {
		fmt.Fprintf(os.Stderr, "error: HTTP %d: %s\n", statusCode, string(body))
	}
}

// runResponse mirrors api.RunResponse (minimal fields)
type runResponse struct {
	ID    string                   `json:"id"`
	State string                   `json:"state"`
	Tasks map[string]taskStatusDTO `json:"tasks,omitempty"`
	Error *errorDTO                `json:"error,omitempty"`
}

type taskStatusDTO struct {
	State string    `json:"state"`
	Error *errorDTO `json:"error,omitempty"`
}

type errorDTO struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Request DTOs for submit-config
type startRunRequest struct {
	ID     string    `json:"id,omitempty"`
	Policy policyDTO `json:"policy"`
	Tasks  []taskDTO `json:"tasks"`
}

type policyDTO struct {
	TimeoutMs      int64   `json:"timeout_ms"`
	MaxParallelism int     `json:"max_parallelism"`
	BudgetLimit    costDTO `json:"budget_limit"`
}

type costDTO struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type taskDTO struct {
	ID       string            `json:"id"`
	Prompt   string            `json:"prompt"`
	Model    string            `json:"model"`
	Deps     []string          `json:"deps,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}
