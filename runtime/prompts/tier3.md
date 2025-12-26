# TIER 3 Prompt Template

**Model:** Opus (or Sonnet with careful review)
**Tokens:** 3,000-4,500
**For:** Critical logic where errors cause money loss, data corruption, or cascading failures

---

## Template

```
<role>
You implement {{COMPONENT_NAME}} in Go for the Runtime Layer.

CRITICAL: This component is {{CRITICALITY_REASON}}.
Errors here cause {{ERROR_CONSEQUENCE}}.
Think carefully before implementing. Consider edge cases.
</role>

<architecture_context>
{{RELEVANT_ARCHITECTURE_SECTION}}
</architecture_context>

<types>
{{ALL_REQUIRED_TYPES}}
</types>

<dependencies>
{{ALL_DEPENDENCY_INTERFACES}}
</dependencies>

<interface>
{{INTERFACE_DEFINITION}}
</interface>

<patterns>
{{THREE_OR_MORE_PROJECT_EXAMPLES}}
</patterns>

<business_rules>
{{DETAILED_BUSINESS_RULES}}
</business_rules>

<edge_cases>
{{COMPREHENSIVE_EDGE_CASES}}
</edge_cases>

<security_considerations>
{{SECURITY_NOTES_IF_APPLICABLE}}
</security_considerations>

<task>
Implement {{INTERFACE_NAME}}.
Write to: {{FILE_PATH}}
Package: {{PACKAGE_NAME}}
</task>

<reasoning_checklist>
Before implementing, verify:
- [ ] Invariants: what must always be true?
- [ ] Failure modes: what can go wrong?
- [ ] Race conditions: concurrent access patterns?
- [ ] Recovery: how to leave state consistent on error?
</reasoning_checklist>

<success_criteria>
- Implements all interface methods correctly
- Handles ALL listed edge cases
- No panics - all errors returned explicitly
- Thread-safe
- Deterministic behavior
- Comprehensive error messages
- Follows project patterns exactly
</success_criteria>
```

---

## Example: Scheduler

