const vscode = require('vscode');
const path = require('path');
const fs = require('fs');

/**
 * WebviewViewProvider that loads the shared Svelte UI bundle.
 *
 * Replaces the old template-string-based ControlPanelProvider.
 * The Svelte app communicates via postMessage; this provider
 * bridges those messages to the ActivateClient (JSON-RPC daemon).
 */
class ControlPanelProvider {
  static viewType = 'activate-framework.controlPanel';

  /** @param {import('./client').ActivateClient|null} client */
  constructor(client, extensionVersion) {
    this._client = client;
    this._view = null;
    this._extensionVersion = extensionVersion || '';
    /** @type {string|null} ISO timestamp of last update check */
    this._lastUpdateCheck = null;
    /** @type {ReturnType<typeof setTimeout>|null} */
    this._refreshTimer = null;
  }

  /** Update the client after CLI is installed and daemon started. */
  setClient(client) {
    this._client = client;
    this._sendInit();
  }

  /** Store the timestamp of the last update check (ISO string). */
  setLastUpdateCheck(isoTimestamp) {
    this._lastUpdateCheck = isoTimestamp || null;
  }

  /** @param {vscode.WebviewView} webviewView */
  resolveWebviewView(webviewView) {
    this._view = webviewView;

    const extensionRoot = path.resolve(__dirname, '..');
    const webviewDistPath = path.join(extensionRoot, 'webview-dist');

    webviewView.webview.options = {
      enableScripts: true,
      localResourceRoots: [vscode.Uri.file(webviewDistPath)],
    };

    webviewView.webview.onDidReceiveMessage((msg) => this._onMessage(msg));

    // Build the HTML that loads the Svelte bundle.
    // Append a cache-bust query so VS Code doesn't serve a stale bundle.
    const cacheBust = `?v=${Date.now()}`;
    const jsUri = webviewView.webview.asWebviewUri(
      vscode.Uri.file(path.join(webviewDistPath, 'webview.js')),
    );
    const cssUri = webviewView.webview.asWebviewUri(
      vscode.Uri.file(path.join(webviewDistPath, 'webview.css')),
    );

    webviewView.webview.html = this._getHtml(`${jsUri}${cacheBust}`, `${cssUri}${cacheBust}`);

    // Send initial state once webview is ready
    setTimeout(() => this._sendInit(), 100);
  }

  async refresh() {
    if (this._refreshTimer) clearTimeout(this._refreshTimer);
    return new Promise((resolve) => {
      this._refreshTimer = setTimeout(async () => {
        this._refreshTimer = null;
        if (this._view) {
          this._view.webview.postMessage({ type: 'stateChanged' });
        }
        resolve();
      }, 100);
    });
  }

  _sendInit() {
    if (!this._view) return;
    this._view.webview.postMessage({
      type: 'init',
      hasCli: !!this._client,
      extensionVersion: this._extensionVersion,
      serverVersion: this._client?.serverVersion || '',
    });
  }

  // ── message bridge ──────────────────────────────────────

