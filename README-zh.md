# claude-workflow (legacy filename)

This file is kept for compatibility with the original fork name but is maintained in English. See `README.md` for the primary overview.

## Overview

`claude-workflow` is a runtime sidecar for executing agent workflows with deterministic scheduling, cost controls, and context routing. It provides an HTTP API, a thin workflow client, and a minimal Python SDK.

## Quick Start (Runtime)

```bash
cd runtime
go build -o sidecar ./cmd/sidecar/
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

## Docs

- Runtime integration: `docs/workflow-integration.md`
- Workflow config: `docs/workflow-config.md`
- Runtime status: `runtime/STATUS.md`
- Architecture intent: `CLAUDE.md`
