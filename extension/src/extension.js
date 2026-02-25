const vscode = require('vscode');
const {
  readInstalledVersion,
  discoverBundledManifests,
  findActivateWorkspaceFolder,
  removeWorkspaceRoot,
} = require('./installer');
const {
  injectFiles,
  readInjectedVersion,
  injectSingleFile,
  removeSingleFile,
  removeAllInjected,
  updateInjectedFiles,
  skipInjectedFileUpdate,
  getWorkspaceRoot,
} = require('./injector');
const { changeTierCommand } = require('./commands/changeTier');
const { changeManifestCommand } = require('./commands/changeManifest');
const { showStatusCommand } = require('./commands/showStatus');
const { ControlPanelProvider } = require('./controlPanel');
const { initTelemetry } = require('./telemetry');
const {
  resolveConfig,
  writeProjectConfig,
  setSkippedVersion,
  clearSkippedVersion,
  ensureGitExclude,
} = require('./config');

function activate(context) {
  const controlPanel = new ControlPanelProvider(context);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      ControlPanelProvider.viewType,
      controlPanel,
    ),
  );

  function refreshAll() {
    controlPanel.refresh();
  }

  // Initialise Copilot telemetry logging (daily quota tracker)
  initTelemetry(context);

  context.subscriptions.push(
    vscode.commands.registerCommand('activate-framework.changeTier', async () => {
      await changeTierCommand(context);
      refreshAll();
    }),
    vscode.commands.registerCommand('activate-framework.changeManifest', async () => {
      const changed = await changeManifestCommand(context);
      if (changed) {
        refreshAll();
      }
    }),
    vscode.commands.registerCommand('activate-framework.showStatus', () =>
      showStatusCommand(context),
    ),
    vscode.commands.registerCommand('activate-framework.remove', async () => {
      const removed = await removeAllInjected();
      if (removed) {
        vscode.window.showInformationMessage('Peregrine Activate files removed from workspace.');
      }
      refreshAll();
    }),
    vscode.commands.registerCommand('activate-framework.refresh', () => refreshAll()),

    // Add/inject — with confirmation
    vscode.commands.registerCommand('activate-framework.addToWorkspace', async () => {
      const answer = await vscode.window.showWarningMessage(
        'Inject Peregrine Activate files into this workspace? Files will be hidden from git.',
        { modal: true },
        'Inject',
      );
      if (answer !== 'Inject') return;

      const cfg = await resolveConfig();
      const result = await injectFiles(context, cfg.tier, cfg.manifest);
      await ensureGitExclude();
      vscode.window.showInformationMessage(
        `Peregrine Activate injected (${result.injected.length} files).`,
      );
      refreshAll();
    }),

    // Remove — with confirmation
    vscode.commands.registerCommand(
      'activate-framework.removeFromWorkspace',
      async () => {
        const answer = await vscode.window.showWarningMessage(
          'Remove all Peregrine Activate files from this workspace?',
          { modal: true },
          'Remove',
        );
        if (answer !== 'Remove') return;

        const removed = await removeAllInjected();
        if (removed) {
          vscode.window.showInformationMessage('Peregrine Activate files removed.');
        }
        refreshAll();
      },
    ),

    // Update only currently-installed files (not all tier files)
    vscode.commands.registerCommand('activate-framework.updateAll', async () => {
      const result = await updateInjectedFiles(context);
      vscode.window.showInformationMessage(
        `Updated ${result.updated.length} files to v${result.version}.`,
      );
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
        const ok = await injectSingleFile(context, file);
        if (ok) {
          // Clear any skipped version since user explicitly installed
          await clearSkippedVersion(file.dest);
          vscode.window.showInformationMessage(`Installed: ${file.dest}`);
        } else {
          vscode.window.showErrorMessage(`Failed to install: ${file.dest}`);
        }
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
        const ok = await removeSingleFile(file);
        if (ok) {
          vscode.window.showInformationMessage(`Uninstalled: ${file.dest}`);
        } else {
          vscode.window.showErrorMessage(`Failed to uninstall: ${file.dest}`);
        }
        refreshAll();
      },
    ),

    // Open an installed file in the editor
    vscode.commands.registerCommand(
      'activate-framework.openFile',
      async (file) => {
        if (!file?.dest) return;
        const wsRoot = getWorkspaceRoot();
        const fileUri = wsRoot ? vscode.Uri.joinPath(wsRoot, '.github', file.dest) : null;
        if (!fileUri) return;
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
        const wsRoot = getWorkspaceRoot();
        const installedUri = wsRoot ? vscode.Uri.joinPath(wsRoot, '.github', file.dest) : null;
        if (!installedUri) return;
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

    // Skip update — record skipped version in config
    vscode.commands.registerCommand(
      'activate-framework.skipFileUpdate',
      async (file) => {
        if (!file?.src || !file?.dest) return;
        // Still stamp the local file so injector knows this version was seen
        const ok = await skipInjectedFileUpdate(context, file);
        if (ok) {
          // Also record in persistent config so it survives across sessions
          const { readBundledFileVersion } = require('./installer');
          const bundledVer = await readBundledFileVersion(context, file);
          if (bundledVer) {
            await setSkippedVersion(file.dest, bundledVer);
          }
          vscode.window.showInformationMessage(`Skipped update for ${file.dest}`);
        } else {
          vscode.window.showWarningMessage(`Could not skip update for ${file.dest}`);
        }
        refreshAll();
      },
    ),
  );

  // Auto-sync files on activation
  autoSetup(context).then(() => refreshAll());
}

async function autoSetup(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) return;

  const cfg = await resolveConfig();
  const bundledVersion = context.extension.packageJSON.version ?? 'unknown';

  // If there's a legacy workspace root from old mode, clean it up
  if (findActivateWorkspaceFolder()) {
    removeWorkspaceRoot();
  }

  const injectedInfo = await readInjectedVersion();
  const injectedVersion = injectedInfo?.version || null;

  // Inject on first run or version mismatch
  if (injectedVersion !== bundledVersion) {
    const manifestId = cfg.manifest || injectedInfo?.manifest || undefined;
    const result = await injectFiles(context, cfg.tier, manifestId);

    // Persist the active manifest/tier if not already saved
    await writeProjectConfig({
      manifest: manifestId || 'activate-framework',
      tier: cfg.tier,
    });

    if (injectedVersion) {
      vscode.window.showInformationMessage(
        `Peregrine Activate updated: ${injectedVersion} → ${bundledVersion}`,
      );
    } else {
      vscode.window.showInformationMessage(
        `Peregrine Activate ${bundledVersion} (${cfg.tier}) is ready.`,
      );
    }
  }

  // Ensure the project config file is git-excluded
  await ensureGitExclude();
}

function deactivate() {}

module.exports = { activate, deactivate };
