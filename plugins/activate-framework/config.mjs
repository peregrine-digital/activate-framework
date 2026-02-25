/**
 * Shared config read/write for Activate framework (CLI side).
 *
 * Two layers:
 *   1. Global  — ~/.activate/config.json  (user-wide defaults)
 *   2. Project — .activate.json           (per-project overrides, git-excluded)
 *
 * Shape (both files share the same schema, project wins on merge):
 *   {
 *     "manifest": "activate-framework",
 *     "tier": "standard",
 *     "fileOverrides": {
 *       "instructions/security.instructions.md": "pinned",
 *       "skills/writing-skills/SKILL.md": "excluded"
 *     },
 *     "skippedVersions": {
 *       "instructions/general.instructions.md": "0.5.0"
 *     }
 *   }
 *
 * fileOverrides values:
 *   - "pinned"   — always install regardless of tier
 *   - "excluded" — never install regardless of tier
 *
 * skippedVersions:
 *   - Maps file dest → version string the user chose to skip
 */

import { readFile, writeFile, mkdir } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { homedir } from 'node:os';

// ── Paths ───────────────────────────────────────────────────────

/** Global config directory */
const GLOBAL_DIR = path.join(homedir(), '.activate');

/** Global config file path */
export const GLOBAL_CONFIG_PATH = path.join(GLOBAL_DIR, 'config.json');

/** Per-project config filename (lives at workspace root) */
export const PROJECT_CONFIG_FILENAME = '.activate.json';

// ── Defaults ────────────────────────────────────────────────────

/** Built-in defaults when no config exists */
const DEFAULTS = Object.freeze({
  manifest: 'activate-framework',
  tier: 'standard',
  fileOverrides: {},
  skippedVersions: {},
});

// ── Read helpers ────────────────────────────────────────────────

/**
 * Read and parse a JSON config file. Returns null if missing/invalid.
 * @param {string} filePath
 * @returns {Promise<object|null>}
 */
async function readJsonFile(filePath) {
  try {
    const raw = await readFile(filePath, 'utf8');
    return JSON.parse(raw);
  } catch {
    return null;
  }
}

/**
 * Read the global config (~/.activate/config.json).
 * @returns {Promise<object|null>}
 */
export async function readGlobalConfig() {
  return readJsonFile(GLOBAL_CONFIG_PATH);
}

/**
 * Read the project config (.activate.json in projectDir).
 * @param {string} projectDir - Absolute path to the project root
 * @returns {Promise<object|null>}
 */
export async function readProjectConfig(projectDir) {
  return readJsonFile(path.join(projectDir, PROJECT_CONFIG_FILENAME));
}

/**
 * Resolve the effective config by merging global → project → overrides.
 *
 * Precedence: built-in defaults < global < project < explicit overrides
 *
 * @param {string} [projectDir] - If provided, merges project config
 * @param {object} [overrides]  - CLI flags or programmatic overrides
 * @returns {Promise<{manifest: string, tier: string, fileOverrides: object, skippedVersions: object}>}
 */
export async function resolveConfig(projectDir, overrides = {}) {
  const global = (await readGlobalConfig()) || {};
  const project = projectDir ? ((await readProjectConfig(projectDir)) || {}) : {};

  // Deep merge: defaults → global → project → overrides
  // For nested objects (fileOverrides, skippedVersions) merge keys
  const merged = {
    manifest: overrides.manifest ?? project.manifest ?? global.manifest ?? DEFAULTS.manifest,
    tier: overrides.tier ?? project.tier ?? global.tier ?? DEFAULTS.tier,
    fileOverrides: {
      ...DEFAULTS.fileOverrides,
      ...global.fileOverrides,
      ...project.fileOverrides,
      ...overrides.fileOverrides,
    },
    skippedVersions: {
      ...DEFAULTS.skippedVersions,
      ...global.skippedVersions,
      ...project.skippedVersions,
      ...overrides.skippedVersions,
    },
  };

  return merged;
}

// ── Write helpers ───────────────────────────────────────────────

/**
 * Write the global config file (~/.activate/config.json).
 * Merges with existing values (does not clobber unset keys).
 * @param {object} updates - Partial config to merge
 */
