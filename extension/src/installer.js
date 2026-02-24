const vscode = require('vscode');

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
 * Read the installed .activate-version from the workspace target directory.
 * Returns null if not installed.
 */
async function readInstalledVersion(workspaceUri, targetSubdir) {
  const versionUri = vscode.Uri.joinPath(workspaceUri, targetSubdir, '.activate-version');
  try {
    const raw = await vscode.workspace.fs.readFile(versionUri);
    return Buffer.from(raw).toString('utf8').trim();
  } catch {
    return null;
  }
}

/**
 * Copy selected manifest files from extension assets into the workspace target directory.
 */
async function installFiles(context, workspaceUri, targetSubdir, files, version) {
  const targetBase = vscode.Uri.joinPath(workspaceUri, targetSubdir);
  const installed = [];

  for (const f of files) {
    const src = vscode.Uri.joinPath(context.extensionUri, 'assets', f.src);
    const dest = vscode.Uri.joinPath(targetBase, f.dest);

    // Ensure parent directory exists
    await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(dest, '..'));
    await vscode.workspace.fs.copy(src, dest, { overwrite: true });
    installed.push(f.dest);
  }

  // Write version file
  const versionUri = vscode.Uri.joinPath(targetBase, '.activate-version');
  await vscode.workspace.fs.createDirectory(vscode.Uri.joinPath(versionUri, '..'));
  await vscode.workspace.fs.writeFile(versionUri, Buffer.from(version + '\n'));

  return { installed, version, targetDir: targetBase.fsPath };
}

module.exports = { readBundledManifest, readBundledVersion, readInstalledVersion, installFiles };
