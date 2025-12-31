package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
	ctxpkg "github.com/anthropics/claude-workflow/runtime/internal/context"
	"github.com/anthropics/claude-workflow/runtime/internal/cost"
	"github.com/anthropics/claude-workflow/runtime/internal/orchestration"
)

// maxRequestBodySize limits the size of incoming request bodies (4MB).
const maxRequestBodySize = 4 * 1024 * 1024

// runRetention controls how long completed runs are kept in memory.
const runRetention = time.Hour

// TaskExecutorFunc is the function type for actual task execution.
// Imported from orchestration package for consistency.
type TaskExecutorFunc = orchestration.TaskExecutorFunc

// Handlers contains the HTTP handler methods for the API.
type Handlers struct {
	store    *RunStore
	executor TaskExecutorFunc
	auditDir string // directory for run audit JSON files (empty = disabled)
}

// NewHandlers creates a new Handlers instance.
// auditDir specifies the directory for run audit JSON files (empty = disabled).
func NewHandlers(store *RunStore, executor TaskExecutorFunc, auditDir string) *Handlers {
	return &Handlers{
		store:    store,
		executor: executor,
		auditDir: auditDir,
	}
}

// HandleStartRun handles POST /api/v1/runs.
func (h *Handlers) HandleStartRun(w http.ResponseWriter, r *http.Request) {
	// Parse request body with size limit to prevent memory exhaustion
	limitedReader := io.LimitReader(r.Body, maxRequestBodySize+1)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		WriteError(w, fmt.Errorf("failed to read request body: %w", contracts.ErrInvalidInput))
		return
	}
	if len(body) > maxRequestBodySize {
		WriteError(w, fmt.Errorf("request body too large (max %d bytes): %w", maxRequestBodySize, contracts.ErrInvalidInput))
		return
	}

	var req StartRunRequest
	if err := json.Unmarshal(body, &req); err != nil {
		WriteError(w, fmt.Errorf("invalid JSON: %w", contracts.ErrInvalidInput))
		return
	}

	// Validate required fields
	if err := validateStartRunRequest(&req); err != nil {
		WriteError(w, err)
		return
	}

	// Generate run ID if not provided
	runID := req.ID
	if runID == "" {
		runID = generateRunID()
	}

	// Convert DTOs to contracts
	policy := req.Policy.ToRunPolicy()
	tasks := make([]contracts.Task, len(req.Tasks))
	taskMap := make(map[contracts.TaskID]*contracts.Task, len(req.Tasks))

	for i, taskDTO := range req.Tasks {
		task := taskDTO.ToTask()
		tasks[i] = *task
		taskMap[task.ID] = task
	}

	// Build and validate DAG
	resolver := orchestration.NewDependencyResolver()
	dag, err := resolver.BuildDAG(tasks)
	if err != nil {
		WriteError(w, err)
		return
	}

	// Validate DAG for cycles
	if err := resolver.Validate(dag); err != nil {
		WriteError(w, err)
		return
	}

	// Create Run
	run := &contracts.Run{
		ID:     contracts.RunID(runID),
		State:  contracts.RunPending,
		Policy: policy,
		DAG:    dag,
		Tasks:  taskMap,
		Memory: make(map[string]string),
	}

	// Create cancellable context for the run
	ctx, cancel := context.WithCancel(context.Background())

	// Store the run
	if err := h.store.Create(run, cancel); err != nil {
		cancel() // clean up context
		WriteError(w, err)
		return
	}

	// Best-effort cleanup of old completed runs
	h.store.PruneCompleted(runRetention)

	// Start orchestrator in background
	go h.runOrchestrator(ctx, run)

	// Return 202 Accepted (use snapshot for consistency, though race unlikely here)
	snap, _ := h.store.GetSnapshot(run.ID)
	resp := SnapshotToResponse(snap)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, resp)
}

// HandleGetStatus handles GET /api/v1/runs/{id}.
func (h *Handlers) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		WriteError(w, fmt.Errorf("missing run ID: %w", contracts.ErrInvalidInput))
		return
	}

	// Use GetSnapshot to avoid data races with orchestrator goroutine
	snap, exists := h.store.GetSnapshot(contracts.RunID(runID))
	if !exists {
		WriteError(w, fmt.Errorf("run %s: %w", runID, contracts.ErrRunNotFound))
		return
	}

	resp := SnapshotToResponse(snap)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, resp)
}

// HandleAbort handles POST /api/v1/runs/{id}/abort.
func (h *Handlers) HandleAbort(w http.ResponseWriter, r *http.Request) {
	runID := r.PathValue("id")
	if runID == "" {
		WriteError(w, fmt.Errorf("missing run ID: %w", contracts.ErrInvalidInput))
		return
	}

	if err := h.store.Abort(contracts.RunID(runID)); err != nil {
		WriteError(w, err)
		return
	}

	// Use GetSnapshot to avoid data races
	snap, exists := h.store.GetSnapshot(contracts.RunID(runID))
	if !exists {
		WriteError(w, fmt.Errorf("run %s: %w", runID, contracts.ErrRunNotFound))
		return
	}

	resp := SnapshotToResponse(snap)

	w.Header().Set("Content-Type", "application/json")
	writeJSON(w, resp)
}

