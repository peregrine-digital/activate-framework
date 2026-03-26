const vscode = require('vscode');
const path = require('path');
const { execFileSync } = require('child_process');
const { createHash } = require('crypto');
const fs = require('fs');
const os = require('os');
const https = require('https');
const { ActivateClient, Method } = require('./client');

// Use Svelte-based control panel if webview bundle exists, otherwise fall back
const webviewBundlePath = path.join(__dirname, '..', 'webview-dist', 'webview.js');
const { ControlPanelProvider } = fs.existsSync(webviewBundlePath)
  ? require('./controlPanel.svelte')
  : require('./controlPanel');

/** @type {ActivateClient|null} */
let client = null;

/** @type {vscode.OutputChannel|null} */
let outputChannel = null;

/** Cached install directory from daemon (e.g. ".github"). */
let installDir = '.github';

/** Temp directories created for diff views; cleaned up on deactivate. */
const tempDirs = [];

// ── Polling constants ─────────────────────────────────────────

/** How often to check for the CLI binary after install (ms). */
const POLL_INTERVAL_MS = 2000;
/** Delay after binary detected before verifying (ms). */
const POST_DETECT_DELAY_MS = 2000;
/** Stop polling after this duration (ms). */
const POLL_TIMEOUT_MS = 300000;

// ── Binary resolution ─────────────────────────────────────────

const GITHUB_OWNER = 'peregrine-digital';
const GITHUB_REPO = 'activate-framework';

/**
 * Resolve the `activate` binary: bundled → dev → ~/.activate/bin → PATH → auto-install.
 * @returns {Promise<string|null>}
 */
async function resolveBinPath(context, outputChannel) {
  // 1. Bundled in extension package (production)
  const bundled = path.join(context.extensionUri.fsPath, 'bin', 'activate');
  if (fs.existsSync(bundled)) { outputChannel.appendLine(`[debug] CLI found: bundled (${bundled})`); return bundled; }

  // 2. Sibling cli/ directory (development — running from repo)
  const dev = path.join(context.extensionUri.fsPath, '..', 'cli', 'activate');
  if (fs.existsSync(dev)) { outputChannel.appendLine(`[debug] CLI found: dev (${dev})`); return dev; }

  // 3. ~/.activate/bin/activate (installed by install-cli.sh)
  const home = os.homedir();
  const managed = path.join(home, '.activate', 'bin', 'activate');
  if (fs.existsSync(managed)) { outputChannel.appendLine(`[debug] CLI found: managed (${managed})`); return managed; }

  // 4. On system PATH
  try {
    const which = process.platform === 'win32' ? 'where' : 'which';
    const found = execFileSync(which, ['activate'], { encoding: 'utf8' }).trim().split('\n')[0];
    outputChannel.appendLine(`[debug] CLI found: PATH (${found})`);
    return found;
  } catch {
    // not on PATH
  }

  // Not found — return null; the panel shows "not installed" with an install button
  outputChannel.appendLine('[debug] CLI not found in any location');
  return null;
}

/**
 * Prompt user and run the install script in a VS Code terminal.
 * @returns {Promise<boolean>} true if install was launched (user must wait for terminal to finish)
 */
async function autoInstallCLI() {
  const action = await vscode.window.showInformationMessage(
    'Activate CLI not found. Install it now?',
    'Install',
    'Cancel',
  );
  if (action !== 'Install') return false;

  try {
    // Get GitHub token for private repo access
    let token = '';
    try {
      const session = await vscode.authentication.getSession('github', ['repo'], {
        createIfNone: true,
      });
      token = session?.accessToken || '';
    } catch {
      // No auth available — will work for public repos only
    }

    // Bundle the install script path (shipped with the extension)
    const scriptPath = path.join(__dirname, '..', 'install-cli.sh');

    const terminal = vscode.window.createTerminal({
      name: 'Activate CLI Install',
      env: token ? { GITHUB_TOKEN: token } : undefined,
    });
    terminal.show();
    terminal.sendText(`sh "${scriptPath}"`);
    return true;
  } catch (err) {
    vscode.window.showErrorMessage(`CLI install failed: ${err.message}`);
    return false;
  }
}

/**
 * Verify a binary is executable by running `<binary> version`.
 * Returns null on success, or an Error on failure.
 * Exported for testability.
 */
