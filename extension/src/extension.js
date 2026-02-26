const vscode = require('vscode');
const path = require('path');
const { execFileSync } = require('child_process');
const fs = require('fs');
const os = require('os');
const { ActivateClient } = require('./client');
const { ControlPanelProvider } = require('./controlPanel');

/** @type {ActivateClient|null} */
let client = null;

// ── Binary resolution ─────────────────────────────────────────

/**
 * Resolve the `activate` binary: bundled first, then PATH.
 * @returns {string|null}
 */
function resolveBinPath(context) {
  // 1. Bundled in extension package (production)
  const bundled = path.join(context.extensionUri.fsPath, 'bin', 'activate');
  if (fs.existsSync(bundled)) return bundled;

  // 2. Sibling cli/ directory (development — running from repo)
  const dev = path.join(context.extensionUri.fsPath, '..', 'cli', 'activate');
  if (fs.existsSync(dev)) return dev;

  // 3. On system PATH
  try {
    return execFileSync('which', ['activate'], { encoding: 'utf8' }).trim();
  } catch {
    // not on PATH
  }

  vscode.window.showErrorMessage(
    'Activate CLI binary not found. Run "make build" in cli/ or install the CLI.',
  );
  return null;
}

// ── Activation ────────────────────────────────────────────────

async function activate(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) return;

  const projectDir = workspaceFolder.uri.fsPath;
  const binPath = resolveBinPath(context);
  if (!binPath) return;

  const outputChannel = vscode.window.createOutputChannel('Activate Framework');

  client = new ActivateClient({
    binPath,
    projectDir,
    log: {
      debug: (msg) => outputChannel.appendLine(`[debug] ${msg}`),
      error: (msg) => outputChannel.appendLine(`[error] ${msg}`),
    },
  });

  await client.start();

  const controlPanel = new ControlPanelProvider(client);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      ControlPanelProvider.viewType,
      controlPanel,
    ),
  );

  // Refresh UI when daemon notifies of state changes
  client.on('notification', (method) => {
    if (method === 'activate/stateChanged') {
      controlPanel.refresh();
    }
  });

  // Auto-restart daemon on unexpected exit
  client.on('exit', () => {
    if (!client._disposed) {
      client.start().catch((err) => {
        outputChannel.appendLine(`[error] Daemon restart failed: ${err.message}`);
      });
    }
  });

  // ── Command registrations ──────────────────────────────────

  context.subscriptions.push(
    vscode.commands.registerCommand('activate-framework.changeTier', async () => {
      try {
        const state = await client.getState();
        const tiers = state.tiers || [];
        if (tiers.length === 0) {
          vscode.window.showWarningMessage('No tiers available for this manifest.');
          return;
        }
        const items = tiers.map((t) => ({
          label: t.label || t.id,
          description: t.id === state.config.tier ? '(current)' : '',
          value: t.id,
        }));
        const picked = await vscode.window.showQuickPick(items, {
          placeHolder: 'Select tier',
        });
        if (!picked) return;
        await client.setConfig({ tier: picked.value, scope: 'project' });
        await client.sync();
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Change tier failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.changeManifest', async () => {
      try {
        const manifests = await client.listManifests();
        if (!manifests || manifests.length === 0) {
          vscode.window.showWarningMessage('No manifests found.');
          return;
        }
        const items = manifests.map((m) => ({
          label: m.name || m.id,
          description: m.id,
          value: m.id,
        }));
        const picked = await vscode.window.showQuickPick(items, {
          placeHolder: 'Select manifest',
        });
        if (!picked) return;
        await client.setConfig({ manifest: picked.value, scope: 'project' });
        await client.sync();
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Change manifest failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.showStatus', async () => {
      try {
        const state = await client.getState();
        outputChannel.clear();
        outputChannel.appendLine('=== Activate Framework Status ===');
        outputChannel.appendLine(`Project:  ${state.projectDir}`);
        outputChannel.appendLine(`State:    ${state.state}`);
        outputChannel.appendLine(`Manifest: ${state.config.manifest}`);
        outputChannel.appendLine(`Tier:     ${state.config.tier}`);
        if (state.files) {
          outputChannel.appendLine(`Files:    ${state.files.length}`);
        }
        outputChannel.show(true);
      } catch (err) {
        vscode.window.showErrorMessage(`Show status failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.remove', async () => {
      try {
        await client.repoRemove();
        vscode.window.showInformationMessage('Peregrine Activate files removed from workspace.');
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Remove failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.refresh', () => {
      controlPanel.refresh();
    }),

    vscode.commands.registerCommand('activate-framework.addToWorkspace', async () => {
      const answer = await vscode.window.showWarningMessage(
        'Inject Peregrine Activate files into this workspace? Files will be hidden from git.',
        { modal: true },
        'Inject',
      );
      if (answer !== 'Inject') return;

      try {
        await client.repoAdd();
        vscode.window.showInformationMessage('Peregrine Activate files injected.');
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Add failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.removeFromWorkspace', async () => {
      const answer = await vscode.window.showWarningMessage(
        'Remove all Peregrine Activate files from this workspace?',
        { modal: true },
        'Remove',
      );
      if (answer !== 'Remove') return;

      try {
        await client.repoRemove();
        vscode.window.showInformationMessage('Peregrine Activate files removed.');
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Remove failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.updateAll', async () => {
      try {
        const result = await client.update();
        const count = result.updated ? result.updated.length : 0;
        vscode.window.showInformationMessage(`Updated ${count} files.`);
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Update failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.installFile', async (fileOrItem) => {
      const file = fileOrItem?.fileData || fileOrItem;
      if (!file?.dest) {
        vscode.window.showWarningMessage('No file selected.');
        return;
      }
      try {
        await client.installFile(file.dest);
        vscode.window.showInformationMessage(`Installed: ${file.dest}`);
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Failed to install: ${file.dest} — ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.uninstallFile', async (arg) => {
      const file = arg?.fileData || arg;
      if (!file?.dest) {
        vscode.window.showWarningMessage('No file selected.');
        return;
      }
      try {
        await client.uninstallFile(file.dest);
        vscode.window.showInformationMessage(`Uninstalled: ${file.dest}`);
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showErrorMessage(`Failed to uninstall: ${file.dest} — ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.openFile', async (file) => {
      if (!file?.dest) return;
      const wsRoot = vscode.workspace.workspaceFolders?.[0]?.uri;
      if (!wsRoot) return;
      const fileUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
      try {
        await vscode.commands.executeCommand('vscode.open', fileUri);
      } catch {
        vscode.window.showWarningMessage(`Could not open ${file.dest}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.diffFile', async (file) => {
      if (!file?.dest) return;
      try {
        const result = await client.diffFile(file.dest);
        if (result.identical) {
          vscode.window.showInformationMessage(`${file.dest} is identical to bundled version.`);
          return;
        }

        const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-diff-'));
        const name = file.dest.split('/').pop();
        const diffPath = path.join(tmpDir, `${name}.diff`);
        fs.writeFileSync(diffPath, result.diff, 'utf8');

        // Show the installed vs bundled via workspace file URIs
        const wsRoot = vscode.workspace.workspaceFolders?.[0]?.uri;
        if (!wsRoot) return;
        const installedUri = vscode.Uri.joinPath(wsRoot, '.github', file.dest);
        const diffUri = vscode.Uri.file(diffPath);
        await vscode.commands.executeCommand(
          'vscode.diff',
          diffUri,
          installedUri,
          `${name} (bundled ↔ installed)`,
        );
      } catch (err) {
        vscode.window.showWarningMessage(`Could not diff ${file.dest}: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.skipFileUpdate', async (file) => {
      if (!file?.dest) return;
      try {
        await client.skipFileUpdate(file.dest);
        vscode.window.showInformationMessage(`Skipped update for ${file.dest}`);
        controlPanel.refresh();
      } catch (err) {
        vscode.window.showWarningMessage(`Could not skip update for ${file.dest}: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.telemetryRunNow', async () => {
      try {
        const session = await vscode.authentication.getSession('github', ['user:email'], {
          createIfNone: false,
        });
        const token = session?.accessToken || '';
        await client.runTelemetry(token);
        vscode.window.showInformationMessage('Telemetry run completed.');
      } catch (err) {
        vscode.window.showErrorMessage(`Telemetry run failed: ${err.message}`);
      }
    }),
  );

  // Auto-setup: sync on activation
  await autoSetup(controlPanel);
}

// ── Auto-setup ────────────────────────────────────────────────

async function autoSetup(controlPanel) {
  try {
    const state = await client.getState();

    // If not yet installed, add files automatically
    if (state.state === 'none' || state.state === 'not_installed') {
      await client.repoAdd();
    } else {
      // Sync to pick up version changes
      const result = await client.sync();
      if (result.action === 'updated') {
        vscode.window.showInformationMessage(
          `Peregrine Activate updated: ${result.previousVersion} → ${result.availableVersion}`,
        );
      }
    }
  } catch (err) {
    vscode.window.showWarningMessage(`Activate auto-setup: ${err.message}`);
  }

  controlPanel.refresh();
}

// ── Deactivation ──────────────────────────────────────────────

async function deactivate() {
  if (client) {
    await client.stop();
    client = null;
  }
}

module.exports = { activate, deactivate };