  async _onMessage(msg) {
    console.log('[ControlPanel] ← webview:', msg.command, msg._reqId ? `(req ${msg._reqId})` : '(fire)');
    const reqId = msg._reqId;

    // Helper to send response back for request/response pattern
    const respond = (result) => {
      if (reqId && this._view) {
        this._view.webview.postMessage({ _responseId: reqId, _result: result });
      }
    };
    const respondError = (err) => {
      if (reqId && this._view) {
        this._view.webview.postMessage({ _responseId: reqId, _error: err?.message || String(err) });
      }
    };

    try {
      switch (msg.command) {
        // ── Request/response (return data to webview) ──

        case 'getState': {
          const state = await this._client.getState();
          respond(state);
          break;
        }
        case 'getConfig': {
          const config = await this._client.getConfig(msg.scope);
          respond(config);
          break;
        }
        case 'setConfig': {
          await this._client.setConfig(msg.updates);
          respond(null);
          break;
        }
        case 'refreshConfig': {
          respond(null);
          break;
        }
        case 'setOverride': {
          const data = msg.file || {};
          await this._client.setFileOverride(data.file, data.override);
          respond(null);
          break;
        }
        case 'listManifests': {
          const state = await this._client.getState();
          respond(state?.manifests || []);
          break;
        }
        case 'listBranches': {
          const branches = await this._client.listBranches();
          respond(branches || []);
          break;
        }
        case 'readTelemetryLog': {
          const log = await this._client.readTelemetryLog();
          respond(log?.entries || []);
          break;
        }

        // ── Commands (all use request/response for reliable delivery) ──

        case 'installCLI':
          vscode.commands.executeCommand('activate-framework.installCLI');
          respond(null);
          break;
        case 'changeTier':
          vscode.commands.executeCommand('activate-framework.changeTier');
          respond(null);
          break;
        case 'changeManifest':
          vscode.commands.executeCommand('activate-framework.changeManifest');
          respond(null);
          break;
        case 'changePreset':
          vscode.commands.executeCommand('activate-framework.changePreset');
          respond(null);
          break;
        case 'editRepo': {
          const currentRepo = msg.current || '';
          const scope = msg.scope || 'project';
          vscode.window.showInputBox({
            title: 'Repository',
            prompt: 'GitHub owner/repo (e.g. peregrine-digital/activate-framework)',
            value: currentRepo,
            placeHolder: 'peregrine-digital/activate-framework',
          }).then(async (value) => {
            if (value === undefined) { respond(null); return; }
            const updates = value === '' ? { repo: '__clear__' } : { repo: value };
            await this._client.setConfig({ ...updates, scope });
            this.refresh();
            respond(null);
          }).catch((err) => respondError(err));
          break;
        }
        case 'editBranch': {
          const currentBranch = msg.current || '';
          const scope = msg.scope || 'project';
          const qp = vscode.window.createQuickPick();
          let disposed = false;
          let fetchedItems = [];
          qp.title = 'Branch';
          qp.placeholder = 'Type a branch name or select from the list…';
          qp.busy = true;
          qp.show();

          this._client.listBranches().then((branches) => {
            if (disposed) return;
            fetchedItems = (branches || []).map(b => ({ label: b }));
            qp.items = fetchedItems;
            const current = qp.items.find(i => i.label === currentBranch);
            if (current) qp.activeItems = [current];
            qp.busy = false;
          }).catch(() => {
            if (disposed) return;
            fetchedItems = currentBranch ? [{ label: currentBranch }] : [];
            qp.items = fetchedItems;
            qp.busy = false;
          });

          qp.onDidChangeValue((value) => {
            if (!value || fetchedItems.some(i => i.label === value)) {
              qp.items = fetchedItems;
            } else {
              qp.items = [{ label: value, description: '(custom)' }, ...fetchedItems];
            }
          });

          qp.onDidAccept(async () => {
            const selected = qp.selectedItems[0];
            disposed = true;
            qp.dispose();
            if (!selected) { respond(null); return; }
            const updates = selected.label === '' ? { branch: '__clear__' } : { branch: selected.label };
            await this._client.setConfig({ ...updates, scope });
            this.refresh();
            respond(null);
          });
          qp.onDidHide(() => {
            if (!disposed) { disposed = true; qp.dispose(); respond(null); }
          });
          break;
        }
        case 'addToWorkspace':
          await vscode.commands.executeCommand('activate-framework.addToWorkspace');
          respond(null);
          break;
        case 'removeFromWorkspace':
          await vscode.commands.executeCommand('activate-framework.removeFromWorkspace');
          respond(null);
          break;
        case 'updateAll':
          vscode.commands.executeCommand('activate-framework.updateAll');
          respond(null);
          break;
        case 'installFile':
          await vscode.commands.executeCommand('activate-framework.installFile', msg.file);
          respond(null);
          break;
        case 'uninstallFile':
          await vscode.commands.executeCommand('activate-framework.uninstallFile', msg.file);
          respond(null);
          break;
        case 'openFile':
          await vscode.commands.executeCommand('activate-framework.openFile', msg.file);
          respond(null);
          break;
        case 'diffFile':
          vscode.commands.executeCommand('activate-framework.diffFile', msg.file);
          respond(null);
          break;
        case 'skipUpdate':
          vscode.commands.executeCommand('activate-framework.skipFileUpdate', msg.file);
          respond(null);
          break;
        case 'refreshUsage':
          vscode.commands.executeCommand('activate-framework.telemetryRunNow').then(
            () => this.refresh(),
            () => this.refresh(),
          );
          respond(null);
          break;
        case 'checkForUpdates':
          vscode.commands.executeCommand('activate-framework.checkForUpdates');
          respond(null);
          break;

        default:
          if (reqId) {
            respondError(new Error(`Unknown command: ${msg.command}`));
          } else {
            console.warn('[ControlPanel] Unknown command:', msg.command);
          }
      }
    } catch (err) {
      console.error('[ControlPanel] Error handling', msg.command, err);
      respondError(err);
    }
  }

  // ── HTML shell ──────────────────────────────────────────

  _getHtml(jsUri, cssUri) {
    return /* html */ `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <link rel="stylesheet" href="${cssUri}">
</head>
<body>
  <div id="app"></div>
  <script type="module" src="${jsUri}"></script>
</body>
</html>`;
  }
}

module.exports = { ControlPanelProvider };
