package orchestration

import (
	"sync"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

// queueManager implements contracts.QueueManager using an in-memory FIFO queue.
// Thread-safe for concurrent access using sync.Mutex.
type queueManager struct {
	mu    sync.Mutex
	items []contracts.TaskID
}

// NewQueueManager creates a new QueueManager.
func NewQueueManager() contracts.QueueManager {
	return &queueManager{
		items: make([]contracts.TaskID, 0),
	}
}

// Enqueue adds a task to the ready queue.
func (q *queueManager) Enqueue(taskID contracts.TaskID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, taskID)
}

// Dequeue removes and returns the next task from the queue.
// Returns ("", false) if the queue is empty.
func (q *queueManager) Dequeue() (contracts.TaskID, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.items) == 0 {
		return "", false
	}

	taskID := q.items[0]
	q.items = q.items[1:]
	return taskID, true
}

// Len returns the number of tasks in the queue.
func (q *queueManager) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
