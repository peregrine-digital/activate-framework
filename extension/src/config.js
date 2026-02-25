/**
 * Config read/write for the VS Code extension.
 *
 * Same schema as the CLI config (plugins/activate-framework/config.mjs)
 * but uses vscode.workspace.fs to stay compatible with remote/web.
 *
 * Two layers:
 *   1. Global  — ~/.activate/config.json  (user-wide defaults)
 *   2. Project — .activate.json           (per-project, auto-excluded from git)
 *
 * Shape:
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
 */
const vscode = require('vscode');
const path = require('path');
const os = require('os');

// ── Constants ───────────────────────────────────────────────────

const GLOBAL_DIR = path.join(os.homedir(), '.activate');
const GLOBAL_CONFIG_PATH = path.join(GLOBAL_DIR, 'config.json');
const PROJECT_CONFIG_FILENAME = '.activate.json';

const DEFAULTS = Object.freeze({
  manifest: 'activate-framework',
  tier: 'standard',
  fileOverrides: {},
  skippedVersions: {},
});

// ── Read helpers ────────────────────────────────────────────────

/**
 * Read and parse a JSON file via vscode.workspace.fs.
 * @param {vscode.Uri} uri
 * @returns {Promise<object|null>}
 */
async function _readJson(uri) {
  try {
    const raw = await vscode.workspace.fs.readFile(uri);
    return JSON.parse(Buffer.from(raw).toString('utf8'));
  } catch {
    return null;
  }
}

/**
 * Write a JSON object to a file via vscode.workspace.fs.
 * @param {vscode.Uri} uri
 * @param {object} data
 */
async function _writeJson(uri, data) {
  const parentUri = vscode.Uri.joinPath(uri, '..');
  await vscode.workspace.fs.createDirectory(parentUri);
  await vscode.workspace.fs.writeFile(uri, Buffer.from(JSON.stringify(data, null, 2) + '\n'));
}

/**
 * Get the URI for the global config file.
 * @returns {vscode.Uri}
 */
function _globalUri() {
  return vscode.Uri.file(GLOBAL_CONFIG_PATH);
}

/**
 * Get the URI for the project config in the primary workspace folder.
 * @returns {vscode.Uri|null}
 */
function _projectUri() {
  const wsRoot = vscode.workspace.workspaceFolders?.[0]?.uri;
  if (!wsRoot) return null;
  return vscode.Uri.joinPath(wsRoot, PROJECT_CONFIG_FILENAME);
}

/**
 * Get the URI of the workspace root folder.
 * @returns {vscode.Uri|null}
 */
function _wsRoot() {
  return vscode.workspace.workspaceFolders?.[0]?.uri ?? null;
}

// ── Public API ──────────────────────────────────────────────────

/**
 * Read the global config.
 * @returns {Promise<object|null>}
 */
async function readGlobalConfig() {
  return _readJson(_globalUri());
}

/**
 * Read the per-project config.
 * @returns {Promise<object|null>}
 */
async function readProjectConfig() {
  const uri = _projectUri();
  if (!uri) return null;
  return _readJson(uri);
}

/**
 * Resolve the effective config: defaults < global < project < overrides.
 * @param {object} [overrides] - Programmatic overrides
 * @returns {Promise<{manifest: string, tier: string, fileOverrides: object, skippedVersions: object}>}
 */
async function resolveConfig(overrides = {}) {
  const global = (await readGlobalConfig()) || {};
  const project = (await readProjectConfig()) || {};

  return {
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
}

/**
 * Write/merge updates to the global config.
 * @param {object} updates - Partial config to merge
 */
async function writeGlobalConfig(updates) {
  const existing = (await readGlobalConfig()) || {};
  const merged = _mergeConfig(existing, updates);
  await _writeJson(_globalUri(), merged);
}

/**
 * Write/merge updates to the per-project config.
 * Also ensures .activate.json is in .git/info/exclude.
 * @param {object} updates - Partial config to merge
 */
async function writeProjectConfig(updates) {
  const uri = _projectUri();
  if (!uri) return;

  const existing = (await readProjectConfig()) || {};
  const merged = _mergeConfig(existing, updates);
  await _writeJson(uri, merged);

  // Automatically git-exclude the config file
  await ensureGitExclude();
}

/**
 * Set a file override (pinned/excluded/null to clear).
 * @param {string} fileDest
 * @param {"pinned"|"excluded"|null} status
 */
async function setFileOverride(fileDest, status) {
  const existing = (await readProjectConfig()) || {};
  const overrides = { ...existing.fileOverrides };
  if (status === null) {
    delete overrides[fileDest];
  } else {
    overrides[fileDest] = status;
  }
  await writeProjectConfig({ fileOverrides: overrides });
}

/**
 * Record a skipped version for a file.
 * @param {string} fileDest
 * @param {string} version
 */
async function setSkippedVersion(fileDest, version) {
  const existing = (await readProjectConfig()) || {};
  const skipped = { ...existing.skippedVersions };
  skipped[fileDest] = version;
  await writeProjectConfig({ skippedVersions: skipped });
}

/**
 * Clear a skipped version (e.g., after updating a file).
 * @param {string} fileDest
 */
async function clearSkippedVersion(fileDest) {
  const existing = (await readProjectConfig()) || {};
  const skipped = { ...existing.skippedVersions };
  delete skipped[fileDest];
  await writeProjectConfig({ skippedVersions: skipped });
}

// ── Git exclude ─────────────────────────────────────────────────

const EXCLUDE_MARKER_START = '# >>> Peregrine Activate config (managed)';
const EXCLUDE_MARKER_END = '# <<< Peregrine Activate config';

/**
 * Ensure .activate.json is listed in .git/info/exclude.
 * Idempotent — safe to call repeatedly.
 */
async function ensureGitExclude() {
  const wsRoot = _wsRoot();
  if (!wsRoot) return;

  const excludeUri = vscode.Uri.joinPath(wsRoot, '.git', 'info', 'exclude');

  // Ensure .git/info/ exists
  try {
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(wsRoot, '.git', 'info'));
  } catch {
    return; // Not a git repo
  }

  let content;
  try {
    const raw = await vscode.workspace.fs.readFile(excludeUri);
    content = Buffer.from(raw).toString('utf8');
  } catch {
    content = '';
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

  await vscode.workspace.fs.writeFile(excludeUri, Buffer.from(content));
}

// ── Internal ────────────────────────────────────────────────────

function _mergeConfig(base, updates) {
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

module.exports = {
  GLOBAL_CONFIG_PATH,
  PROJECT_CONFIG_FILENAME,
  readGlobalConfig,
  readProjectConfig,
  resolveConfig,
  writeGlobalConfig,
  writeProjectConfig,
  setFileOverride,
  setSkippedVersion,
  clearSkippedVersion,
  ensureGitExclude,
};
