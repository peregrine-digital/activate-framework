/**
 * Silent injector — copies Activate managed files directly into the
 * workspace's `.github/` directory and hides them from git using
 * `.git/info/exclude`.
 *
 * Design goals:
 * - Zero multi-root workspace — no restart, no workspace ceremony
 * - Invisible to git — managed paths added to `.git/info/exclude`
 * - Never overwrites pre-existing user files
 * - Tracks what was injected via `.github/.activate-installed.json`
 * - Clean uninstall removes only files we placed
 *
 * PIPELINE: Every file mutation flows through writeSidecar(), which
 * automatically syncs `.git/info/exclude`. No caller ever touches
 * the exclude directly — the sidecar is the single source of truth.
 */
const vscode = require('vscode');
const { selectFiles, parseManifestData } = require('./manifest');

// ── Constants ───────────────────────────────────────────────────

/** Sidecar file that records which files we injected */
const SIDECAR_REL = '.github/.activate-installed.json';

/** Marker comment used in .git/info/exclude */
const EXCLUDE_MARKER_START = '# >>> Peregrine Activate (managed — do not edit)';
const EXCLUDE_MARKER_END = '# <<< Peregrine Activate';

// ── Helpers ─────────────────────────────────────────────────────

/**
 * Get the primary workspace folder URI.
 * Returns undefined if no workspace is open.
 */
function getWorkspaceRoot() {
  return vscode.workspace.workspaceFolders?.[0]?.uri;
}

/**
 * Read the sidecar JSON from the workspace.
 * Returns null if not present.
 *
 * Shape: { manifest: string, version: string, tier: string, files: string[] }
 */
async function readSidecar(wsRoot) {
  const uri = vscode.Uri.joinPath(wsRoot, SIDECAR_REL);
  try {
    const raw = await vscode.workspace.fs.readFile(uri);
    return JSON.parse(Buffer.from(raw).toString('utf8'));
  } catch {
    return null;
  }
}

/**
 * Write the sidecar JSON to the workspace, then AUTOMATICALLY sync
 * `.git/info/exclude` from the sidecar's file list.
 *
 * This is the ONLY way exclude gets updated — no other function
 * touches it directly. Single source of truth.
 */
async function writeSidecar(wsRoot, data) {
  const uri = vscode.Uri.joinPath(wsRoot, SIDECAR_REL);
  await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(wsRoot, '.github'));
  await vscode.workspace.fs.writeFile(uri, Buffer.from(JSON.stringify(data, null, 2) + '\n'));

  // Derive exclude paths from sidecar — always in sync
  const excludePaths = [SIDECAR_REL, ...(data.files || [])];
  await _syncGitExclude(wsRoot, excludePaths);
}

/**
 * Delete the sidecar and remove the exclude block.
 */
async function deleteSidecar(wsRoot) {
  const uri = vscode.Uri.joinPath(wsRoot, SIDECAR_REL);
  try {
    await vscode.workspace.fs.delete(uri);
  } catch {
    // already gone
  }
  await _removeGitExclude(wsRoot);
}

/**
 * Check whether a file exists at the given URI.
 */
async function fileExists(uri) {
  try {
    await vscode.workspace.fs.stat(uri);
    return true;
  } catch {
    return false;
  }
}

// ── Git exclude (PRIVATE — only called by writeSidecar/deleteSidecar) ──

/**
 * Get the URI for `.git/info/exclude` in the workspace.
 */
function _getGitExcludeUri(wsRoot) {
  return vscode.Uri.joinPath(wsRoot, '.git', 'info', 'exclude');
}

/**
 * Sync `.git/info/exclude` with the given paths.
 * Replaces any existing Activate block, or appends a new one.
 * PRIVATE — called automatically by writeSidecar().
 */
async function _syncGitExclude(wsRoot, paths) {
  const excludeUri = _getGitExcludeUri(wsRoot);

  // Ensure .git/info/ exists
  try {
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(wsRoot, '.git', 'info'));
  } catch {
    // .git may not exist (not a git repo) — bail silently
    return;
  }

  let content;
  try {
    const raw = await vscode.workspace.fs.readFile(excludeUri);
    content = Buffer.from(raw).toString('utf8');
  } catch {
    content = '';
  }

  // Build the new block
  const block = [
    EXCLUDE_MARKER_START,
    ...paths,
    EXCLUDE_MARKER_END,
  ].join('\n');

  // Replace existing block or append
  const startIdx = content.indexOf(EXCLUDE_MARKER_START);
  const endIdx = content.indexOf(EXCLUDE_MARKER_END);

  if (startIdx !== -1 && endIdx !== -1) {
    content = content.slice(0, startIdx) + block + content.slice(endIdx + EXCLUDE_MARKER_END.length);
  } else {
    if (content.length > 0 && !content.endsWith('\n')) {
      content += '\n';
    }
    content += '\n' + block + '\n';
  }

  await vscode.workspace.fs.writeFile(excludeUri, Buffer.from(content));
}

/**
 * Remove the Activate block from `.git/info/exclude`.
 * PRIVATE — called automatically by deleteSidecar().
 */
