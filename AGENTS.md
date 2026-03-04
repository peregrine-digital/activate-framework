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
├── mise.toml                        # Go + Node toolchain
│
├── cli/                             # Go CLI (sub-packages, single binary)
│   ├── main.go                      #   entry point, arg parsing, printJSON
│   ├── model/                       #   pure types + config schema (stdlib only)
│   │   ├── config.go                #     Config type, MergeConfig, defaults
│   │   ├── manifest.go              #     Manifest, ManifestFile, TierDef
│   │   ├── tiers.go                 #     tier resolution, SelectFiles, InferCategory
│   │   ├── types.go                 #     RepoSidecar, InstallState, FileStatus
│   │   ├── versions.go              #     ParseFrontmatterVersion, FileDisplayName
│   │   └── helpers.go               #     FindManifestByID, FindManifestFile
│   ├── transport/                   #   JSON-RPC wire format (stdlib only)
│   │   ├── jsonrpc.go               #     Transport type, Request/Response/Notification
│   │   └── protocol.go             #     method constants, typed params/results
│   ├── storage/                     #   disk I/O primitives (→ model)
│   │   ├── config.go                #     ResolveConfig, Read/Write config
│   │   ├── sidecar.go               #     Read/Write/DeleteRepoSidecar
│   │   ├── gitexclude.go            #     SyncGitExclude, RemoveGitExcludeBlock
│   │   ├── filewriter.go            #     WriteManifestFile (always remote fetch)
│   │   ├── manifest_cache.go        #     WriteManifestCache, ReadManifestCache
│   │   ├── mcp.go                   #     MCP server config read/write/merge
│   │   └── fetcher.go               #     FetchFile, FetchJSON, DefaultRepo/DefaultBranch
│   ├── engine/                      #   business logic (→ storage, model)
│   │   ├── manifest.go              #     DiscoverRemoteManifests, InstallFilesFromRemote
│   │   ├── repo.go                  #     RepoAdd, RepoRemove
│   │   ├── operations.go            #     UpdateFiles, InstallSingleFile, DiffFile
│   │   ├── status.go                #     ComputeFileStatuses, DetectInstallState
│   │   ├── diff.go                  #     UnifiedDiff (LCS algorithm)
│   │   └── telemetry.go             #     Copilot quota tracking (IsTelemetryEnabled, RunTelemetry)
│   ├── commands/                    #   command processors (→ engine, storage, model, transport)
│   │   ├── service.go               #     ActivateAPI interface, ActivateService facade
│   │   ├── cli.go                   #     RunUpdateCommand, RunDiffCommand, RunSyncCommand
│   │   └── daemon.go                #     JSON-RPC daemon, all handlers
│   ├── selfupdate/                  #   binary self-update via GitHub releases
│   │   └── selfupdate.go            #     CheckUpdate, Run (→ go-selfupdate)
│   ├── tui/                         #   interactive Bubbletea client
│   │   ├── app.go                   #     RunInteractiveInstall, RunList
│   │   ├── menu.go                  #     RunInteractiveMenu, main menu model
│   │   ├── style/
│   │   │   └── style.go             #     brand colors, lipgloss styles, RenderBanner
│   │   └── screens/
│   │       ├── files.go             #     RunFileBrowser
│   │       ├── settings.go          #     RunSettings
│   │       └── telemetry.go         #     RunTelemetryScreen
│   ├── Makefile                     #   cross-compile, npm-stage, publish
│   ├── go.mod / go.sum              #   Go module (Charm dependencies)
│   └── npm/                         #   npm distribution wrapper
│       ├── package.json             #     @anthropic/activate-cli
│       ├── bin/activate             #     JS shim → spawns Go binary
│       ├── install.js               #     postinstall: validate binary
│       └── platforms/               #     per-platform packages
│           ├── darwin-arm64/         #       @anthropic/activate-cli-darwin-arm64
│           ├── darwin-x64/           #       @anthropic/activate-cli-darwin-x64
│           ├── linux-arm64/          #       @anthropic/activate-cli-linux-arm64
│           ├── linux-x64/            #       @anthropic/activate-cli-linux-x64
│           └── win32-x64/            #       @anthropic/activate-cli-win32-x64
│
├── skills/                          # shared skills (cross-plugin)
├── mcp-servers/                     # shared MCP server configs (cross-plugin)
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
│   ├── scripts/prepare-assets.mjs   #   copies install.sh → assets/ at build time
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
  "repo": "peregrine-digital/activate-framework",
  "branch": "main",
  "fileOverrides": { "dest/path.md": "pinned" | "excluded" },
  "skippedVersions": { "dest/path.md": "0.5.0" }
}
```

- `.activate.json` is **auto-excluded from git** via `.git/info/exclude` (managed marker block). It must never be committed.
- CLI modules: `cli/model/config.go` (type + merge), `cli/storage/config.go` (read/write)
- Extension module: `extension/src/config.js` (CJS, auto-discovers workspace root)

### Delivery Mode

**Remote-only** — manifests and source files are always fetched from GitHub (`repo`/`branch` in config, defaults in `storage.DefaultRepo`/`storage.DefaultBranch`). There are no local bundles. A manifest cache at `~/.activate/repos/<hash>/manifest-cache.json` provides offline fallback. Files are injected into the workspace's `.github/` directory and hidden from git via `.git/info/exclude`. The sidecar `.github/.activate-installed.json` tracks what's installed.