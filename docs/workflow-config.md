# Workflow Configuration

Static workflow configurations define fixed agent chains for the claude-workflow runtime.

**Primary format**: JSON. YAML support may be added in a future version.

## JSON Format

```json
{
  "workflow": {
    "name": "default-spec-flow",
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
      }
    ]
  }
}
```

## Fields

### workflow.name (required)

A human-readable name for the workflow.

### workflow.steps (required)

An array of step definitions. Must contain at least one step.

### step.id (required)

Unique identifier for the step within the workflow.

### step.role (required)

Agent role for this step. Required roles that must be present:

- `spec-analyst`
- `spec-architect`
- `spec-developer`
- `spec-validator`

### step.depends_on (optional)

Array of step IDs that must complete before this step can run.

### step.outputs (optional)

Array of output artifact paths produced by this step.

## Validation Rules

1. `workflow.name` must be non-empty
2. `workflow.steps` must contain at least one step
3. Each `step.id` must be unique
4. Each `step.role` must be non-empty
5. All `depends_on` references must point to existing step IDs
6. No cycles allowed in dependencies
7. All required roles must be present

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
| `required role is missing` | Missing spec-analyst/architect/developer/validator |

## Usage

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

## Custom Workflows

By default, the validator requires all spec workflow roles (spec-analyst, spec-architect, spec-developer, spec-validator). For custom workflows with different roles, disable this check:

```go
validator := config.NewValidatorWithOptions(config.ValidatorOptions{
    RequireDefaultRoles: false,
})
err := validator.Validate(cfg)
```

This allows workflows like:

```json
{
  "workflow": {
    "name": "data-pipeline",
    "steps": [
      {"id": "fetch", "role": "data-fetcher"},
      {"id": "process", "role": "data-processor", "depends_on": ["fetch"]},
      {"id": "store", "role": "data-writer", "depends_on": ["process"]}
    ]
  }
}
```
