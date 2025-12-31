# Contracts-First Parallel: Problems and Solutions

## Context

**Project:** Runtime Layer for agentic LLM systems (sidecar, Go, LangChain)

**Approach:** Contracts-First Parallel Development
- spec-architect generates contracts (interfaces, DTO, errors)
- Simple components -> parallel via Haiku
- Complex components -> Sonnet/Opus
- Codex -> only for GitHub code review

**Architecture document:** `docs/2025_12_26/design/runtime-layer-v1-draft.md`

---

## Problem 1: Token consumption ✅ RESOLVED

### Essence
With parallel agent runs, each agent receives the full system prompt, contracts, and context. This duplication increases cost by 80%.

### Solution: Tiered Prompts

```
TIER 1 (800-1,000 tokens) — Haiku
  Structure: <role> + <contract> (inline) + <pattern> (1 example) + <task> + <success_criteria>
  For: deterministic logic, pure computation, stateless

TIER 2 (1,500-2,200 tokens) — Sonnet
  Adds: <dependencies> + <business_rules> + <patterns> (2-3) + <edge_cases>
  For: coordination, state management, component integration

TIER 3 (3,000-4,500 tokens) — Opus
  Adds: <security> + <architecture_context> + <thinking_instruction>
  For: critical logic, cannot be wrong
```

### Best Practices for prompts (from Anthropic docs)

**Structure:**
- XML tags: `<role>`, `<contract>`, `<pattern>`, `<task>`, `<success_criteria>`
- Logical order: context -> data -> task -> criteria

**Key principles:**
1. Smallest high-signal tokens — minimum tokens, maximum value
2. Goldilocks zone — do not hardcode logic, but be specific
3. Inline contracts — pass in the prompt, do not read via Read
4. 1-3 diverse examples — from the real project, not a laundry list
5. Explicit instructions — "Implement X" not "Can you suggest"
6. Success criteria — testable conditions in every prompt
7. Say WHAT to do, not WHAT NOT — with explanation WHY

**Exclude from prompts:**
- CLAUDE.md (for the main session)
- Workflow instructions (the agent does one task)
- Laundry list edge cases
- Redundant tool outputs
- Interfaces of layers without direct dependency

### Savings
- Tokens: 50% less
- Cost vs Sonnet for all: 79% less
- Cost vs Opus for all: 96% less

### Sources
- https://www.anthropic.com/engineering/claude-code-best-practices
- https://www.anthropic.com/engineering/effective-context-engineering-for-ai-agents
- https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-4-best-practices

---

## Problem 2: Complexity classification ✅ RESOLVED

### Essence
How to determine: is this component for Haiku (TIER 1) or Opus (TIER 3)? Who decides? Based on what?

### Solution: Criteria for Runtime Layer

Complexity is not defined by the layer, but by task characteristics:

**TIER 1 — HAIKU (deterministic logic)**
- Pure computations (formulas, calculators)
- Simple data transformations
- Stateless operations
- No side effects
- Easy to cover with unit tests
- Formula/algorithm known in advance

**TIER 2 — SONNET (coordination and state)**
- State management
- Coordination between components
- Decision-making based on state
- Handling edge cases
- Integration of multiple components

**TIER 3 — OPUS (critical logic)**
- Error = customer money (BudgetEnforcer)
- Error = cascade failure (CircuitBreaker)
- Error = information loss (ContextCompactor)
- Error = incorrect execution (Scheduler, ParallelExecutor)
- Integration with external code (Adapters)

### Classification of Runtime Layer v1 components

| Component | TIER | Rationale |
|-----------|------|-----------|
| TokenEstimator | 1 | Pure formula: tokens = f(text) |
| CostCalculator | 1 | Pure formula: cost = tokens × price |
| UsageTracker | 1 | Simple accumulator |
| QueueManager | 2 | State, but simple (in-memory queue) |
| MemoryManager | 2 | Key-value, short-term |
| ContextBuilder | 2 | Assembly by known rules |
| ContextRouter | 2 | Passing data between tasks |
| DependencyResolver | 2 | DAG build and validation |
| **Scheduler** | **3** | Order = correctness of the entire system |
| **ParallelExecutor** | **3** | Race conditions, bounded concurrency |
| **BudgetEnforcer** | **3** | Error = customer overspend |
| **ContextCompactor** | **3** | Error = information loss |

---

## Problem 3: Context Sharing ✅ RESOLVED

### Essence
Agents run in parallel — how do they share context? Is it needed at all?

### Solution: Contracts-First WITHOUT runtime sharing

**Shared types and interfaces in contracts/ eliminate divergence risk.**
Sharing is needed only if a real shared utility or serialization protocol appears.

### What is defined in contracts (runtime-layer-v1-draft.md)

**Base types:**
```go
type RunID string
type TaskID string
type ModelID string
type TokenCount int64
type Currency string
type RunState int   // enum: Pending, Running, Completed, Failed, Aborted
type TaskState int  // enum: Pending, Ready, Running, Completed, Failed, Skipped
```

