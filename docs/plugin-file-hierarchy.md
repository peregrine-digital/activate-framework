# Plugin File Hierarchy

This document defines the required directory structure for plugins in the Activate Framework. All plugins under `plugins/` must follow this hierarchy.

## Overview

The hierarchy establishes four tiers of AI agent guidance, from project-wide to task-specific:

```
plugins/{plugin-name}/
├── AGENTS.md                              # Tier 1: Always active
├── instructions/*.instructions.md         # Tier 2: Auto-applied by glob
├── prompts/*.prompt.md                    # Tier 2: Manual /command
├── skills/{skill-name}/SKILL.md           # Tier 3: On-demand procedures
└── agents/*.agent.md                      # Tier 4: Specialized personas
```

## Tier Definitions

### Tier 1: AGENTS.md (Required)

| Aspect | Description |
|--------|-------------|
| **Location** | Plugin root |
| **Scope** | All work using this plugin |
| **Invocation** | Always active |
| **Content** | Workflow, process, behavioral requirements |

### Tier 2: Instructions & Prompts

#### Instructions (Auto-Applied)

| Aspect | Description |
|--------|-------------|
| **Location** | `instructions/*.instructions.md` |
| **Scope** | Specific languages, roles, or file types |
| **Invocation** | Triggered by `applyTo` glob pattern in frontmatter |
| **Content** | Conventions, patterns, checklists |

**Required frontmatter:**
```yaml
---
description: 'Brief description'
applyTo: '**/*.js'
---
```

#### Prompts (Manual)

| Aspect | Description |
|--------|-------------|
| **Location** | `prompts/*.prompt.md` |
| **Scope** | Single focused task |
| **Invocation** | Manually via `/command-name` in chat |
| **Content** | Task instructions, output format |

**Required frontmatter:**
```yaml
---
description: 'What this prompt does'
---
```

### Tier 3: Skills (On-Demand Procedures)

| Aspect | Description |
|--------|-------------|
| **Location** | `skills/{skill-name}/SKILL.md` |
| **Scope** | Multi-step procedures |
| **Invocation** | Explicitly by user or agent |
| **Content** | Step-by-step workflows |

**Directory structure:**
```
skills/
├── context-discovery/
│   └── SKILL.md
├── session-management/
│   ├── SKILL.md
│   └── assets/          # Optional supporting files
│       └── template.md
```

**Required frontmatter:**
```yaml
---
name: skill-name
description: When and why to use this skill
---
```

### Tier 4: Agents (Specialized Personas)

| Aspect | Description |
|--------|-------------|
| **Location** | `agents/*.agent.md` |
| **Scope** | Defines agent persona and capabilities |
| **Invocation** | Explicitly selected by user |
| **Content** | Role, skills to use, handoff rules |

**Required frontmatter:**
```yaml
---
name: Agent Name
description: What this agent specializes in
tools: ['list', 'of', 'tools']
model: Claude Opus 4.6 (copilot)
handoffs:
  - label: Next Step → Agent
    agent: NextAgent
    prompt: Handoff context
    send: true
---
```

## Precedence

```
┌─────────────────────────────────────────────────────────┐
│                      AGENTS.md                          │
│              (Always active, project-wide)              │
├─────────────────────────────────────────────────────────┤
│          Instructions        │     Prompts              │
│     (Auto via glob pattern)  │  (Manual via /command)   │
├─────────────────────────────────────────────────────────┤
│                       Skills                            │
│            (Invoked explicitly on demand)               │
├─────────────────────────────────────────────────────────┤
│                  Agent Definitions                      │
│      (Invoked explicitly, combines other tiers)         │
└─────────────────────────────────────────────────────────┘
```

1. `AGENTS.md` provides baseline expectations
2. Instructions add context-specific guidance; prompts provide task workflows
3. Skills provide procedures that follow both
4. Agents orchestrate which instructions, prompts, and skills apply

## Validation

Plugins are validated against this structure. Required checks:

- [ ] `AGENTS.md` exists at plugin root
- [ ] Each `*.instructions.md` has `applyTo` frontmatter
- [ ] Each skill folder contains `SKILL.md` with `name` and `description`
- [ ] Each `*.agent.md` has `name` and `description` frontmatter
- [ ] Manifest `category` values match directory structure

## Reference

This hierarchy is derived from [ADR-001](https://github.com/adhocteam/activate-copilot/blob/main/docs/dev/adrs/ADR-001-agent-instructions-skills-files.md) in the activate-copilot repository.
