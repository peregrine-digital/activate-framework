---
name: creating-github-agents
description: Use when creating GitHub Copilot custom agents, agent profiles, or .agent.md files for repository, organization, or enterprise use
---

# Creating GitHub Copilot Custom Agents

## Overview

A GitHub Copilot custom agent is a Markdown file (`*.agent.md`) with YAML frontmatter (metadata + tool access) and a prompt body (behavior + guardrails). Keep agents narrowly-scoped, explicit about tool access, and accurate about GitHub.com vs IDE support.

## When to Use

Use this skill when you need to:
- Add or update a `*.agent.md` file
- Decide repo vs org/enterprise agent placement
- Choose a safe tool allowlist
- Document MCP requirements without incorrectly embedding repo-level MCP config

## File Structure

### Naming
- File must end with `.agent.md`.
- Filename allowed chars: `.`, `-`, `_`, `a-z`, `A-Z`, `0-9`.

### Placement (scope)

| Scope | Location |
|------:|----------|
| Repository | `.github/agents/<name>.agent.md` |
| Organization / enterprise | `agents/<name>.agent.md` |

Note: naming conflicts are resolved by *lowest level wins* (repo overrides org, org overrides enterprise) based on the filename (minus suffix).

## Agent Profile Template

```yaml
---
name: my-agent               # optional; defaults to filename
description: Required; what this agent is for
# tools: ["read", "search", "edit"]
# target: github-copilot     # optional
---

Prompt body (max 30,000 chars)
```

### Frontmatter rules that matter on GitHub.com
- `description` is required.
- `model`, `argument-hint`, and `handoffs` are IDE-oriented and ignored on GitHub.com Copilot coding agent.

## Tools (important guardrail)

Best practice: use a **minimal allowlist**.

### Valid tool aliases (don’t invent tool names)
Use these aliases (case-insensitive):
- `execute` (compatible: `shell`, `bash`, `powershell`)
- `read`
- `edit`
- `search`
- `agent` (for invoking other custom agents)

Notes:
- `web` / `WebSearch` / `WebFetch` are not applicable to GitHub.com Copilot coding agent.
- Use MCP server tool names with namespacing like `some-mcp/tool-1` or `some-mcp/*`.

### Tools syntax
Both of these are valid YAML:

```yaml
tools: ["read", "search", "edit", "execute"]
```

```yaml
tools:
  - read
  - search
  - edit
  - execute
```

## MCP server guidance (GitHub.com)

- Repository-level custom agents **cannot** configure `mcp-servers` in the agent file.
- Repo agents *can* use MCP tools that are configured in repository settings.
- Org/enterprise agents may include `mcp-servers:` in YAML frontmatter, and should still explicitly allowlist the tools they need.

## Prompt best practices

Include, in this order:
1. **Role & scope** (first 1–2 sentences)
2. **Responsibilities** (bullets)
3. **Out-of-scope / guardrails** (explicit “do not” list)
4. **Validation** (what commands/checks must be run before completion)
5. **PR output requirements** (what evidence to include)

Avoid:
- Over-promising access (“can configure MCP in repo-level agent”)
- Vague tool lists (“terminal”, “git”) that don’t match supported aliases
- Mega-prompts; prefer `AGENTS.md` / `.github/copilot-instructions.md` / `.github/instructions/*.instructions.md` for repo-wide or path-scoped guidance

## Minimal example (planner-style)

```markdown
---
name: implementation-planner
description: Creates implementation plans and acceptance criteria for a change
tools: ["read", "search", "edit"]
---

You are a technical planning specialist.

You must:
- Restate requirements as acceptance criteria
- Identify unknowns and propose 2 options with tradeoffs
- Produce a checklist plan and call out validation steps

You must not:
- Implement code
- Modify infrastructure state
```

## Common mistakes

| Mistake | Fix |
|---|---|
| Putting org-wide agents under `.github/agents/` | Use `agents/` at repo root for org/enterprise-level agents |
| Using unsupported tool names (`terminal`, `git`, etc.) | Use aliases like `execute`, `read`, `edit`, `search` |
| Including `mcp-servers` in a repo-level agent file | Configure MCP in repo settings; reference MCP tools via `server/tool` |
| Relying on `model` on GitHub.com | Treat `model` as IDE-only; GitHub.com ignores it |
