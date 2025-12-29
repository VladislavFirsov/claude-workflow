package api

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// RunEntry represents a run stored in the RunStore.
type RunEntry struct {
	mu sync.RWMutex // protects shadowState

	// Run is the actual run object, modified by orchestrator.
	// WARNING: Do not read from this directly - use shadowState for reads.
	Run    *contracts.Run
	Cancel context.CancelFunc
	Done   chan struct{} // closed when Run() completes
	Error  error         // error from Run()

	// shadowState is a synchronized copy of Run state for safe reads.
	// Updated by UpdateShadowState after each task completes.
	shadowState *RunShadowState

	Aborting  bool // true after Abort() is called, until goroutine finishes
	CreatedAt time.Time
	UpdatedAt time.Time
}

// RunShadowState is a thread-safe copy of Run state.
type RunShadowState struct {
	State contracts.RunState
	Tasks map[contracts.TaskID]TaskShadow
	Usage contracts.Usage
}

// TaskShadow is a copy of task state.
type TaskShadow struct {
	State  contracts.TaskState
	Output string
	Error  *contracts.TaskError // deep copy
}

// RunStore provides thread-safe in-memory storage for runs.
type RunStore struct {
	mu   sync.RWMutex
	runs map[contracts.RunID]*RunEntry
}

// NewRunStore creates a new RunStore.
func NewRunStore() *RunStore {
	return &RunStore{
		runs: make(map[contracts.RunID]*RunEntry),
	}
}

// Create stores a new run. Returns ErrRunExists if the ID already exists.
func (s *RunStore) Create(run *contracts.Run, cancel context.CancelFunc) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.runs[run.ID]; exists {
		return fmt.Errorf("run %s: %w", run.ID, ErrRunExists)
	}

	now := time.Now()

	// Create initial shadow state
	shadow := &RunShadowState{
		State: run.State,
		Tasks: make(map[contracts.TaskID]TaskShadow, len(run.Tasks)),
		Usage: run.Usage,
	}
	for id, task := range run.Tasks {
		ts := TaskShadow{State: task.State}
		if task.Outputs != nil {
			ts.Output = task.Outputs.Output
		}
		if task.Error != nil {
			ts.Error = &contracts.TaskError{
				Code:    task.Error.Code,
				Message: task.Error.Message,
			}
		}
		shadow.Tasks[id] = ts
	}

	s.runs[run.ID] = &RunEntry{
		Run:         run,
		Cancel:      cancel,
		Done:        make(chan struct{}),
		shadowState: shadow,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return nil
}

// Get retrieves a run entry by ID.
// WARNING: The returned entry contains a pointer to Run which may be modified
// by the orchestrator goroutine. Use GetSnapshot for safe concurrent access.
func (s *RunStore) Get(id contracts.RunID) (*RunEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.runs[id]
	return entry, exists
}

// RunSnapshot is a thread-safe copy of run state for API responses.
type RunSnapshot struct {
	ID        contracts.RunID
	State     contracts.RunState
	Tasks     map[contracts.TaskID]TaskSnapshot
	Usage     contracts.Usage
	CreatedAt int64
	UpdatedAt int64
	APIState  string // "aborting" if abort was called but not finished
	Error     error
}

// TaskSnapshot is a thread-safe copy of task state.
type TaskSnapshot struct {
	State  contracts.TaskState
	Output string
	Error  *contracts.TaskError
}

// GetSnapshot returns a thread-safe copy of run state for API responses.
// Reads from shadowState which is synchronized via entry.mu.
func (s *RunStore) GetSnapshot(id contracts.RunID) (*RunSnapshot, bool) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return nil, false
	}
	// Copy entry-level fields under store lock (immutable or separately protected)
	aborting := entry.Aborting
	done := s.isDone(entry)
	createdAt := entry.CreatedAt.UnixMilli() // immutable after create
	runErr := entry.Error
	runID := entry.Run.ID
	s.mu.RUnlock()

	// Lock entry's shadowState for reading (also protects UpdatedAt)
	entry.mu.RLock()
	defer entry.mu.RUnlock()
	updatedAt := entry.UpdatedAt.UnixMilli()

	shadow := entry.shadowState
	if shadow == nil {
		return nil, false
	}

	// Determine API state
	apiState := shadow.State.String()
	if aborting && !done {
		apiState = "aborting"
	}

	// Copy tasks from shadow (already deep-copied)
	tasks := make(map[contracts.TaskID]TaskSnapshot, len(shadow.Tasks))
	for id, task := range shadow.Tasks {
		ts := TaskSnapshot{
			State:  task.State,
			Output: task.Output,
		}
		if task.Error != nil {
			ts.Error = &contracts.TaskError{
				Code:    task.Error.Code,
				Message: task.Error.Message,
			}
		}
		tasks[id] = ts
	}

	return &RunSnapshot{
		ID:        runID,
		State:     shadow.State,
		Tasks:     tasks,
		Usage:     shadow.Usage,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		APIState:  apiState,
		Error:     runErr,
	}, true
}