async function _removeGitExclude(wsRoot) {
  const excludeUri = _getGitExcludeUri(wsRoot);

  let content;
  try {
    const raw = await vscode.workspace.fs.readFile(excludeUri);
    content = Buffer.from(raw).toString('utf8');
  } catch {
    return;
  }

  const startIdx = content.indexOf(EXCLUDE_MARKER_START);
  const endIdx = content.indexOf(EXCLUDE_MARKER_END);

  if (startIdx === -1 || endIdx === -1) return;

  content = content.slice(0, startIdx) + content.slice(endIdx + EXCLUDE_MARKER_END.length);
  content = content.replace(/\n{3,}/g, '\n\n');

  await vscode.workspace.fs.writeFile(excludeUri, Buffer.from(content));
}

// ── Manifest reading (reuse from installer) ─────────────────────

const {
  discoverBundledManifests,
  readBundledManifestById,
  parseFrontmatterVersion,
} = require('./installer');

// ── Core injection logic ────────────────────────────────────────

/**
 * Inject managed files into the workspace's `.github/` directory.
 *
 * Pipeline: copy files → write sidecar → exclude auto-synced.
 *
 * @param {vscode.ExtensionContext} context
 * @param {string} tier
 * @param {string} [manifestId]
 * @returns {Promise<{injected: string[], skipped: string[], version: string, manifestId: string}>}
 */
async function injectFiles(context, tier, manifestId) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) throw new Error('No workspace folder open');

  const chosen = manifestId
    ? await readBundledManifestById(context, manifestId)
    : (await discoverBundledManifests(context))[0];

  if (!chosen) throw new Error('No manifest available');

  const files = selectFiles(chosen.files, tier);
  const oldSidecar = await readSidecar(wsRoot);
  const previouslyInjected = new Set(oldSidecar?.files || []);

  const injected = [];
  const skipped = [];

  for (const f of files) {
    const destRel = `.github/${f.dest}`;
    const destUri = vscode.Uri.joinPath(wsRoot, destRel);
    const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);

    // If file exists and we didn't put it there, skip
    if ((await fileExists(destUri)) && !previouslyInjected.has(destRel)) {
      skipped.push(f.dest);
      continue;
    }

    try {
      await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(destUri, '..'));
      await vscode.workspace.fs.copy(srcUri, destUri, { overwrite: true });
      injected.push(destRel);
    } catch {
      // Asset may not exist in bundle
    }
  }

  // Inject AGENTS.md at workspace root
  const agentsRel = 'AGENTS.md';
  const agentsDestUri = vscode.Uri.joinPath(wsRoot, agentsRel);
  const agentsSrcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', 'AGENTS.md');

  if ((await fileExists(agentsDestUri)) && !previouslyInjected.has(agentsRel)) {
    skipped.push(agentsRel);
  } else {
    try {
      await vscode.workspace.fs.copy(agentsSrcUri, agentsDestUri, { overwrite: true });
      injected.push(agentsRel);
    } catch {
      // AGENTS.md may not be in the bundle
    }
  }

  // Remove stale files that were in the old sidecar but not in the new set.
  // This is critical when switching manifests — old manifest files must be
  // deleted from disk, not just dropped from the sidecar/exclude.
  const newFileSet = new Set(injected);
  for (const oldRel of previouslyInjected) {
    if (!newFileSet.has(oldRel)) {
      const staleUri = vscode.Uri.joinPath(wsRoot, oldRel);
      try {
        await vscode.workspace.fs.delete(staleUri);
      } catch {
        // file may already be gone
      }
    }
  }

  // Write sidecar (this automatically syncs git exclude)
  await writeSidecar(wsRoot, {
    manifest: chosen.id,
    version: chosen.version,
    tier,
    files: injected,
  });

  return { injected, skipped, version: chosen.version, manifestId: chosen.id };
}

/**
 * Read the injected version info from the sidecar.
 * Returns { manifest, version, tier, files } or null.
 */
async function readInjectedVersion() {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return null;
  return readSidecar(wsRoot);
}

/**
 * Check if a file is injected in the workspace.
 */
async function isFileInjected(file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;
  const destUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
  return fileExists(destUri);
}

/**
 * Inject a single file into the workspace.
 * @param {vscode.ExtensionContext} context
 * @param {{src: string, dest: string}} file
 * @returns {Promise<boolean>}
 */
async function injectSingleFile(context, file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const destRel = `.github/${file.dest}`;
  const destUri = vscode.Uri.joinPath(wsRoot, destRel);
  const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);

  try {
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(destUri, '..'));
    await vscode.workspace.fs.copy(srcUri, destUri, { overwrite: true });

    // Update sidecar (this automatically syncs git exclude)
    const sidecar = (await readSidecar(wsRoot)) || { manifest: '', version: '', tier: '', files: [] };
    if (!sidecar.files.includes(destRel)) {
      sidecar.files.push(destRel);
    }
    await writeSidecar(wsRoot, sidecar);

    return true;
  } catch {
    return false;
  }
}

