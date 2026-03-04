# Architecture

Activate Framework has three delivery surfaces — a Go CLI, a VS Code extension, and a terminal TUI — unified by a shared service layer and JSON-RPC daemon protocol. This document describes how the pieces fit together and how each component works internally.

## System Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                        User Interfaces                           │
│                                                                  │
│   ┌──────────────┐   ┌──────────────┐   ┌──────────────────┐     │
│   │   CLI        │   │   TUI        │   │   VS Code        │     │
│   │   Commands   │   │   (Charm)    │   │   Extension      │     │
│   └──────┬───────┘   └──────┬───────┘   └────────-┬────────┘     │
│          │                  │                     │              │
│          │   Direct call    │   Direct call       │  JSON-RPC    │
│          │                  │                     │  over stdio  │
│          ▼                  ▼                     ▼              │
│   ┌─────────────────────────────────────────────────────────┐    │
│   │                  ActivateService (Go)                   │    │
│   │                                                         │    │
│   │   State · Config · Manifests · Files · Tiers · MCP      │    │
│   └──────────────────────────┬──────────────────────────────┘    │
│                              │                                   │
│          ┌───────────┬───────┼───────┬────────────┐              │
│          ▼           ▼       ▼       ▼            ▼              │
│       Config      Manifest  Installer  Fetcher   Repo            │
│       (2-layer)   Discovery  (local)   (GitHub)  Sidecar         │
│                                                  + gitexclude    │
└──────────────────────────────────────────────────────────────────┘
```

The CLI and TUI call the service directly (same process). The extension spawns the daemon (`activate serve --stdio`) and communicates via JSON-RPC 2.0 over Content-Length framed stdio.

---

## CLI (Go)

### Source Layout

The CLI is organized into sub-packages with a strict dependency DAG (no import cycles):

```
cli/
├── main.go                  # Entry point, arg parsing, printJSON
├── model/                   # Pure types + config schema (stdlib only)
│   ├── config.go            # Config type, MergeConfig, defaults
│   ├── manifest.go          # Manifest, ManifestFile, TierDef, FormatManifestList
│   ├── tiers.go             # Tier resolution, SelectFiles, InferCategory, ListByCategory
│   ├── types.go             # RepoSidecar, InstallState, FileStatus, TelemetryEntry
│   ├── versions.go          # ParseFrontmatterVersion, FileDisplayName
│   └── helpers.go           # FindManifestByID, FindManifestFile, ContainsString
├── transport/               # JSON-RPC wire format (stdlib only)
│   ├── jsonrpc.go           # Transport type, Request/Response/Notification
│   └── protocol.go          # Method constants, typed params/results
├── storage/                 # Disk I/O primitives (→ model)
│   ├── config.go            # ActivateBaseDir, ResolveConfig, Read/Write config
│   ├── sidecar.go           # SidecarPath, Read/Write/DeleteRepoSidecar
│   ├── gitexclude.go        # SyncGitExclude, RemoveGitExcludeBlock
│   ├── filewriter.go        # WriteManifestFile
│   ├── mcp.go               # MCP server config read/write/merge
│   └── fetcher.go           # FetchFile, FetchJSON (GitHub raw/API)
├── engine/                  # Business logic (→ storage, model)
│   ├── manifest.go          # DiscoverManifests, DiscoverRemoteManifests
│   ├── repo.go              # RepoAdd, RepoRemove
│   ├── operations.go        # UpdateFiles, InstallSingleFile, DiffFile, SyncNeeded
│   ├── installer.go         # InstallFiles, ResolveBundleDir
│   ├── diff.go              # UnifiedDiff (LCS algorithm)
│   └── telemetry.go         # Copilot quota tracking, ComputeFileStatuses, DetectInstallState
├── commands/                # Command processors (→ engine, storage, model, transport)
│   ├── service.go           # ActivateAPI interface, ActivateService facade
│   ├── cli.go               # RunUpdateCommand, RunDiffCommand, RunSyncCommand
│   └── daemon.go            # JSON-RPC daemon, all handlers
├── tui/                     # Interactive Bubbletea client
│   ├── app.go               # RunInteractiveInstall, RunList, installer model
│   ├── menu.go              # RunInteractiveMenu, main menu model
│   ├── style/
│   │   └── style.go         # Brand colors, lipgloss styles, RenderBanner
│   └── screens/
│       ├── files.go         # RunFileBrowser
│       ├── settings.go      # RunSettings
│       └── telemetry.go     # RunTelemetryScreen
├── go.mod / go.sum          # Go module (Charm dependencies only)
├── Makefile                 # Cross-compile, npm-stage, publish
└── npm/                     # npm distribution wrapper
    ├── package.json         #   @anthropic/activate-cli
    ├── bin/activate         #   JS shim → spawns Go binary
    ├── install.js           #   postinstall: validate binary
    └── platforms/           #   per-platform binary packages
