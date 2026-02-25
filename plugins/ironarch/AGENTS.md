# AGENTS.md

Guidelines for AI agents working with the IronArch plugin.
Legend (from RFC2119): !=MUST, ~=SHOULD, ≉=SHOULD NOT, ⊗=MUST NOT, ?=MAY.

## User Approval Required

⛔ This project uses agents as the primary interaction mode. Instructions and skills are NOT auto-applied.

**Before using any instruction or skill:**
- !: Announce which guidance you intend to use
- !: Wait for explicit user approval before proceeding
- !: Quote the specific rules that will be applied

## Working Principles

**Do**
- !: Verify required tools before starting (GitHub MCP, gh CLI, Screenshot Viewer MCP for frontend)
- !: Detect repo type first: `src/applications/` = vets-website, `modules/` = vets-api
- !: Follow handoff chain: Planner → Implementer → Tester → Reviewer → Documenter → PR_Writer
- !: Update `tmp/copilot-session/session.md` at each handoff
- !: Get user approval before applying any instruction or skill
- ~: Atomic commits following conventional commit format

**Do Not**
- ⊗: Auto-apply instructions or skills without user approval
- ⊗: Create session artifacts in plugin directory (use target project's `tmp/copilot-session/`)
- ⊗: Guess file paths, PR numbers, or issue details — ask if unfetchable

## Required Tools

| Tool | Verify | Scope |
|------|--------|-------|
| GitHub MCP | `mcp_github_get_me` | Both repos |
| Screenshot Viewer MCP | `mcp_screenshot-vi_list_screenshots` | vets-website |
| gh CLI | `gh auth status` | Both repos |

## Guidance Tiers

| Tier | Location | Approval? |
|------|----------|-----------|
| 1 | `AGENTS.md` | No — always active |
| 2 | `instructions/*.instructions.md` | **Yes** |
| 3 | `skills/[name]/SKILL.md` | **Yes** |
| 4 | `agents/*.agent.md` | No — user selects |

## Adding Instructions

- ?: Copy template from `skills/instruction-authoring/assets/`
- !: Only add patterns agents wouldn't discover on their own
- ~: Every instruction adds cognitive load — be minimal

## Troubleshooting

- **Agents not appearing** → Save and reopen VS Code workspace
- **MCP not starting** → Open `.vscode/mcp.json`, click "Start"
- **Environment check failing** → Run `gh auth status`
