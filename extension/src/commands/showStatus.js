const vscode = require('vscode');
const { readInstalledVersion, readBundledManifest, getActivateRoot, findActivateWorkspaceFolder } = require('../installer');

async function showStatusCommand(context) {
  const installedVersion = await readInstalledVersion(context);
  const bundledVersion = context.extension.packageJSON.version ?? 'unknown';
  const config = vscode.workspace.getConfiguration('activate-framework');
  const tier = config.get('defaultTier', 'standard');
  const root = getActivateRoot(context);
  const isActive = !!findActivateWorkspaceFolder();

  const manifest = await readBundledManifest(context);

  // Check which files exist in the managed root
  const installedFiles = [];
  const missingFiles = [];

  for (const f of manifest.files) {
    const fileUri = vscode.Uri.joinPath(root, '.github', f.dest);
    try {
      await vscode.workspace.fs.stat(fileUri);
      installedFiles.push(`${f.dest} [${f.tier}]`);
    } catch {
      missingFiles.push(`${f.dest} [${f.tier}]`);
    }
  }

  const channel = vscode.window.createOutputChannel('Peregrine Activate');
  channel.clear();
  channel.appendLine('Peregrine Activate — Status');
  channel.appendLine(`Bundled version: ${bundledVersion}`);
  channel.appendLine(`Synced version:  ${installedVersion ?? 'not synced'}`);
  channel.appendLine(`Tier:            ${tier}`);
  channel.appendLine(`Workspace root:  ${isActive ? 'active' : 'not active'}`);
  channel.appendLine(`Storage:         ${root.fsPath}`);
  channel.appendLine('');
  channel.appendLine(`Synced files (${installedFiles.length}):`);
  installedFiles.forEach((f) => channel.appendLine(`  ✓ ${f}`));

  if (missingFiles.length > 0) {
    channel.appendLine('');
    channel.appendLine(`Available at higher tier (${missingFiles.length}):`);
    missingFiles.forEach((f) => channel.appendLine(`  ○ ${f}`));
  }

  channel.show(true);
}

module.exports = { showStatusCommand };
