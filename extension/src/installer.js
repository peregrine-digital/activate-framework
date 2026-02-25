const vscode = require('vscode');
const { selectFiles, parseManifestData } = require('./manifest');
const {
  isRemoteMode,
  getSourceConfig,
  discoverRemoteManifests,
  loadRemoteManifest,
} = require('./fetcher');

const WORKSPACE_ROOT_NAME = 'Peregrine Activate';

// ── Manifest discovery ──────────────────────────────────────────

/**
 * Discover all bundled manifests from the extension's assets directory.
 * Looks in assets/manifests/ first, falls back to assets/manifest.json.
 * Returns an array of { id, name, description, version, files } objects.
 */
async function discoverBundledManifests(context) {
  const manifestsDir = vscode.Uri.joinPath(context.extensionUri, 'assets', 'manifests');
  try {
    const entries = await vscode.workspace.fs.readDirectory(manifestsDir);
    const jsonFiles = entries
      .filter(([name, type]) => name.endsWith('.json') && type === vscode.FileType.File)
      .map(([name]) => name)
      .sort();

    if (jsonFiles.length > 0) {
      const manifests = [];
      for (const file of jsonFiles) {
        const uri = vscode.Uri.joinPath(manifestsDir, file);
        const raw = await vscode.workspace.fs.readFile(uri);
        const data = JSON.parse(Buffer.from(raw).toString('utf8'));
        const id = file.replace(/\.json$/, '');
        manifests.push(parseManifestData(id, data));
      }
      return manifests;
    }
  } catch {
    // manifests/ directory doesn't exist — fall back to legacy
  }

  // Legacy fallback: single manifest.json
  try {
    const manifest = await readBundledManifest(context);
    const version = await readBundledVersion(context);
    return [parseManifestData('activate-framework', { ...manifest, version })];
  } catch {
    return [];
  }
}

/**
 * Read a single bundled manifest by id.
 * @param {vscode.ExtensionContext} context
 * @param {string} manifestId
 * @returns {Promise<{id: string, name: string, description: string, version: string, files: Array}>}
 */
async function readBundledManifestById(context, manifestId) {
  const uri = vscode.Uri.joinPath(context.extensionUri, 'assets', 'manifests', `${manifestId}.json`);
  try {
    const raw = await vscode.workspace.fs.readFile(uri);
    const data = JSON.parse(Buffer.from(raw).toString('utf8'));
    return parseManifestData(manifestId, data);
  } catch {
    // Fall back to legacy manifest.json
    const manifest = await readBundledManifest(context);
    const version = await readBundledVersion(context);
    return parseManifestData('activate-framework', { ...manifest, version });
  }
}

// ── Smart manifest discovery (bundled or remote) ────────────────

/**
 * Discover all available manifests (bundled or remote based on settings).
 * @param {vscode.ExtensionContext} context
 * @returns {Promise<Array<{id: string, name: string, description: string, version: string, files: Array, tiers: Array|undefined}>>}
 */
async function discoverManifests(context) {
  if (isRemoteMode()) {
    try {
      const manifests = await discoverRemoteManifests();
      if (manifests.length > 0) return manifests;
    } catch (err) {
      console.warn('Failed to fetch remote manifests, falling back to bundled:', err.message);
    }
  }
  return discoverBundledManifests(context);
}

/**
 * Read a single manifest by ID (bundled or remote based on settings).
 * @param {vscode.ExtensionContext} context
 * @param {string} manifestId
 * @returns {Promise<{id: string, name: string, description: string, version: string, files: Array, tiers: Array|undefined}>}
 */
async function readManifestById(context, manifestId) {
  if (isRemoteMode()) {
    try {
      const manifest = await loadRemoteManifest(manifestId);
      if (manifest) return manifest;
    } catch (err) {
      console.warn(`Failed to fetch remote manifest ${manifestId}, falling back to bundled:`, err.message);
    }
  }
  return readBundledManifestById(context, manifestId);
}

// ── Legacy helpers (backward compatible) ────────────────────────

/**
 * Read the bundled manifest.json from the extension's assets directory.
 * @deprecated Use discoverBundledManifests or readBundledManifestById instead.
 */
async function readBundledManifest(context) {
  const manifestUri = vscode.Uri.joinPath(context.extensionUri, 'assets', 'manifest.json');
  const raw = await vscode.workspace.fs.readFile(manifestUri);
  return JSON.parse(Buffer.from(raw).toString('utf8'));
}

/**
 * Read the bundled .activate-version from the extension's assets directory.
 * @deprecated Version is now embedded in each manifest's version field.
 */
async function readBundledVersion(context) {
  const versionUri = vscode.Uri.joinPath(context.extensionUri, 'assets', '.activate-version');
  const raw = await vscode.workspace.fs.readFile(versionUri);
  return Buffer.from(raw).toString('utf8').trim();
}

/**
 * Get the root directory inside globalStorage where we lay out the
 * .github/ structure that Copilot will discover via multi-root workspace.
 */
