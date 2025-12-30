# Workflow Configuration

Static workflow configurations define fixed agent chains for the claude-workflow runtime.

**Primary format**: JSON. YAML support may be added in a future version.

## Quick Start

End-to-end example using the spec-default workflow:

```bash
# 1. Build the binaries (from runtime directory)
cd runtime
go build ./cmd/sidecar
go build ./cmd/workflow-client

# 2. Start the sidecar (in a separate terminal)
./sidecar

# 3. Submit a workflow
./workflow-client submit-config --file ../examples/spec-default-workflow.json
# Output: run_id=spec-default-example state=running (or pending)

# 4. Check status
./workflow-client status --id spec-default-example
# Output: run_id=spec-default-example state=running
# tasks: analysis=running, architecture=pending, implementation=pending, validation=pending
```

## Workflow Types

The `workflow.type` field controls validation behavior:

| Type | Required Roles | Order | Chain | Optional Placement |
|------|---------------|-------|-------|-------------------|
| `""` (empty) | Must be present | No | No | No |
| `"custom"` | Skipped entirely | No | No | No |
| `"spec-default"` | Exactly once each | Yes | Yes | Yes |

## spec-default Workflow

The canonical spec workflow with strict validation rules.

```json
{
  "workflow": {
    "name": "default-spec-flow",
    "type": "spec-default",
    "steps": [
      {
        "id": "analysis",
        "role": "spec-analyst",
        "outputs": ["requirements.md", "user-stories.md"]
      },
      {
        "id": "architecture",
        "role": "spec-architect",
        "depends_on": ["analysis"],
        "outputs": ["architecture.md", "api-spec.md"]
      },
      {
        "id": "implementation",
        "role": "spec-developer",
        "depends_on": ["architecture"],
        "outputs": ["src/", "tests/"]
      },
      {
        "id": "validation",
        "role": "spec-validator",
        "depends_on": ["implementation"],
        "outputs": ["validation-report.md"]
      },
      {
        "id": "testing",
        "role": "spec-tester",
        "depends_on": ["validation"]
      }
    ]
  }
}
```

### Required Roles (in canonical order)

1. `spec-analyst`
2. `spec-architect`
3. `spec-developer`
4. `spec-validator`

### Optional Roles

- `spec-tester`
- `spec-reviewer`

### Validation Rules for spec-default

1. Each required role must appear exactly once
2. Required roles must appear in canonical order
3. Each required step must depend on the previous required step
4. Optional roles must depend on `spec-validator` only
5. No unknown roles allowed

## Custom Workflows

Use `type: "custom"` to skip required role validation entirely:

```json
{
  "workflow": {
    "name": "data-pipeline",
    "type": "custom",
    "steps": [
      {"id": "fetch", "role": "data-fetcher"},
      {"id": "process", "role": "data-processor", "depends_on": ["fetch"]},
      {"id": "store", "role": "data-writer", "depends_on": ["process"]}
    ]
  }
}
```

## Fields

### workflow.name (required)

A human-readable name for the workflow.

### workflow.type (optional)

Workflow type for validation. Values: `"spec-default"`, `"custom"`, or empty.

### workflow.steps (required)

An array of step definitions. Must contain at least one step.

### step.id (required)

Unique identifier for the step within the workflow.

### step.role (required)

Agent role for this step.

### step.depends_on (optional)

Array of step IDs that must complete before this step can run.

### step.outputs (optional)

Array of output artifact paths produced by this step.

### workflow.models (optional)

Map of role names to Claude model IDs. Overrides CLI default model selection.

```json
{
  "workflow": {
    "name": "custom-models",
    "models": {
      "spec-analyst": "claude-opus-4-20250514",
      "spec-architect": "claude-sonnet-4-20250514",
      "spec-developer": "claude-sonnet-4-20250514",
      "spec-validator": "claude-haiku-4-20250514"
    },
    "steps": [...]
  }
}
```

Model resolution order:
1. `workflow.models[role]` if defined
2. CLI default for known roles
3. Fallback model with warning

## Error Messages

| Error | Description |
|-------|-------------|
| `workflow.name is required` | Name field is empty |
| `workflow.steps must not be empty` | No steps defined |
| `step.id is required` | Step has empty ID |
| `duplicate step.id` | Two steps have the same ID |
| `step.role is required` | Step has empty role |
| `depends_on references unknown step id` | Invalid dependency reference |
| `cycle detected in step dependencies` | Circular dependency found |
| `required role is missing` | Missing required role |
| `required role appears more than once` | Duplicate required role in spec-default |
| `required roles must be in canonical order` | Wrong order in spec-default |
| `required step must depend on previous required step` | Broken chain in spec-default |
| `optional role must depend on spec-validator` | Optional role in wrong position |
| `unknown role for spec-default workflow` | Role not in required or optional list |

## CLI Submission

Submit a workflow config directly to the runtime:

```bash
# Basic submission (uses workflow.name as run ID)
workflow-client submit-config --file workflow.json

# With custom server address
workflow-client submit-config --file workflow.json --addr http://localhost:8080

# With custom run ID
workflow-client submit-config --file workflow.json --run-id my-run-123

# Check status
workflow-client status --id my-run-123
```

The CLI converts workflow config to a StartRunRequest with:
- Default timeout: 5 minutes
- Default parallelism: 1 (sequential)
- Default budget: $10 USD

## Go API Usage

```go
import "github.com/anthropics/claude-workflow/runtime/config"

loader := config.NewLoader()

// Load from file
cfg, err := loader.LoadFromFile("workflow.json")
if err != nil {
    // Handle error
}

// Or load from bytes
cfg, err := loader.LoadFromBytes(jsonData)
```
