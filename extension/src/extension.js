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
  getActivateRoot,
  skipFileUpdate,
} = require('./installer');
const { changeTierCommand } = require('./commands/changeTier');
const { showStatusCommand } = require('./commands/showStatus');
const { ControlPanelProvider } = require('./controlPanel');

function activate(context) {
  const controlPanel = new ControlPanelProvider(context);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      ControlPanelProvider.viewType,
      controlPanel,
    ),
  );

  function markDirty() {
    controlPanel.markDirty();
  }

  function refreshAll() {
    controlPanel.refresh();
  }

  context.subscriptions.push(
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
      async (arg) => {
        const file = arg?.fileData || arg;
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

    // Open an installed file in the editor
    vscode.commands.registerCommand(
      'activate-framework.openFile',
      async (file) => {
        if (!file?.dest) return;
        const root = getActivateRoot(context);
        const fileUri = vscode.Uri.joinPath(root, '.github', file.dest);
        try {
          await vscode.commands.executeCommand('vscode.open', fileUri);
        } catch {
          vscode.window.showWarningMessage(`Could not open ${file.dest}`);
        }
      },
    ),

    // Diff installed vs bundled version
    vscode.commands.registerCommand(
      'activate-framework.diffFile',
      async (file) => {
        if (!file?.src || !file?.dest) return;
        const root = getActivateRoot(context);
        const installedUri = vscode.Uri.joinPath(root, '.github', file.dest);
        const bundledUri = vscode.Uri.joinPath(context.extensionUri, 'assets', file.src);
        try {
          const name = file.dest.split('/').pop();
          await vscode.commands.executeCommand(
            'vscode.diff',
            bundledUri,
            installedUri,
            `${name} (bundled ↔ installed)`,
          );
        } catch {
          vscode.window.showWarningMessage(`Could not diff ${file.dest}`);
        }
      },
    ),

    // Skip update — stamp local frontmatter with bundled version
    vscode.commands.registerCommand(
      'activate-framework.skipFileUpdate',
      async (file) => {
        if (!file?.src || !file?.dest) return;
        const ok = await skipFileUpdate(context, file);
        if (ok) {
          vscode.window.showInformationMessage(`Skipped update for ${file.dest}`);
        } else {
          vscode.window.showWarningMessage(`Could not skip update for ${file.dest}`);
        }
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

  const installedInfo = await readInstalledVersion(context);
  const installedVersion = installedInfo?.version || null;
  const bundledVersion = context.extension.packageJSON.version ?? 'unknown';

  // Sync files if first run or version mismatch
  if (installedVersion !== bundledVersion) {
    const manifestId = installedInfo?.manifest || undefined;
    await syncFiles(context, tier, manifestId);

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