```
<role>
You implement Scheduler in Go for the Runtime Layer.

CRITICAL: This component determines execution order.
Errors here cause incorrect task sequencing, deadlocks, or skipped tasks.
Think carefully before implementing. Consider edge cases.
</role>

<architecture_context>
The Runtime Layer is a sidecar service that enforces execution constraints
for LLM agent systems. The Scheduler is responsible for:
- Determining which tasks are ready to execute (all dependencies satisfied)
- Tracking task completion and updating the DAG state
- Using topological order with priority as tie-breaker (Option B)
</architecture_context>

<types>
type RunID string
type TaskID string

type RunState int
const (
    RunPending RunState = iota
    RunRunning
    RunCompleted
    RunFailed
    RunAborted
)

type TaskState int
const (
    TaskPending TaskState = iota
    TaskReady
    TaskRunning
    TaskCompleted
    TaskFailed
    TaskSkipped
)

type Run struct {
    ID        RunID
    State     RunState
    DAG       *DAG
    Tasks     map[TaskID]*Task
}

type Task struct {
    ID      TaskID
    State   TaskState
    Deps    []TaskID
    Outputs *TaskResult
}

type DAG struct {
    Nodes map[TaskID]*DAGNode
}

type DAGNode struct {
    ID      TaskID
    Deps    []TaskID
    Next    []TaskID
    Pending int  // count of incomplete dependencies
}

type TaskResult struct {
    Output  string
    Outputs map[string]string
}
</types>

<dependencies>
// Scheduler has no dependencies on other interfaces
// It operates directly on Run and DAG structures
</dependencies>

<interface>
// Scheduler determines which tasks are ready to execute and tracks completion.
type Scheduler interface {
    // NextReady returns task IDs that are ready to execute (all deps satisfied).
    // Returns empty slice if no tasks are ready.
    // Returns error if run is in invalid state.
    NextReady(run *Run) ([]TaskID, error)

    // MarkComplete marks a task as completed and updates the run state.
    // Updates Pending counts for dependent tasks.
    // Returns error if task not found or already completed.
    MarkComplete(run *Run, taskID TaskID, result *TaskResult) error
}
</interface>

<patterns>
// Pattern 1: Iterating DAG nodes
for taskID, node := range dag.Nodes {
    if node.Pending == 0 {
        ready = append(ready, taskID)
    }
}

// Pattern 2: Updating dependent tasks
for _, nextID := range node.Next {
    nextNode := dag.Nodes[nextID]
    nextNode.Pending--
}

// Pattern 3: Stable sorting for determinism
sort.Slice(ready, func(i, j int) bool {
    return string(ready[i]) < string(ready[j])
})

// Pattern 4: State validation
if run.State != RunRunning {
    return nil, fmt.Errorf("run %s is not running: %s", run.ID, run.State)
}
</patterns>

<business_rules>
- Task is ready when Pending == 0 (all deps completed)
- NextReady only returns tasks in TaskPending or TaskReady state
- MarkComplete updates task state to TaskCompleted
- MarkComplete decrements Pending for all dependent tasks
- Results are sorted by TaskID for determinism
- Run must be in RunRunning state for scheduling
</business_rules>

<edge_cases>
- run.DAG is nil → return error
- run.Tasks is nil or empty → return empty slice, no error
- taskID not found in run.Tasks → return error
- task already completed → return error (idempotency decision: error)
- task in TaskFailed state → don't return in NextReady
- circular dependency (should not happen if DAG validated) → handle gracefully
- concurrent calls to NextReady → must be safe (caller holds lock)
- concurrent calls to MarkComplete → must be safe (caller holds lock)
- all tasks completed → return empty from NextReady
</edge_cases>

<security_considerations>
Not applicable for Scheduler - no external input validation needed.
All data comes from validated internal structures.
</security_considerations>

<task>
Implement Scheduler.
Write to: internal/orchestration/scheduler.go
Package: orchestration
</task>

<reasoning_checklist>
Before implementing, verify:
- [ ] Invariants: Pending count accuracy, valid state transitions, deterministic ordering
- [ ] Failure modes: nil pointers, invalid task states, missing map keys
- [ ] Race conditions: concurrent NextReady/MarkComplete calls
- [ ] Recovery: return error, never panic, leave DAG consistent
</reasoning_checklist>

<success_criteria>
- NextReady returns only truly ready tasks
- MarkComplete correctly updates Pending counts
- Deterministic ordering (sorted by TaskID)
- All edge cases handled with appropriate errors
- No panics on nil inputs
- Thread-safe (assumes caller synchronization)
- State remains consistent after errors
</success_criteria>
```

---

## Placeholders

| Placeholder | Description |
|-------------|-------------|
| `{{COMPONENT_NAME}}` | Name of component |
| `{{CRITICALITY_REASON}}` | Why this is TIER 3 |
| `{{ERROR_CONSEQUENCE}}` | What happens on error |
| `{{RELEVANT_ARCHITECTURE_SECTION}}` | From architecture doc |
| `{{ALL_REQUIRED_TYPES}}` | All types including nested |
| `{{ALL_DEPENDENCY_INTERFACES}}` | All interfaces used |
| `{{INTERFACE_DEFINITION}}` | Full interface with detailed comments |
| `{{THREE_OR_MORE_PROJECT_EXAMPLES}}` | Multiple patterns |
| `{{DETAILED_BUSINESS_RULES}}` | All behavioral rules |
| `{{COMPREHENSIVE_EDGE_CASES}}` | Every possible edge case |
| `{{SECURITY_NOTES_IF_APPLICABLE}}` | Security considerations |
| `{{INTERFACE_NAME}}` | Interface name |
| `{{FILE_PATH}}` | Target file path |
| `{{PACKAGE_NAME}}` | Go package name |
