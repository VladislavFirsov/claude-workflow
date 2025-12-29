# Runtime Layer — Project Status

> **Last Updated:** 2025-12-28
> **Status:** Core Complete, HTTP API Done

## Quick Start for New Session

```bash
# Verify all tests pass
cd /Users/vladislav/GoProjects/claude-workflow/runtime
go test ./... -v
```

---

## Completed Components (19/19)

| Domain | Component | Path | Tests |
|--------|-----------|------|-------|
| **Orchestration** | Scheduler | `internal/orchestration/scheduler.go` | ✅ |
| | DependencyResolver | `internal/orchestration/dependency_resolver.go` | ✅ |
| | ParallelExecutor | `internal/orchestration/parallel_executor.go` | ✅ |
| | QueueManager | `internal/orchestration/queue_manager.go` | ✅ |
| | **Orchestrator** | `internal/orchestration/orchestrator.go` | ✅ |
| | Factory | `internal/orchestration/factory.go` | ✅ |
| **Cost** | TokenEstimator | `internal/cost/token_estimator.go` | ✅ |
| | CostCalculator | `internal/cost/cost_calculator.go` | ✅ |
| | BudgetEnforcer | `internal/cost/budget_enforcer.go` | ✅ |
| | UsageTracker | `internal/cost/usage_tracker.go` | ✅ |
| | ModelCatalog | `internal/cost/model_catalog.go` | ✅ |
| **Context** | ContextBuilder | `internal/context/context_builder.go` | ✅ |
| | ContextCompactor | `internal/context/context_compactor.go` | ✅ |
| | ContextRouter | `internal/context/context_router.go` | ✅ |
| | MemoryManager | `internal/context/memory_manager.go` | ✅ |
| **API** | Server | `api/server.go` | ✅ |
| | Handlers | `api/handlers.go` | ✅ |
| | RunStore | `api/store.go` | ✅ |
| | Models/Errors | `api/models.go`, `api/errors.go` | ✅ |

---

## Development Plan

### Done
- Runtime core components implemented and tested (Orchestration, Cost, Context).
- Contracts-first interfaces and models defined.
- Orchestrator main loop implemented with deadlock detection.
- v3 bug fixes applied (terminal checks, token floor, context gating, usage sync).
- Tiered prompts and manifest in place.
- **E2E integration tests** (`orchestrator_integration_test.go`) — 9 test cases:
  - Linear DAG (A→B→C), Fan-in, Diamond patterns
  - Context routing verification
  - Budget enforcement with deterministic token calculation
  - Task failure and context cancellation handling
- **Factory/DI helper** (`factory.go`) — unified orchestrator assembly:
  - `NewOrchestratorWithDefaults(policy, executor)` — simple API
  - `NewOrchestratorWithOptions(policy, executor, opts)` — custom ModelCatalog/Currency
  - 6 tests including single-task and multi-task E2E
- **HTTP API surface** (`api/`) — REST API for sidecar runtime:
  - `POST /api/v1/runs` — StartRun (202 Accepted, async execution)
  - `GET /api/v1/runs/{id}` — GetStatus (includes "aborting" API state)
  - `POST /api/v1/runs/{id}/abort` — AbortRun (fire-and-forget)
  - `POST /api/v1/runs/{id}/tasks` — EnqueueTask (501 Not Implemented in V1)
  - RunStore with mutex, DTOs, error mapping to HTTP status codes
  - 14 tests (5 store + 7 handler + 2 integration)
  - Sidecar binary: `cmd/sidecar/main.go`

### Next
1. Config system for ModelCatalog + policies (YAML/JSON).
2. LangChain adapter (Python SDK).
3. Observability (logs, metrics, tracing).
4. CLI for local runs and debugging.

---

## Documentation Update Rule

After adding a new feature, review and update these files if needed:
- `runtime/STATUS.md`
- `docs/2025_12_26/design/runtime-layer-v1-draft.md`
- `runtime/manifest.json`

---

## Architecture

### Contracts (interfaces)
```
runtime/contracts/
├── interfaces.go      # All 12 component interfaces
├── models.go          # Run, Task, DAG, Usage, Cost, etc.
├── errors.go          # All sentinel errors
├── orchestrator.go    # Orchestrator interface
└── models_catalog.go  # ModelInfo, ModelRole, ModelCatalog
```

### API Layer
```
runtime/api/
├── server.go          # HTTP server, http.ServeMux router
├── handlers.go        # StartRun, GetStatus, AbortRun, EnqueueTask
├── models.go          # Request/Response DTOs
├── errors.go          # Error → HTTP status mapping
└── store.go           # In-memory RunStore

runtime/cmd/sidecar/
└── main.go            # Entry point for sidecar binary
```

### Orchestrator Main Loop
```
1. Init: Validate DAG → RunRunning
2. Ready Queue: Scheduler.NextReady → QueueManager.Enqueue
3. Execute Loop:
   - Dequeue task
   - ContextBuilder.Build → ContextCompactor.Compact
   - TokenEstimator.Estimate → CostCalculator.Estimate
   - BudgetEnforcer.Allow (pre-check)
   - task.State = TaskRunning
   - ParallelExecutor.Execute
   - BudgetEnforcer.Record + UsageTracker.Add
   - ContextRouter.Route to dependents
   - Scheduler.MarkComplete
4. Finalize: RunCompleted / RunFailed / RunAborted
```

