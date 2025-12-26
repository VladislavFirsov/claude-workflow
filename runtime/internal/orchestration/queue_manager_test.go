package orchestration

import (
	"sync"
	"testing"

	"github.com/anthropics/claude-workflow/runtime/contracts"
)

func TestQueueManager_Enqueue(t *testing.T) {
	q := NewQueueManager()

	q.Enqueue("task-1")
	if q.Len() != 1 {
		t.Errorf("Len() = %d, want 1", q.Len())
	}

	q.Enqueue("task-2")
	if q.Len() != 2 {
		t.Errorf("Len() = %d, want 2", q.Len())
	}
}

func TestQueueManager_Dequeue(t *testing.T) {
	q := NewQueueManager()

	// Dequeue from empty queue
	taskID, ok := q.Dequeue()
	if ok {
		t.Error("Dequeue() from empty queue should return false")
	}
	if taskID != "" {
		t.Errorf("Dequeue() from empty queue should return empty TaskID, got %q", taskID)
	}

	// Enqueue and dequeue
	q.Enqueue("task-1")
	q.Enqueue("task-2")

	taskID, ok = q.Dequeue()
	if !ok {
		t.Error("Dequeue() should return true")
	}
	if taskID != "task-1" {
		t.Errorf("Dequeue() = %q, want task-1", taskID)
	}

	taskID, ok = q.Dequeue()
	if !ok {
		t.Error("Dequeue() should return true")
	}
	if taskID != "task-2" {
		t.Errorf("Dequeue() = %q, want task-2", taskID)
	}

	// Queue should be empty now
	_, ok = q.Dequeue()
	if ok {
		t.Error("Dequeue() from empty queue should return false")
	}
}

func TestQueueManager_FIFO(t *testing.T) {
	q := NewQueueManager()

	// Enqueue in order
	tasks := []contracts.TaskID{"task-a", "task-b", "task-c", "task-d"}
	for _, taskID := range tasks {
		q.Enqueue(taskID)
	}

	// Dequeue should preserve FIFO order
	for i, expected := range tasks {
		got, ok := q.Dequeue()
		if !ok {
			t.Fatalf("Dequeue() %d should return true", i)
		}
		if got != expected {
			t.Errorf("Dequeue() %d = %q, want %q", i, got, expected)
		}
	}
}

func TestQueueManager_Len(t *testing.T) {
	q := NewQueueManager()

	if q.Len() != 0 {
		t.Errorf("Len() on empty queue = %d, want 0", q.Len())
	}

	q.Enqueue("task-1")
	q.Enqueue("task-2")
	q.Enqueue("task-3")

	if q.Len() != 3 {
		t.Errorf("Len() = %d, want 3", q.Len())
	}

	q.Dequeue()
	if q.Len() != 2 {
		t.Errorf("Len() after dequeue = %d, want 2", q.Len())
	}
}

func TestQueueManager_Concurrent(t *testing.T) {
	q := NewQueueManager()
	var wg sync.WaitGroup

	// Concurrent enqueue
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			q.Enqueue(contracts.TaskID(string(rune('a' + id%26))))
		}(i)
	}
	wg.Wait()

	if q.Len() != 100 {
		t.Errorf("Len() after concurrent enqueue = %d, want 100", q.Len())
	}

	// Concurrent dequeue
	dequeued := 0
	var mu sync.Mutex

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok := q.Dequeue()
			if ok {
				mu.Lock()
				dequeued++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if dequeued != 100 {
		t.Errorf("dequeued count = %d, want 100", dequeued)
	}

	if q.Len() != 0 {
		t.Errorf("Len() after concurrent dequeue = %d, want 0", q.Len())
	}
}

func TestQueueManager_ConcurrentEnqueueDequeue(t *testing.T) {
	q := NewQueueManager()
	var wg sync.WaitGroup

	// Mix of enqueue and dequeue operations
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func(id int) {
			defer wg.Done()
			q.Enqueue(contracts.TaskID(string(rune('a' + id%26))))
		}(i)

		go func() {
			defer wg.Done()
			q.Dequeue() // May or may not succeed
		}()
	}
	wg.Wait()

	// Queue should be in a consistent state (not panicked, len >= 0)
	length := q.Len()
	if length < 0 {
		t.Errorf("Len() = %d, should be >= 0", length)
	}
}

func TestQueueManager_EmptyTaskID(t *testing.T) {
	q := NewQueueManager()

	// Empty TaskID is valid
	q.Enqueue("")
	if q.Len() != 1 {
		t.Errorf("Len() = %d, want 1", q.Len())
	}

	taskID, ok := q.Dequeue()
	if !ok {
		t.Error("Dequeue() should return true")
	}
	if taskID != "" {
		t.Errorf("Dequeue() = %q, want empty string", taskID)
	}
}
