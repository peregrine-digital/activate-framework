---
name: instruction-authoring
description: Create or update app/module-specific instruction files in host repos. Provides templates for vets-website (React) and vets-api (Rails) instruction files. Use when the Documenter needs to scaffold or improve an instruction file.
---

# Instruction Authoring

Create or update `.github/instructions/{name}.instructions.md` files in host repos.

**Asset:** `assets/template.instructions.md`

## When to Use

- No instruction file exists yet for the app/module — scaffold one from template
- Existing instruction file is missing sections or patterns discovered during the session

## Rules

- **Target repo only** — instruction files go in the host project, never in copilot-config
- **Replace all placeholders** — no `{like-this}` left in the final file
- **Prune empty sections** — remove anything not applicable rather than leaving blanks
- **Frontmatter required** — `applyTo` must match the app/module path
- **Self-maintenance section stays** — always keep the "When to Update" guidance so future agents maintain the file

## Flow

```mermaid
flowchart TD
    Start[Documenter identifies gap] --> Exists{Instruction file exists?}
    Exists -->|Yes| Update[Add missing sections/patterns]
    Exists -->|No| Copy[Copy assets/template.instructions.md to target repo .github/instructions/]
    Copy --> Fill[Fill sections from session context + codebase]
    Fill --> Prune[Remove sections that don't apply]
    Update & Prune --> Done[Instruction file ready]
```
