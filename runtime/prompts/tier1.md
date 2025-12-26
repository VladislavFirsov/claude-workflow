# TIER 1 Prompt Template

**Model:** Haiku
**Tokens:** 800-1,000
**For:** Deterministic logic, pure computations, stateless operations

---

## Template

```
<role>
You implement {{COMPONENT_NAME}} in Go for the Runtime Layer.
Write production-quality code. No placeholders or TODOs.
</role>

<types>
{{REQUIRED_TYPES}}
</types>

<interface>
{{INTERFACE_DEFINITION}}
</interface>

<pattern>
{{ONE_PROJECT_EXAMPLE}}
</pattern>

<task>
Implement {{INTERFACE_NAME}}.
Write to: {{FILE_PATH}}
Package: {{PACKAGE_NAME}}
</task>

<do_not>
- Do NOT modify contracts/ files
- Do NOT add public types (export only the struct implementing interface)
- Do NOT add dependencies on other internal packages
- Do NOT use external libraries
</do_not>

<success_criteria>
- Implements all interface methods
- Uses provided types only
- Follows the pattern style
- Handles errors explicitly
- No external dependencies beyond stdlib
</success_criteria>
```

---

## Example: TokenEstimator

```
<role>
You implement TokenEstimator in Go for the Runtime Layer.
Write production-quality code. No placeholders or TODOs.
</role>

<types>
type TokenCount int64

type TaskInput struct {
    Prompt   string
    Inputs   map[string]string
    Metadata map[string]string
}

type ContextBundle struct {
    Messages []string
    Memory   map[string]string
    Tools    map[string]string
}
</types>

<interface>
// TokenEstimator estimates the number of tokens for a task before execution.
type TokenEstimator interface {
    // Estimate returns the estimated token count for a task.
    Estimate(input *TaskInput, ctx *ContextBundle) (TokenCount, error)
}
</interface>

<pattern>
// Example from project: simple estimator pattern
type simpleEstimator struct {
    charsPerToken int
}

func NewSimpleEstimator(charsPerToken int) *simpleEstimator {
    return &simpleEstimator{charsPerToken: charsPerToken}
}

func (e *simpleEstimator) DoSomething(input string) int {
    return len(input) / e.charsPerToken
}
</pattern>

<task>
Implement TokenEstimator.
Write to: internal/cost/token_estimator.go
Package: cost
</task>

<do_not>
- Do NOT modify contracts/ files
- Do NOT add public types
- Do NOT add dependencies on other packages
</do_not>

<success_criteria>
- Implements Estimate method
- Counts tokens from Prompt, Inputs, Messages, Memory, Tools
- Uses ~4 chars per token as default heuristic
- Returns error only for nil inputs
- No external dependencies
</success_criteria>
```

---

## Placeholders

| Placeholder | Description |
|-------------|-------------|
| `{{COMPONENT_NAME}}` | Name of component (e.g., "TokenEstimator") |
| `{{REQUIRED_TYPES}}` | Only types used by this interface |
| `{{INTERFACE_DEFINITION}}` | Full interface with comments |
| `{{ONE_PROJECT_EXAMPLE}}` | One similar pattern from project |
| `{{INTERFACE_NAME}}` | Interface name |
| `{{FILE_PATH}}` | Target file path |
| `{{PACKAGE_NAME}}` | Go package name |
