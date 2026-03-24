# Activate Copilot

A starter kit for agent-driven development workflows.

## Installation

### Option 1: Script (recommended)

```bash
node install.mjs
```

Follow the prompts to choose assistant target, tier, and install directory.

### Option 2: Skill

Open `../plugins/adhoc/skills/activate-installer/SKILL.md` in your AI agent and invoke it.

> **Note:** The CLI engine lives in `../framework/` (install, config, manifest discovery). Plugin content (instructions, prompts, skills, agents) lives under `../plugins/`.

## Tiers

| Choice | Contents |
|--------|----------|
| **minimal** | Core workflow guidance: AGENTS.md, instructions, prompts |
| **standard** | Core + ad-hoc: adds language/practice instructions, skills, agents |
| **advanced** | Standard + advanced tooling (when available) |

## Version

The installed version is recorded in `manifests/adhoc.json`.
