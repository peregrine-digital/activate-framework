---
version: '0.1.0'
applyTo: "extension/**/*"
description: "Patterns and conventions for the VS Code extension"
---

# Extension Instructions

> Supplements general instructions with patterns specific to the VS Code extension (`extension/`).

## 🔄 Self-Maintenance

Update this file when you add or change anything listed below. Future agents depend on this being accurate.

## Overview
- **Purpose**: VS Code extension that manages Activate framework files via a CLI daemon over JSON-RPC
- **Entry point**: `extension/src/extension.js` → `activate(context)`
- **Test runner**: `node:test` — run with `cd extension && npm test`

## Non-Obvious Architecture

- **Daemon lifecycle**: `activate()` → `startDaemon()` → `autoSetup()`. The daemon is a Go binary spawned as a child process. `startDaemon` registers `onDidChangeSessions` and an exit handler before `autoSetup` runs. If daemon setup throws, `autoSetup` is silently skipped due to a top-level try/catch.
- **`autoSetup` flow**: Two distinct paths based on daemon state:
  - `!state.state.hasInstallMarker` → `showQuickStartPrompt()` → `repoAdd()` (first-time file injection)
  - `state.state.hasInstallMarker` → `sync()` (pick up version changes)
  - **Note:** `state.state` is an `InstallState` **object** (`{hasInstallMarker, hasGlobalConfig, hasProjectConfig}`), not a string. Never compare it with `===`.
  - **Do not** call `sync()` for first install — it short-circuits when no sidecar exists. `repoAdd()` is the correct entry point.
- **Config scopes**: `client.getConfig('global')` reads `~/.activate/config.json`; `client.setConfig({ ..., scope: 'project' })` writes project config. The `scope` parameter is passed through JSON-RPC as `{ scope: 'global' | 'project' }`.

## Testing Shortcuts

### Mock completeness for `vscode.authentication`
**When to use:** Adding any test that exercises the `activate()` → `startDaemon()` → `autoSetup()` path.

The `vscodeMock.authentication` object **must** include `onDidChangeSessions`:
```js
authentication: {
  getSession: async () => authSession,
  onDidChangeSessions: () => ({ dispose: () => {} }),
},
```
**Anti-pattern:**
```js
authentication: {
  getSession: async () => authSession,
  // Missing onDidChangeSessions — startDaemon() will throw TypeError
},
```
**Why:** `startDaemon()` calls `vscode.authentication.onDidChangeSessions(...)` and pushes the disposable to `context.subscriptions`. If the method is missing, a TypeError is thrown inside `activate()`'s try/catch — silently swallowed. `autoSetup()` never runs, and tests that depend on it will fail with misleading assertion errors (e.g., "setConfig was never called").

*Surfaced in: quickstart-defaults session — caused a debugging spiral until the missing mock was identified.*

### Config mock scoping convention
**When to use:** Writing tests that call `client.getConfig(scope)`.

Use `config_${scope}` keys in `_mockResults` to return scope-specific config:
```js
// Extension test mock (aligned with controlPanel test pattern)
mockClient._mockResults.config_global = { manifest: 'ironarch', tier: 'workflow' };
mockClient._mockResults.config_project = { tier: 'standard' };
```
**Anti-pattern:**
```js
// Returns same result regardless of scope — fragile
mockClient._mockResults.getConfig = { manifest: 'ironarch' };
```
**Why:** The `MockClient.getConfig(scope)` resolves `_mockResults[config_${scope}]` with fallback to `_mockResults.getConfig`. The `config_${scope}` convention is already used in `controlPanel.test.js` — using it consistently prevents scope-blind mocking that hides bugs when code calls `getConfig('global')` vs `getConfig('project')` in the same flow.

*Surfaced in: quickstart-defaults review finding #1.*

### Command handler error convention
**When to use:** Registering any new `vscode.commands.registerCommand` handler that calls async client methods.

Wrap the handler body in try/catch with `showErrorMessage`:
```js
vscode.commands.registerCommand('activate-framework.myCommand', async () => {
  if (!requireClient()) return;
  try {
    const state = await client.getState();
    // ... do work ...
  } catch (err) {
    vscode.window.showErrorMessage(`My command failed: ${err.message}`);
  }
});
```
**Anti-pattern:**
```js
// Missing try/catch — errors silently disappear, user gets no feedback
vscode.commands.registerCommand('activate-framework.myCommand', async () => {
  if (!requireClient()) return;
  await client.getState();
});
```
**Why:** `activate()` has a top-level try/catch that silently swallows errors. If a command handler throws without its own try/catch, the user sees nothing — the command appears to do nothing.

*Surfaced in: quickstart-defaults review Pass 5 — quickStart handler was missing try/catch.*

### Queue-based mock for sequential `showInformationMessage` calls
**When to use:** Testing code paths that call `vscode.window.showInformationMessage` with buttons where you need to control the user's response.

Use a `infoMessageResults` array (queue) that shifts values:
```js
let infoMessageResults = [];
// In mock:
showInformationMessage: (msg, ...rest) => {
  shownMessages.push(msg);
  const buttons = rest.filter((r) => typeof r === 'string');
  if (buttons.length > 0 && infoMessageResults.length > 0) {
    return Promise.resolve(infoMessageResults.shift());
  }
  return Promise.resolve(undefined);
},
```
**Anti-pattern:**
```js
// Single value — can't test flows that show multiple sequential dialogs
let infoMessageResult = 'Quick Start';
```
**Why:** Code may show multiple `showInformationMessage` dialogs in sequence (e.g., a prompt followed by a confirmation). A single-value mock returns the same answer for all of them. The queue pattern lets you script sequential responses: `infoMessageResults = ['Quick Start', 'Yes']`.

*Surfaced in: quickstart-defaults session — modal dialog refactor required sequential mock control.*

## Anti-patterns

- ❌ **Calling `sync()` for first-time installs** — `sync()` short-circuits with "not installed" when no sidecar (`.github/.activate-installed.json`) exists. Use `repoAdd()` for first install, `sync()` only for already-installed repos. Mixing them up produces silent no-ops.
- ❌ **Using `_mockResults.getConfig` for scope-specific tests** — Returns same data for all scopes. Use `config_global` / `config_project` keys instead (see Testing Shortcuts above).

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Tests pass but `autoSetup` assertions fail (e.g., "setConfig never called") | `vscode.authentication.onDidChangeSessions` missing from mock — `startDaemon()` throws, caught silently | Add `onDidChangeSessions: () => ({ dispose: () => {} })` to auth mock |
| `sync()` returns "not installed" on first activation | Wrong RPC — `sync` requires existing sidecar | Use `repoAdd()` for first-time install |
| Config mock returns same data for global and project scope | Using unscoped `_mockResults.getConfig` | Use `_mockResults.config_global` / `_mockResults.config_project` keys |
