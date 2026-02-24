const vscode = require('vscode');
const { installCommand } = require('./commands/install');
const { updateCommand } = require('./commands/update');
const { showInstalledCommand } = require('./commands/showInstalled');

function activate(context) {
  context.subscriptions.push(
    vscode.commands.registerCommand('activate-framework.install', () => installCommand(context)),
    vscode.commands.registerCommand('activate-framework.update', () => updateCommand(context)),
    vscode.commands.registerCommand('activate-framework.showInstalled', () => showInstalledCommand(context)),
  );

  checkForUpdatesOnStartup(context);
}

async function checkForUpdatesOnStartup(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) return;

  const config = vscode.workspace.getConfiguration('activate-framework');
  const targetSubdir = config.get('targetSubdir', '.github');
  const versionFileUri = vscode.Uri.joinPath(workspaceFolder.uri, targetSubdir, '.activate-version');

  try {
    const content = await vscode.workspace.fs.readFile(versionFileUri);
    const installedVersion = Buffer.from(content).toString('utf8').trim();
    const bundledVersion = getBundledVersion(context);

    if (installedVersion !== bundledVersion) {
      const choice = await vscode.window.showInformationMessage(
        `Activate Framework update available: ${installedVersion} → ${bundledVersion}`,
        'Update Now',
        'Dismiss',
      );
      if (choice === 'Update Now') {
        await vscode.commands.executeCommand('activate-framework.update');
      }
    }
  } catch {
    // No version file = not installed yet
  }
}

function getBundledVersion(context) {
  return context.extension.packageJSON.version ?? 'unknown';
}

function deactivate() {}

module.exports = { activate, deactivate, getBundledVersion };