function getActivateRoot(context) {
  return vscode.Uri.joinPath(context.globalStorageUri, 'activate-root');
}

/**
 * Read the installed .activate-version from the managed root.
 * Returns a string (legacy) or parsed JSON { manifest, version }.
 * Returns null if not installed.
 */
async function readInstalledVersion(context) {
  const versionUri = vscode.Uri.joinPath(getActivateRoot(context), '.activate-version');
  try {
    const raw = Buffer.from(await vscode.workspace.fs.readFile(versionUri)).toString('utf8').trim();
    try {
      return JSON.parse(raw);
    } catch {
      // Legacy format: plain version string
      return { manifest: 'activate-framework', version: raw };
    }
  } catch {
    return null;
  }
}

/**
 * Copy selected manifest files from extension assets into globalStorage,
 * laid out under a .github/ directory so Copilot discovers them when the
 * folder is added as a workspace root.
 *
 * @param {vscode.ExtensionContext} context
 * @param {string} tier
 * @param {string} [manifestId] - If set, sync only this manifest. Otherwise sync the active manifest.
 */
async function syncFiles(context, tier, manifestId) {
  const chosen = manifestId
    ? await readBundledManifestById(context, manifestId)
    : (await discoverBundledManifests(context))[0];

  if (!chosen) throw new Error('No manifest available');

  const files = selectFiles(chosen.files, tier);
  const root = getActivateRoot(context);

  // Ensure the root exists
  await vscode.workspace.fs.createDirectory(root);

  const installed = [];

  for (const f of files) {
    const src = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);
    const dest = vscode.Uri.joinPath(root, '.github', f.dest);

    try {
      await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(dest, '..'));
      await vscode.workspace.fs.copy(src, dest, { overwrite: true });
      installed.push(f.dest);
    } catch {
      // File may not exist in the bundle (e.g. manifest references unreleased content)
    }
  }

  // Copy AGENTS.md to the root (not inside .github/) so Copilot finds it
  try {
    const agentsSrc = vscode.Uri.joinPath(context.extensionUri, 'assets', 'AGENTS.md');
    const agentsDest = vscode.Uri.joinPath(root, 'AGENTS.md');
    await vscode.workspace.fs.copy(agentsSrc, agentsDest, { overwrite: true });
  } catch {
    // AGENTS.md may not be in the bundle
  }

  // Write version file with manifest info
  const versionUri = vscode.Uri.joinPath(root, '.activate-version');
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(
    JSON.stringify({ manifest: chosen.id, version: chosen.version }) + '\n',
  ));

  return { installed, version: chosen.version, manifestId: chosen.id, rootUri: root };
}

/**
 * Check if the managed activate root is already a workspace folder.
 */
function findActivateWorkspaceFolder() {
  const folders = vscode.workspace.workspaceFolders ?? [];
  return folders.find((f) => f.name === WORKSPACE_ROOT_NAME);
}

/**
 * Add the managed activate root as a workspace folder.
 * Returns true if added, false if already present.
 */
function addWorkspaceRoot(context) {
  if (findActivateWorkspaceFolder()) return false;

  const root = getActivateRoot(context);
  const count = vscode.workspace.workspaceFolders?.length ?? 0;

  // Add at the end so it doesn't displace the user's primary folder
  vscode.workspace.updateWorkspaceFolders(count, 0, {
    uri: root,
    name: WORKSPACE_ROOT_NAME,
  });
  return true;
}

/**
 * Remove the managed activate root from the workspace.
 * Returns true if removed.
 */
function removeWorkspaceRoot() {
  const folder = findActivateWorkspaceFolder();
  if (!folder) return false;

  vscode.workspace.updateWorkspaceFolders(folder.index, 1);
  return true;
}

/**
 * Install a single file from the bundle into globalStorage.
 * @param {vscode.ExtensionContext} context
 * @param {{src: string, dest: string}} file
 * @returns {Promise<boolean>} true if installed successfully
 */
async function installFile(context, file) {
  const root = getActivateRoot(context);
  const src = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);
  const dest = vscode.Uri.joinPath(root, '.github', file.dest);

  try {
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(dest, '..'));
    await vscode.workspace.fs.copy(src, dest, { overwrite: true });
    return true;
  } catch {
    return false;
  }
}

/**
 * Remove a single file from globalStorage.
 * @param {vscode.ExtensionContext} context
 * @param {{dest: string}} file
 * @returns {Promise<boolean>} true if removed successfully
 */
async function uninstallFile(context, file) {
  const root = getActivateRoot(context);
  const dest = vscode.Uri.joinPath(root, '.github', file.dest);

  try {
    await vscode.workspace.fs.delete(dest);
    return true;
  } catch {
    return false;
  }
}

/**
 * Check if a file is installed in globalStorage.
 * @param {vscode.ExtensionContext} context
 * @param {{dest: string}} file
 * @returns {Promise<boolean>}
 */
