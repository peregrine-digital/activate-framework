const vscode = require('vscode');
const {
  syncFiles,
  addWorkspaceRoot,
  readInstalledVersion,
  removeWorkspaceRoot,
  findActivateWorkspaceFolder,
  installFile,
  uninstallFile,
  updateInstalledFiles,
} = require('./installer');
const { changeTierCommand } = require('./commands/changeTier');
const { showStatusCommand } = require('./commands/showStatus');
const { ActivateTreeProvider } = require('./treeView');
const { ControlPanelProvider } = require('./controlPanel');

function activate(context) {
  // Control panel (WebviewView with real buttons)
  const controlPanel = new ControlPanelProvider(context);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      ControlPanelProvider.viewType,
      controlPanel,
    ),
  );

  // File browser tree
  const treeProvider = new ActivateTreeProvider(context);
  const treeView = vscode.window.createTreeView('activate-framework.filesView', {
    treeDataProvider: treeProvider,
    showCollapseAll: true,
  });

  /** Mark config dirty → refresh both views + show banner */
  function markDirty() {
    controlPanel.markDirty();
  }

  /** Refresh both sidebar views */
  function refreshAll() {
    treeProvider.refresh();
    controlPanel.refresh();
  }

  context.subscriptions.push(
    treeView,
    vscode.commands.registerCommand('activate-framework.changeTier', async () => {
      await changeTierCommand(context);
      markDirty();
      refreshAll();
    }),
    vscode.commands.registerCommand('activate-framework.showStatus', () =>
      showStatusCommand(context),
    ),
    vscode.commands.registerCommand('activate-framework.remove', async () => {
      const removed = removeWorkspaceRoot();
      if (removed) {
        vscode.window.showInformationMessage(
          'Peregrine Activate workspace root removed.',
        );
      }
      refreshAll();
    }),
    vscode.commands.registerCommand('activate-framework.refresh', () => refreshAll()),

    // Add workspace root — with confirmation
    vscode.commands.registerCommand('activate-framework.addToWorkspace', async () => {
      const answer = await vscode.window.showWarningMessage(
        'Add Peregrine Activate as a workspace root? Copilot will discover the installed configuration files.',
        { modal: true },
        'Add',
      );
      if (answer !== 'Add') return;

      const added = addWorkspaceRoot(context);
      if (added) {
        vscode.window.showInformationMessage(
          'Peregrine Activate added to workspace.',
        );
      } else {
        vscode.window.showInformationMessage(
          'Peregrine Activate is already in the workspace.',
        );
      }
      refreshAll();
    }),

    // Remove workspace root — with confirmation
    vscode.commands.registerCommand(
      'activate-framework.removeFromWorkspace',
      async () => {
        const answer = await vscode.window.showWarningMessage(
          'Remove Peregrine Activate from workspace? Copilot will no longer see the configuration files.',
          { modal: true },
          'Remove',
        );
        if (answer !== 'Remove') return;

        const removed = removeWorkspaceRoot();
        if (removed) {
          vscode.window.showInformationMessage(
            'Peregrine Activate removed from workspace.',
          );
        }
        refreshAll();
      },
    ),

    // Update only currently-installed files (not all tier files)
    vscode.commands.registerCommand('activate-framework.updateAll', async () => {
      const result = await updateInstalledFiles(context);
      vscode.window.showInformationMessage(
        `Updated ${result.updated.length} files to v${result.version}.`,
      );
      markDirty();
      refreshAll();
    }),

    // Install a single file
    vscode.commands.registerCommand(
      'activate-framework.installFile',
      async (fileOrItem) => {
        const file = fileOrItem?.fileData || fileOrItem;
        if (!file?.src || !file?.dest) {
          vscode.window.showWarningMessage('No file selected.');
          return;
        }
        const ok = await installFile(context, file);
        if (ok) {
          vscode.window.showInformationMessage(`Installed: ${file.dest}`);
        } else {
          vscode.window.showErrorMessage(`Failed to install: ${file.dest}`);
        }
        markDirty();
        refreshAll();
      },
    ),

    // Uninstall a single file
    vscode.commands.registerCommand(
      'activate-framework.uninstallFile',
      async (treeItem) => {
        const file = treeItem?.fileData;
        if (!file?.dest) {
          vscode.window.showWarningMessage('No file selected.');
          return;
        }
        const ok = await uninstallFile(context, file);
        if (ok) {
          vscode.window.showInformationMessage(`Uninstalled: ${file.dest}`);
        } else {
          vscode.window.showErrorMessage(`Failed to uninstall: ${file.dest}`);
        }
        markDirty();
        refreshAll();
      },
    ),
  );

  // Auto-sync files and add workspace root on activation
  autoSetup(context).then(() => refreshAll());
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

function deactivate() {}

module.exports = { activate, deactivate };
