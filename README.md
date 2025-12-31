# claude-workflow

`claude-workflow` is a runtime sidecar for executing agent workflows with deterministic scheduling, cost controls, and context routing. It provides an HTTP API, a thin workflow client, and a minimal Python SDK.

## What This Repo Contains

- **Runtime sidecar (Go)**: orchestrates DAG execution with policy enforcement.
- **Workflow client (Go CLI)**: submits runs or workflow configs to the sidecar.
- **Python SDK (v1)**: sync-only client that sends opaque StartRunRequest payloads.
- **Workflow agents**: optional sub-agents for Claude Code (workflow layer).

## Quick Start (Runtime)

```bash
# Build sidecar
cd runtime
go build -o sidecar ./cmd/sidecar/

# Start sidecar
./sidecar -addr :8080
```

Submit a run:

```bash
curl -X POST http://localhost:8080/api/v1/runs \
  -H "Content-Type: application/json" \
  -d '{
    "id": "workflow-001",
    "policy": {
      "max_parallelism": 2,
      "timeout_ms": 300000,
      "budget_limit": {"amount": 5.0, "currency": "USD"}
    },
    "tasks": [
      {"id": "analyze", "prompt": "Analyze requirements", "model": "claude-3-haiku-20240307"},
      {"id": "design", "prompt": "Design architecture", "model": "claude-3-sonnet-20240229", "deps": ["analyze"]}
    ]
  }'
```

## Workflow Config (CLI)

```bash
# Build client
cd runtime
go build -o workflow-client ./cmd/workflow-client/

# Submit a workflow config
./workflow-client submit-config --file ../examples/spec-default-workflow.json
```

## Execution Audit

The sidecar can write per-run JSON snapshots:

```bash
./sidecar -addr :8080 -audit-dir ./runtime/audit
```

On completion, it writes `run-<id>.json` to the audit directory and emits structured `[AUDIT]` logs.

## Python SDK (v1)

```bash
export PYTHONPATH="${PYTHONPATH}:$(pwd)/sdk/python"
python examples/python/start_run.py
```

The SDK is sync-only and treats the request payload as opaque (no schema helpers).

## Repo Structure

```
claude-workflow/
├── agents/                 # Workflow agents (optional, for Claude Code)
├── commands/               # Claude Code slash commands
├── docs/                   # Project documentation
├── examples/               # Example workflows and SDK usage
├── runtime/                # Go runtime sidecar + client
└── sdk/                    # Python SDK (v1)
```

## Documentation

- Runtime integration: `docs/workflow-integration.md`
- Workflow config: `docs/workflow-config.md`
- Runtime status: `runtime/STATUS.md`
- Architecture intent: `CLAUDE.md`

## License

See `LICENSE`.