```

### Dependency DAG

Packages form a strict, acyclic dependency graph:

```
main → tui, commands, engine, storage, model, transport
tui → tui/screens, tui/style, commands, engine, model
tui/screens → tui/style, commands, model
commands → engine, storage, model, transport
engine → storage, model
storage → model
transport, tui/style, model → stdlib only
```

Key design choices behind this split:

- **Config TYPE in `model/`, config PERSISTENCE in `storage/`** — avoids circular dependencies between the pure data layer and I/O layer.
- **`RepoSidecar` type in `model/`** — allows both `storage/` and `engine/` to reference it without import cycles.
- **`ComputeFileStatuses` and `DetectInstallState` in `engine/`** — these perform I/O via `storage/`, so they belong in the business-logic layer, not in `model/`.
- **Fetcher split** — HTTP primitives (`FetchFile`, `FetchJSON`) live in `storage/`; discovery logic (`DiscoverManifests`, `DiscoverRemoteManifests`) lives in `engine/`.
- **`tui/style/` sub-package** — breaks the potential `tui` ↔ `tui/screens` import cycle by extracting shared styles.
- **`version` const stays in `main.go`**, passed to the Daemon constructor.
- **`printJSON` stays in `main.go`**, passed as a callback to CLI formatters and `RunList`.

### Subcommands

Parsed in `parseArgs()` in `main.go`:

| Command | Purpose | Key Flags |
|---------|---------|-----------|
| `menu` | State-aware interactive menu (default) | `--project-dir` |
| `install` | Interactive installer | `--manifest`, `--tier`, `--target`, `--file`, `--remote` |
| `list` | List available manifests/files | `--manifest`, `--tier`, `--category`, `--json` |
| `state` | Print install/config state | `--json` |
| `config` | Read/write settings | `get`/`set`, `--scope` (global\|project\|resolved) |
| `repo` | Manage repo-level installations | `add`/`remove` |
| `update` | Re-install currently installed files | `--json` |
| `sync` | Detect version mismatch, re-inject | `--json` |
| `diff` | Show diff between bundled & installed | `--file` (required) |
| `serve` | JSON-RPC daemon for extensions | `--stdio` (required) |
| `version` | Print version | — |
| `help` | Show usage | — |

### Command Dispatch Flow

```
main()
  ├─ parseArgs()
  ├─ Discover manifests (local bundle or remote GitHub)
  ├─ Resolve config (defaults < global < project < CLI flags)
  ├─ Create ActivateService
  └─ Route to command handler
