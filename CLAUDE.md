# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Claude Sub-Agent Spec Workflow System - A comprehensive AI-driven development workflow system built on Claude Code's Sub-Agents feature. This system transforms project ideas into production-ready code through specialized AI agents working in coordinated phases.

## Runtime Layer (Go)

A sidecar runtime for LLM agent orchestration with cost control and context management.

**Quick Reference:**
- **Status**: See `runtime/STATUS.md` for current state and next steps
- **Design**: See `docs/2025_12_26/design/runtime-layer-v1-draft.md`
- **Components**: See `runtime/manifest.json`

**Key Directories:**
```
runtime/
├── contracts/           # Interfaces, types, errors
├── internal/
│   ├── orchestration/   # Scheduler, Executor, Orchestrator
│   ├── cost/            # TokenEstimator, BudgetEnforcer, ModelCatalog
│   └── context/         # ContextBuilder, Compactor, Router
└── STATUS.md            # Current status and TODO
```

**Run Tests:**
```bash
cd runtime && go test ./... -v
```

## Project Documentation Conventions (Important)

**Documentation Files:** All new documentation or task files must be saved under the `docs/` folder in this repository.For example:

- **Tasks & TODOs**: Save in `docs/{YYYY_MM_DD}/tasks/` (e.g., `docs/t2025_08_08/asks/ReleaseTodo.md` for a release checklist).
- **Requirements/Specs**: Save in `docs/{YYYY_MM_DD}/specs/` (e.g., `docs/2025_08_08/specs/AuthModuleRequirements.md`).
- **Design Docs**: Save in `docs/{YYYY_MM_DD}/design/` (e.g., `docs/2025_08_08/design/ArchitectureOverview.md`).
- **Code Files:** Follow the project structure (place new code in the appropriate src/module folder as discussed).
- **Tests:** Put new test files under the `tests/` directory, mirroring the code structure.

> **Important:** When creating a new file, ensure the directory exists or create it. Never default to the root directory for these files.

## Common Development Commands

### Workflow Execution

```bash
# Execute complete development workflow using slash command
/agent-workflow "Create a todo list web application with user authentication"

# Start workflow manually with orchestrator
Use spec-orchestrator: Create an enterprise CRM system with multi-tenancy support

# Phase-specific execution
Use spec-analyst: Analyze requirements for an e-commerce platform
Use spec-architect: Design system architecture for microservices
Use spec-developer: Implement user authentication based on specifications
```

### Quality Gates and Testing

```bash
# The system includes three automated quality gates:
# Gate 1: Planning Quality (95% threshold) - After spec-planner
# Gate 2: Development Quality (80% threshold) - After spec-tester  
# Gate 3: Production Readiness (85% threshold) - After spec-validator

# Manual validation
Use spec-validator: Evaluate code quality and provide scoring
Use spec-tester: Generate comprehensive test suite for the implementation
```

### Project Structure Operations

```bash
# Copy agents to a new project
mkdir -p .claude/agents .claude/commands
cp agents/* .claude/agents/
cp commands/agent-workflow.md .claude/commands/

# Organize agent files (current reorganization in progress)
# Backend agents: agents/backend/
# Frontend agents: agents/frontend/ 
# Spec workflow agents: agents/spec-agents/
# UI/UX agents: agents/ui-ux/
# Utility agents: agents/utility/
```

## System Architecture

### Multi-Phase Workflow Design

The system follows a three-phase approach with quality gates:

1. **Planning Phase (20-25% of project time)**
   - spec-analyst: Requirements analysis and user stories
   - spec-architect: System architecture and API design
   - spec-planner: Task breakdown and estimation
   - Quality Gate 1: 95% compliance threshold

2. **Development Phase (60-65% of project time)**
   - spec-developer: Code implementation following specifications
   - spec-tester: Comprehensive test suite generation
   - Quality Gate 2: 80% compliance threshold

3. **Validation Phase (15-20% of project time)**
   - spec-reviewer: Code review and best practices validation
   - spec-validator: Final production readiness assessment
   - Quality Gate 3: 85% compliance threshold

