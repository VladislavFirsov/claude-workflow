# TIER 2 Prompt Template

**Model:** Sonnet
**Tokens:** 1,500-2,200
**For:** State management, coordination, integration of components

---

## Template

```
<role>
You implement {{COMPONENT_NAME}} in Go for the Runtime Layer.
This component manages state and coordinates between other components.
Write production-quality code. No placeholders or TODOs.
</role>

<types>
{{REQUIRED_TYPES}}
</types>

<dependencies>
{{DEPENDENCY_INTERFACES}}
</dependencies>

<interface>
{{INTERFACE_DEFINITION}}
</interface>

<patterns>
{{TWO_OR_THREE_PROJECT_EXAMPLES}}
</patterns>

<business_rules>
{{KEY_BUSINESS_RULES}}
</business_rules>

<edge_cases>
{{CRITICAL_EDGE_CASES}}

Always consider:
- State transitions: valid/invalid state changes
- Error propagation: how errors from dependencies bubble up
</edge_cases>

<task>
Implement {{INTERFACE_NAME}}.
Write to: {{FILE_PATH}}
Package: {{PACKAGE_NAME}}
</task>

<success_criteria>
- Implements all interface methods
- Handles all listed edge cases
- Thread-safe if accessed concurrently
- Uses dependency interfaces, not concrete types
- Follows the pattern style
</success_criteria>
```

---

## Example: QueueManager

```
<role>
You implement QueueManager in Go for the Runtime Layer.
This component manages the queue of tasks ready for execution.
Write production-quality code. No placeholders or TODOs.
</role>

<types>
type TaskID string
</types>

<dependencies>
// None for QueueManager - it's self-contained
</dependencies>

<interface>
// QueueManager manages the queue of tasks ready for execution.
type QueueManager interface {
    // Enqueue adds a task to the ready queue.
    Enqueue(taskID TaskID)

    // Dequeue removes and returns the next task from the queue.
    Dequeue() (TaskID, bool)

    // Len returns the number of tasks in the queue.
    Len() int
}
</interface>

<patterns>
// Pattern 1: Simple in-memory queue with mutex
type memoryQueue struct {
    mu    sync.Mutex
    items []string
}

func (q *memoryQueue) Push(item string) {
    q.mu.Lock()
    defer q.mu.Unlock()
    q.items = append(q.items, item)
}

// Pattern 2: Constructor pattern
func NewMemoryQueue() *memoryQueue {
    return &memoryQueue{
        items: make([]string, 0),
    }
}
</patterns>

<business_rules>
- FIFO ordering (first in, first out)
- Thread-safe for concurrent access
- Dequeue returns false if queue is empty
- No blocking - return immediately
</business_rules>

<edge_cases>
- Dequeue from empty queue → return ("", false)
- Concurrent Enqueue/Dequeue → must be safe
- Len() during modifications → consistent snapshot
- State transitions: N/A for stateless queue
- Error propagation: N/A (no errors returned)
</edge_cases>

<task>
Implement QueueManager.
Write to: internal/orchestration/queue_manager.go
Package: orchestration
</task>

<success_criteria>
- Implements Enqueue, Dequeue, Len
- FIFO ordering preserved
- Thread-safe with sync.Mutex
- Dequeue returns (TaskID, false) when empty
- No external dependencies beyond stdlib
</success_criteria>
```

---

## Placeholders

| Placeholder | Description |
|-------------|-------------|
| `{{COMPONENT_NAME}}` | Name of component |
| `{{REQUIRED_TYPES}}` | Types used by this interface |
| `{{DEPENDENCY_INTERFACES}}` | Interfaces this component depends on |
| `{{INTERFACE_DEFINITION}}` | Full interface with comments |
| `{{TWO_OR_THREE_PROJECT_EXAMPLES}}` | 2-3 relevant patterns |
| `{{KEY_BUSINESS_RULES}}` | Important behavioral rules |
| `{{CRITICAL_EDGE_CASES}}` | 3-5 edge cases to handle |
| `{{INTERFACE_NAME}}` | Interface name |
| `{{FILE_PATH}}` | Target file path |
| `{{PACKAGE_NAME}}` | Go package name |