```

For interactive commands (`menu`, `install`), the handler launches the TUI. For non-interactive commands, the handler executes directly and prints output (plain text or `--json`).

### Service Layer

The `ActivateAPI` interface (defined in `commands/service.go`) is the central API surface. Both the TUI and daemon use it.

```go
type ActivateAPI interface {
    Initialize(projectDir string)
    GetState() StateResult
    GetConfig(scope string) (*Config, error)
    SetConfig(scope string, updates *Config) (*SetConfigResult, error)
    ListManifests() []Manifest
    ListFiles(manifestID, tierID, category string) (*ListFilesResult, error)
    RepoAdd() (*RepoAddResult, error)
    RepoRemove() error
    Sync() (*SyncResult, error)
    Update() (*UpdateResult, error)
    InstallFile(file string) (*FileResult, error)
    UninstallFile(file string) (*FileResult, error)
    DiffFile(file string) (*DiffResult, error)
    SkipUpdate(file string) (*FileResult, error)
    SetOverride(file, override string) (*FileResult, error)
    RunTelemetry(token string) (*TelemetryRunResult, error)
    ReadTelemetryLog() ([]TelemetryEntry, error)
    RefreshConfig()
    CurrentConfig() Config
    CurrentManifests() []Manifest
    CurrentProjectDir() string
    IsRemoteMode() bool
    RemoteRepo() string
    RemoteBranch() string
}
```

`ActivateService` (in `commands/service.go`) implements this interface and holds runtime state:

```go
type ActivateService struct {
    ProjectDir string
    Manifests  []Manifest
    Config     Config
    UseRemote  bool
    Repo       string
    Branch     string
}
```

---

## Manifest System

### Types

```go
type Manifest struct {
    ID          string         // derived from filename (e.g., "ironarch")
    Name        string         // display name (e.g., "IronArch")
    Description string
    Version     string
    BasePath    string         // relative to repo root (e.g., "plugins/ironarch")
    Tiers       []TierDef      // available tiers
    Files       []ManifestFile // all files in this manifest
}

type ManifestFile struct {
    Src         string // source path relative to basePath
    Dest        string // destination path relative to install dir
    Tier        string // tier ID this file belongs to
    Category    string // instructions, prompts, skills, agents, mcp-servers, other
    Description string
}

