# Activate Framework

Activate is a plugin-based system for distributing AI coding agent configuration to development teams. It packages instructions, prompts, skills, and agent definitions into installable plugins that are injected into your workspace's `.github/` directory — where tools like GitHub Copilot, Claude Code, and Cursor automatically pick them up.

The framework has three delivery surfaces: a **compiled Go CLI** with an interactive TUI, a **VS Code extension** with a sidebar control panel, and a **JSON-RPC daemon** that bridges the two. All three share the same service layer, manifest system, and config schema.

## What It Does

1. **Discovers plugins** — Manifests define what files a plugin contains, organized into selectable tiers (core, standard, advanced)
2. **Installs agent configuration** — Copies `.instructions.md`, `.prompt.md`, `.agent.md`, `SKILL.md`, and `AGENTS.md` files into your workspace's `.github/` directory
3. **Hides from git** — Installed files are auto-excluded via `.git/info/exclude` so they never get committed
4. **Tracks state** — A sidecar file (`~/.activate/repos/<hash>/installed.json`) tracks what's installed, versions, and checksums
5. **Keeps you current** — Both CLI and extension self-update from GitHub Releases, with passive update hints and one-click upgrades

## Quick Start

### VS Code Extension (recommended)

1. Download the latest `.vsix` from [Releases](https://github.com/peregrine-digital/activate-framework/releases)
2. In VS Code: **Extensions** → **⋯** → **Install from VSIX…** → select the downloaded file
3. Reload VS Code — the extension auto-installs the CLI and sets up your workspace

The extension provides a sidebar control panel for switching manifests, changing tiers, browsing installed files, and checking for updates.

### CLI Only

```bash
url -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" \
"https://raw.githubusercontent.com/peregrine-digital/activate-framework/main/install-cli.sh" \
| GITHUB_TOKEN="$GITHUB_TOKEN" sh
```

Then run the interactive installer:

```bash
activate install
```

> **Private repo:** `GITHUB_TOKEN` must be a personal access token with `repo` scope.

## Architecture

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
│       (2-layer)   Discovery  (remote)  (GitHub)  Sidecar         │
│                                                  + gitexclude    │
└──────────────────────────────────────────────────────────────────┘
```

The CLI and TUI call the service directly (same process). The extension spawns a daemon (`activate serve --stdio`) and communicates via JSON-RPC 2.0 over Content-Length framed stdio.

### Config System

Two-layer JSON config with merge semantics:

| Layer | Path | Scope |
|:-------|:------|:-------|
| Global | `~/.activate/config.json` | User-wide defaults |
| Project | `~/.activate/repos/<hash>/config.json` | Per-project overrides |

**Precedence:** built-in defaults < global < project < CLI flags

## Project Structure

```
activate-framework/
├── cli/                             # Go CLI (compiled binary)
│   ├── main.go                      #   Entry point, arg parsing
│   ├── model/                       #   Pure types + config schema
│   ├── transport/                   #   JSON-RPC wire format
│   ├── storage/                     #   Disk I/O (config, sidecar, git)
│   ├── engine/                      #   Business logic (install, diff, update)
│   ├── commands/                    #   CLI commands + JSON-RPC daemon
│   ├── selfupdate/                  #   Self-update from GitHub Releases
│   └── tui/                         #   Interactive Bubbletea UI
│
├── extension/                       # VS Code extension
│   ├── src/                         #   Extension source (thin daemon wrapper)
│   │   ├── extension.js             #     Activation, commands, daemon lifecycle
│   │   ├── controlPanel.js          #     Sidebar WebviewView (files, settings, usage)
│   │   ├── client.js                #     JSON-RPC client for CLI daemon
│   │   └── __tests__/               #     Tests (97 tests across 5 files)
│   └── package.json                 #   Extension manifest + commands
│
├── manifests/                       # Plugin registry (one JSON per plugin)
│   ├── activate-framework.json      #   Core framework manifest
│   └── ironarch.json                #   VA workflow manifest
│
├── plugins/                         # Content plugins (deliverable assets)
│   ├── activate-framework/          #   Core: instructions, prompts, skills, agents
│   └── ironarch/                    #   VA: specialized workflow agents
│
├── skills/                          # Shared skills (cross-plugin)
├── mcp-servers/                     # Shared MCP server configs
├── install-cli.sh                   # CLI installer script (curl | sh)
└── docs/                            # Documentation
```

## Plugin System

Plugins follow a four-tier guidance hierarchy:

```
plugins/{plugin-name}/
├── AGENTS.md                              # Tier 1: Always active (recommended)
├── instructions/*.instructions.md         # Tier 2: Auto-applied by glob
├── prompts/*.prompt.md                    # Tier 2: Manual /command
├── skills/{skill-name}/SKILL.md           # Tier 3: On-demand procedures
└── agents/*.agent.md                      # Tier 4: Specialized personas
```

Each manifest defines **tiers** (e.g., core, standard, advanced) that let teams choose how much guidance to install. Files are tagged by category (instruction, prompt, skill, agent, mcp-server, other) and selected based on the active tier.

### Available Plugins

| Plugin | Description | Tiers |
|--------|-------------|-------|
| **activate-framework** | Core AI dev framework — general instructions, prompts, skills, agents | core, ad-hoc, ad-hoc-advanced |
| **ironarch** | VA-oriented workflow — planning, implementing, testing, reviewing, documenting | core, skills, workflow |

### Creating a Plugin

1. Create a directory under `plugins/`
2. Add `AGENTS.md` at the root (recommended)
3. Add instructions, prompts, skills, and agents as needed
4. Create a manifest in `manifests/{plugin-name}.json`
5. Run validation: `npm run validate:plugins`

## CI/CD

Two GitHub Actions workflows run on every push and PR:

- **CLI** (`cli.yml`) — Builds the Go binary, runs tests, cross-compiles for 5 platforms on release (darwin-arm64/amd64, linux-arm64/amd64, windows-amd64), attaches archives + SHA256 checksums to the GitHub Release
- **Extension** (`extension.yml`) — Installs dependencies, runs tests, packages the VSIX, attaches it to the GitHub Release

Releases are cut with `mise run release`, which bumps versions, tags, and creates a GitHub Release. CI builds and attaches all artifacts automatically.

## Development

### Prerequisites

- Go 1.25+ and Node.js 20+ (see `mise.toml`)

### Running Tests

```bash
# Go CLI tests (349 tests)
cd cli && go test ./...

# Extension tests (97 tests)
cd extension && npm test

# Plugin structure validation (10 tests)
npm run validate:plugins
```

### Contributing

See [AGENTS.md](AGENTS.md) for development workflow, code map, and conventions. Key practices: trunk-based development, conventional commits, TDD, and all tests green before PR.

## Documentation

- [Architecture](docs/architecture.md) — Full system design: CLI, extension, TUI, daemon protocol
- [Plugin File Hierarchy](docs/plugin-file-hierarchy.md) — Structure requirements for plugins
- [Creating Customization Files](docs/creating-customization-files.md) — How to author instructions, prompts, skills, and agents
- [Example Usage](docs/EXAMPLE-USAGE.md) — Installation and usage examples

## License

MIT