function verifyBinary(binaryPath) {
  try {
    execFileSync(binaryPath, ['version'], { encoding: 'utf8', timeout: 5000 });
    return null;
  } catch (err) {
    return err;
  }
}

// ── Workspace refresh helpers ─────────────────────────────────

/**
 * Notify VS Code that workspace files changed so its internal watchers
 * (including Copilot's agent/instruction scanner) fire immediately.
 *
 * The daemon writes files via Go's os.WriteFile which bypasses VS Code's
 * file-system layer.  We re-write each affected file through
 * vscode.workspace.fs so the proper onDidCreate / onDidChange / onDidDelete
 * events propagate to all listening extensions.
 *
 * @param {'add'|'remove'|'bulk'} kind
 * @param {string} [fileDest] - relative dest path for single-file ops
 */
async function refreshWorkspace(kind, fileDest) {
  const wsRoot = vscode.workspace.workspaceFolders?.[0]?.uri;
  if (!wsRoot) return;

  // WorkspaceEdit.createFile / deleteFile fires onDidCreateFiles /
  // onDidDeleteFiles — the workspace-level events that Copilot and other
  // extensions listen to.  workspace.fs.writeFile does NOT fire these.
  try {
    if (kind === 'remove' && fileDest) {
      const uri = vscode.Uri.joinPath(wsRoot, installDir, fileDest);
      const edit = new vscode.WorkspaceEdit();
      edit.deleteFile(uri, { ignoreIfNotExists: true });
      await vscode.workspace.applyEdit(edit);
    } else if (kind === 'add' && fileDest) {
      const uri = vscode.Uri.joinPath(wsRoot, installDir, fileDest);
      const data = await vscode.workspace.fs.readFile(uri);
      const edit = new vscode.WorkspaceEdit();
      edit.createFile(uri, { overwrite: true, contents: data });
      await vscode.workspace.applyEdit(edit);
    } else {
      // Bulk — re-create every file tracked in the sidecar.
      const sidecarUri = vscode.Uri.joinPath(wsRoot, installDir, '.activate-installed.json');
      try {
        const raw = await vscode.workspace.fs.readFile(sidecarUri);
        const sidecar = JSON.parse(Buffer.from(raw).toString('utf8'));
        const edit = new vscode.WorkspaceEdit();
        for (const rel of (sidecar.files || [])) {
          const uri = vscode.Uri.joinPath(wsRoot, rel);
          try {
            const data = await vscode.workspace.fs.readFile(uri);
            edit.createFile(uri, { overwrite: true, contents: data });
          } catch { /* file may not exist yet */ }
        }
        await vscode.workspace.applyEdit(edit);
      } catch { /* sidecar may not exist */ }
    }
  } catch {
    // Best-effort — don't break the main flow
  }
}

// ── Activation ────────────────────────────────────────────────

