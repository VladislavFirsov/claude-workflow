package context

import (
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// memoryManager implements contracts.MemoryManager for managing short-term memory within a run.
type memoryManager struct {
	mu sync.RWMutex
}

// NewMemoryManager creates a new MemoryManager.
func NewMemoryManager() contracts.MemoryManager {
	return &memoryManager{}
}

// Get retrieves a value from memory.
// Returns the value and true if the key exists, or "" and false if not found or run is nil.
func (m *memoryManager) Get(run *contracts.Run, key string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if run == nil || run.Memory == nil {
		return "", false
	}

	val, ok := run.Memory[key]
	return val, ok
}

// Put stores a value in memory.
// Creates the Memory map if it is nil. Handles nil run gracefully by doing nothing.
func (m *memoryManager) Put(run *contracts.Run, key string, value string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if run == nil {
		return
	}

	// Initialize Memory map if nil
	if run.Memory == nil {
		run.Memory = make(map[string]string)
	}

	run.Memory[key] = value
}
