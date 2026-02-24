const vscode = require('vscode');
const { selectFiles } = require('../manifest');
const { readBundledManifest, readBundledVersion, readInstalledVersion, installFiles } = require('../installer');

async function updateCommand(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) {
    vscode.window.showErrorMessage('Activate Framework: Open a workspace folder first.');
    return;
  }

  const config = vscode.workspace.getConfiguration('activate-framework');
  const targetSubdir = config.get('targetSubdir', '.github');

  const installedVersion = await readInstalledVersion(workspaceFolder.uri, targetSubdir);
  const bundledVersion = await readBundledVersion(context);

  if (!installedVersion) {
    const choice = await vscode.window.showInformationMessage(
      'Activate Framework is not installed in this workspace. Install now?',
      'Install',
      'Cancel',
    );
    if (choice === 'Install') {
      await vscode.commands.executeCommand('activate-framework.install');
    }
    return;
  }

  if (installedVersion === bundledVersion) {
    vscode.window.showInformationMessage(`Activate Framework is already up to date (${bundledVersion}).`);
    return;
  }

  const tier = config.get('defaultTier', 'standard');
  const manifest = await readBundledManifest(context);
  const files = selectFiles(manifest.files, tier);

  const confirm = await vscode.window.showInformationMessage(
    `Update Activate Framework from ${installedVersion} to ${bundledVersion}? (${files.length} files, ${tier} tier)`,
    { modal: true },
    'Update',
  );
  if (confirm !== 'Update') return;

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Activate Framework',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: `Updating to ${bundledVersion}…` });
      const result = await installFiles(context, workspaceFolder.uri, targetSubdir, files, bundledVersion);
      vscode.window.showInformationMessage(
        `Activate Framework updated: ${installedVersion} → ${result.version} (${result.installed.length} files).`,
      );
    },
  );
}

module.exports = { updateCommand };