async function activate(context) {
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) return;

  const projectDir = workspaceFolder.uri.fsPath;
  outputChannel = vscode.window.createOutputChannel('Activate Framework');

  // Register the control panel immediately (shows "CLI not found" state if needed)
  const extVersion = context.extension?.packageJSON?.version || '';
  const controlPanel = new ControlPanelProvider(null, extVersion);
  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider(
      ControlPanelProvider.viewType,
      controlPanel,
    ),
  );

  /** Guard: return early with a warning if the daemon isn't running yet. */
  function requireClient() {
    if (!client) {
      vscode.window.showWarningMessage('Activate CLI is not installed yet.');
      return false;
    }
    return true;
  }

  // ── Command registrations ──────────────────────────────────

  context.subscriptions.push(
    vscode.commands.registerCommand('activate-framework.installCLI', () => {
      autoInstallCLI().then((launched) => {
        if (!launched) return;
        // Poll for the binary to appear, then start the daemon
        const managed = path.join(os.homedir(), '.activate', 'bin', 'activate');
        const poll = setInterval(async () => {
          if (fs.existsSync(managed)) {
            clearInterval(poll);
            // Brief delay — let the install script finish fully
            await new Promise((r) => setTimeout(r, POST_DETECT_DELAY_MS));
            // Verify binary is executable
            const verifyErr = verifyBinary(managed);
            if (verifyErr) {
              outputChannel.appendLine(`[error] Binary exists but not runnable: ${verifyErr.message}`);
              vscode.window.showErrorMessage('Activate CLI was installed but cannot run. Check the Output panel for details.');
              return;
            }
            try {
              await startDaemon(context, managed, projectDir, outputChannel, controlPanel);
              vscode.window.showInformationMessage('Activate CLI installed and running!');
            } catch (err) {
              outputChannel.appendLine(`[error] Failed to start daemon after install: ${err.message}`);
              vscode.window.showErrorMessage(`Activate CLI installed but failed to start: ${err.message}`);
            }
          }
        }, POLL_INTERVAL_MS);
        // Stop polling after 5 minutes
        setTimeout(() => clearInterval(poll), POLL_TIMEOUT_MS);
      });
    }),

    vscode.commands.registerCommand('activate-framework.changeTier', async () => {
      if (!requireClient()) return;
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
        const currentItem = items.find((i) => i.value === state.config.tier);

        const picked = await new Promise((resolve) => {
          const qp = vscode.window.createQuickPick();
          qp.items = items;
          qp.placeholder = 'Select tier';
          if (currentItem) qp.activeItems = [currentItem];
          qp.onDidAccept(() => { resolve(qp.activeItems[0]); qp.dispose(); });
          qp.onDidHide(() => { resolve(undefined); qp.dispose(); });
          qp.show();
        });
        if (!picked) return;
        await client.setConfig({ tier: picked.value, scope: 'project' });
        await client.sync();
        refreshWorkspace('bulk');
      } catch (err) {
        vscode.window.showErrorMessage(`Change tier failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.changeManifest', async () => {
      if (!requireClient()) return;
      try {
        const state = await client.getState();
        const currentManifest = state?.config?.manifest || '';
        const manifests = state.manifests || [];
        if (!manifests || manifests.length === 0) {
          vscode.window.showWarningMessage('No manifests found.');
          return;
        }
        const items = manifests.map((m) => ({
          label: m.name || m.id,
          description: m.id === currentManifest ? `${m.id} (current)` : m.id,
          value: m.id,
        }));
        const currentItem = items.find((i) => i.value === currentManifest);

        const picked = await new Promise((resolve) => {
          const qp = vscode.window.createQuickPick();
          qp.items = items;
          qp.placeholder = 'Select manifest';
          if (currentItem) qp.activeItems = [currentItem];
          qp.onDidAccept(() => { resolve(qp.activeItems[0]); qp.dispose(); });
          qp.onDidHide(() => { resolve(undefined); qp.dispose(); });
          qp.show();
        });
        if (!picked) return;
        await client.setConfig({ manifest: picked.value, scope: 'project' });
        await client.sync();
        refreshWorkspace('bulk');
      } catch (err) {
        vscode.window.showErrorMessage(`Change manifest failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.showStatus', async () => {
      if (!requireClient()) return;
      try {
        const state = await client.getState();
        outputChannel.clear();
        outputChannel.appendLine('=== Activate Framework Status ===');
        outputChannel.appendLine(`Project:  ${state.projectDir}`);
        outputChannel.appendLine(`State:    ${state.state.hasInstallMarker ? 'installed' : 'not_installed'}`);
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
      if (!requireClient()) return;
      try {
        await client.repoRemove();
        vscode.window.showInformationMessage('Peregrine Activate files removed from workspace.');
      } catch (err) {
        vscode.window.showErrorMessage(`Remove failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.refresh', () => {
      controlPanel.refresh();
    }),

    vscode.commands.registerCommand('activate-framework.addToWorkspace', async () => {
      if (!requireClient()) return;
      const answer = await vscode.window.showWarningMessage(
        'Inject Peregrine Activate files into this workspace? Files will be hidden from git.',
        { modal: true },
        'Inject',
      );
      if (answer !== 'Inject') return;

      try {
        await client.repoAdd();
        vscode.window.showInformationMessage('Peregrine Activate files injected.');
        refreshWorkspace('bulk');
      } catch (err) {
        vscode.window.showErrorMessage(`Add failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.removeFromWorkspace', async () => {
      if (!requireClient()) return;
      const answer = await vscode.window.showWarningMessage(
        'Remove all Peregrine Activate files from this workspace?',
        { modal: true },
        'Remove',
      );
      if (answer !== 'Remove') return;

      try {
        await client.repoRemove();
        vscode.window.showInformationMessage('Peregrine Activate files removed.');
        refreshWorkspace('bulk');
      } catch (err) {
        vscode.window.showErrorMessage(`Remove failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.updateAll', async () => {
      if (!requireClient()) return;
      try {
        const result = await client.update();
        const count = result.updated ? result.updated.length : 0;
        vscode.window.showInformationMessage(`Updated ${count} files.`);
        refreshWorkspace('bulk');
      } catch (err) {
        vscode.window.showErrorMessage(`Update failed: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.installFile', async (fileOrItem) => {
      if (!requireClient()) return;
      const file = fileOrItem?.fileData || fileOrItem;
      if (!file?.dest) {
        vscode.window.showWarningMessage('No file selected.');
        return;
      }
      try {
        await client.installFile(file.dest);
        vscode.window.showInformationMessage(`Installed: ${file.dest}`);
        refreshWorkspace('add', file.dest);
      } catch (err) {
        vscode.window.showErrorMessage(`Failed to install: ${file.dest} — ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.uninstallFile', async (arg) => {
      if (!requireClient()) return;
      const file = arg?.fileData || arg;
      if (!file?.dest) {
        vscode.window.showWarningMessage('No file selected.');
        return;
      }
      try {
        await client.uninstallFile(file.dest);
        vscode.window.showInformationMessage(`Uninstalled: ${file.dest}`);
        refreshWorkspace('remove', file.dest);
      } catch (err) {
        vscode.window.showErrorMessage(`Failed to uninstall: ${file.dest} — ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.openFile', async (file) => {
      if (!file?.dest) { outputChannel.appendLine(`[openFile] no file.dest — file=${JSON.stringify(file)}`); return; }
      const wsRoot = vscode.workspace.workspaceFolders?.[0]?.uri;
      if (!wsRoot) { outputChannel.appendLine('[openFile] no workspace root'); return; }
      const fileUri = vscode.Uri.joinPath(wsRoot, installDir, file.dest);
      outputChannel.appendLine(`[openFile] opening ${fileUri.fsPath}`);
      try {
        await vscode.commands.executeCommand('vscode.open', fileUri);
      } catch (err) {
        outputChannel.appendLine(`[openFile] error: ${err?.message || err}`);
        vscode.window.showWarningMessage(`Could not open ${file.dest}: ${err?.message || err}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.diffFile', async (file) => {
      if (!requireClient()) return;
      if (!file?.dest) return;
      try {
        const result = await client.diffFile(file.dest);
        if (result.identical) {
          vscode.window.showInformationMessage(`${file.dest} is identical to bundled version.`);
          return;
        }

        const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-diff-'));
        tempDirs.push(tmpDir);
        const name = file.dest.split('/').pop();
        const diffPath = path.join(tmpDir, `${name}.diff`);
        fs.writeFileSync(diffPath, result.diff, 'utf8');

        // Show the installed vs bundled via workspace file URIs
        const wsRoot = vscode.workspace.workspaceFolders?.[0]?.uri;
        if (!wsRoot) return;
        const installedUri = vscode.Uri.joinPath(wsRoot, installDir, file.dest);
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
      if (!requireClient()) return;
      if (!file?.dest) return;
      try {
        await client.skipFileUpdate(file.dest);
        vscode.window.showInformationMessage(`Skipped update for ${file.dest}`);
      } catch (err) {
        vscode.window.showWarningMessage(`Could not skip update for ${file.dest}: ${err.message}`);
      }
    }),

    vscode.commands.registerCommand('activate-framework.telemetryRunNow', async () => {
      if (!requireClient()) return;
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

    vscode.commands.registerCommand('activate-framework.checkForUpdates', async () => {
      if (!requireClient()) return;
      await checkForUpdates(context, controlPanel, true);
    }),

    vscode.commands.registerCommand('activate-framework.quickStart', async () => {
      if (!requireClient()) return;
      try {
        await context.workspaceState.update('quickStartDismissed', false);
        await showQuickStartPrompt(context, { skipGuards: true });
        controlPanel.refresh();
        refreshWorkspace('bulk');
      } catch (err) {
        vscode.window.showErrorMessage(`Quick Start failed: ${err.message}`);
      }
    }),
  );

  // ── Resolve CLI and start daemon ──────────────────────────

  const binPath = await resolveBinPath(context, outputChannel);
  if (!binPath) return; // Panel shows "not installed" state with install button

  try {
    await startDaemon(context, binPath, projectDir, outputChannel, controlPanel);
  } catch (err) {
    outputChannel.appendLine(`[error] Failed to start daemon: ${err.message}`);
    vscode.window.showErrorMessage(`Activate CLI failed to start: ${err.message}`);
  }
}

async function startDaemon(context, binPath, projectDir, outputChannel, controlPanel) {
  // Get GitHub token for private repo access in daemon.
  // createIfNone: true prompts the user to sign in if no session exists —
  // required for private repos where unauthenticated fetches always 404.
  let token = '';
  try {
    const session = await vscode.authentication.getSession('github', ['repo'], {
      createIfNone: true,
    });
    token = session?.accessToken || '';
  } catch {
    // User declined sign-in or auth unavailable — daemon will fall back to gh CLI
  }
  if (!token) {
    outputChannel.appendLine('[warn] No GitHub token available — private repo access will fail');
  }

  client = new ActivateClient({
    binPath,
    projectDir,
    token,
    log: {
      debug: (msg) => outputChannel.appendLine(`[debug] ${msg}`),
      error: (msg) => outputChannel.appendLine(`[error] ${msg}`),
    },
  });

  await client.start();

  controlPanel.setClient(client);
  controlPanel.refresh();

  // Refresh UI when daemon notifies of state changes
  client.on('notification', (method) => {
    if (method === Method.NotifyStateChanged) {
      controlPanel.refresh();
    }
  });

  // Update daemon token when the user signs in/out of GitHub
  context.subscriptions.push(
    vscode.authentication.onDidChangeSessions(async (e) => {
      if (e.provider.id !== 'github') return;
      try {
        const session = await vscode.authentication.getSession('github', ['repo'], {
          createIfNone: false,
        });
        const newToken = session?.accessToken || '';
        if (newToken !== client.token) {
          outputChannel.appendLine('[info] GitHub auth changed — restarting daemon with new token');
          client.token = newToken;
          client._disposed = false;  // allow restart after stop()
          await client.stop();
          client._disposed = false;  // stop() sets this true
          await client.start();
          controlPanel.refresh();
        }
      } catch { /* ignore */ }
    }),
  );

  // Auto-restart daemon on unexpected exit (skip during intentional update)
  client.on('exit', () => {
    if (!client._disposed && !client._updating) {
      client.start().catch((err) => {
        outputChannel.appendLine(`[error] Daemon restart failed: ${err.message}`);
      });
    }
  });

  // Auto-setup: sync on activation
  await autoSetup(controlPanel, context);
}

// ── Auto-setup ────────────────────────────────────────────────

async function autoSetup(controlPanel, context) {
  try {
    const state = await client.getState();

    if (!state.state.hasInstallMarker) {
      await showQuickStartPrompt(context);
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
  refreshWorkspace('bulk');

  // Check for updates (non-blocking)
  checkForUpdates(context, controlPanel);
}

/**
 * Show a quick-start prompt on first activation in a new workspace.
 * Offers "Quick Start" which installs ironarch/workflow. Cancelling
 * sets a per-workspace dismiss flag.
 *
 * Guards (skipped when `skipGuards` is true, e.g. manual re-trigger):
 *  1. If global config already has a manifest → auto-install silently.
 *  2. If user previously dismissed in this workspace → do nothing.
 */
async function showQuickStartPrompt(context, { skipGuards = false } = {}) {
  if (!skipGuards) {
    // 1. Global preference already set → auto-install silently
    const globalCfg = await client.getConfig('global');
    if (globalCfg?.manifest) {
      await client.repoAdd();
      return;
    }

    // 2. Previously dismissed in this workspace → do nothing
    if (context.workspaceState.get('quickStartDismissed')) {
      return;
    }
  }

  // 3. Show modal prompt
  const picked = await vscode.window.showInformationMessage(
    'Set up Activate with a pre-configured workflow?\n\n'
    + 'Quick Start installs 6 specialized agents for planning, coding, testing, review, documentation, and PR creation.\n\n'
    + 'If you skip this, no files will be installed. You can configure your setup manually from the Settings panel, or run Quick Start later from the Command Palette.',
    { modal: true },
    'Quick Start',
  );

  if (picked !== 'Quick Start') {
    // User dismissed or declined — remember so we don't ask again
    await context.workspaceState.update('quickStartDismissed', true);
    return;
  }

  // Install ironarch/workflow
  await client.setConfig({ manifest: 'ironarch', tier: 'workflow', scope: 'project' });
  await client.repoAdd();
}

/**
 * Build HTTP headers for a VSIX download request.
 * Auth + Accept are sent on the initial API request only, not on S3 redirects.
 * Exported for testability.
 */
function buildDownloadHeaders(token, isRedirect = false) {
  const headers = { 'User-Agent': 'activate-extension' };
  if (!isRedirect) {
    headers['Accept'] = 'application/octet-stream';
    if (token) headers['Authorization'] = `Bearer ${token}`;
  }
  return headers;
}

/**
 * Perform a CLI binary self-update: set _updating flag, call selfUpdate RPC,
 * restart the daemon, and clear the flag. The flag prevents the auto-restart
 * exit handler from racing with the intentional restart.
 * Exported for testability.
 */
async function performCliUpdate(targetClient, token) {
  const log = (msg) => outputChannel?.appendLine(`[update] ${msg}`);
  targetClient._updating = true;
  try {
    try {
      await targetClient.selfUpdate(token);
      log('selfUpdate RPC completed normally');
    } catch (err) {
      // Expected: daemon may die when its binary is replaced (SIGTERM from macOS code signing).
      // The binary was likely updated successfully — proceed with restart.
      log(`selfUpdate RPC interrupted (expected): ${err.message}`);
    }
    // Ensure old daemon is stopped (may already be dead)
    try { await targetClient.stop(); } catch { /* already dead */ }
    // Brief pause to let filesystem settle
    await new Promise((r) => setTimeout(r, 500));
    await targetClient.start();
    log(`Daemon restarted, new version: ${targetClient.serverVersion || '?'}`);
  } finally {
    targetClient._updating = false;
  }
}

async function checkForUpdates(context, controlPanel, force = false) {
  const log = (msg) => outputChannel?.appendLine(`[update] ${msg}`);
  try {
    if (force && outputChannel) {
      outputChannel.show(true);
    }
    log('Checking for updates...');

    // Get GitHub token for private repo API access
    let token = '';
    try {
      const session = await vscode.authentication.getSession('github', ['repo'], {
        createIfNone: false,
      });
      token = session?.accessToken || '';
      log(token ? 'GitHub auth token acquired' : 'No GitHub auth token');
    } catch {
      log('GitHub auth unavailable');
    }

    const extVersion = context.extension?.packageJSON?.version || '';
    log(`Current CLI: ${client?.serverVersion || '?'}, Extension: ${extVersion}`);
    const update = await client.checkUpdate(extVersion, force, token);

    // Store the check timestamp for display in Settings
    if (update?.checkedAt && controlPanel) {
      controlPanel.setLastUpdateCheck(update.checkedAt);
    }

    if (!update) {
      log('No update info returned from daemon');
      if (force) vscode.window.showInformationMessage('Activate is up to date.');
      return;
    }

    log(`Latest CLI: ${update.latestVersion || '(none)'}, updateAvailable: ${update.updateAvailable}`);
    if (update.extension) {
      log(`Extension update: available=${update.extension.available}, version=${update.extension.version || '?'}, downloadUrl=${update.extension.downloadUrl ? 'yes' : 'no'}`);
    }

    const hasCli = !!update.updateAvailable;
    const ext = update.extension;
    const hasExt = !!(ext && ext.available && ext.downloadUrl);

    if (!hasCli && !hasExt) {
      log('No updates available');
      if (force) vscode.window.showInformationMessage('Activate is up to date.');
      // Refresh the panel to show updated timestamp
      if (controlPanel) controlPanel.refresh();
      return;
    }

    // Build a combined update message
    const parts = [];
    if (hasCli) parts.push(`CLI v${update.currentVersion} → v${update.latestVersion}`);
    if (hasExt) parts.push(`Extension v${extVersion} → v${ext.version}`);
    const msg = `Activate update available: ${parts.join(', ')}`;

    const action = await vscode.window.showInformationMessage(msg, 'Update Now', 'Dismiss');
    if (action === 'Update Now') {
      try {
        await vscode.window.withProgress(
          { location: vscode.ProgressLocation.Notification, title: 'Updating Activate…' },
          async () => {
            if (hasCli) {
              log('Updating CLI…');
              await performCliUpdate(client, token);
              log('CLI update completed');
            }
            if (hasExt) {
              log(`Downloading VSIX: ${ext.assetName} from ${ext.downloadUrl}`);
              await downloadAndInstallVsix(ext.downloadUrl, ext.assetName, ext.sha256, token);
              log('VSIX install completed');
            }
          },
        );
        if (hasCli && !hasExt) {
          vscode.window.showInformationMessage(
            `Activate CLI updated to v${update.latestVersion}. Daemon restarted.`,
          );
        }
        if (hasExt) {
          const reload = await vscode.window.showInformationMessage(
            'Activate extension updated. Reload to apply changes.',
            'Reload',
          );
          if (reload === 'Reload') {
            await vscode.commands.executeCommand('workbench.action.reloadWindow');
          }
        }
      } catch (err) {
        log(`Update FAILED: ${err.message}\n${err.stack || ''}`);
        vscode.window.showErrorMessage(`Update failed: ${err.message}`);
      }
    }

    // Refresh the panel to show updated timestamp
    if (controlPanel) controlPanel.refresh();
  } catch (err) {
    log(`Update check FAILED: ${err.message}\n${err.stack || ''}`);
    if (force) {
      vscode.window.showErrorMessage(`Update check failed: ${err.message}`);
    }
  }
}

/**
 * Verify SHA-256 checksum of a downloaded file.
 * Throws on mismatch, no-ops if expectedSha256 is falsy.
 * Exported for testability.
 */
function verifyChecksum(filePath, expectedSha256) {
  if (!expectedSha256) return;
  const data = fs.readFileSync(filePath);
  const actual = createHash('sha256').update(data).digest('hex');
  if (actual !== expectedSha256) {
    throw new Error(
      `Checksum mismatch: expected ${expectedSha256}, got ${actual}. ` +
      'Download may be corrupted or tampered with.',
    );
  }
}

/**
 * Download a file, following redirects with correct auth headers.
 * Uses http or https based on URL protocol (enables testing with http.createServer).
 * Exported for testability.
 */
function downloadFile(url, destPath, token) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(destPath);
    const get = (reqUrl, isRedirect = false) => {
      const headers = buildDownloadHeaders(token, isRedirect);
      const mod = reqUrl.startsWith('https') ? https : require('http');
      mod.get(reqUrl, { headers }, (resp) => {
        if (resp.statusCode >= 300 && resp.statusCode < 400 && resp.headers.location) {
          get(resp.headers.location, true);
          return;
        }
        if (resp.statusCode !== 200) {
          file.close();
          reject(new Error(`Download failed: ${resp.statusCode}`));
          return;
        }
        resp.pipe(file);
        file.on('finish', () => {
          file.close();
          resolve();
        });
      }).on('error', (err) => {
        file.close();
        reject(err);
      });
    };
    get(url);
  });
}

async function downloadAndInstallVsix(url, filename, expectedSha256, token) {
  const log = (msg) => outputChannel?.appendLine(`[update] ${msg}`);
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'activate-vsix-'));
  tempDirs.push(tmpDir);
  const dest = path.join(tmpDir, filename);

  log(`Downloading VSIX to ${dest}`);
  await downloadFile(url, dest, token);
  const stat = fs.statSync(dest);
  log(`Downloaded ${stat.size} bytes`);

  verifyChecksum(dest, expectedSha256);
  log('Checksum verified (or skipped)');

  log('Installing VSIX via workbench.extensions.installExtension...');
  await vscode.commands.executeCommand(
    'workbench.extensions.installExtension',
    vscode.Uri.file(dest),
  );
  log('VSIX install command completed');
}

// ── Deactivation ──────────────────────────────────────────────

async function deactivate() {
  if (client) {
    await client.stop();
    client = null;
  }
  // Clean up temp directories created for diff views
  for (const dir of tempDirs) {
    try {
      fs.rmSync(dir, { recursive: true, force: true });
    } catch {
      // best-effort cleanup
    }
  }
  tempDirs.length = 0;
}

module.exports = {
  activate, deactivate, buildDownloadHeaders, performCliUpdate,
  verifyBinary, resolveBinPath, verifyChecksum, downloadFile,
  showQuickStartPrompt,
  POLL_INTERVAL_MS, POST_DETECT_DELAY_MS, POLL_TIMEOUT_MS,
};