// Abort cancels a running run. Returns:
// - ErrRunNotFound if the run doesn't exist
// - ErrRunCompleted if the run is already in a terminal state
func (s *RunStore) Abort(id contracts.RunID) error {
	s.mu.Lock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.Unlock()
		return fmt.Errorf("run %s: %w", id, contracts.ErrRunNotFound)
	}

	// Check if already aborting
	if entry.Aborting {
		s.mu.Unlock()
		return nil // idempotent
	}

	// Check if Done channel is closed (run finished)
	if s.isDone(entry) {
		s.mu.Unlock()
		return fmt.Errorf("run %s: %w", id, contracts.ErrRunCompleted)
	}

	// Check shadow state for terminal status (thread-safe)
	entry.mu.RLock()
	shadowState := entry.shadowState.State
	entry.mu.RUnlock()

	switch shadowState {
	case contracts.RunCompleted, contracts.RunFailed, contracts.RunAborted:
		s.mu.Unlock()
		return fmt.Errorf("run %s: %w", id, contracts.ErrRunCompleted)
	}

	// Mark as aborting, update timestamp, and cancel
	entry.Aborting = true
	entry.mu.Lock()
	entry.UpdatedAt = time.Now()
	entry.mu.Unlock()

	if entry.Cancel != nil {
		entry.Cancel()
	}
	s.mu.Unlock()

	return nil
}

// UpdateShadowState updates the shadow state for tasks.
// Run.State is updated separately in SetShadowRunState to avoid race with orchestrator.
// IMPORTANT: Only call when orchestrator has finished (e.g., from MarkDone).
func (s *RunStore) UpdateShadowState(id contracts.RunID) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return
	}
	run := entry.Run
	s.mu.RUnlock()

	// Lock shadow for writing
	entry.mu.Lock()
	defer entry.mu.Unlock()

	// Update usage (struct copy, safe)
	entry.shadowState.Usage = run.Usage

	// Update task states - orchestrator has finished modifying at this point
	for id, task := range run.Tasks {
		ts := TaskShadow{State: task.State}
		if task.Outputs != nil {
			ts.Output = task.Outputs.Output
		}
		if task.Error != nil {
			ts.Error = &contracts.TaskError{
				Code:    task.Error.Code,
				Message: task.Error.Message,
			}
		}
		entry.shadowState.Tasks[id] = ts
	}

	// Also update timestamp
	entry.UpdatedAt = time.Now()
}

// UpdateProgress updates only the timestamp during execution.
// Use this instead of UpdateShadowState when orchestrator may still be modifying tasks.
func (s *RunStore) UpdateProgress(id contracts.RunID) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	entry.mu.Lock()
	entry.UpdatedAt = time.Now()
	entry.mu.Unlock()
}

// UpdateTaskRunning updates shadow state for a task entering running state.
func (s *RunStore) UpdateTaskRunning(id contracts.RunID, taskID contracts.TaskID) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.shadowState == nil {
		return
	}

	task := entry.shadowState.Tasks[taskID]
	task.State = contracts.TaskRunning
	entry.shadowState.Tasks[taskID] = task
	entry.UpdatedAt = time.Now()
}

// UpdateTaskSuccess updates shadow state for a completed task and usage.
func (s *RunStore) UpdateTaskSuccess(id contracts.RunID, taskID contracts.TaskID, result *contracts.TaskResult) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.shadowState == nil {
		return
	}

	task := entry.shadowState.Tasks[taskID]
	task.State = contracts.TaskCompleted
	if result != nil {
		task.Output = result.Output
		entry.shadowState.Usage.Tokens += result.Usage.Tokens
		entry.shadowState.Usage.Cost.Amount += result.Usage.Cost.Amount
		if entry.shadowState.Usage.Cost.Currency == "" {
			entry.shadowState.Usage.Cost.Currency = result.Usage.Cost.Currency
		}
	}
	entry.shadowState.Tasks[taskID] = task
	entry.UpdatedAt = time.Now()
}

// UpdateTaskFailure updates shadow state for a failed task.
func (s *RunStore) UpdateTaskFailure(id contracts.RunID, taskID contracts.TaskID, err error) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()

	if entry.shadowState == nil {
		return
	}

	task := entry.shadowState.Tasks[taskID]
	task.State = contracts.TaskFailed
	if err != nil {
		task.Error = &contracts.TaskError{
			Code:    string(CodeTaskFailed),
			Message: err.Error(),
		}
	}
	entry.shadowState.Tasks[taskID] = task
	entry.UpdatedAt = time.Now()
}

