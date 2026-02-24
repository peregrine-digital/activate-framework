const vscode = require('vscode');
const { syncFiles, addWorkspaceRoot, readInstalledVersion, readBundledVersion, removeWorkspaceRoot, findActivateWorkspaceFolder } = require('./installer');
const { changeTierCommand } = require('./commands/changeTier');
const { showStatusCommand } = require('./commands/showStatus');
const { ActivateTreeProvider } = require('./treeView');

function activate(context) {
  // Create the sidebar tree view
  const treeProvider = new ActivateTreeProvider(context);
  const treeView = vscode.window.createTreeView('activate-framework.filesView', {
    treeDataProvider: treeProvider,
    showCollapseAll: true,
  });

  context.subscriptions.push(
    treeView,
    vscode.commands.registerCommand('activate-framework.changeTier', async () => {
      await changeTierCommand(context);
      treeProvider.refresh();
    }),
    vscode.commands.registerCommand('activate-framework.showStatus', () => showStatusCommand(context)),
    vscode.commands.registerCommand('activate-framework.remove', async () => {
      await removeCommand(context);
      treeProvider.refresh();
    }),
    vscode.commands.registerCommand('activate-framework.refresh', () => treeProvider.refresh()),
  );

  // Auto-sync files and add workspace root on activation
  autoSetup(context).then(() => treeProvider.refresh());
}

async function autoSetup(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) return;

  const config = vscode.workspace.getConfiguration('activate-framework');
  const tier = config.get('defaultTier', 'standard');

  const installedVersion = await readInstalledVersion(context);
  const bundledVersion = context.extension.packageJSON.version ?? 'unknown';

  // Sync files if first run or version mismatch
  if (installedVersion !== bundledVersion) {
    await syncFiles(context, tier);

    if (installedVersion) {
      vscode.window.showInformationMessage(
        `Peregrine Activate updated: ${installedVersion} → ${bundledVersion}`,
      );
    }
  }

  // Ensure the root is in the workspace
  const added = addWorkspaceRoot(context);
  if (added && !installedVersion) {
    vscode.window.showInformationMessage(
      `Peregrine Activate ${bundledVersion} (${tier}) is ready.`,
    );
  }
}

async function removeCommand(context) {
  const removed = removeWorkspaceRoot();
  if (removed) {
    vscode.window.showInformationMessage('Peregrine Activate workspace root removed.');
  } else {
    vscode.window.showInformationMessage('Peregrine Activate is not in the workspace.');
  }
}

function deactivate() {}

module.exports = { activate, deactivate };
