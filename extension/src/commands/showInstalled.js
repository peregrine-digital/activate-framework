const vscode = require('vscode');
const { readBundledManifest, readInstalledVersion } = require('../installer');

async function showInstalledCommand(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) {
    vscode.window.showErrorMessage('Activate Framework: Open a workspace folder first.');
    return;
  }

  const config = vscode.workspace.getConfiguration('activate-framework');
  const targetSubdir = config.get('targetSubdir', '.github');

  const installedVersion = await readInstalledVersion(workspaceFolder.uri, targetSubdir);
  if (!installedVersion) {
    const choice = await vscode.window.showInformationMessage(
      'Activate Framework is not installed in this workspace.',
      'Install Now',
    );
    if (choice === 'Install Now') {
      await vscode.commands.executeCommand('activate-framework.install');
    }
    return;
  }

  // Read manifest to show what's available
  const manifest = await readBundledManifest(context);
  const targetBase = vscode.Uri.joinPath(workspaceFolder.uri, targetSubdir);

  // Check which manifest files actually exist on disk
  const installedFiles = [];
  const missingFiles = [];

  for (const f of manifest.files) {
    const fileUri = vscode.Uri.joinPath(targetBase, f.dest);
    try {
      await vscode.workspace.fs.stat(fileUri);
      installedFiles.push(`${f.dest} [${f.tier}]`);
    } catch {
      missingFiles.push(`${f.dest} [${f.tier}]`);
    }
  }

  const channel = vscode.window.createOutputChannel('Activate Framework');
  channel.clear();
  channel.appendLine('Activate Framework — Installed Files');
  channel.appendLine(`Version: ${installedVersion}`);
  channel.appendLine(`Location: ${targetBase.fsPath}`);
  channel.appendLine('');
  channel.appendLine(`Installed (${installedFiles.length}):`);
  installedFiles.forEach((f) => channel.appendLine(`  ✓ ${f}`));

  if (missingFiles.length > 0) {
    channel.appendLine('');
    channel.appendLine(`Available but not installed (${missingFiles.length}):`);
    missingFiles.forEach((f) => channel.appendLine(`  ○ ${f}`));
  }

  channel.show(true);
}

module.exports = { showInstalledCommand };