// HandleEnqueueTask handles POST /api/v1/runs/{id}/tasks.
// V1: Returns 501 Not Implemented.
func (h *Handlers) HandleEnqueueTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "POST /api/v1/runs")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	writeJSON(w, ErrorDTO{
		Code:    string(CodeNotImplemented),
		Message: "Dynamic task addition not supported in V1. Submit all tasks in StartRun.",
	})
}

// runOrchestrator runs the orchestrator for a run in a goroutine.
//
// RACE SAFETY NOTE:
// The orchestrator modifies run.Tasks and run.State during execution.
// To avoid concurrent reads of run, API handlers read only the shadow state
// maintained by RunStore. The progress callback updates shadow state after each
// successful batch, and MarkDone performs a final sync after the run completes.
func (h *Handlers) runOrchestrator(ctx context.Context, run *contracts.Run) {
	execFn := h.executor
	if execFn == nil {
		execFn = defaultExecutor
	}

	// Mark run as running in shadow state
	h.store.SetShadowRunState(run.ID, contracts.RunRunning)
	h.store.UpdateTimestamp(run.ID)

	// Progress callback: sync shadow after each successful batch merge
	onProgress := func(run *contracts.Run) {
		h.store.UpdateShadowState(run.ID)
	}

	deps := orchestration.OrchestratorDeps{
		Scheduler:      orchestration.NewScheduler(),
		DepResolver:    orchestration.NewDependencyResolver(),
		Queue:          orchestration.NewQueueManager(),
		Executor:       orchestration.NewParallelExecutorFromPolicy(run.Policy, execFn),
		ContextBuilder: ctxpkg.NewContextBuilder(),
		Compactor:      ctxpkg.NewContextCompactor(),
		TokenEstimator: cost.NewTokenEstimator(),
		CostCalc:       cost.NewCostCalculator(),
		BudgetEnforcer: cost.NewBudgetEnforcer(),
		UsageTracker:   cost.NewUsageTracker(),
		Router:         ctxpkg.NewContextRouter(),
	}

	// Create orchestrator with progress callback
	orch := orchestration.NewOrchestratorWithCallback(deps, onProgress)
	err := orch.Run(ctx, run)
	h.store.MarkDone(run.ID, err)

	// Write audit file if configured
	if h.auditDir != "" {
		h.writeAuditFile(run.ID)
	}
}

// writeAuditFile writes the run audit to a JSON file in the configured audit directory.
func (h *Handlers) writeAuditFile(runID contracts.RunID) {
	snap, exists := h.store.GetSnapshot(runID)
	if !exists {
		log.Printf("[AUDIT] warning: cannot write audit file, run %s not found", runID)
		return
	}

	resp := SnapshotToResponse(snap)
	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		log.Printf("[AUDIT] error: failed to marshal audit JSON for run %s: %v", runID, err)
		return
	}

	filename := filepath.Join(h.auditDir, fmt.Sprintf("run-%s.json", runID))
	if err := os.MkdirAll(h.auditDir, 0755); err != nil {
		log.Printf("[AUDIT] error: failed to create audit dir %s: %v", h.auditDir, err)
		return
	}
	if err := os.WriteFile(filename, data, 0644); err != nil {
		log.Printf("[AUDIT] error: failed to write audit file %s: %v", filename, err)
		return
	}

	log.Printf("[AUDIT] event=audit_file_written run_id=%s path=%s", runID, filename)
}

// defaultExecutor is a fallback TaskExecutorFunc when none is provided.
// It returns a minimal successful result with non-zero usage.
func defaultExecutor(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	return &contracts.TaskResult{
		Output: fmt.Sprintf("executed:%s", task.ID),
		Usage: contracts.Usage{
			Tokens: 100,
			Cost:   contracts.Cost{Amount: 0.001, Currency: "USD"},
		},
	}, nil
}

// validateStartRunRequest validates a StartRunRequest.
func validateStartRunRequest(req *StartRunRequest) error {
	// Policy is required
	if req.Policy.MaxParallelism <= 0 {
		return fmt.Errorf("policy.max_parallelism must be > 0: %w", contracts.ErrInvalidInput)
	}

	// Budget must be positive
	if req.Policy.BudgetLimit.Amount <= 0 {
		return fmt.Errorf("policy.budget_limit.amount must be > 0: %w", contracts.ErrInvalidInput)
	}

	// At least one task required
	if len(req.Tasks) == 0 {
		return fmt.Errorf("at least one task is required: %w", contracts.ErrInvalidInput)
	}

	// Validate each task
	taskIDs := make(map[string]bool)
	for _, task := range req.Tasks {
		if task.ID == "" {
			return fmt.Errorf("task.id is required: %w", contracts.ErrInvalidInput)
		}
		if taskIDs[task.ID] {
			return fmt.Errorf("duplicate task.id: %s: %w", task.ID, contracts.ErrInvalidInput)
		}
		taskIDs[task.ID] = true

		if task.Prompt == "" {
			return fmt.Errorf("task %s: prompt is required: %w", task.ID, contracts.ErrInvalidInput)
		}

		// Model is required (prevents ErrModelUnknown at runtime → 500)
		if task.Model == "" {
			return fmt.Errorf("task %s: model is required: %w", task.ID, contracts.ErrInvalidInput)
		}
	}

	return nil
}

// generateRunID generates a unique run ID.
func generateRunID() string {
	return fmt.Sprintf("run-%d", timeNowFunc().UnixNano())
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, v interface{}) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Log error but can't write to response at this point
		_ = err
	}
}

// timeNowFunc is a variable for testing time-dependent code.
var timeNowFunc = time.Now
