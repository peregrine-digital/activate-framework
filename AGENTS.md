# AGENTS.md
For agents developing with Activate Framework
Legend (from RFC2119): !=MUST, ~=SHOULD, ≉=SHOULD NOT, ⊗=MUST NOT, ?=MAY.

## Working principles

**Do**
- !: Trunk-based development best practices
- !: Implement small, well scoped changes that follow existing conventions for a single task at a time.
- !: Keep all tests and policy checks green. Test prior to PR
- !: Maintain quality and automation and ADR discipline.
- ~: Create simple, maintainable designs over clever abstractions.
- ~: Follow TDD. 
- !: Atomic commits as you go, following conventional commit format
- !: After finishing work (all todos done, create pr), review session and propose recommended improvements to AGENTS.md, custom agents, and skills.
- !: Push branch changes to remote, open PR

**Do Not**

- ⊗: introduce secrets, tokens, credentials, or private keys in any form.
- ⊗: Redesign the architecture without explicit instruction or approval
- ⊗: Introduce new tools or services without explicit instruction or approval
- ⊗: Make large sweeping changes across many apps or modules without explicit approval

## Code Map

```
activate-framework/
├── AGENTS.md                        # ← you are here
├── install.mjs                      # root CLI entry point (thin wrapper)
├── mise.toml                        # Node 20 toolchain
│
├── skills/                          # shared skills (cross-plugin)
├── mcp-servers/                     # shared MCP server configs (cross-plugin)
│
├── framework/                       # shared CLI engine (plugin-agnostic)
│   ├── install.mjs                  #   interactive CLI installer
│   ├── core.mjs                     #   manifest discovery, tier maps, category grouping
│   ├── config.mjs                   #   config read/write (ESM) — shared schema
│   ├── fetcher.mjs                  #   GitHub fetcher for remote installs
│   ├── list.mjs                     #   list files as JSON
│   └── __tests__/                   #   framework-side tests (node --test, ESM)
│
├── manifests/                       # manifest registry (one JSON per plugin)
│   ├── activate-framework.json      #   basePath → plugins/activate-framework
│   └── ironarch.json                #   basePath → plugins/ironarch
│
├── plugins/                         # content plugins (deliverable assets)
│   ├── activate-framework/          #   core plugin
│   │   ├── instructions/            #     .instructions.md files
│   │   ├── prompts/                 #     .prompt.md files
│   │   ├── skills/                  #     plugin-specific skills
│   │   └── agents/                  #     .agent.md files
│   └── ironarch/                    #   VA copilot-config plugin
│       ├── skills/                  #     ironarch-specific skills
│       ├── agents/                  #     workflow agents (planner, implementer, etc.)
│       └── mcp-server/              #     screenshot-viewer MCP server
│
├── extension/                       # VS Code extension (GUI wrapper around CLI logic)
│   ├── package.json                 #   extension manifest, commands, views
│   ├── scripts/prepare-assets.mjs   #   copies plugin files → assets/ at build time
│   └── src/
│       ├── extension.js             #   activation, command registration, autoSetup
│       ├── controlPanel.js          #   WebviewView sidebar (HTML render + messages)
│       ├── config.js                #   config read/write (CJS, vscode.workspace.fs)
│       ├── injector.js              #   inject files into workspace .github/, sidecar tracking
│       ├── installer.js             #   legacy workspace-mode helpers (read bundled manifests, etc.)
│       ├── manifest.js              #   selectFiles, TIER_MAP, listByCategory, inferCategory
│       ├── commands/
│       │   ├── changeTier.js        #     QuickPick → write config → re-inject
│       │   ├── changeManifest.js    #     QuickPick → write config → re-inject
│       │   └── showStatus.js        #     info message with install state
│       └── __tests__/               #   extension-side tests (node --test, CJS)
│
└── docs/                            # documentation & templates
    ├── README.md
    ├── EXAMPLE-USAGE.md
    └── templates/                   #   scaffold templates for new plugins
```

### Config System

Two-layer JSON config, same schema everywhere:

| Layer | Path | Scope |
|-------|------|-------|
| Global | `~/.activate/config.json` | User-wide defaults |
| Project | `.activate.json` (workspace root) | Per-project overrides |

**Precedence:** built-in defaults < global < project < CLI flags / programmatic overrides

**Schema:**
```json
{
  "manifest": "activate-framework",
  "tier": "standard",
  "fileOverrides": { "dest/path.md": "pinned" | "excluded" },
  "skippedVersions": { "dest/path.md": "0.5.0" }
}
```

- `.activate.json` is **auto-excluded from git** via `.git/info/exclude` (managed marker block). It must never be committed.
- CLI module: `framework/config.mjs` (ESM, takes `projectDir`)
- Extension module: `extension/src/config.js` (CJS, auto-discovers workspace root)

### Delivery Mode

**Inject-only** — files are copied into the workspace's `.github/` directory and hidden from git via `.git/info/exclude`. The sidecar `.github/.activate-installed.json` tracks what's installed. There is no workspace-mode / multi-root option.