### Key Design Decisions
- **ParallelExecutor is "pure"**: doesn't mutate task.State or task.Outputs
- **UsageTracker**: only updates `run.Usage.Tokens` (Cost via BudgetEnforcer.Record)
- **Defensive checks**: ParallelExecutor rejects terminal states (Completed/Failed/Skipped)
- **Deadlock detection**: if no progress and empty queue → ErrDeadlock

---

## Bug Fixes Applied

| # | Issue | Fix | File |
|---|-------|-----|------|
| 1 | ParallelExecutor could execute terminal tasks | Added defensive check | parallel_executor.go:147 |
| 2 | UsageTracker didn't sync run.Usage | Writes directly to run.Usage.Tokens | usage_tracker.go:33 |
| 3 | TokenEstimator returned 0 for short text | Minimum 1 token for non-empty | token_estimator.go:68 |
| 4 | ContextBuilder included non-completed deps | Check TaskCompleted | context_builder.go:52 |

---

## Next Steps (TODO)

### 1. E2E Integration Test ✅ DONE
```
File: internal/orchestration/orchestrator_integration_test.go

Tests (9 total):
- TestIntegration_LinearDAG_ABC     # A → B → C, context routing
- TestIntegration_FanInDAG          # A,B → C (parallel)
- TestIntegration_DiamondDAG        # A → B,C → D
- TestIntegration_SingleTask        # Single task
- TestIntegration_EmptyDAG          # No tasks
- TestIntegration_ContextRouting    # Verify Inputs.Inputs[depID]
- TestIntegration_BudgetExceeded    # Deterministic budget test
- TestIntegration_TaskFailure       # B fails → RunFailed
- TestIntegration_ContextCancellation # Cancel → ErrTaskCancelled

Key implementation details:
- Uses real components (Scheduler, ContextBuilder, BudgetEnforcer, etc.)
- Only ParallelExecutor uses stub (via NewParallelExecutorFromPolicy)
- DAG built through DependencyResolver.BuildDAG (not manual)
- Token calculation accounts for context routing (A=100, B=102, C=102 tokens)
```

### 2. Factory Function ✅ DONE
```go
// File: internal/orchestration/factory.go

// Simple API - uses all defaults
func NewOrchestratorWithDefaults(
    policy contracts.RunPolicy,
    executor TaskExecutorFunc,
) contracts.Orchestrator

// Extended API - custom ModelCatalog/Currency
func NewOrchestratorWithOptions(
    policy contracts.RunPolicy,
    executor TaskExecutorFunc,
    opts FactoryOptions,
) contracts.Orchestrator

// Tests: 6 tests in factory_test.go
// - TestNewOrchestratorWithDefaults
// - TestNewOrchestratorWithDefaults_NilExecutor
// - TestNewOrchestratorWithOptions_CustomCatalog
// - TestNewOrchestratorWithOptions_CustomCurrencyOnly
// - TestFactory_SingleTaskE2E
// - TestFactory_MultiTaskE2E
```

### 3. HTTP API ✅ DONE
```
Files:
- api/models.go      # Request/Response DTOs
- api/errors.go      # Error mapping to HTTP status codes
- api/store.go       # In-memory RunStore with mutex
- api/handlers.go    # HTTP handlers
- api/server.go      # Server + http.ServeMux router (Go 1.22+)
- api/server_test.go # 14 tests
- cmd/sidecar/main.go # Entry point

Endpoints:
- POST /api/v1/runs           → StartRun (202)
- GET  /api/v1/runs/{id}      → GetStatus (200)
- POST /api/v1/runs/{id}/abort → AbortRun (200, fire-and-forget)
- POST /api/v1/runs/{id}/tasks → EnqueueTask (501, V1 limitation)

Key design:
- Policy from request only (stateless API)
- "aborting" is API-level state (not contracts.RunState)
- Duplicate run ID → 409 Conflict
- DAG cycle → 422 Unprocessable Entity
- Budget exceeded → 422 (not 402)
```

---

## Related Documentation

- **Design Doc**: `docs/2025_12_26/design/runtime-layer-v1-draft.md`
- **Component Manifest**: `runtime/manifest.json`
- **Tiered Prompts**: `runtime/prompts/tier1.md`, `tier2.md`, `tier3.md`
- **Project Instructions**: `CLAUDE.md`

---

## Test Commands

```bash
# Run all tests
go test ./... -v

# Run with coverage
go test ./... -cover

# Run specific package
go test ./internal/orchestration/... -v
go test ./api/... -v

# Run integration tests only
go test ./internal/orchestration/... -run Integration -v
go test ./api/... -run TestServer -v

# Run specific test
go test ./internal/orchestration/... -run TestOrchestrator -v

# Build sidecar binary
go build -o sidecar ./cmd/sidecar/

# Run sidecar (default :8080)
./sidecar -addr :8080
```
