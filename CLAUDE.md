# Architecture Intent: claude-workflow

## Document Purpose

This document captures the **architectural intent of the `claude-workflow` project**:
- why it exists;
- the fundamental problem it solves;
- what it should become over time;
- which responsibility boundaries are considered invariant.

The document is for:
- the architect;
- core runtime developers;
- decision-making on the project's дальнейшее развитие.

This is **not usage documentation** and **not a product pitch**.

---

## 1. What `claude-workflow` Is Today (Current State)

`claude-workflow` is a **working prototype (PoC with a completed core)** consisting of two logically connected parts:

### 1. Workflow Layer (Claude Code)
- A set of **sub-agents** (spec-analyst, spec-architect, spec-developer, validator, etc.).
- A formalized development workflow:
   - analysis → architecture → implementation → validation → tests.
- Implemented as a **slash command** for Claude Code.
- Responsible for *what to do and in what order*.

### 2. Runtime Layer (Go Sidecar)
- A separate **runtime-sidecar service**.
- Implements:
   - DAG and dependencies;
   - scheduling and parallel execution;
   - context routing;
   - token accounting and budget enforcement;
   - HTTP API.
- The core is marked **Complete** and covered by tests.
- Actual LLM/tool execution is **injected externally** (runtime is not tied to a specific model).

Important:
> The project **already implements key architectural ideas**,  
> but **is not yet a finished product**.

---

## 2. Why This Project Exists (Core Reason)

The project did not originate from the idea of "building another agent framework."

It arose from a **structural problem** observed in practice:

> LLM systems quickly stop being "scripts"  
> and become **distributed systems without runtime discipline**.

### Symptoms
- cost growth is nonlinear and hard to explain;
- parallel agents duplicate context;
- execution order affects results;
- behavior is hard to reproduce;
- humans become the "message bus" between agents.

This problem:
- **is not solved by prompts**;
- **is not solved by best practices**;
- **is not solved at the workflow-DSL level**.

It is a **runtime problem**, not a logic problem.

---

## 3. Why `claude-workflow` Exists (Essence)

### Short Formulation

> `claude-workflow` exists to **separate the description of an LLM process  
> from its execution** and make that execution **manageable, predictable, and reproducible**.

---

## 4. What Is the Core of the Project (Invariant)

### Architectural Invariant

`claude-workflow` is an **execution & governance runtime**, not:

- UI;
- SaaS;
- a user-facing agent;
- a prompt-writing framework.

### Fundamental Function

> **To be the runtime layer for complex LLM workflows,  
> where execution is a managed resource.**

---

## 5. Responsibility Boundary

### The Project IS responsible for:
- step orchestration (DAG, dependencies);
- parallel execution;
- context routing and isolation;
- cost accounting and limits;
- execution policy (what is allowed / prohibited);
- reproducibility and auditability.

### The Project is NOT responsible for:
- UX and interfaces;
- business logic;
- LLM provider selection;
- generation quality;
- tools (IDE, GitHub, CI).

This boundary is strict.  
Violating it dilutes the project.

---

## 6. Why the Workflow Layer (Sub-Agents) Is Important but Not Primary

The workflow layer:
- demonstrates a **real use case**;
- serves as **dogfooding** for the runtime;
- validates architectural hypotheses.

But strategically:

> Workflow is the **runtime's client**,  
> and the runtime is the **core value**.

---

## 7. What the Project Should Evolve Into

### Target State (North Star)

`claude-workflow` should become:

> a **universal runtime sidecar  
> for complex LLM workflows and agent systems**,  
> regardless of the framework or model.

Characteristics:
- self-hosted;
- contracts-first;
- model-agnostic;
- workflow-agnostic;
- team-oriented (not just for individual users).

---

## 8. How This Relates to CLI Orchestration (Claude / Codex)

CLI orchestration:
- is not a separate product;
- is a **specific use case of the runtime**.

The current manual process:
- Claude writes the plan and code;
- Codex reviews;
- a human routes tasks.

This case:
- fits the runtime architecture perfectly;
- demonstrates why execution control matters;
- validates that the runtime works outside the API world.

---

## 9. Architectural Success Criteria

The project is moving in the right direction if:

- the runtime can be used without Claude Code;
- the workflow can be rewritten without changing the runtime;
- execution policy is explicitly defined and enforced;
- humans stop being the message bus;
- the runtime remains simple and explainable.

---

## 10. Key Thought for the Architect

> `claude-workflow` is not about agents.  
> It is about **execution discipline for LLM processes**.

All architectural decisions should be tested by the question:

> *Does this make execution more manageable,  
> or does it just add "cleverness"?*

If the latter, it is outside the project's scope.