async function isFileInstalled(context, file) {
  const root = getActivateRoot(context);
  const dest = vscode.Uri.joinPath(root, '.github', file.dest);
  try {
    await vscode.workspace.fs.stat(dest);
    return true;
  } catch {
    return false;
  }
}

/**
 * Parse the `version` field from YAML frontmatter in a markdown buffer.
 * Returns the version string or null if not found.
 */
function parseFrontmatterVersion(buffer) {
  const text = Buffer.from(buffer).toString('utf8');
  const match = text.match(/^---\s*\n([\s\S]*?)\n---/);
  if (!match) return null;
  const fm = match[1];
  const versionLine = fm.match(/^version:\s*['"]?([^'"\n]+)['"]?\s*$/m);
  return versionLine ? versionLine[1].trim() : null;
}

/**
 * Read the frontmatter version from an installed file in globalStorage.
 * @param {vscode.ExtensionContext} context
 * @param {{dest: string}} file
 * @returns {Promise<string|null>}
 */
async function readInstalledFileVersion(context, file) {
  const root = getActivateRoot(context);
  const dest = vscode.Uri.joinPath(root, '.github', file.dest);
  try {
    const raw = await vscode.workspace.fs.readFile(dest);
    return parseFrontmatterVersion(raw);
  } catch {
    return null;
  }
}

/**
 * Read the frontmatter version from a bundled (source) file.
 * @param {vscode.ExtensionContext} context
 * @param {{src: string}} file
 * @returns {Promise<string|null>}
 */
async function readBundledFileVersion(context, file) {
  const src = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);
  try {
    const raw = await vscode.workspace.fs.readFile(src);
    return parseFrontmatterVersion(raw);
  } catch {
    return null;
  }
}

/**
 * "Skip" an update by rewriting the frontmatter version in the installed
 * copy to match the bundled version, without changing any other content.
 * @param {vscode.ExtensionContext} context
 * @param {{src: string, dest: string}} file
 * @returns {Promise<boolean>}
 */
async function skipFileUpdate(context, file) {
  const bundledVersion = await readBundledFileVersion(context, file);
  if (!bundledVersion) return false;

  const root = getActivateRoot(context);
  const dest = vscode.Uri.joinPath(root, '.github', file.dest);

  try {
    const raw = await vscode.workspace.fs.readFile(dest);
    let text = Buffer.from(raw).toString('utf8');

    // Replace the version line in frontmatter
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
    await vscode.workspace.fs.writeFile(dest, Buffer.from(text));
    return true;
  } catch {
    return false;
  }
}

/**
 * Update only the files that are currently installed on disk.
 * Unlike syncFiles(), this does NOT re-add files the user has removed.
 *
 * @param {vscode.ExtensionContext} context
 * @returns {Promise<{updated: string[], version: string}>}
 */
async function updateInstalledFiles(context) {
  // Determine which manifest is active
  const installedInfo = await readInstalledVersion(context);
  const manifestId = installedInfo?.manifest || 'activate-framework';

  let chosen;
  try {
    chosen = await readBundledManifestById(context, manifestId);
  } catch {
    // Fall back to discovering all and using the first
    const all = await discoverBundledManifests(context);
    chosen = all[0];
  }

  if (!chosen) throw new Error('No manifest available for update');

  const root = getActivateRoot(context);
  const updated = [];

  for (const f of chosen.files) {
    if (!(await isFileInstalled(context, f))) continue;

    const src = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);
    const dest = vscode.Uri.joinPath(root, '.github', f.dest);

    try {
      await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(dest, '..'));
      await vscode.workspace.fs.copy(src, dest, { overwrite: true });
      updated.push(f.dest);
    } catch {
      // file may not exist in the bundle
    }
  }

  // Re-copy AGENTS.md if it exists
  try {
    const agentsSrc = vscode.Uri.joinPath(context.extensionUri, 'assets', 'AGENTS.md');
    const agentsDest = vscode.Uri.joinPath(root, 'AGENTS.md');
    await vscode.workspace.fs.copy(agentsSrc, agentsDest, { overwrite: true });
  } catch {
    // AGENTS.md may not be in the bundle
  }

  // Write version file
  const versionUri = vscode.Uri.joinPath(root, '.activate-version');
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(
    JSON.stringify({ manifest: chosen.id, version: chosen.version }) + '\n',
  ));

  return { updated, version: chosen.version };
}

module.exports = {
  // Smart manifest discovery (respects remote mode setting)
  discoverManifests,
  readManifestById,
  // Bundled-only (direct access)
  discoverBundledManifests,
  readBundledManifestById,
  readBundledManifest,
  readBundledVersion,
  // Install state
  readInstalledVersion,
  getActivateRoot,
  syncFiles,
  // Legacy workspace cleanup (for migration)
  findActivateWorkspaceFolder,
  removeWorkspaceRoot,
  // File operations
  installFile,
  uninstallFile,
  isFileInstalled,
  updateInstalledFiles,
  parseFrontmatterVersion,
  readInstalledFileVersion,
  readBundledFileVersion,
  skipFileUpdate,
};

