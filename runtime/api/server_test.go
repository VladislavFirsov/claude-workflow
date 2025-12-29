package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// ============================================================================
// RunStore Tests
// ============================================================================

func TestRunStore_CreateGet(t *testing.T) {
	store := NewRunStore()

	run := &contracts.Run{
		ID:    "test-run-1",
		State: contracts.RunPending,
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create
	err := store.Create(run, cancel)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Get
	entry, exists := store.Get("test-run-1")
	if !exists {
		t.Fatal("expected run to exist")
	}
	if entry.Run.ID != "test-run-1" {
		t.Errorf("expected ID 'test-run-1', got '%s'", entry.Run.ID)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// Get non-existent
	_, exists = store.Get("non-existent")
	if exists {
		t.Error("expected non-existent run to not exist")
	}
}

func TestRunStore_CreateDuplicateID(t *testing.T) {
	store := NewRunStore()

	run := &contracts.Run{ID: "dup-1", State: contracts.RunPending}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := store.Create(run, cancel)
	if err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	// Try to create duplicate
	err = store.Create(run, cancel)
	if err == nil {
		t.Fatal("expected error for duplicate ID")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' error, got: %v", err)
	}
}

func TestRunStore_Abort(t *testing.T) {
	store := NewRunStore()

	run := &contracts.Run{ID: "abort-1", State: contracts.RunRunning}
	ctx, cancel := context.WithCancel(context.Background())

	err := store.Create(run, cancel)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Abort
	err = store.Abort("abort-1")
	if err != nil {
		t.Fatalf("Abort failed: %v", err)
	}

	// Verify aborting state
	if !store.IsAborting("abort-1") {
		t.Error("expected IsAborting to return true")
	}

	// Verify context was cancelled
	select {
	case <-ctx.Done():
		// expected
	default:
		t.Error("expected context to be cancelled")
	}

	// Abort non-existent
	err = store.Abort("non-existent")
	if err == nil {
		t.Error("expected error for non-existent run")
	}
}

func TestRunStore_AbortCompleted(t *testing.T) {
	store := NewRunStore()

	run := &contracts.Run{ID: "abort-2", State: contracts.RunCompleted}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := store.Create(run, cancel)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Abort completed run
	err = store.Abort("abort-2")
	if err == nil {
		t.Error("expected error for completed run")
	}
}

func TestRunStore_UpdateTimestamp(t *testing.T) {
	store := NewRunStore()

	run := &contracts.Run{ID: "ts-1", State: contracts.RunRunning}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := store.Create(run, cancel)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	_, created := store.GetTimestamps("ts-1")

	// Wait a bit and mark done
	time.Sleep(10 * time.Millisecond)
	store.MarkDone("ts-1", nil)

	_, updated := store.GetTimestamps("ts-1")

	if updated <= created {
		t.Errorf("expected UpdatedAt > CreatedAt, got created=%d, updated=%d", created, updated)
	}
}

// ============================================================================
// Handler Tests
// ============================================================================

func TestHandleStartRun_Success(t *testing.T) {
	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		return &contracts.TaskResult{
			Output: "ok:" + string(task.ID),
			Usage:  contracts.Usage{Tokens: 100, Cost: contracts.Cost{Amount: 0.001, Currency: "USD"}},
		}, nil
	}

	server := NewServer(":0", executor)

	reqBody := `{
		"id": "test-run",
		"policy": {
			"timeout_ms": 30000,
			"max_parallelism": 2,
			"budget_limit": {"amount": 1.0, "currency": "USD"}
		},
		"tasks": [
			{"id": "A", "prompt": "Hello", "model": "claude-3-haiku-20240307"}
		]
	}`

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d: %s", w.Code, w.Body.String())
	}

	var resp RunResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != "test-run" {
		t.Errorf("expected ID 'test-run', got '%s'", resp.ID)
	}
}

