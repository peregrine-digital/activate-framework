const vscode = require('vscode');

/**
 * WebviewViewProvider for the Peregrine Activate control panel.
 *
 * Renders real HTML buttons styled with VS Code's CSS custom properties
 * so they look native. Communicates with the extension host via postMessage.
 */
class ControlPanelProvider {
  static viewType = 'activate-framework.controlPanel';

  constructor(context) {
    this._context = context;
    this._view = null;
    this._dirty = false;
  }

  /** @param {vscode.WebviewView} webviewView */
  resolveWebviewView(webviewView) {
    this._view = webviewView;
    webviewView.webview.options = { enableScripts: true };
    webviewView.webview.onDidReceiveMessage((msg) => this._onMessage(msg));
    this._render();
  }

  /** Mark config as dirty and show reload banner */
  markDirty() {
    this._dirty = true;
    this._render();
  }

  /** Clear dirty flag (e.g. after reload) */
  clearDirty() {
    this._dirty = false;
    this._render();
  }

  /** Re-render the panel with latest state */
  async refresh() {
    this._render();
  }

  async _render() {
    if (!this._view) return;

    const config = vscode.workspace.getConfiguration('activate-framework');
    const tier = config.get('defaultTier', 'standard');
    const version = this._context.extension.packageJSON.version ?? 'unknown';

    // Check workspace status
    const { findActivateWorkspaceFolder, readInstalledVersion, readBundledManifest, isFileInstalled } =
      require('./installer');

    const isActive = !!findActivateWorkspaceFolder();
    const installedVersion = await readInstalledVersion(this._context);

    // Count installed files
    let installedCount = 0;
    let totalCount = 0;
    try {
      const manifest = await readBundledManifest(this._context);
      totalCount = manifest.files.length;
      for (const f of manifest.files) {
        if (await isFileInstalled(this._context, f)) {
          installedCount++;
        }
      }
    } catch {
      // ignore
    }

    this._view.webview.html = this._getHtml(
      installedVersion || version,
      tier,
      isActive,
      installedCount,
      totalCount,
      this._dirty,
    );
  }

  _onMessage(msg) {
    switch (msg.command) {
      case 'changeTier':
        vscode.commands.executeCommand('activate-framework.changeTier');
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
      case 'reload':
        vscode.commands.executeCommand('workbench.action.reloadWindow');
        break;
    }
  }

  _getHtml(version, tier, isActive, installedCount, totalCount, dirty) {
    const wsLabel = isActive ? 'Active' : 'Not Active';
    const wsIcon = isActive ? '✓' : '○';
    const wsAction = isActive ? 'removeFromWorkspace' : 'addToWorkspace';
    const wsButtonLabel = isActive ? 'Remove' : 'Add';

    return /* html */ `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    :root {
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
      color: var(--vscode-foreground);
    }
    body {
      padding: 0 12px 12px;
      margin: 0;
    }

    /* ── Status rows ── */
    .status-grid {
      display: grid;
      grid-template-columns: auto 1fr auto;
      gap: 4px 8px;
      align-items: center;
      margin-bottom: 12px;
    }
    .status-icon {
      opacity: 0.7;
      font-size: 14px;
      width: 16px;
      text-align: center;
    }
    .status-label {
      opacity: 0.8;
      white-space: nowrap;
    }
    .status-value {
      text-align: right;
      font-weight: 500;
    }
    .status-value .badge {
      display: inline-block;
      background: var(--vscode-badge-background);
      color: var(--vscode-badge-foreground);
      border-radius: 10px;
      padding: 1px 8px;
      font-size: 11px;
    }

    /* ── Buttons ── */
    .button-group {
      display: flex;
      flex-direction: column;
      gap: 6px;
    }
    button {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 6px;
      width: 100%;
      padding: 6px 14px;
      border: 1px solid var(--vscode-button-border, transparent);
      border-radius: 2px;
      cursor: pointer;
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
      line-height: 20px;
    }
    button.primary {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
    }
    button.primary:hover {
      background: var(--vscode-button-hoverBackground);
    }
    button.secondary {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    button.secondary:hover {
      background: var(--vscode-button-secondaryHoverBackground);
    }

    /* ── Dirty banner ── */
    .dirty-banner {
      display: ${dirty ? 'block' : 'none'};
      background: var(--vscode-inputValidation-warningBackground, #5a5000);
      border: 1px solid var(--vscode-inputValidation-warningBorder, #b89500);
      border-radius: 3px;
      padding: 8px 10px;
      margin-bottom: 10px;
      font-size: 12px;
    }
    .dirty-banner p {
      margin: 0 0 8px;
    }
    .dirty-banner button {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
    }

    /* ── Divider ── */
    hr {
      border: none;
      border-top: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, #444));
      margin: 10px 0;
    }
  </style>
</head>
<body>
  <div class="dirty-banner">
    <p>⚠ Copilot config changed. Reload window to apply.</p>
    <button class="primary" onclick="send('reload')">↻ Reload Window</button>
  </div>

  <div class="status-grid">
    <span class="status-icon">🏷</span>
    <span class="status-label">Version</span>
    <span class="status-value">${escHtml(version)}</span>

    <span class="status-icon">◆</span>
    <span class="status-label">Tier</span>
    <span class="status-value"><span class="badge">${escHtml(tier)}</span></span>

    <span class="status-icon">${wsIcon}</span>
    <span class="status-label">Workspace</span>
    <span class="status-value">${escHtml(wsLabel)}</span>

    <span class="status-icon">📦</span>
    <span class="status-label">Installed</span>
    <span class="status-value">${installedCount} / ${totalCount}</span>
  </div>

  <hr>

  <div class="button-group">
    <button class="secondary" onclick="send('changeTier')">◆ Change Tier</button>
    <button class="secondary" onclick="send('${wsAction}')">${wsIcon === '✓' ? '−' : '+'} ${escHtml(wsButtonLabel)} Workspace Root</button>
    <button class="primary" onclick="send('updateAll')">↻ Update All Installed</button>
  </div>

  <script>
    const vscode = acquireVsCodeApi();
    function send(command) {
      vscode.postMessage({ command });
    }
  </script>
</body>
</html>`;
  }
}

function escHtml(str) {
  return String(str).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

module.exports = { ControlPanelProvider };