// SetShadowRunState updates the Run.State in shadow.
// Called by MarkDone when orchestrator has finished.
func (s *RunStore) SetShadowRunState(id contracts.RunID, state contracts.RunState) {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return
	}
	s.mu.RUnlock()

	entry.mu.Lock()
	defer entry.mu.Unlock()
	entry.shadowState.State = state
}

// UpdateTimestamp updates the UpdatedAt timestamp for a run.
// Safe to call during execution - only updates timestamp, not task state.
func (s *RunStore) UpdateTimestamp(id contracts.RunID) {
	s.UpdateProgress(id)
}

// MarkDone marks a run as completed, updating the error and closing the Done channel.
// Should be called when the orchestrator.Run goroutine finishes.
func (s *RunStore) MarkDone(id contracts.RunID, err error) {
	// First update shadow task states one final time
	s.UpdateShadowState(id)

	s.mu.Lock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.Unlock()
		return
	}
	// Get final run state (safe now - orchestrator has finished)
	finalState := entry.Run.State
	s.mu.Unlock()

	// Update shadow with final run state
	s.SetShadowRunState(id, finalState)

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists = s.runs[id]
	if !exists {
		return
	}

	entry.Error = err
	entry.UpdatedAt = time.Now()

	// Close Done channel to signal completion
	select {
	case <-entry.Done:
		// already closed
	default:
		close(entry.Done)
	}
}

// IsAborting returns true if Abort was called but the run hasn't finished yet.
func (s *RunStore) IsAborting(id contracts.RunID) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.runs[id]
	if !exists {
		return false
	}

	return entry.Aborting && !s.isDone(entry)
}

// isDone checks if the Done channel is closed (must be called with lock held for RLock).
func (s *RunStore) isDone(entry *RunEntry) bool {
	select {
	case <-entry.Done:
		return true
	default:
		return false
	}
}

// GetAPIState returns the API-level state for a run.
// This handles the "aborting" state that doesn't exist in contracts.RunState.
func (s *RunStore) GetAPIState(id contracts.RunID) string {
	s.mu.RLock()
	entry, exists := s.runs[id]
	if !exists {
		s.mu.RUnlock()
		return ""
	}
	aborting := entry.Aborting
	done := s.isDone(entry)
	s.mu.RUnlock()

	entry.mu.RLock()
	defer entry.mu.RUnlock()

	// If aborting but not yet done, return "aborting"
	if aborting && !done {
		return "aborting"
	}

	if entry.shadowState != nil {
		return entry.shadowState.State.String()
	}

	return ""
}

// GetTimestamps returns the created and updated timestamps for a run.
func (s *RunStore) GetTimestamps(id contracts.RunID) (createdAt, updatedAt int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, exists := s.runs[id]
	if !exists {
		return 0, 0
	}

	return entry.CreatedAt.UnixMilli(), entry.UpdatedAt.UnixMilli()
}

// CancelAll cancels all active runs. Used for graceful shutdown.
// Returns the number of runs that were cancelled.
func (s *RunStore) CancelAll() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancelled := 0
	for _, entry := range s.runs {
		// Skip already completed or aborting runs
		if entry.Aborting {
			continue
		}
		entry.mu.RLock()
		shadowState := contracts.RunPending
		if entry.shadowState != nil {
			shadowState = entry.shadowState.State
		}
		entry.mu.RUnlock()
		switch shadowState {
		case contracts.RunCompleted, contracts.RunFailed, contracts.RunAborted:
			continue
		}

		// Cancel the run
		entry.Aborting = true
		entry.UpdatedAt = time.Now()
		if entry.Cancel != nil {
			entry.Cancel()
		}
		cancelled++
	}
	return cancelled
}

// WaitAll waits for all active runs to complete, with a timeout.
// Returns the number of runs still active after timeout.
func (s *RunStore) WaitAll(timeout time.Duration) int {
	deadline := time.Now().Add(timeout)

	for {
		s.mu.RLock()
		active := 0
		var doneChannels []chan struct{}
		for _, entry := range s.runs {
			if !s.isDone(entry) {
				active++
				doneChannels = append(doneChannels, entry.Done)
			}
		}
		s.mu.RUnlock()

		if active == 0 {
			return 0
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return active
		}

		// Wait for any run to complete or timeout
		select {
		case <-time.After(remaining):
			return active
		case <-doneChannels[0]:
			// One run completed, loop to check others
		}
	}
}

// PruneCompleted removes completed runs older than the retention duration.
// Returns the number of removed runs.
func (s *RunStore) PruneCompleted(retention time.Duration) int {
	if retention <= 0 {
		return 0
	}

	cutoff := time.Now().Add(-retention)
	removed := 0

	s.mu.Lock()
	defer s.mu.Unlock()

	for id, entry := range s.runs {
		if !s.isDone(entry) {
			continue
		}
		if entry.UpdatedAt.Before(cutoff) {
			delete(s.runs, id)
			removed++
		}
	}

	return removed
}
