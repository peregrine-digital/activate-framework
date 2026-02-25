# AGENTS.md

Guidelines for AI agents and human contributors working with the IronArch plugin.

## ⛔ User Approval Required

**CRITICAL: This project uses agents as the primary interaction mode.** Guidance files (instructions and skills) are NOT automatically applied.

Before using ANY instruction file (`.instructions.md`) or skill (`SKILL.md`) when modifying, reviewing, or generating code, agents MUST:

1. **Announce** which instruction or skill they intend to use
2. **Wait for explicit user approval** before proceeding
3. **Quote the specific guidance** that will be applied

Example interaction:
```
Agent: I'll use the context-discovery skill to identify the target application.
       Specifically, I'll check for:
       - src/applications/ (vets-website)
       - modules/ + app/controllers/ (vets-api)
       
       Proceed? (yes/no)

User: yes

Agent: [proceeds with skill]
```

This ensures users maintain control over which automated guidance influences their code.

---

## Required Tools

These tools MUST be available before agent work begins.

| Tool | How to Verify | Required For |
|------|---------------|--------------|
| GitHub MCP | `mcp_github_get_me` returns login | Both repos |
| Screenshot Viewer MCP | `mcp_screenshot-vi_list_screenshots` returns data | vets-website only |
| gh CLI | `gh auth status` shows authenticated | Both repos |

**Detect repo type first:** Check for `src/applications/` (vets-website) or `modules/` + `app/controllers/` (vets-api).

---

## Behavioral Requirements

### Path Resolution

This plugin injects files into workspaces alongside target projects:

- **Skills** → Injected to `.github/skills/`
- **Instructions** → Injected to `.github/instructions/`
- **Agents** → Injected to `.github/agents/`
- **Session artifacts** → Create in the **target project's** `tmp/copilot-session/` directory

### Agent Workflow

Agents follow a handoff chain. Each updates `tmp/copilot-session/session.md`:

```
Planner → Implementer → Tester → Reviewer → Documenter → PR_Writer
```

The `session-management` skill defines the session lifecycle.

### Guidance Approval

| Tier | Location | Approval Required? |
|------|----------|--------------------|
| 1 | `AGENTS.md` | No — always active |
| 2 | `instructions/*.instructions.md` | **Yes** |
| 3 | `skills/[name]/SKILL.md` | **Yes** |
| 4 | `agents/[name].agent.md` | No — user explicitly selects |

---

## Adding Application-Specific Instructions

When patterns emerge that should persist across sessions:

1. Copy the template from `skills/instruction-authoring/assets/`
2. Rename to `instructions/{app-or-module-name}.instructions.md`
3. Update `applyTo` frontmatter to match the target path
4. Fill in **only patterns that agents wouldn't discover on their own**

**Remember:** Every instruction adds cognitive load. Include only what changes agent behavior in ways the project requires.

---

## Troubleshooting

### Agents not appearing
- Save and reopen the VS Code workspace
- Ensure Activate extension is installed and enabled

### MCP servers not starting
- Open `.vscode/mcp.json` and click "Start" on each server
- Complete GitHub browser authentication when prompted

### Environment check failing
- Run `gh auth status` to verify CLI authentication
- Ensure Screenshot Viewer MCP is running (vets-website only)
