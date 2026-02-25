const vscode = require('vscode');
const { listByCategory, TIER_MAP } = require('./manifest');

/**
 * Single WebviewView that replaces both the old control-panel and TreeView.
 *
 * Layout:
 *   ┌─ dirty-banner (hidden unless config changed) ─────┐
 *   │  ⚠ Copilot config changed.  [Reload Window]       │
 *   ├────────────────────────────────────────────────────┤
 *   │  v0.5.0  ·  standard  ·  Workspace ✓              │
 *   │  [Change Tier]  [+/− Workspace]  [↻ Update All]   │
 *   ├────────────────────────────────────────────────────┤
 *   │  ▸ Instructions  ─────────────────────────         │
 *   │    ✓ general  Universal coding conventions…  [🗑]  │
 *   │    ○ python   Python conventions…           [⬇]   │
 *   │  ▸ Skills  ────────────────────────────────        │
 *   │    …                                                │
 *   └────────────────────────────────────────────────────┘
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

  markDirty() {
    this._dirty = true;
    this._render();
  }

  clearDirty() {
    this._dirty = false;
    this._render();
  }

  async refresh() {
    await this._render();
  }

  // ── data helpers ──────────────────────────────────────

  async _gatherState() {
    const {
      findActivateWorkspaceFolder,
      readInstalledVersion,
      discoverBundledManifests,
      readBundledManifestById,
      isFileInstalled,
      readInstalledFileVersion,
      readBundledFileVersion,
    } = require('./installer');

    const config = vscode.workspace.getConfiguration('activate-framework');
    const tier = config.get('defaultTier', 'standard');
    const version = this._context.extension.packageJSON.version ?? 'unknown';
    const isActive = !!findActivateWorkspaceFolder();
    const installedInfo = await readInstalledVersion(this._context);
    const installedVersion = installedInfo?.version || null;
    const activeManifestId = installedInfo?.manifest || 'activate-framework';

    let files = [];
    let manifestName = 'Activate Framework';
    try {
      const chosen = await readBundledManifestById(this._context, activeManifestId);
      files = chosen.files;
      manifestName = chosen.name;
    } catch {
      // Fall back to first discovered manifest
      try {
        const all = await discoverBundledManifests(this._context);
        if (all.length > 0) {
          files = all[0].files;
          manifestName = all[0].name;
        }
      } catch {
        /* empty */
      }
    }

    // Determine which are currently on disk + version info
    /** @type {Set<string>} */
    const installedSet = new Set();
    /** @type {Map<string, {installed: string|null, bundled: string|null}>} */
    const versionMap = new Map();

    for (const f of files) {
      if (await isFileInstalled(this._context, f)) {
        installedSet.add(f.dest);
        const iv = await readInstalledFileVersion(this._context, f);
        const bv = await readBundledFileVersion(this._context, f);
        versionMap.set(f.dest, { installed: iv, bundled: bv });
      }
    }

    // Determine which are available for this tier but not installed
    const allowed = TIER_MAP[tier] ?? TIER_MAP.standard;
    const tierFiles = files.filter((f) => allowed.has(f.tier));

    const installedFiles = files.filter((f) => installedSet.has(f.dest));
    const availableFiles = tierFiles.filter((f) => !installedSet.has(f.dest));

    return {
      version: installedVersion || version,
      tier,
      isActive,
      installedFiles,
      availableFiles,
      versionMap,
    };
  }

  // ── render ────────────────────────────────────────────

  async _render() {
    if (!this._view) return;
    const state = await this._gatherState();
    this._view.webview.html = this._getHtml(state);
  }

  // ── messages from webview ─────────────────────────────

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
      case 'installFile':
        vscode.commands.executeCommand('activate-framework.installFile', msg.file);
        break;
      case 'uninstallFile':
        // Build a pseudo tree-item the command handler expects
        vscode.commands.executeCommand('activate-framework.uninstallFile', {
          fileData: msg.file,
        });
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
    }
  }

  // ── HTML ──────────────────────────────────────────────

  _getHtml({ version, tier, isActive, installedFiles, availableFiles, versionMap }) {
    const dirty = this._dirty;
    const wsAction = isActive ? 'removeFromWorkspace' : 'addToWorkspace';
    const wsButtonLabel = isActive ? '− Remove Workspace' : '+ Add Workspace';

    const CATEGORY_ICONS = {
      instructions: '📝',
      prompts: '💬',
      skills: '🛠',
      agents: '🤖',
      other: '📄',
    };

    /** Build HTML for one file card */
    const fileCard = (f, installed) => {
      const name = esc(displayName(f));
      const desc = esc(f.description || '');
      const tierBadge = esc(f.tier);
      const json = esc(JSON.stringify(f));

      // Version info for installed files
      let versionHtml = '';
      let outdated = false;
      if (installed && versionMap) {
        const vi = versionMap.get(f.dest);
        if (vi) {
          const iv = vi.installed || '?';
          const bv = vi.bundled || '?';
          outdated = vi.installed && vi.bundled && vi.installed !== vi.bundled;
          versionHtml = outdated
            ? `<span class="file-version outdated" title="Installed: ${esc(iv)} → Available: ${esc(bv)}">v${esc(iv)} → v${esc(bv)}</span>`
            : `<span class="file-version" title="Version ${esc(iv)}">v${esc(iv)}</span>`;
        }
      }

      // Action buttons
      let actionButtons = '';
      if (installed) {
        if (outdated) {
          actionButtons = `
            <button class="icon-btn" title="Show diff" onclick="event.stopPropagation(); send('diffFile', ${json})">⇔</button>
            <button class="icon-btn" title="Skip this update" onclick="event.stopPropagation(); send('skipUpdate', ${json})">✓</button>
            <button class="icon-btn" title="Update to latest" onclick="event.stopPropagation(); send('installFile', ${json})">↑</button>
            <button class="icon-btn danger" title="Uninstall" onclick="event.stopPropagation(); send('uninstallFile', ${json})">✕</button>`;
        } else {
          actionButtons = `
            <button class="icon-btn danger" title="Uninstall" onclick="event.stopPropagation(); send('uninstallFile', ${json})">✕</button>`;
        }
      } else {
        actionButtons = `
          <button class="icon-btn" title="Install" onclick="event.stopPropagation(); send('installFile', ${json})">↓</button>`;
      }

      const openClick = installed
        ? `onclick="send('openFile', ${json})"` : '';
      const cursorClass = installed ? 'clickable' : '';

      return `
        <div class="file-card ${installed ? 'installed' : 'available'}${outdated ? ' outdated-card' : ''}">
          <div class="file-main ${cursorClass}" ${openClick}>
            <span class="file-status">${installed ? (outdated ? '⬆' : '✓') : '○'}</span>
            <div class="file-info">
              <span class="file-name">${name} ${versionHtml}</span>
              <span class="file-desc">${desc}</span>
            </div>
          </div>
          <div class="file-actions">
            <span class="file-tier">${tierBadge}</span>
            ${actionButtons}
          </div>
        </div>`;
    };

    /** Build HTML for a category group */
    const categorySection = (label, icon, files, installed, sectionPrefix) => {
      if (!files.length) return '';
      const id = `${sectionPrefix}-${label.toLowerCase()}`;
      const cards = files.map((f) => fileCard(f, installed)).join('');
      return `
        <details class="category" data-cat-id="${esc(id)}">
          <summary>${icon} ${esc(label)} <span class="count">${files.length}</span></summary>
          ${cards}
        </details>`;
    };

    // Group files by category
    const installedGroups = listByCategory(installedFiles);
    const availableGroups = listByCategory(availableFiles);

    const installedHtml = installedGroups
      .map((g) => categorySection(g.label, CATEGORY_ICONS[g.category] || '📄', g.files, true, 'installed'))
      .join('');

    const availableHtml = availableGroups
      .map((g) => categorySection(g.label, CATEGORY_ICONS[g.category] || '📄', g.files, false, 'available'))
      .join('');

    return /* html */ `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    :root {
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
      color: var(--vscode-foreground);
    }
    body { padding: 0 10px 16px; }

    /* ── Dirty banner ── */
    .dirty-banner {
      display: ${dirty ? 'flex' : 'none'};
      align-items: center;
      gap: 8px;
      background: var(--vscode-inputValidation-warningBackground, #5a5000);
      border: 1px solid var(--vscode-inputValidation-warningBorder, #b89500);
      border-radius: 4px;
      padding: 6px 10px;
      margin: 8px 0;
      font-size: 12px;
    }
    .dirty-banner span { flex: 1; }

    /* ── Status bar ── */
    .status-bar {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 0;
      font-size: 12px;
      opacity: 0.85;
      flex-wrap: wrap;
    }
    .status-bar .badge {
      background: var(--vscode-badge-background);
      color: var(--vscode-badge-foreground);
      border-radius: 10px;
      padding: 1px 8px;
      font-size: 11px;
    }
    .status-bar .dot {
      opacity: 0.4;
    }
    .status-bar .ws-status {
      display: inline-flex;
      align-items: center;
      gap: 3px;
    }

    /* ── Buttons ── */
    .button-row {
      display: flex;
      gap: 6px;
      padding-bottom: 10px;
      flex-wrap: wrap;
    }
    button {
      border: 1px solid var(--vscode-button-border, transparent);
      border-radius: 3px;
      cursor: pointer;
      font-family: var(--vscode-font-family);
      font-size: 12px;
      line-height: 20px;
      padding: 4px 10px;
      white-space: nowrap;
    }
    button.primary {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
    }
    button.primary:hover { background: var(--vscode-button-hoverBackground); }
    button.secondary {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    button.secondary:hover { background: var(--vscode-button-secondaryHoverBackground); }

    /* ── Divider ── */
    hr {
      border: none;
      border-top: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, #333));
      margin: 2px 0 8px;
    }

    /* ── Section headers ── */
    .section-label {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      opacity: 0.6;
      margin: 10px 0 4px;
      font-weight: 600;
    }

    /* ── Category groups ── */
    details.category {
      margin-bottom: 2px;
    }
    details.category summary {
      cursor: pointer;
      padding: 5px 4px;
      font-weight: 600;
      font-size: 12px;
      border-radius: 3px;
      user-select: none;
      list-style: none;
    }
    details.category summary::-webkit-details-marker { display: none; }
    details.category summary::before {
      content: '▸ ';
      display: inline;
      font-size: 10px;
    }
    details[open].category summary::before {
      content: '▾ ';
    }
    details.category summary:hover {
      background: var(--vscode-list-hoverBackground);
    }
    details.category summary .count {
      opacity: 0.5;
      font-weight: 400;
      margin-left: 4px;
    }

    /* ── File card ── */
    .file-card {
      display: flex;
      align-items: center;
      gap: 6px;
      padding: 4px 6px 4px 20px;
      border-radius: 3px;
      min-height: 32px;
    }
    .file-card:hover {
      background: var(--vscode-list-hoverBackground);
    }
    .file-main {
      display: flex;
      align-items: flex-start;
      gap: 6px;
      flex: 1;
      min-width: 0;
    }
    .file-main.clickable { cursor: pointer; }
    .file-status {
      flex-shrink: 0;
      width: 14px;
      text-align: center;
      font-size: 12px;
      line-height: 18px;
    }
    .installed .file-status {
      color: var(--vscode-testing-iconPassed, #73c991);
    }
    .available .file-status {
      opacity: 0.4;
    }
    .file-info {
      display: flex;
      flex-direction: column;
      min-width: 0;
    }
    .file-name {
      font-size: 12px;
      font-weight: 500;
      white-space: nowrap;
      overflow: hidden;
      text-overflow: ellipsis;
    }
    .file-desc {
      font-size: 11px;
      opacity: 0.65;
      line-height: 1.3;
      display: -webkit-box;
      -webkit-line-clamp: 2;
      -webkit-box-orient: vertical;
      overflow: hidden;
    }
    .file-actions {
      display: flex;
      align-items: center;
      gap: 4px;
      flex-shrink: 0;
    }
    .file-tier {
      font-size: 10px;
      opacity: 0.4;
      white-space: nowrap;
    }
    .icon-btn {
      background: none;
      border: 1px solid transparent;
      color: var(--vscode-foreground);
      cursor: pointer;
      padding: 2px 5px;
      border-radius: 3px;
      font-size: 13px;
      opacity: 0;
      transition: opacity 0.1s;
    }
    .file-card:hover .icon-btn {
      opacity: 0.6;
    }
    .icon-btn:hover {
      opacity: 1 !important;
      background: var(--vscode-toolbar-hoverBackground);
    }
    .icon-btn.danger:hover {
      color: var(--vscode-errorForeground, #f48771);
    }

    /* ── Empty state ── */
    .empty {
      opacity: 0.5;
      font-style: italic;
      padding: 8px 20px;
      font-size: 12px;
    }

    /* ── Version badges ── */
    .file-version {
      font-size: 10px;
      opacity: 0.45;
      font-weight: 400;
      margin-left: 4px;
    }
    .file-version.outdated {
      color: var(--vscode-editorWarning-foreground, #cca700);
      opacity: 0.9;
      font-weight: 500;
    }
    .outdated-card {
      border-left: 2px solid var(--vscode-editorWarning-foreground, #cca700);
    }
    .outdated-card .file-status {
      color: var(--vscode-editorWarning-foreground, #cca700);
    }
  </style>
</head>
<body>
  <div class="dirty-banner">
    <span>⚠ Copilot config changed. Reload to apply.</span>
    <button class="primary" onclick="send('reload')">↻ Reload</button>
  </div>

  <div class="status-bar">
    <span>v${esc(version)}</span>
    <span class="dot">·</span>
    <span class="badge">${esc(tier)}</span>
    <span class="dot">·</span>
    <span class="ws-status">${isActive ? '✓' : '○'} Workspace</span>
  </div>

  <div class="button-row">
    <button class="secondary" onclick="send('changeTier')">◆ Tier</button>
    <button class="secondary" onclick="send('${wsAction}')">${esc(wsButtonLabel)}</button>
    <button class="primary" onclick="send('updateAll')">↻ Update</button>
  </div>

  <hr>

  <div class="section-label">Installed · ${installedFiles.length}</div>
  ${installedHtml || '<div class="empty">No files installed</div>'}

  <div class="section-label">Available · ${availableFiles.length}</div>
  ${availableHtml || '<div class="empty">All tier files installed</div>'}

  <script>
    const vscode = acquireVsCodeApi();
    function send(command, file) {
      vscode.postMessage({ command, file });
    }

    // Restore open/closed state of category sections
    (function restoreState() {
      const prev = vscode.getState() || {};
      const openCats = prev.openCategories || {};
      document.querySelectorAll('details[data-cat-id]').forEach(d => {
        if (openCats[d.dataset.catId]) d.open = true;
      });
    })();

    // Persist open/closed state when user toggles a category
    document.querySelectorAll('details[data-cat-id]').forEach(d => {
      d.addEventListener('toggle', () => {
        const prev = vscode.getState() || {};
        const openCats = prev.openCategories || {};
        openCats[d.dataset.catId] = d.open;
        vscode.setState({ ...prev, openCategories: openCats });
      });
    });
  </script>
</body>
</html>`;
  }
}

/** Derive a display name from the file dest path */
function displayName(f) {
  const parts = f.dest.split('/');
  const filename = parts[parts.length - 1];
  if (filename === 'SKILL.md' && parts.length >= 2) {
    return parts[parts.length - 2];
  }
  return filename
    .replace(/\.(instructions|prompt|agent)\.md$/, '')
    .replace(/\.md$/, '');
}

/** HTML-escape */
function esc(str) {
  return String(str)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

module.exports = { ControlPanelProvider };