### Agent Categories

**Workflow Agents (spec-agents/)**

- spec-orchestrator: Workflow coordination and quality gate management
- spec-analyst: Requirements analysis specialist
- spec-architect: System architecture designer  
- spec-planner: Task breakdown and planning
- spec-developer: Implementation specialist
- spec-tester: Testing expert
- spec-reviewer: Code review specialist
- spec-validator: Final validation expert

**Domain Specialists**

- senior-frontend-architect: React/Vue/Next.js expert
- senior-backend-architect: Go/TypeScript backend systems
- ui-ux-master: UI/UX design and implementation

**Utility Agents**

- refactor-agent: Code quality and refactoring specialist

### Quality Framework

Each phase includes automated quality gates with specific thresholds:

- Requirements completeness validation
- Architecture feasibility assessment
- Code quality metrics and test coverage
- Security vulnerability scanning
- Production deployment readiness

### Agent Communication Protocol

Agents communicate through structured artifacts:

- Each agent produces specific documentation (requirements.md, architecture.md, etc.)
- Next agent uses previous outputs as input
- Orchestrator manages the workflow progression
- Quality gates ensure consistency and standards compliance

## Expected Output Structure

```
project/
├── docs/
│   ├── requirements.md      # Detailed requirements specification
│   ├── architecture.md      # System architecture design
│   ├── api-spec.md         # API specifications and contracts
│   └── user-stories.md     # User stories with acceptance criteria
├── src/
│   ├── components/         # Reusable components
│   ├── services/          # Business logic services
│   ├── utils/             # Utility functions
│   └── types/             # Type definitions
├── tests/
│   ├── unit/              # Unit tests
│   ├── integration/       # Integration tests
│   └── e2e/               # End-to-end tests
├── package.json           # Project dependencies
└── README.md              # Project documentation
```

## Key Integration Points

### Slash Command Integration

The `/agent-workflow` command provides one-command execution of the entire development pipeline:

- Supports quality threshold configuration (--quality=75-95)
- Allows agent skipping (--skip-agent=spec-analyst)
- Phase-specific execution (--phase=planning|development|validation)
- Language selection (--language=zh|en)

### Sub-Agent Chain Process

The system uses Claude Code's sub-agent syntax for coordinated execution:

```
First use the spec-analyst sub agent → then spec-architect sub agent → then spec-developer sub agent → then spec-validator sub agent → quality gate decision → if score ≥95% continue to spec-tester, otherwise loop back with feedback
```

### Quality Gate Mechanism

- Validation Score ≥95%: Proceed to next phase
- Validation Score <95%: Loop back with specific feedback
- Maximum 3 iterations to prevent infinite loops
- Expected progression: Round 1 (80-90%) → Round 2 (90-95%) → Round 3 (95%+)

## Best Practices

### For Working with Agents

- Start with spec-orchestrator for complete projects
- Use domain specialists for specific expertise areas
- Allow each agent to complete their phase before intervention
- Trust the quality gate system for consistent standards
- Review artifacts between phases for course correction

### For Project Setup

- Copy all agents and slash command to project's .claude directory
- Provide clear project descriptions with constraints and requirements
- Specify quality expectations (75% for prototypes, 95% for enterprise)
- Include existing documentation when available

### For Customization

- Adjust quality thresholds based on project needs
- Skip agents for simpler projects (e.g., skip spec-analyst if requirements exist)
- Use phase-specific execution for targeted improvements
- Integrate with existing CI/CD workflows

## Troubleshooting

### Common Issues

- **Agent Not Found**: Verify agents are in correct .claude/agents directory
- **Quality Gate Failures**: Review specific criteria, allow agents to revise work
- **Workflow Stuck**: Check orchestrator status, restart from last checkpoint

### Debug Mode

Enable verbose logging by requesting: "Use spec-orchestrator with debug mode and show all agent interactions"

## Integration with External Systems

The system can be integrated with:

- GitHub Actions for CI/CD validation
- Custom quality gates and validation criteria  
- Domain-specific workflows and specialized orchestrators
- Existing development tools and frameworks
