# Workflow → Runtime Integration

Runtime sidecar provides HTTP API for executing DAG-based workflows.
Workflow layer is a **thin client** — it only submits DAGs and polls status.

## System Flow

```
User / LangChain
      |
      | 1) Define workflow (DAG + policy)
      v
Workflow Layer (agents/skills)
      |
      | 2) Serialize StartRunRequest (JSON)
      v
workflow-client (thin CLI client)
      |
      | 3) POST /api/v1/runs
      v
Runtime Sidecar (execution engine)
      |
      | 4) Execute DAG with policy (budget/parallelism)
      v
Run Status / Results (GET /api/v1/runs/{id})
```

Key points:
- Workflow defines *what* to do; runtime enforces *how* it executes.
- Runtime is model-agnostic; agents/skills can change without touching runtime.
- Any tool (CLI, LangChain, IDE) can submit a run via the same HTTP API.

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

## CLI Client

A thin CLI client is provided for submitting runs and checking status.

### Build

```bash
cd runtime && go build -o workflow-client ./cmd/workflow-client/
```

### Usage

```bash
# Submit a run from JSON file
./workflow-client submit --file run.json --addr http://localhost:8080

# Check run status
./workflow-client status --id workflow-001 --addr http://localhost:8080
```

### Example JSON (run.json)

Note: `id` is optional. If omitted, runtime generates a run ID (e.g., `run-<unix_nano>`).

```json
{
  "id": "workflow-001",
  "policy": {
    "max_parallelism": 4,
    "timeout_ms": 300000,
    "budget_limit": { "amount": 5.0, "currency": "USD" },
    "context_policy": {
      "max_tokens": 8000,
      "strategy": "truncate",
      "keep_last_n": 50
    }
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
}
```

### Output

```bash
# submit
run_id=workflow-001 state=running

# status (completed)
run_id=workflow-001 state=completed
tasks: analyze=completed, design=completed, implement=completed

# status (failed with task error code)
run_id=workflow-001 state=failed
tasks: analyze=completed, design=failed(execution_failed)
error: [task_failed] task design execution failed: ...
```

## Python SDK (v1)

Minimal Python SDK for programmatic access.

**Key characteristics:**
- **Sync-only, blocking** — no async/await
- **Opaque request** — `start_run(request)` sends dict as-is, no validation
- **X-Runtime-Version: v1** — header included in all requests
- **No dependencies** — uses only Python standard library

### Installation

```bash
# From project root
export PYTHONPATH="${PYTHONPATH}:$(pwd)/sdk/python"
```

### Usage

```python
from claude_workflow import RuntimeClient
from claude_workflow import RuntimeError  # Note: not builtin RuntimeError

client = RuntimeClient("http://localhost:8080")

# Start a run (request is opaque dict - SDK doesn't validate)
request = {
    "policy": {
        "timeout_ms": 300000,
        "max_parallelism": 2,
        "budget_limit": {"amount": 5.0, "currency": "USD"}
    },
    "tasks": [
        {"id": "analyze", "prompt": "Analyze this", "model": "claude-3-haiku-20240307"},
        {"id": "design", "prompt": "Design that", "model": "claude-3-sonnet-20240229", "deps": ["analyze"]}
    ]
}
response = client.start_run(request)
print("Started run: {}".format(response["id"]))

# Poll status
status = client.get_status(response["id"])
print("State: {}".format(status["state"]))

# Abort if needed
try:
    client.abort_run(response["id"])
except RuntimeError as e:
    print("Cannot abort: [{}] {}".format(e.code, e.message))
```

**Note:** `claude_workflow.RuntimeError` is a custom exception (not Python's builtin). Use explicit import to avoid confusion.

## Execution Audit (v1)

The sidecar can emit structured audit logs and write a per-run JSON snapshot.

### Enable audit files

```bash
./sidecar -addr :8080 -audit-dir ./runtime/audit
```

On completion, the sidecar writes `run-<id>.json` to the audit directory and logs events with the `[AUDIT]` prefix.

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