export async function writeGlobalConfig(updates) {
  const existing = (await readGlobalConfig()) || {};
  const merged = _shallowMergeConfig(existing, updates);
  await mkdir(GLOBAL_DIR, { recursive: true });
  await writeFile(GLOBAL_CONFIG_PATH, JSON.stringify(merged, null, 2) + '\n');
}

/**
 * Write the per-project config (.activate.json).
 * Merges with existing values (does not clobber unset keys).
 * @param {string} projectDir
 * @param {object} updates - Partial config to merge
 */
export async function writeProjectConfig(projectDir, updates) {
  const filePath = path.join(projectDir, PROJECT_CONFIG_FILENAME);
  const existing = (await readProjectConfig(projectDir)) || {};
  const merged = _shallowMergeConfig(existing, updates);
  await writeFile(filePath, JSON.stringify(merged, null, 2) + '\n');
}

/**
 * Set a file override in the project config.
 * @param {string} projectDir
 * @param {string} fileDest - The file's dest path (e.g. "instructions/general.instructions.md")
 * @param {"pinned"|"excluded"|null} status - null to remove the override
 */
export async function setFileOverride(projectDir, fileDest, status) {
  const existing = (await readProjectConfig(projectDir)) || {};
  const overrides = { ...existing.fileOverrides };
  if (status === null) {
    delete overrides[fileDest];
  } else {
    overrides[fileDest] = status;
  }
  await writeProjectConfig(projectDir, { fileOverrides: overrides });
}

/**
 * Record a skipped version in the project config.
 * @param {string} projectDir
 * @param {string} fileDest
 * @param {string} version - The version being skipped
 */
export async function setSkippedVersion(projectDir, fileDest, version) {
  const existing = (await readProjectConfig(projectDir)) || {};
  const skipped = { ...existing.skippedVersions };
  skipped[fileDest] = version;
  await writeProjectConfig(projectDir, { skippedVersions: skipped });
}

/**
 * Clear a skipped version (e.g., when the user installs a newer file).
 * @param {string} projectDir
 * @param {string} fileDest
 */
export async function clearSkippedVersion(projectDir, fileDest) {
  const existing = (await readProjectConfig(projectDir)) || {};
  const skipped = { ...existing.skippedVersions };
  delete skipped[fileDest];
  await writeProjectConfig(projectDir, { skippedVersions: skipped });
}

// ── Git exclude helper ──────────────────────────────────────────

const EXCLUDE_MARKER_START = '# >>> Peregrine Activate config (managed)';
const EXCLUDE_MARKER_END = '# <<< Peregrine Activate config';

/**
 * Ensure .activate.json is in .git/info/exclude so it's invisible to git.
 * Safe to call repeatedly (idempotent).
 * @param {string} projectDir
 */
export async function ensureGitExclude(projectDir) {
  const excludePath = path.join(projectDir, '.git', 'info', 'exclude');

  let content;
  try {
    content = await readFile(excludePath, 'utf8');
  } catch {
    // .git/info/ may not exist — try to create it
    try {
      await mkdir(path.join(projectDir, '.git', 'info'), { recursive: true });
      content = '';
    } catch {
      // Not a git repo — nothing to do
      return;
    }
  }

  // Already present?
  if (content.includes(EXCLUDE_MARKER_START)) return;

  const block = [
    '',
    EXCLUDE_MARKER_START,
    PROJECT_CONFIG_FILENAME,
    EXCLUDE_MARKER_END,
    '',
  ].join('\n');

  if (!content.endsWith('\n') && content.length > 0) {
    content += '\n';
  }
  content += block;

  await writeFile(excludePath, content);
}

// ── Internal ────────────────────────────────────────────────────

/**
 * Merge two config objects. Top-level scalars are overwritten.
 * fileOverrides and skippedVersions are fully replaced when provided
 * (callers are responsible for constructing the complete sub-object).
 */
function _shallowMergeConfig(base, updates) {
  const result = { ...base };

  if (updates.manifest !== undefined) result.manifest = updates.manifest;
  if (updates.tier !== undefined) result.tier = updates.tier;

  if (updates.fileOverrides !== undefined) {
    result.fileOverrides = { ...updates.fileOverrides };
  }
  if (updates.skippedVersions !== undefined) {
    result.skippedVersions = { ...updates.skippedVersions };
  }

  return result;
}
