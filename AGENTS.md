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