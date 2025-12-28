# Runtime Layer v1 Draft (Sidecar, Go, LangChain)

## Decision Summary
- Form factor: Sidecar service (enforcement, isolation, B2B)
- Protocol: HTTP/JSON (adoption > perf)
- Guarantees: At-most-once
- Core domains v1: Orchestration + Cost + Context
- Target framework: LangChain first
- Core runtime: Go
- SDK: Python first, then JS

## Goals
- Enforce execution constraints without rewriting agent business logic.
- Make cost, context size, and execution order predictable.
- Provide a stable integration surface for LangChain.
- Keep v1 simple, safe, and observable.

## Non-Goals (v1)
- Exactly-once or at-least-once execution guarantees.
- Full multi-tenant billing or metering system.
- Deep agent memory long-term storage.
- Support for multiple frameworks beyond LangChain.

## Architecture Overview
- SDK (Python) sits inside the client app and proxies agent calls to the sidecar.
- Sidecar runtime (Go) owns execution orchestration and enforcement.
- Core services are modular in-process components.
- All policies are explicit and enforced at runtime boundaries.

## Core Components (v1)

### Orchestration
- Scheduler: deterministic ordering based on dependency graph.
- DependencyResolver: DAG build and validation.
- ParallelExecutor: bounded concurrency with simple worker pool.
- QueueManager: in-memory queue for ready tasks.

### Cost Control
- TokenEstimator: estimate prompt + context tokens before execution.
- CostCalculator: estimate cost using configured price tables.
- BudgetEnforcer: hard ceilings per run (fail fast).
- UsageTracker: in-memory usage for a run (exposed in response).

### Context Management
- ContextBuilder: compile tool outputs + memory + prompt templates.
- ContextCompactor: deterministic truncation/summary based on rules.
- ContextRouter: pass context slices between agents/tasks.
- MemoryManager: short-term in-run memory only.

## Runtime Contracts

### SDK -> Sidecar
- StartRun: create a run with policy, budget, and graph.
- EnqueueTask: submit task with inputs and dependencies.
- ExecuteTask: request execution when task is ready.
- GetRunStatus: poll for progress and usage.
- AbortRun: cancel and release resources.

### Sidecar -> SDK
- TaskResult: output, usage, cost, metadata.
- RunStatus: state, completed tasks, errors, total usage.

## Data Flow (Happy Path)
1. SDK creates run with policy + budget.
2. SDK submits task graph.
3. Sidecar builds DAG and validates.
4. Scheduler releases ready tasks.
5. Cost + Context modules evaluate before execution.
6. Task executes; results stored in run context.
7. Usage accumulates; budget checked.
8. SDK polls status and consumes outputs.

## V1 Scope (Concrete)
- HTTP/JSON API with documented request/response schemas.
- Go runtime with in-memory state only.
- Deterministic scheduling + bounded parallelism.
- Pre-execution cost estimation + hard budget cutoffs.
- Rule-based context compaction (deterministic).
- LangChain adapter in Python SDK (minimal wrapper).

## Go Interfaces (Contracts)

### Orchestration
```go
type Scheduler interface {
	NextReady(run *Run) ([]TaskID, error)
	MarkComplete(run *Run, taskID TaskID, result *TaskResult) error
}

type DependencyResolver interface {
	BuildDAG(tasks []Task) (*DAG, error)
	Validate(dag *DAG) error
}

type ParallelExecutor interface {
	Execute(run *Run, taskID TaskID) (*TaskResult, error)
}

type QueueManager interface {
	Enqueue(taskID TaskID)
	Dequeue() (TaskID, bool)
	Len() int
}
```

### Cost Control
```go
type TokenEstimator interface {
	Estimate(input *TaskInput, ctx *ContextBundle) (TokenCount, error)
}

type CostCalculator interface {
	Estimate(tokens TokenCount, model ModelID) (Cost, error)
}

type BudgetEnforcer interface {
	Allow(run *Run, estimate Cost) error
	Record(run *Run, actual Cost) error
}

type UsageTracker interface {
	Add(run *Run, usage Usage)
	Snapshot(run *Run) Usage
}
```

