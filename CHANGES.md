# Changes Summary

Summary of changes made on the `feat/vscode-extension` branch by @mheadd

## Overview

Aligned the ironarch plugin with ADR-001 file hierarchy from activate-copilot, added structure validation tooling, and expanded project documentation.

---

## Commits

### 1. feat(ironarch): align with ADR-001 file hierarchy (`09f9a08`)

Added required ADR-001 structure to ironarch plugin:
- Created `plugins/ironarch/AGENTS.md` with user approval requirements
- Created `plugins/ironarch/instructions/.gitkeep` placeholder
- Created `plugins/ironarch/prompts/.gitkeep` placeholder

### 2. docs: add plugin file hierarchy reference (`f97ac08`)

Created `docs/plugin-file-hierarchy.md` documenting the four-tier guidance hierarchy:
- Tier 1: AGENTS.md (always active)
- Tier 2: Instructions (auto by glob) and Prompts (manual /command)
- Tier 3: Skills (on-demand procedures)
- Tier 4: Agents (specialized personas)

Includes required frontmatter for each tier and validation checklist.

### 3. refactor(ironarch): convert AGENTS.md to concise RFC2119 notation (`d7f4631`)

Reformatted ironarch AGENTS.md to match activate-framework style:
- Use RFC2119 legend symbols (!, ~, ⊗, ?, ≉)
- Condensed verbose prose into action lists
- Reduced from 83 lines to 36 lines

### 4. feat(framework): add ADR-001 structure validation script (`bfb5286`)

Created `framework/validate-structure.mjs` to validate plugins:
- Checks AGENTS.md exists (Tier 1)
- Checks instructions have `applyTo` frontmatter (Tier 2)
- Checks skills have SKILL.md with `name` and `description` (Tier 3)
- Checks agents have `name` and `description` frontmatter (Tier 4)

Added 10 tests in `framework/__tests__/validate-structure.test.mjs`.

### 5. feat(ironarch): add AGENTS.md to manifest, bump to v0.2.0 (`23ba9cf`)

Updated `manifests/ironarch.json`:
- Added `core` tier for always-active files
- Added AGENTS.md as first file entry
- Version bump from 0.1.0 to 0.2.0

### 6. chore: add root package.json with validation scripts (`f6abd24`)

Created root `package.json` with npm scripts:
- `npm run validate:plugins` — Run ADR-001 structure validation
- `npm run test` — Run framework tests
- `npm run validate` — Run both

### 7. docs: expand README with full project documentation (`95d667b`)

Expanded top-level README.md with:
- Project description and key features
- Installation options (extension + CLI)
- Full project structure diagram
- Plugin file hierarchy overview
- Creating new plugins guide
- Manifest structure example
- Validation commands
- Available plugins table
- Development prerequisites and workflow
- Documentation links

### 8. docs: add framework-level customization guide (`1c2346b`)

Created `docs/creating-customization-files.md` for plugin developers:
- Plugin architecture overview
- Creating files within plugins
- Adding to manifest
- Validation workflow
- Creating new plugins guide

Updated README to link to new guide.

---

## New Files

| File | Purpose |
|------|---------|
| `plugins/ironarch/AGENTS.md` | User approval requirement for VA workflow |
| `plugins/ironarch/instructions/.gitkeep` | Tier 2 placeholder |
| `plugins/ironarch/prompts/.gitkeep` | Tier 2 placeholder |
| `docs/plugin-file-hierarchy.md` | ADR-001 structure documentation |
| `docs/creating-customization-files.md` | Plugin developer guide |
| `framework/validate-structure.mjs` | Structure validation script |
| `framework/__tests__/validate-structure.test.mjs` | Validation tests |
| `package.json` | Root npm scripts |

## Modified Files

| File | Change |
|------|--------|
| `manifests/ironarch.json` | Added AGENTS.md, core tier, version bump |
| `README.md` | Full rewrite with comprehensive documentation |

---

## Validation

Both plugins now pass structure validation:

```
✅ activate-framework — All checks passed
✅ ironarch — All checks passed
```

26 framework tests passing.