/**
 * Remove a single injected file from the workspace.
 * @param {{dest: string}} file
 * @returns {Promise<boolean>}
 */
async function removeSingleFile(file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const destRel = `.github/${file.dest}`;
  const destUri = vscode.Uri.joinPath(wsRoot, destRel);

  try {
    await vscode.workspace.fs.delete(destUri);

    // Update sidecar (this automatically syncs git exclude)
    const sidecar = await readSidecar(wsRoot);
    if (sidecar) {
      sidecar.files = sidecar.files.filter((p) => p !== destRel);
      await writeSidecar(wsRoot, sidecar);
    }

    return true;
  } catch {
    return false;
  }
}

/**
 * Remove all injected files from the workspace and clean up.
 */
async function removeAllInjected() {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const sidecar = await readSidecar(wsRoot);
  if (!sidecar) return false;

  // Delete each injected file
  for (const relPath of sidecar.files) {
    const uri = vscode.Uri.joinPath(wsRoot, relPath);
    try {
      await vscode.workspace.fs.delete(uri);
    } catch {
      // file may already be gone
    }
  }

  // Delete sidecar (this automatically removes git exclude block)
  await deleteSidecar(wsRoot);

  return true;
}

/**
 * Update only the files that are currently injected.
 * Does not re-add files the user removed.
 *
 * @param {vscode.ExtensionContext} context
 * @returns {Promise<{updated: string[], version: string}>}
 */
async function updateInjectedFiles(context) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) throw new Error('No workspace folder open');

  const sidecar = await readSidecar(wsRoot);
  const manifestId = sidecar?.manifest || 'activate-framework';

  let chosen;
  try {
    chosen = await readBundledManifestById(context, manifestId);
  } catch {
    const all = await discoverBundledManifests(context);
    chosen = all[0];
  }
  if (!chosen) throw new Error('No manifest available for update');

  const previousFiles = new Set(sidecar?.files || []);
  const updated = [];

  for (const f of chosen.files) {
    const destRel = `.github/${f.dest}`;
    if (!previousFiles.has(destRel)) continue;

    const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);
    const destUri = vscode.Uri.joinPath(wsRoot, destRel);

    try {
      await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(destUri, '..'));
      await vscode.workspace.fs.copy(srcUri, destUri, { overwrite: true });
      updated.push(f.dest);
    } catch {
      // file may not exist in bundle
    }
  }

  // Re-copy AGENTS.md if it was injected
  if (previousFiles.has('AGENTS.md')) {
    try {
      const agentsSrc = vscode.Uri.joinPath(context.extensionUri, 'assets', 'AGENTS.md');
      const agentsDest = vscode.Uri.joinPath(wsRoot, 'AGENTS.md');
      await vscode.workspace.fs.copy(agentsSrc, agentsDest, { overwrite: true });
    } catch {}
  }

  // Update sidecar version (this automatically syncs git exclude)
  if (sidecar) {
    sidecar.version = chosen.version;
    await writeSidecar(wsRoot, sidecar);
  }

  return { updated, version: chosen.version };
}

/**
 * Read the frontmatter version from an injected file in the workspace.
 */
async function readInjectedFileVersion(file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return null;
  const destUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
  try {
    const raw = await vscode.workspace.fs.readFile(destUri);
    return parseFrontmatterVersion(raw);
  } catch {
    return null;
  }
}

/**
 * Skip an update by stamping the frontmatter version in the injected file.
 */
async function skipInjectedFileUpdate(context, file) {
  const wsRoot = getWorkspaceRoot();
  if (!wsRoot) return false;

  const srcUri = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);
  let bundledVersion;
  try {
    const raw = await vscode.workspace.fs.readFile(srcUri);
    bundledVersion = parseFrontmatterVersion(raw);
  } catch {
    return false;
  }
  if (!bundledVersion) return false;

  const destUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
  try {
    const raw = await vscode.workspace.fs.readFile(destUri);
    let text = Buffer.from(raw).toString('utf8');

    const fmMatch = text.match(/^(---\s*\n)([\s\S]*?)(\n---)/);
    if (!fmMatch) return false;

    const before = fmMatch[1];
    let fm = fmMatch[2];
    const after = fmMatch[3];

    if (fm.match(/^version:\s/m)) {
      fm = fm.replace(/^version:\s*['"]?[^'"\n]+['"]?\s*$/m, `version: '${bundledVersion}'`);
    } else {
      fm += `\nversion: '${bundledVersion}'`;
    }

    text = before + fm + after + text.slice(fmMatch[0].length);
    await vscode.workspace.fs.writeFile(destUri, Buffer.from(text));
    return true;
  } catch {
    return false;
  }
}

module.exports = {
  SIDECAR_REL,
  getWorkspaceRoot,
  readSidecar,
  readInjectedVersion,
  injectFiles,
  injectSingleFile,
  removeSingleFile,
  removeAllInjected,
  isFileInjected,
  updateInjectedFiles,
  readInjectedFileVersion,
  skipInjectedFileUpdate,
};
