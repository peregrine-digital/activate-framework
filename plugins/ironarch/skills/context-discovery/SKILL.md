---
name: context-discovery
description: Detect the target repository and identify the specific application or module being worked on. Use when starting any task to determine workspace context from git changes, open files, ticket URLs, or user input.
---

# Context Discovery

Detect repository and identify the target application/module.

## Prerequisites

- **GitHub MCP** (optional) — Only for ticket URL detection. Verify with `mcp_github_get_me`.

## Flow

```mermaid
flowchart TD
    Start[Start] --> Signals{Context signals?}
    Signals -->|Ticket URL| GH[GitHub MCP: issue_read / pull_request_read]
    Signals -->|Git changes| Diff[git diff --name-only main...HEAD]
    Signals -->|Open file| Path[Extract from file path]
    Signals -->|None| Ask[Ask user]
    
    GH --> Extract[Extract app from labels/paths]
    Diff --> Extract
    Path --> Extract
    Ask --> Extract
    
    Extract --> Session[Write to session.md]
    Session --> Confirm[Confirm with user]
```

## Detection Methods

1. **From ticket** — Check labels, scan body for file paths
2. **From git diff** — Extract app folder from changed files
3. **From current file** — Parse path for app/module name

## Output

Write to `session.md` header:

- **Application:** `{app-name}`
- **Application Path:** `{path/to/app}`

## Rules

- Ignore `copilot-config` folders
- Never guess — use signals or ask user
