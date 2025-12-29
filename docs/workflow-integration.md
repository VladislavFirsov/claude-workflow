# Workflow → Runtime Integration

Runtime sidecar provides HTTP API for executing DAG-based workflows.
Workflow layer is a **thin client** — it only submits DAGs and polls status.

## Quick Start

### 1. Start Runtime Sidecar

```bash
cd runtime && go build -o sidecar ./cmd/sidecar/
./sidecar -addr :8080
```

### 2. Submit a Run

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H "Content-Type: application/json" \
  -d '{
    "id": "workflow-001",
    "policy": {
      "max_parallelism": 4,
      "timeout_ms": 300000,
      "budget_limit": {"amount": 5.0, "currency": "USD"}
    },
    "tasks": [
      {
        "id": "analyze",
        "prompt": "Analyze requirements",
        "model": "claude-3-haiku-20240307"
      },
      {
        "id": "design",
        "prompt": "Design architecture",
        "model": "claude-3-sonnet-20240229",
        "deps": ["analyze"]
      },
      {
        "id": "implement",
        "prompt": "Implement solution",
        "model": "claude-3-sonnet-20240229",
        "deps": ["design"]
      }
    ]
  }'
```

### 3. Poll Status

```bash
curl http://localhost:8080/api/v1/runs/workflow-001
```

### 4. Abort Run

```bash
curl -X POST http://localhost:8080/api/v1/runs/workflow-001/abort
```

## Response Format

```json
{
  "id": "workflow-001",
  "state": "completed",
  "tasks": {
    "analyze": {"state": "completed", "output": "..."},
    "design": {"state": "completed", "output": "..."},
    "implement": {"state": "completed", "output": "..."}
  },
  "usage": {"tokens": 1500, "cost": {"amount": 0.015, "currency": "USD"}},
  "created_at": 1704067200000,
  "updated_at": 1704067300000
}
```

## Error Codes

When a task fails, the response includes an error with a specific code:

| Code | Description |
|------|-------------|
| `task_not_found` | Task ID referenced but not in run |
| `context_build_failed` | Failed to build task context |
| `context_compact_failed` | Failed to compact context within limits |
| `token_estimation_failed` | Failed to estimate token usage |
| `model_unknown` | Unknown model ID for cost estimation |
| `budget_exceeded` | Execution would exceed budget limit |
| `execution_failed` | Task execution failed |
| `invalid_result` | Executor returned nil or zero usage |
| `scheduler_error` | Internal scheduler error |
| `dag_inconsistent` | DAG node not found (internal error) |
| `routing_failed` | Failed to route output to dependent task |

## Execution Model

### Batched Execution

Tasks are executed in batches:

1. **Scheduler** returns all ready tasks (deps satisfied)
2. **Pre-check** validates budget for each task (sequential, deterministic)
3. **Execute** runs tasks in parallel (bounded by `max_parallelism`)
4. **Merge** applies results sequentially, sorted by TaskID (deterministic)

### Fail-Fast Policy

Any task failure terminates the run immediately:
- No downstream tasks are executed
- `run.state` becomes `failed`
- Error details are in the failed task's `error` field

### Progress Visibility

- Poll `/api/v1/runs/{id}` to see current state
- Shadow state is updated after each successful batch
- Final state is synced when run completes

## Notes

- Runtime is **model-agnostic** — executor is injected
- Workflow layer should NOT contain provider-specific logic
- Context routing happens automatically based on `deps`
- Budget is enforced both pre-execution (estimate) and post-execution (actual)