### Context Management
```go
type ContextBuilder interface {
	Build(run *Run, taskID TaskID) (*ContextBundle, error)
}

type ContextCompactor interface {
	Compact(bundle *ContextBundle, policy ContextPolicy) (*ContextBundle, error)
}

type ContextRouter interface {
	Route(run *Run, from TaskID, to TaskID, output *TaskResult) error
}

type MemoryManager interface {
	Get(run *Run, key string) (string, bool)
	Put(run *Run, key string, value string)
}
```

## Data Structures (Core)
```go
type RunID string
type TaskID string
type ModelID string
type TokenCount int64
type Currency string

type RunState int
type TaskState int

const (
	RunPending RunState = iota
	RunRunning
	RunCompleted
	RunFailed
	RunAborted
)

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
	Policy    RunPolicy
	DAG       *DAG
	Tasks     map[TaskID]*Task
	Usage     Usage
	CreatedAt int64
	UpdatedAt int64
}

type Task struct {
	ID           TaskID
	State        TaskState
	Inputs       *TaskInput
	Deps         []TaskID
	Outputs      *TaskResult
	Error        *TaskError
	Model        ModelID
	EstimatedUse Usage
	ActualUse    Usage
}

type DAG struct {
	Nodes map[TaskID]*DAGNode
	Edges map[TaskID][]TaskID
}

type DAGNode struct {
	ID      TaskID
	Deps    []TaskID
	Next    []TaskID
	Pending int
}

type Usage struct {
	Tokens TokenCount
	Cost   Cost
}

type Cost struct {
	Amount   float64
	Currency Currency
}

type TaskInput struct {
	Prompt   string
	Inputs   map[string]string
	Metadata map[string]string
}

type TaskResult struct {
	Output   string
	Outputs  map[string]string
	Usage    Usage
	Metadata map[string]string
}

type TaskError struct {
	Code    string
	Message string
}

type ContextBundle struct {
	Messages []string
	Memory   map[string]string
	Tools    map[string]string
}

type ContextPolicy struct {
	MaxTokens   TokenCount
	Strategy    string
	KeepLastN   int
	TruncateTo  TokenCount
}

type RunPolicy struct {
	TimeoutMs      int64
	MaxParallelism int
	BudgetLimit    Cost
}
```

## Open Questions (Decisions for v1 Coding)

### Scheduler Semantics
- Option A: strict topological order (deterministic, no priorities).
- Option B: topological + priority queue (deterministic tie-breaker).
- Proposed v1: Option B with stable ordering (priority, then TaskID).

### Budget Granularity
- Option A: per run only (simple, predictable).
- Option B: per task + per run (more control, more config).
- Proposed v1: Option A, with optional per-task override if set.

## Risks and Mitigations
- Risk: in-memory state loss on crash.
  - Mitigation: explicit at-most-once; client retries are opt-in.
- Risk: inaccurate token estimation.
  - Mitigation: plug-in estimator + per-model calibration tables.
- Risk: context compaction loses critical info.
  - Mitigation: deterministic rules + test fixtures per agent type.

## Testing (Current)
- E2E integration tests for Orchestrator are implemented with real components
  and a stub executor in `runtime/internal/orchestration/orchestrator_integration_test.go`.
- Scenarios covered: linear DAG, fan-in, diamond, single/empty DAG, context routing,
  budget enforcement, task failure, and context cancellation.
- Factory/DI helper tests cover defaults, custom catalog/currency, and minimal E2E flows
  in `runtime/internal/orchestration/factory_test.go`.

## Open Questions
- Policy surface: which knobs are exposed in v1 (timeouts, max parallelism)?
- Budget granularity: per run vs per task?
- Scheduler semantics: strict topological order vs priority queue?

## Next Steps
- Define HTTP API schema and versioning.
- Draft LangChain adapter interface and minimal SDK package layout.
- Decide first internal storage (in-memory only or optional disk snapshot).
- Create test matrix for Tier 3 components (Scheduler, BudgetEnforcer, ContextCompactor).