type TierDef struct {
    ID    string // e.g., "core", "skills", "workflow"
    Label string // display name
}
```

### Discovery

```
DiscoverManifests(baseDir)
├─ Try baseDir/manifests/*.json (sorted alphabetically)
├─ Walk up parent directories looking for manifests/
└─ Fallback: baseDir/manifest.json (legacy single-manifest)
```

Each `.json` file in `manifests/` is one manifest. The filename (without extension) becomes the manifest ID.

### Remote Discovery

When `--remote` is passed, manifests are fetched from GitHub instead:

```
DiscoverRemoteManifests(repo, branch)
├─ Try manifests/index.json (lists manifest IDs)
├─ Fallback: hardcoded known IDs
└─ LoadRemoteManifest(id, repo, branch) for each
    └─ FetchJSON("manifests/{id}.json", ...) → Manifest
```

---

## Tier System

### Concept

A **tier** is a cumulative file selection level. Each tier includes its own files plus all lower tiers. This lets teams adopt gradually — start with core files, add more as they're ready.

### Resolution

```go
type ResolvedTier struct {
    ID       string   // e.g., "standard"
    Label    string   // display name
    Includes []string // cumulative list of tier IDs included
}
```

Default tiers (when manifest doesn't define its own):

| Tier | Includes |
|------|----------|
| `minimal` | `[core]` |
| `standard` | `[core, ad-hoc]` |
| `advanced` | `[core, ad-hoc, ad-hoc-advanced]` |

When a manifest defines custom tiers (like ironarch's `core`, `skills`, `workflow`), those are used instead. Tier resolution is cumulative — selecting the Nth tier includes all tiers before it.

### File Selection

```go
SelectFiles(files []ManifestFile, manifest Manifest, tierID string) []ManifestFile
```

1. `GetAllowedFileTiers(manifest, tierID)` → set of tier IDs included
2. Filter files where `file.Tier` is in the allowed set
3. Return filtered list

### Category System

Files are organized into display categories for the UI:

| Category | Label |
|----------|-------|
| `instructions` | Instructions |
| `prompts` | Prompts |
| `skills` | Skills |
| `agents` | Agents |
| `mcp-servers` | MCP Servers |
| `other` | Other |

`InferCategory(filePath)` guesses the category from the source path prefix when not explicitly set.

---

## Config System

### Two-Layer Merge

Config is stored as JSON with two layers that merge at resolution time:

| Layer | Path | Scope |
|-------|------|-------|
| Global | `~/.activate/config.json` | User-wide defaults |
| Project | `~/.activate/repos/<sha256(projectDir)>/config.json` | Per-project overrides |

### Schema

```go
type Config struct {
    Manifest         string            // selected manifest ID
    Tier             string            // selected tier ID
    FileOverrides    map[string]string // file dest → "pinned" or "excluded"
    SkippedVersions  map[string]string // file dest → version string to skip
    TelemetryEnabled *bool
}
```

### Resolution Order

```
Built-in defaults (manifest="activate-framework", tier="standard")
    ↓
Global config (~/.activate/config.json)
    ↓
Project config (~/.activate/repos/<hash>/config.json)
    ↓
CLI flags (--manifest, --tier)
    ↓
ResolveConfig() → final merged Config
```

Each layer only overrides non-zero fields. The special value `"__clear__"` (`ClearValue`) unsets a string field back to default.

### Read/Write

```bash
# Read resolved config
activate config get

# Read specific scope
activate config get --scope global
activate config get --scope project

# Write to project scope
activate config set --scope project --tier advanced

# Write to global scope
activate config set --scope global --manifest ironarch
```

---

## Installer

### Local File Copy

```go
InstallFiles(files []ManifestFile, basePath, targetDir, version, manifestID string)
```

For each file:
1. Resolve source: `basePath + file.Src`
2. Resolve destination: `targetDir + file.Dest`
3. Create parent directories
4. Copy file contents

### Bundle Directory Resolution

```
ResolveBundleDir(startDir)
├─ If hasManifests(startDir) → return startDir
├─ Walk up parent directories until hasManifests(dir)
├─ Fallback: startDir/plugins/activate-framework
└─ Error if not found
```

`hasManifests(dir)` checks for `manifests/*.json` or `manifest.json`.

---

## Fetcher (GitHub Remote)

Two transport modes for fetching files from GitHub:

### Raw Mode (Public Repos)

```
GET https://raw.githubusercontent.com/{repo}/{branch}/{filePath}
```

No authentication required. Used when `GITHUB_TOKEN` is not set.

### API Mode (Private Repos or Authenticated)

```
GET https://api.github.com/repos/{repo}/contents/{filePath}?ref={branch}
Authorization: Bearer {GITHUB_TOKEN}
Accept: application/vnd.github.raw+json
```

Used when `GITHUB_TOKEN` environment variable is set. Supports private repositories.

### Key Functions

- `FetchFile(filePath, repo, branch)` → raw bytes (10 MB limit)
- `FetchJSON(filePath, repo, branch, target)` → unmarshal JSON into target
- `InstallFilesFromRemote(files, basePath, targetDir, version, manifestID, repo, branch)` → fetch and write each file

---

## Repo Sidecar & Git Exclude

### Sidecar

When files are installed into a workspace (via `repo add`), a sidecar file tracks what was installed:

```go
type repoSidecar struct {
    Manifest   string   // e.g., "activate-framework"
    Version    string   // e.g., "0.5.0"
    Tier       string   // e.g., "standard"
    Files      []string // relative paths installed
    McpServers []string // MCP server names injected
    Source     string   // "bundled" or "remote"
}
```

The sidecar lives at `{installDir}/.activate-sidecar.json` (typically `.github/.activate-sidecar.json`).

### Git Exclude

Installed files are hidden from git via `.git/info/exclude` (not `.gitignore`, so the exclusions don't affect other developers):

```
# >>> Peregrine Activate (managed — do not edit)
.github/instructions/general.instructions.md
.github/agents/planner.agent.md
.activate.json
# <<< Peregrine Activate
```

The managed block is automatically updated when files are added/removed. On `repo remove`, the entire block is deleted.

---

## Install State

```go
type InstallState struct {
    HasGlobalConfig   bool   // ~/.activate/config.json exists
    HasProjectConfig  bool   // project config exists
    HasInstallMarker  bool   // sidecar exists in workspace
    InstalledManifest string // from sidecar
    InstalledVersion  string // from sidecar
}
```

State drives the TUI's behavior (show installer vs. menu) and the extension's auto-setup logic.

---

## File Versioning

Each file can carry a version in its YAML frontmatter:

```yaml
---
version: '0.5.0'
---
```

`ParseFrontmatterVersion(content)` extracts this. The `FileStatus` type tracks per-file state:

```go
type FileStatus struct {
    Dest             string // destination path
    DisplayName      string
    Category         string
    Tier             string
    Installed        bool   // file exists in workspace
    InTier           bool   // file is in current tier
    BundledVersion   string // version in source bundle
    InstalledVersion string // version in workspace
    UpdateAvailable  bool   // bundled > installed
    Skipped          bool   // user skipped this version
    Override         string // "pinned", "excluded", or ""
}
```

---

## MCP Integration

The CLI manages `.vscode/mcp.json` in the workspace to configure MCP (Model Context Protocol) servers that plugins provide:

- `ReadMcpConfig(projectDir)` → parse existing config
- `WriteMcpConfig(projectDir, config)` → write back
- `MergeMcpServers(existing, new)` → merge without overwriting user additions
- `InjectMcpFromManifest(manifest, projectDir)` → add servers from manifest files with category `mcp-servers`

MCP server configs are JSON files in the plugin (e.g., `mcp-server/screenshot-viewer.json`) that get merged into the workspace's `.vscode/mcp.json`.

---

## Daemon (JSON-RPC Server)

The daemon bridges the CLI's service layer to external consumers (the VS Code extension) via JSON-RPC 2.0 over stdio.

### Launch

```bash
activate serve --stdio
```

### Protocol

Content-Length framed messages over stdin/stdout:

```
Content-Length: 123\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"activate/state","params":{}}
```

### Request/Response Types

```go
type Request struct {
    JSONRPC string          `json:"jsonrpc"` // "2.0"
    ID      json.RawMessage                  // string or number
    Method  string
    Params  json.RawMessage
}

type Response struct {
    JSONRPC string          `json:"jsonrpc"`
    ID      json.RawMessage
    Result  interface{}     // on success
    Error   *RPCError       // on failure
}
```

### RPC Methods

| Method | Direction | Mutating | Description |
|--------|-----------|----------|-------------|
| `activate/initialize` | request | no | Set project dir, return capabilities |
| `activate/state` | request | no | Full state snapshot |
| `activate/configGet` | request | no | Read config (by scope) |
| `activate/configSet` | request | yes | Write config |
| `activate/manifestList` | request | no | List all manifests |
| `activate/manifestFiles` | request | no | List files for manifest + tier |
| `activate/repoAdd` | request | yes | Install files into workspace |
| `activate/repoRemove` | request | yes | Remove installed files |
| `activate/sync` | request | yes | Detect and apply updates |
| `activate/update` | request | yes | Re-install all files |
| `activate/fileInstall` | request | yes | Install single file |
| `activate/fileUninstall` | request | yes | Remove single file |
| `activate/fileDiff` | request | no | Diff bundled vs. installed |
| `activate/fileSkip` | request | yes | Skip update for a file |
| `activate/fileOverride` | request | yes | Pin or exclude a file |
| `activate/telemetryRun` | request | yes | Run telemetry collection |
| `activate/telemetryLog` | request | no | Read telemetry log |
| `activate/stateChanged` | notification | — | Sent after mutating operations |

After every mutating operation, the daemon automatically sends an `activate/stateChanged` notification so clients can refresh their UI.

---

## TUI (Terminal User Interface)

### Technology

Built with the [Charm](https://charm.sh) library stack:

- **Bubble Tea** — Terminal UI framework implementing the Elm architecture (`Model`, `Update`, `View`)
- **Huh** — High-level form builder (select lists, inputs, confirms) on top of Bubble Tea
- **Lipgloss** — Styling (colors, borders, layout)

### Branding

```go
colorGold   = "#E8C228"   // Peregrine falcon gold
colorPurple = "#7B61FF"
```

The TUI displays an ASCII art Peregrine falcon logo (rendered with half-block glyphs) and a 7-segment "PEREGRINE DIGITAL SERVICES" wordmark.

### Screens

#### Interactive Menu (`RunInteractiveMenu`)

The default screen when running `activate` with no arguments (or `activate menu`). State-aware — shows different options based on install state.

```
┌─────────────────────────────────────────┐
│  [Peregrine Logo]                       │
│  PEREGRINE DIGITAL SERVICES             │
│                                         │
│  ◉ List manifests & files               │
│  ○ Show install state                   │
│  ○ Browse & manage files                │
│  ○ Settings                             │
│  ○ Sync / check for updates             │
│  ○ Update all files                     │
└─────────────────────────────────────────┘
```

Selecting an option routes to:
- **List / State** → fullscreen text display (read-only output)
- **Browse files** → file browser sub-screen
- **Settings** → settings form sub-screen

#### Interactive Installer (`RunInteractiveInstall`)

Launched via `activate install`. Two-phase form:

1. **Manifest selection** — Choose from discovered manifests (skipped if only one)
2. **Configure** — Select tier, target directory, confirm

On completion, calls `installWithResolvedConfig()` to execute the install.

#### File Browser (`RunFileBrowser`)

Three-mode interactive screen:

| Mode | Description |
|------|-------------|
| `browse` | Category-grouped file list with status icons (✓ installed, ○ available, ⬆ update) |
| `actions` | Per-file action menu (install, uninstall, diff, skip, pin, exclude) |
| `text` | Read-only display for diff output or action results |

```
┌─────────────────────────────────────────┐
│  Browse Files                           │
│                                         │
│  Instructions                           │
│    ✓ general.instructions.md  v0.5.0    │
│    ⬆ security.instructions.md v0.4→0.5  │
│                                         │
│  Skills                                 │
│    ○ ci-debugger/SKILL.md               │
│    ✓ pr-writing/SKILL.md      v0.3.0    │
│                                         │
│  [↑/↓ navigate] [enter select] [q back] │
└─────────────────────────────────────────┘
```

#### Settings (`RunSettings`)

Form with:
- Telemetry toggle (on/off)
- Manifest selection (dropdown)
- Tier selection (dropdown)

Returns a `changed` boolean so the caller knows whether to refresh state.

#### Telemetry Consent (`RunTelemetryScreen`)

Simple yes/no prompt for enabling crash and event reporting. Shown on first run or from settings.

### Navigation

All TUI screens use Bubble Tea's event loop. Common key bindings:

- `↑`/`↓` — Navigate options
- `Enter` — Select
- `q` / `Esc` — Back to previous screen
- `Ctrl+C` — Quit

---

## VS Code Extension

### Source Layout

```
extension/
├── package.json                 # Extension manifest, commands, views
├── scripts/prepare-assets.mjs   # Copies plugin files → assets/ at build time
└── src/
    ├── extension.js             # Activation, command registration, autoSetup
    ├── client.js                # JSON-RPC client for CLI daemon
    ├── controlPanel.js          # WebviewView sidebar (HTML render + messages)
    ├── config.js                # Config read/write (CJS, vscode.workspace.fs)
    ├── injector.js              # Inject files into workspace, sidecar tracking
    ├── installer.js             # Legacy workspace-mode helpers
    ├── manifest.js              # selectFiles, TIER_MAP, listByCategory, inferCategory
    ├── commands/
    │   ├── changeTier.js        #   QuickPick → write config → re-inject
    │   ├── changeManifest.js    #   QuickPick → write config → re-inject
    │   └── showStatus.js        #   Info message with install state
    └── __tests__/               # Extension tests (node --test, CJS)
```

### Activation Flow

```
activate(context)
  ├─ Resolve workspace folder
  ├─ resolveBinPath(context)
  │   ├─ Try extension bundled bin/activate
  │   ├─ Try sibling cli/activate (dev mode)
  │   └─ Fall back to system PATH
  ├─ Create OutputChannel("Activate Framework")
  ├─ new ActivateClient(binPath, projectDir, log)
  │   └─ client.start()
  │       └─ Spawn: activate serve --stdio
  │       └─ Send: activate/initialize {projectDir}
  ├─ new ControlPanelProvider(client)
  ├─ Register WebviewView provider for sidebar
  ├─ Listen for activate/stateChanged → controlPanel.refresh()
  ├─ Listen for daemon exit → auto-restart
  ├─ Register 14 commands
  └─ autoSetup()
      └─ If not installed: repoAdd()
         Else: sync() + check for updates
```

### Client (JSON-RPC)

`client.js` implements a JSON-RPC 2.0 client that communicates with the Go daemon:

```javascript
const Method = {
    Initialize:    'activate/initialize',
    StateGet:      'activate/state',
    ConfigGet:     'activate/configGet',
    ConfigSet:     'activate/configSet',
    ManifestList:  'activate/manifestList',
    ManifestFiles: 'activate/manifestFiles',
    RepoAdd:       'activate/repoAdd',
    RepoRemove:    'activate/repoRemove',
    Sync:          'activate/sync',
    Update:        'activate/update',
    FileInstall:   'activate/fileInstall',
    FileUninstall: 'activate/fileUninstall',
    FileDiff:      'activate/fileDiff',
    FileSkip:      'activate/fileSkip',
    FileOverride:  'activate/fileOverride',
    TelemetryRun:  'activate/telemetryRun',
    TelemetryLog:  'activate/telemetryLog',
};
```

The client:
1. Spawns the daemon process
2. Reads Content-Length framed responses from stdout via `FrameReader`
3. Matches responses to pending requests by ID (30s timeout)
4. Emits `notification` events for server-pushed messages
5. Auto-restarts the daemon on unexpected exit

### Control Panel (Sidebar)

`controlPanel.js` implements a `WebviewViewProvider` that renders the sidebar UI as HTML.

#### Pages

| Page | Content |
|------|---------|
| `main` | File browser with install/update/diff actions |
| `usage` | Copilot telemetry dashboard |
| `settings` | Config inspector (global + project scopes) |

#### Main Page Layout

```
┌──────────────────────────────────────┐
│ v0.5.0 · [standard] · [ironarch]    │
│ ✓ Installed                          │
│                                      │
│ [◆ Tier] [⇋ Manifest] [± Install]   │
│ [↻ Update] [📊 Usage]               │
├──────────────────────────────────────┤
│ ▼ Installed                          │
│   ▼ Instructions                     │
│     ✓ general.instructions.md v0.5.0 │
│       [Open] [Uninstall] [Pin]       │
│     ⬆ security.instructions.md      │
│       [Open] [Diff] [Update] [Skip]  │
│   ▼ Skills                           │
│     ✓ pr-writing/SKILL.md    v0.3.0  │
│                                      │
│ ▼ Available                          │
│   ▼ Agents                           │
│     ○ planner.agent.md               │
│       [Install] [Exclude]            │
│                                      │
│ ▸ Outside Current Tier               │
│ ▸ Excluded                           │
└──────────────────────────────────────┘
```

Files are organized into four sections:
- **Installed** — files present in the workspace
- **Available** — files in the current tier, not yet installed
- **Outside Tier** — files requiring a higher tier (dimmed)
- **Excluded** — user-excluded files

Each file shows status icons, version info, override badges, and contextual action buttons.

#### Message Protocol (Webview ↔ Extension)

Messages sent from the webview via `vscode.postMessage()`:

| Command | Payload | Action |
|---------|---------|--------|
| `changeTier` | — | QuickPick → `setConfig({tier})` |
| `changeManifest` | — | QuickPick → `setConfig({manifest})` |
| `addToWorkspace` | — | Confirmation → `repoAdd()` |
| `removeFromWorkspace` | — | Confirmation → `repoRemove()` |
| `updateAll` | — | `update()` → show count |
| `installFile` | `{file}` | `installFile(dest)` |
| `uninstallFile` | `{file}` | `uninstallFile(dest)` |
| `openFile` | `{file}` | Open in editor |
| `diffFile` | `{file}` | Show bundled vs. installed diff |
| `skipUpdate` | `{file}` | Skip this version |
| `setOverride` | `{file, override}` | Pin, exclude, or clear |
| `showUsage` | — | Switch to usage page |
| `showSettings` | — | Switch to settings page |
| `backToMain` | — | Switch to main page |
| `toggleTelemetry` | `{enabled}` | `setConfig({telemetryEnabled})` |
| `setGlobalDefault` | `{updates}` | `setConfig({..., scope: 'global'})` |
| `clearProjectOverride` | `{updates}` | `setConfig({..., scope: 'project'})` |

### Commands

| Command | Description |
|---------|-------------|
| `changeTier` | QuickPick with available tiers → write project config → sync |
| `changeManifest` | QuickPick with available manifests → write project config → sync |
| `showStatus` | Print state to output channel |
| `addToWorkspace` | Install files into `.github/` with confirmation |
| `removeFromWorkspace` | Remove installed files with confirmation |
| `updateAll` | Re-install all files with latest versions |
| `installFile` | Install a single file |
| `uninstallFile` | Remove a single file |
| `openFile` | Open an installed file in the editor |
| `diffFile` | Show diff between bundled and installed versions |
| `skipFileUpdate` | Skip update for a file at its current version |
| `refresh` | Re-render the control panel |
| `telemetryRunNow` | Collect Copilot usage data |

### Build: prepare-assets

`scripts/prepare-assets.mjs` runs at build time to bundle plugin content into the extension:

1. Read all manifests from `manifests/*.json`
2. Resolve source files using each manifest's `basePath`
3. Copy manifests to `extension/assets/manifests/`
4. Copy source files to `extension/assets/` (preserving dest paths)

This makes the extension self-contained — it ships with all plugin files embedded, so the daemon can find them via `ResolveBundleDir()`.

---

## npm Distribution

The Go binary is distributed via npm using a platform-specific package strategy:

### Package Structure

```
@anthropic/activate-cli                  # Main package (shim)
├── bin/activate                         # JS shim → spawns Go binary
├── install.js                           # postinstall: verify binary
└── optionalDependencies:
    ├── @anthropic/activate-cli-darwin-arm64   # macOS Apple Silicon
    ├── @anthropic/activate-cli-darwin-x64     # macOS Intel
    ├── @anthropic/activate-cli-linux-arm64    # Linux ARM
    ├── @anthropic/activate-cli-linux-x64      # Linux x86_64
    └── @anthropic/activate-cli-win32-x64      # Windows x86_64
```

### Flow

```
npm install @anthropic/activate-cli
  ↓
npm installs main package + matching platform package
  ↓
postinstall (install.js):
  ├─ Detect platform + arch
  ├─ Locate platform package binary
  ├─ chmod +x (Unix)
  └─ Verify binary exists
  ↓
./node_modules/.bin/activate ready to use
```

The `bin/activate` shim is a small JS script that resolves the platform-specific binary and spawns it with the same arguments.

### Cross-Compilation

The `Makefile` in `cli/` handles cross-compilation:

```bash
make build          # Build for current platform
make all            # Build for all platforms
make npm-stage      # Copy binaries into npm/platforms/
make publish        # Publish all packages to npm
```

---

## Design Decisions

### Why Sub-Packages?

The CLI was originally a flat `package main` directory with 26+ source files. As the codebase grew, the flat layout made it hard to reason about dependencies and risked import cycles. The sub-package structure enforces a strict dependency DAG at the compiler level: `model` (pure types) → `storage` (disk I/O) → `engine` (business logic) → `commands` (service + daemon) → `main` / `tui`. Each layer can only import downward, making the architecture self-documenting.

### Why Go + Node?

- **Go** for the CLI and daemon: zero runtime dependencies, single binary, fast startup, excellent cross-compilation. The CLI ships as a static binary — no Node, Python, or other runtime needed.
- **Node** for the extension and validation: VS Code extensions require JavaScript. The validation script validates JavaScript/Markdown content and runs in the same Node environment the extension already requires.

### Why a Daemon?

The extension could shell out to the CLI for each operation, but the daemon provides:
- **Shared state** — config and manifests are loaded once, not on every command
- **Notifications** — the daemon pushes `stateChanged` events so the UI stays in sync
- **Atomic operations** — mutating operations can be sequenced without race conditions
- **Performance** — no process spawn overhead per operation

### Why Inject into .github/?

VS Code's Copilot discovers instructions, prompts, skills, and agents from the `.github/` directory. By injecting files there and excluding them from git, we get zero-config Copilot integration without polluting the repository's commit history.

### Why Two Config Layers?

- **Global** — user-wide preferences (default manifest, telemetry) that apply to every project
- **Project** — per-repo overrides (different tier, excluded files) that don't affect other repos

The project config is stored in `~/.activate/repos/<hash>/` (not in the workspace) so it never touches the repo's version control.