**Data structures:**
```go
type Run struct { ID, State, Policy, DAG, Tasks, Usage, CreatedAt, UpdatedAt }
type Task struct { ID, State, Inputs, Deps, Outputs, Error, Model, EstimatedUse, ActualUse }
type DAG struct { Nodes, Edges }
type DAGNode struct { ID, Deps, Next, Pending }
type Usage struct { Tokens, Cost }
type Cost struct { Amount, Currency }
type TaskInput struct { Prompt, Inputs, Metadata }
type TaskResult struct { Output, Outputs, Usage, Metadata }
type TaskError struct { Code, Message }
type ContextBundle struct { Messages, Memory, Tools }
type ContextPolicy struct { MaxTokens, Strategy, KeepLastN, TruncateTo }
type RunPolicy struct { TimeoutMs, MaxParallelism, BudgetLimit }
```

**12 interfaces across 3 domains** — fully defined.

### Rules for agents

1. **Agents receive:** only their interface + required types (inline in the prompt)
2. **Agents do NOT create:** new public types, shared helpers, changes in contracts/
3. **Internal code:** each agent writes in its own package (internal/{domain}/{component}.go)
4. **Integration phase:** go build ./... — compile check, find duplicates

### When sharing is NOT needed

| Scenario | Sharing? | Solution |
|----------|----------|----------|
| Shared types (TokenCount) | ❌ | Inline in the prompt |
| Shared structures (Run, Task) | ❌ | Inline in the prompt |
| Shared interfaces | ❌ | Inline in the prompt |
| Shared errors | ❌ | Inline in the prompt |
| Internal helpers | ❌ | Each in its own package |
| Calling another component | ❌ | Via interface |

### When sharing MAY be needed (future)

- A shared utility used by 3+ components
- Serialization protocol (JSON/Protobuf helpers)
- Shared middleware/interceptor

→ Solution: add to contracts/helpers.go or pkg/

---

## Architecture of the solution

```
/parallel-dev "Implement Runtime Layer v1"
      │
      ▼
PHASE 0: Classification
      │ Determine components and their TIER
      ▼
PHASE 1: Contracts (already in runtime-layer-v1-draft.md)
      │ interfaces/, data structures
      ▼
PHASE 2: Implementation (parallel by TIER)
      │
      ├── TIER 1 (Haiku, in parallel):
      │   ├── TokenEstimator
      │   ├── CostCalculator
      │   └── UsageTracker
      │
      ├── TIER 2 (Sonnet, in parallel):
      │   ├── QueueManager
      │   ├── MemoryManager
      │   ├── ContextBuilder
      │   ├── ContextRouter
      │   └── DependencyResolver
      │
      └── TIER 3 (Opus, sequential or with review):
          ├── Scheduler
          ├── ParallelExecutor
          ├── BudgetEnforcer
          └── ContextCompactor
      │
      ▼
PHASE 3: Integration
      │ Compile check, tests, refactor
      ▼
DONE
```

---

## Next steps

1. [x] Solve problem 1 (tokens) — Tiered Prompts
2. [x] Solve problem 2 (classification) — criteria for Runtime
3. [x] Solve problem 3 (context sharing) — Contracts-First without runtime sharing
4. [x] Create contracts/ files — runtime/contracts/ (5 files, compiles)
5. [x] Create prompt templates — runtime/prompts/ (tier1.md, tier2.md, tier3.md)
6. [x] Implement /parallel-dev command — .claude/commands/parallel-dev.md + manifest.json
7. [x] PoC: TokenEstimator (TIER 1, Haiku) — runtime/internal/cost/token_estimator.go
8. [x] PoC: Scheduler (TIER 3, Opus) — runtime/internal/orchestration/scheduler.go
9. [x] PoC: QueueManager (TIER 2, Sonnet) — runtime/internal/orchestration/queue_manager.go
10. [x] Implement all 12 Runtime Layer components

---

## Final report

### Statistics

| Metric | Value |
|---------|----------|
| Components | 12/12 |
| Tests | 195 passed |
| Coverage (context) | 100.0% |
| Coverage (cost) | 98.7% |
| Coverage (orchestration) | 97.8% |
| Race conditions | 0 (verified with -race) |

### Components by TIER

**TIER 1 (Haiku):**
- TokenEstimator ✓
- CostCalculator ✓

**TIER 2 (Sonnet, in parallel):**
- QueueManager ✓
- UsageTracker ✓
- DependencyResolver ✓
- ContextBuilder ✓
- ContextRouter ✓
- MemoryManager ✓

**TIER 3 (Opus, sequential):**
- Scheduler ✓
- BudgetEnforcer ✓
- ParallelExecutor ✓
- ContextCompactor ✓

### Quality Gates

| Tier | Target | Actual | Status |
|------|--------|--------|--------|
| TIER 1 | ≥90% | 98.7% | ✓ |
| TIER 2 | ≥90% | 97-100% | ✓ |
| TIER 3 | ≥95% | 97-100% | ✓ |
