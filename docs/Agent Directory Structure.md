# Agent Directory Structure

This repo includes optional Claude Code sub-agent profiles. They are used by the workflow layer and are not required for the runtime sidecar.

## Directory Organization

```
agents/
├── spec-agents/                 # Spec workflow agents
│   ├── spec-orchestrator.md     # Master workflow coordinator
│   ├── spec-analyst.md          # Requirements analysis specialist
│   ├── spec-architect.md        # System architecture designer
│   ├── spec-planner.md          # Task breakdown and planning
│   ├── spec-developer.md        # Implementation specialist
│   ├── spec-tester.md           # Comprehensive testing expert
│   ├── spec-reviewer.md         # Code review specialist
│   └── spec-validator.md        # Final validation expert
├── frontend/                    # Frontend specialized agents
│   └── senior-frontend-architect.md
├── backend/                     # Backend specialized agents
│   └── senior-backend-architect.md
├── ui-ux/                       # UI/UX design agents
│   └── ui-ux-master.md
└── utility/                     # Utility agents
    └── refactor-agent.md
```

## Usage (Claude Code)

Copy the agents you want into your project:

```bash
mkdir -p .claude/agents
cp agents/*/*.md .claude/agents/
```

## Notes

- The runtime sidecar does not require these agents to function.
- The workflow layer is a client of the runtime and can be swapped or removed.
