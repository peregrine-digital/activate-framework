const vscode = require('vscode');
const { selectFiles } = require('./manifest');

const WORKSPACE_ROOT_NAME = 'Peregrine Activate';

/**
 * Read the bundled manifest.json from the extension's assets directory.
 */
async function readBundledManifest(context) {
  const manifestUri = vscode.Uri.joinPath(context.extensionUri, 'assets', 'manifest.json');
  const raw = await vscode.workspace.fs.readFile(manifestUri);
  return JSON.parse(Buffer.from(raw).toString('utf8'));
}

/**
 * Read the bundled .activate-version from the extension's assets directory.
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
 * Returns null if not installed.
 */
async function readInstalledVersion(context) {
  const versionUri = vscode.Uri.joinPath(getActivateRoot(context), '.activate-version');
  try {
    const raw = await vscode.workspace.fs.readFile(versionUri);
    return Buffer.from(raw).toString('utf8').trim();
  } catch {
    return null;
  }
}

/**
 * Copy selected manifest files from extension assets into globalStorage,
 * laid out under a .github/ directory so Copilot discovers them when the
 * folder is added as a workspace root.
 */
async function syncFiles(context, tier) {
  const manifest = await readBundledManifest(context);
  const version = await readBundledVersion(context);
  const files = selectFiles(manifest.files, tier);
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

  // Write version file
  const versionUri = vscode.Uri.joinPath(root, '.activate-version');
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(version + '\n'));

  return { installed, version, rootUri: root };
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
 * Update only the files that are currently installed on disk.
 * Unlike syncFiles(), this does NOT re-add files the user has removed.
 *
 * @param {vscode.ExtensionContext} context
 * @returns {Promise<{updated: string[], version: string}>}
 */
async function updateInstalledFiles(context) {
  const manifest = await readBundledManifest(context);
  const version = await readBundledVersion(context);
  const root = getActivateRoot(context);

  const updated = [];

  for (const f of manifest.files) {
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
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(version + '\n'));

  return { updated, version };
}

module.exports = {
  WORKSPACE_ROOT_NAME,
  readBundledManifest,
  readBundledVersion,
  readInstalledVersion,
  getActivateRoot,
  syncFiles,
  findActivateWorkspaceFolder,
  addWorkspaceRoot,
  removeWorkspaceRoot,
  installFile,
  uninstallFile,
  isFileInstalled,
  updateInstalledFiles,
};

