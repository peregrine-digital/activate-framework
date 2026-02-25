# Activate Framework

A cross-functional agentic framework for government delivery teams. Activate provides a plugin-based system for distributing AI agent guidance—instructions, prompts, skills, and agent definitions—to development teams working on government technology projects.

## Key Features

- **Plugin Architecture** — Modular content packages that can be installed independently
- **Four-Tier Guidance Hierarchy** — Clear structure for AI agent instructions (AGENTS.md → Instructions → Skills → Agents)
- **VS Code Extension** — GUI for installing and managing plugins in workspaces
- **CLI Installer** — Interactive script for non-extension installations
- **Validation Tooling** — Automated checks for plugin structure compliance

## Quick Start

### Option 1: VS Code Extension

Install the Activate extension from the VS Code marketplace, then use the sidebar to select and install plugins.

### Option 2: CLI

```bash
node install.mjs
```

Follow the prompts to choose a plugin, tier level, and installation directory.

## Project Structure

```
activate-framework/
├── AGENTS.md                        # Project-wide agent guidance
├── install.mjs                      # CLI entry point
├── package.json                     # npm scripts (test, validate)
├── mise.toml                        # Node 20 toolchain
│
├── framework/                       # Shared CLI engine (plugin-agnostic)
│   ├── install.mjs                  #   Interactive CLI installer
│   ├── core.mjs                     #   Manifest discovery, tier maps
│   ├── config.mjs                   #   Config read/write
│   ├── validate-structure.mjs       #   ADR-001 structure validation
│   └── __tests__/                   #   Framework tests
│
├── manifests/                       # Plugin registry (one JSON per plugin)
│   ├── activate-framework.json
│   └── ironarch.json
│
├── plugins/                         # Content plugins
│   ├── activate-framework/          #   Core plugin
│   └── ironarch/                    #   VA-specific workflow plugin
│
├── skills/                          # Shared skills (cross-plugin)
├── mcp-servers/                     # Shared MCP server configs
│
├── extension/                       # VS Code extension
│   ├── src/                         #   Extension source code
│   └── package.json                 #   Extension manifest
│
└── docs/                            # Documentation
    ├── plugin-file-hierarchy.md     #   Plugin structure requirements
    └── EXAMPLE-USAGE.md             #   Usage examples
```

## Plugin File Hierarchy

All plugins must follow the four-tier guidance hierarchy. See [docs/plugin-file-hierarchy.md](docs/plugin-file-hierarchy.md) for full details.

```
plugins/{plugin-name}/
├── AGENTS.md                              # Tier 1: Always active (required)
├── instructions/*.instructions.md         # Tier 2: Auto-applied by glob
├── prompts/*.prompt.md                    # Tier 2: Manual /command
├── skills/{skill-name}/SKILL.md           # Tier 3: On-demand procedures
└── agents/*.agent.md                      # Tier 4: Specialized personas
```

### Creating a New Plugin

1. Create a directory under `plugins/`
2. Add `AGENTS.md` at the root (required)
3. Add directories for instructions, prompts, skills, and agents as needed
4. Create a manifest in `manifests/{plugin-name}.json`
5. Run validation: `npm run validate:plugins`

### Manifest Structure

```json
{
  "name": "Plugin Name",
  "description": "What this plugin provides",
  "version": "0.1.0",
  "basePath": "plugins/your-plugin",
  "tiers": [
    { "id": "core", "label": "Core" },
    { "id": "standard", "label": "Standard" }
  ],
  "files": [
    {
      "src": "AGENTS.md",
      "dest": "AGENTS.md",
      "tier": "core",
      "category": "other",
      "description": "Project-wide agent guidance"
    }
  ]
}
```

## Validation

Validate plugin structure compliance:

```bash
# Validate all plugins
npm run validate:plugins

# Validate a specific plugin
node framework/validate-structure.mjs ironarch

# Run all tests
npm run test

# Run validation + tests
npm run validate
```

Validation checks:
- `AGENTS.md` exists at plugin root
- Instructions have `applyTo` frontmatter
- Skills have `SKILL.md` with `name` and `description`
- Agents have `name` and `description` frontmatter

## Available Plugins

| Plugin | Description |
|--------|-------------|
| **activate-framework** | Core framework with general instructions, prompts, skills, and agents |
| **ironarch** | VA-oriented workflow with specialized agents for planning, implementing, testing, reviewing, and PR creation |

## Development

### Prerequisites

- Node.js 20+ (see `mise.toml`)

### Running Tests

```bash
npm test
```

### Contributing

See [AGENTS.md](AGENTS.md) for development workflow guidance:
- Trunk-based development
- Atomic commits with conventional commit format
- TDD approach
- All tests green before PR

## Documentation

- [Plugin File Hierarchy](docs/plugin-file-hierarchy.md) — Structure requirements for plugins
- [Creating Customization Files](docs/creating-customization-files.md) — How to create instructions, prompts, skills, and agents
- [Example Usage](docs/EXAMPLE-USAGE.md) — Installation and usage examples
- [AGENTS.md](AGENTS.md) — Development workflow and code map

## License

MIT
