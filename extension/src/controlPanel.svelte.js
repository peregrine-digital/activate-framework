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

    // Build the HTML that loads the Svelte bundle
    const jsUri = webviewView.webview.asWebviewUri(
      vscode.Uri.file(path.join(webviewDistPath, 'webview.js')),
    );
    const cssUri = webviewView.webview.asWebviewUri(
      vscode.Uri.file(path.join(webviewDistPath, 'webview.css')),
    );

    webviewView.webview.html = this._getHtml(jsUri, cssUri);

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

        // ── Fire-and-forget (dispatch to extension commands) ──

        case 'installCLI':
          vscode.commands.executeCommand('activate-framework.installCLI');
          break;
        case 'changeTier':
          vscode.commands.executeCommand('activate-framework.changeTier');
          break;
        case 'changeManifest':
          vscode.commands.executeCommand('activate-framework.changeManifest');
          break;
        case 'addToWorkspace':
          vscode.commands.executeCommand('activate-framework.addToWorkspace');
          break;
        case 'removeFromWorkspace':
          vscode.commands.executeCommand('activate-framework.removeFromWorkspace');
          break;
        case 'updateAll':
          vscode.commands.executeCommand('activate-framework.updateAll');
          break;
        case 'installFile':
          vscode.commands.executeCommand('activate-framework.installFile', msg.file);
          break;
        case 'uninstallFile':
          vscode.commands.executeCommand('activate-framework.uninstallFile', msg.file);
          break;
        case 'openFile':
          vscode.commands.executeCommand('activate-framework.openFile', msg.file);
          break;
        case 'diffFile':
          vscode.commands.executeCommand('activate-framework.diffFile', msg.file);
          break;
        case 'skipUpdate':
          vscode.commands.executeCommand('activate-framework.skipFileUpdate', msg.file);
          break;
        case 'refreshUsage':
          vscode.commands.executeCommand('activate-framework.telemetryRunNow').then(
            () => this.refresh(),
            () => this.refresh(),
          );
          break;
        case 'checkForUpdates':
          vscode.commands.executeCommand('activate-framework.checkForUpdates');
          break;

        default:
          console.warn('[ControlPanel] Unknown command:', msg.command);
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
