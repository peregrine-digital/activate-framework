---
version: '0.2.0'
applyTo: "{path-to-app-or-module}/**/*"
description: "Patterns and conventions for {Name}"
---

# {Name} Instructions

> Supplements `.github/copilot-instructions.md` with patterns specific to this app/module.

## 🔄 Self-Maintenance

Update this file when you add or change anything listed below. Future agents depend on this being accurate.

## Overview
- **Purpose**: [What this does for veterans — one sentence]
- **Entry point**: [Main file path]
- **State namespace**: [How to access this app's state, if applicable]

## Non-Obvious Architecture

Document only what you can't easily infer from reading the code:

- [Key architectural decision and why it was made]
- [Unusual indirection or abstraction that would confuse a new reader]
- [External service dependencies and how auth works]

## Key Abstractions

Patterns that save significant time across tickets:

### {Abstraction Name}
- **Where**: [file path]
- **What it does**: [one sentence]
- **Usage**: 
```
// Example
```

## Business Rules

Rules that live in people's heads, not in code comments:

- [Rule that affects implementation but isn't obvious]
- [Validation or workflow restriction with context on WHY]

## Constants & Config

Only document if locations are non-obvious:

- **Constants**: [path]
- **Feature flags**: [which ones and what they gate]
- **Error codes**: [where defined, how mapped to UI]

## Testing Shortcuts

Patterns that prevent re-discovery on every ticket:

- **Test setup**: [non-obvious setup steps]
- **Fixtures/mocks**: [where they live, how to add new ones]
- **Common gotcha**: [thing that breaks tests and the fix]

## Anti-patterns

Mistakes that have actually happened:

- ❌ [Thing that seems right but breaks something — and why]
- ❌ [Pattern that's tempting but has a better alternative]

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| [What you see] | [Why it happens] | [How to fix] |