func TestHandleStartRun_InvalidJSON(t *testing.T) {
	server := NewServer(":0", nil)

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString("{invalid json"))
	w := httptest.NewRecorder()

	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestHandleStartRun_DAGCycle(t *testing.T) {
	server := NewServer(":0", nil)

	reqBody := `{
		"policy": {"max_parallelism": 1, "budget_limit": {"amount": 1.0, "currency": "USD"}},
		"tasks": [
			{"id": "A", "prompt": "Hello", "model": "claude-3-haiku-20240307", "deps": ["B"]},
			{"id": "B", "prompt": "World", "model": "claude-3-haiku-20240307", "deps": ["A"]}
		]
	}`

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()

	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStartRun_DuplicateID(t *testing.T) {
	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		// Slow executor to keep run active
		time.Sleep(100 * time.Millisecond)
		return &contracts.TaskResult{Output: "ok"}, nil
	}

	server := NewServer(":0", executor)

	reqBody := `{
		"id": "dup-run",
		"policy": {"max_parallelism": 1, "budget_limit": {"amount": 1.0, "currency": "USD"}},
		"tasks": [{"id": "A", "prompt": "Hello", "model": "claude-3-haiku-20240307"}]
	}`

	// First request
	req1 := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w1 := httptest.NewRecorder()
	server.Handlers().HandleStartRun(w1, req1)

	if w1.Code != http.StatusAccepted {
		t.Fatalf("first request failed: %d", w1.Code)
	}

	// Second request with same ID
	req2 := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w2 := httptest.NewRecorder()
	server.Handlers().HandleStartRun(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestHandleGetStatus_NotFound(t *testing.T) {
	server := NewServer(":0", nil)

	req := httptest.NewRequest("GET", "/api/v1/runs/non-existent", nil)
	req.SetPathValue("id", "non-existent")
	w := httptest.NewRecorder()

	server.Handlers().HandleGetStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestHandleAbort_AlreadyCompleted(t *testing.T) {
	server := NewServer(":0", nil)

	// Create a completed run directly
	run := &contracts.Run{ID: "completed-run", State: contracts.RunCompleted}
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	server.Store().Create(run, cancel)

	req := httptest.NewRequest("POST", "/api/v1/runs/completed-run/abort", nil)
	req.SetPathValue("id", "completed-run")
	w := httptest.NewRecorder()

	server.Handlers().HandleAbort(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStartRun_MissingModel(t *testing.T) {
	server := NewServer(":0", nil)

	reqBody := `{
		"policy": {"max_parallelism": 1, "budget_limit": {"amount": 1.0, "currency": "USD"}},
		"tasks": [{"id": "A", "prompt": "Hello"}]
	}`

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()

	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStartRun_ZeroBudget(t *testing.T) {
	server := NewServer(":0", nil)

	reqBody := `{
		"policy": {"max_parallelism": 1, "budget_limit": {"amount": 0, "currency": "USD"}},
		"tasks": [{"id": "A", "prompt": "Hello", "model": "claude-3-haiku-20240307"}]
	}`

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()

	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRunStore_GetSnapshot(t *testing.T) {
	store := NewRunStore()

	run := &contracts.Run{
		ID:    "snap-1",
		State: contracts.RunRunning,
		Tasks: map[contracts.TaskID]*contracts.Task{
			"A": {
				ID:    "A",
				State: contracts.TaskCompleted,
				Outputs: &contracts.TaskResult{
					Output: "result-A",
				},
			},
		},
		Usage: contracts.Usage{Tokens: 100},
	}

	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := store.Create(run, cancel)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	snap, exists := store.GetSnapshot("snap-1")
	if !exists {
		t.Fatal("expected snapshot to exist")
	}

	if snap.APIState != "running" {
		t.Errorf("expected state 'running', got '%s'", snap.APIState)
	}

	if snap.Tasks["A"].Output != "result-A" {
		t.Errorf("expected task A output 'result-A', got '%s'", snap.Tasks["A"].Output)
	}
}

func TestHandleEnqueueTask_NotImplemented(t *testing.T) {
	server := NewServer(":0", nil)

	req := httptest.NewRequest("POST", "/api/v1/runs/any/tasks", nil)
	req.SetPathValue("id", "any")
	w := httptest.NewRecorder()

	server.Handlers().HandleEnqueueTask(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("expected status 501, got %d", w.Code)
	}

	// Check Allow header
	allow := w.Header().Get("Allow")
	if allow != "POST /api/v1/runs" {
		t.Errorf("expected Allow header 'POST /api/v1/runs', got '%s'", allow)
	}
}

// ============================================================================
// Integration Tests
// ============================================================================

func TestServer_FullCycle(t *testing.T) {
	completed := make(chan struct{})

	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		return &contracts.TaskResult{
			Output: "result:" + string(task.ID),
			Usage:  contracts.Usage{Tokens: 50, Cost: contracts.Cost{Amount: 0.0001, Currency: "USD"}},
		}, nil
	}

	server := NewServer(":0", executor)

	// 1. Start run
	reqBody := `{
		"id": "full-cycle",
		"policy": {"max_parallelism": 1, "budget_limit": {"amount": 1.0, "currency": "USD"}},
		"tasks": [
			{"id": "A", "prompt": "Test", "model": "claude-3-haiku-20240307"}
		]
	}`

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()
	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("StartRun failed: %d - %s", w.Code, w.Body.String())
	}

	// 2. Poll GetStatus until complete
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)

			req := httptest.NewRequest("GET", "/api/v1/runs/full-cycle", nil)
			req.SetPathValue("id", "full-cycle")
			w := httptest.NewRecorder()
			server.Handlers().HandleGetStatus(w, req)

			var resp RunResponse
			json.NewDecoder(w.Body).Decode(&resp)

			if resp.State == "completed" {
				close(completed)
				return
			}
		}
	}()

	select {
	case <-completed:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for run to complete")
	}

	// 3. Verify final state
	req = httptest.NewRequest("GET", "/api/v1/runs/full-cycle", nil)
	req.SetPathValue("id", "full-cycle")
	w = httptest.NewRecorder()
	server.Handlers().HandleGetStatus(w, req)

	var resp RunResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.State != "completed" {
		t.Errorf("expected state 'completed', got '%s'", resp.State)
	}

	if resp.Tasks == nil || resp.Tasks["A"].Output != "result:A" {
		t.Errorf("expected task A output 'result:A', got: %+v", resp.Tasks)
	}
}

func TestServer_AbortRunning(t *testing.T) {
	aborted := make(chan struct{})

	executor := func(ctx context.Context, task *contracts.Task) (*contracts.TaskResult, error) {
		// Wait for abort or context cancel
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Second):
			return &contracts.TaskResult{Output: "should not reach"}, nil
		}
	}

	server := NewServer(":0", executor)

	// 1. Start run
	reqBody := `{
		"id": "abort-test",
		"policy": {"max_parallelism": 1, "budget_limit": {"amount": 1.0, "currency": "USD"}},
		"tasks": [{"id": "A", "prompt": "Test", "model": "claude-3-haiku-20240307"}]
	}`

	req := httptest.NewRequest("POST", "/api/v1/runs", bytes.NewBufferString(reqBody))
	w := httptest.NewRecorder()
	server.Handlers().HandleStartRun(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("StartRun failed: %d", w.Code)
	}

	// Wait for run to start executing
	time.Sleep(50 * time.Millisecond)

	// 2. Abort
	req = httptest.NewRequest("POST", "/api/v1/runs/abort-test/abort", nil)
	req.SetPathValue("id", "abort-test")
	w = httptest.NewRecorder()
	server.Handlers().HandleAbort(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Abort failed: %d - %s", w.Code, w.Body.String())
	}

	var abortResp RunResponse
	json.NewDecoder(w.Body).Decode(&abortResp)

	if abortResp.State != "aborting" {
		t.Errorf("expected state 'aborting', got '%s'", abortResp.State)
	}

	// 3. Wait for run to actually abort
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)

			req := httptest.NewRequest("GET", "/api/v1/runs/abort-test", nil)
			req.SetPathValue("id", "abort-test")
			w := httptest.NewRecorder()
			server.Handlers().HandleGetStatus(w, req)

			var resp RunResponse
			json.NewDecoder(w.Body).Decode(&resp)

			if resp.State == "aborted" || resp.State == "failed" {
				close(aborted)
				return
			}
		}
	}()

	select {
	case <-aborted:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for run to abort")
	}
}
