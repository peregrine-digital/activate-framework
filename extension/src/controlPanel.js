const vscode = require('vscode');

/**
 * Single WebviewView that replaces both the old control-panel and TreeView.
 *
 * Layout:
 *   ┌────────────────────────────────────────────────────┐
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

  /** @param {import('./client').ActivateClient|null} client */
  constructor(client, extensionVersion) {
    this._client = client;
    this._view = null;
    /** @type {'main'|'usage'|'settings'|'no-cli'} */
    this._currentPage = client ? 'main' : 'no-cli';
    this._extensionVersion = extensionVersion || '';
    /** @type {string|null} ISO timestamp of last update check */
    this._lastUpdateCheck = null;
    /** @type {ReturnType<typeof setTimeout>|null} */
    this._refreshTimer = null;
  }

  /** Update the client after CLI is installed and daemon started. */
  setClient(client) {
    this._client = client;
    this._currentPage = 'main';
  }

  /** Store the timestamp of the last update check (ISO string). */
  setLastUpdateCheck(isoTimestamp) {
    this._lastUpdateCheck = isoTimestamp || null;
  }

  /** @param {vscode.WebviewView} webviewView */
  resolveWebviewView(webviewView) {
    this._view = webviewView;
    webviewView.webview.options = { enableScripts: true };
    webviewView.webview.onDidReceiveMessage((msg) => this._onMessage(msg));
    this._render();
  }

  async refresh() {
    // Debounce: if a refresh is already scheduled, skip this one.
    // This prevents duplicate work when both the command handler and the
    // stateChanged notification trigger a refresh within a short window.
    if (this._refreshTimer) clearTimeout(this._refreshTimer);
    return new Promise((resolve) => {
      this._refreshTimer = setTimeout(async () => {
        this._refreshTimer = null;
        await this._render();
        resolve();
      }, 100);
    });
  }

  // ── data helpers ──────────────────────────────────────

  async _gatherState() {
    const state = await this._client.getState();

    const cfg = state.config || {};
    const tier = cfg.tier || '';
    const fileOverrides = cfg.fileOverrides || {};
    const skippedVersions = cfg.skippedVersions || {};
    const isActive = state.state?.hasInstallMarker || false;
    const manifestName = cfg.manifest || '';
    const manifests = state.manifests || [];
    const manifestCount = manifests.length || 1;

    // Cache metadata from daemon for use by other methods
    this._categories = state.categories || [];
    this._telemetryLogPath = state.telemetryLogPath || '';

    // Build file lists from daemon FileStatus[] — uses daemon-computed inTier
    const files = state.files || [];
    const versionMap = new Map();
    const installedFiles = [];
    const availableFiles = [];
    const outsideTierFiles = [];
    const excludedFiles = [];

    for (const f of files) {
      if (f.installed) {
        versionMap.set(f.dest, {
          installed: f.installedVersion || null,
          bundled: f.bundledVersion || null,
        });
      }

      const isExcluded = f.override === 'excluded';
      const isPinned = f.override === 'pinned';
      const inTier = isPinned || f.inTier;

      if (isExcluded) {
        excludedFiles.push(f);
        continue;
      }

      if (f.installed) {
        installedFiles.push(f);
      } else if (inTier) {
        availableFiles.push(f);
      } else {
        outsideTierFiles.push(f);
      }
    }

    // Tier label from daemon tier definitions
    const tiers = state.tiers || [];
    const activeTier = tiers.find((t) => t.id === tier);
    const tierLabel = activeTier ? activeTier.label : tier;

    return {
      tier,
      tierLabel,
      isActive,
      manifestName,
      manifestCount,
      installedFiles,
      availableFiles,
      outsideTierFiles,
      excludedFiles,
      versionMap,
      fileOverrides,
      skippedVersions,
    };
  }

  // ── render ────────────────────────────────────────────

  async _render() {
    if (!this._view) return;
    if (this._currentPage === 'no-cli') {
      this._view.webview.html = this._getNoCliHtml();
      return;
    }
    try {
      if (this._currentPage === 'usage') {
        const state = await this._client.getState();
        const telemetryEnabled = state?.config?.telemetryEnabled === true;
        const entries = await this._client.readTelemetryLog();
        this._view.webview.html = this._getUsageHtml(entries || [], telemetryEnabled);
      } else if (this._currentPage === 'settings') {
        const [state, globalCfg, projectCfg] = await Promise.all([
          this._client.getState(),
          this._client.getConfig('global'),
          this._client.getConfig('project'),
        ]);
        this._view.webview.html = this._getSettingsHtml(state, globalCfg, projectCfg);
      } else {
        const state = await this._gatherState();
        this._view.webview.html = this._getHtml(state);
      }
    } catch (err) {
      // Daemon may not be ready yet — log for diagnostics
      if (typeof console !== 'undefined') {
        console.warn('[ControlPanel] render failed:', err?.message || err);
      }
    }
  }

  // ── messages from webview ─────────────────────────────

  _onMessage(msg) {
    switch (msg.command) {
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
      case 'setOverride': {
        const data = msg.file || {};
        this._client.setFileOverride(data.file, data.override).then(() => this._render());
        break;
      }
      case 'showUsage':
        this._currentPage = 'usage';
        this._render();
        break;
      case 'showSettings':
        this._currentPage = 'settings';
        this._render();
        break;
      case 'backToMain':
        this._currentPage = 'main';
        this._render();
        break;
      case 'refreshUsage':
        vscode.commands.executeCommand('activate-framework.telemetryRunNow').then(
          () => this._render(),
          () => this._render(),
        );
        break;
      case 'toggleTelemetry':
        this._client.setConfig({
          telemetryEnabled: msg.enabled,
          scope: 'global',
        }).then(
          () => this._render(),
          () => this._render(),
        );
        break;
      case 'setGlobalDefault':
        this._client.setConfig({
          ...msg.updates,
          scope: 'global',
        }).then(
          () => this._render(),
          () => this._render(),
        );
        break;
      case 'clearProjectOverride':
        this._client.setConfig({
          ...msg.updates,
          scope: 'project',
        }).then(
          () => this._render(),
          () => this._render(),
        );
        break;
      case 'editRepo': {
        const currentRepo = msg.current || '';
        vscode.window.showInputBox({
          title: 'Repository',
          prompt: 'GitHub owner/repo (e.g. peregrine-digital/activate-framework)',
          value: currentRepo,
          placeHolder: 'peregrine-digital/activate-framework',
        }).then((value) => {
          if (value === undefined) return; // cancelled
          const scope = msg.scope || 'project';
          const updates = value === ''
            ? { repo: '__clear__' }
            : { repo: value };
          this._client.setConfig({ ...updates, scope }).then(
            () => this._render(),
            () => this._render(),
          );
        });
        break;
      }
      case 'editBranch': {
        const currentBranch = msg.current || '';
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

        // Allow freeform input: add typed value as first item if not in list
        qp.onDidChangeValue((value) => {
          if (!value || fetchedItems.some(i => i.label === value)) {
            qp.items = fetchedItems;
          } else {
            qp.items = [{ label: value, description: '(custom)' }, ...fetchedItems];
          }
        });

        qp.onDidAccept(() => {
          const selected = qp.selectedItems[0];
          disposed = true;
          qp.dispose();
          if (!selected) return;
          const scope = msg.scope || 'project';
          const value = selected.label;
          const updates = value === ''
            ? { branch: '__clear__' }
            : { branch: value };
          this._client.setConfig({ ...updates, scope }).then(
            () => this._render(),
            () => this._render(),
          );
        });
        qp.onDidHide(() => {
          if (!disposed) {
            disposed = true;
            qp.dispose();
          }
        });
        break;
      }
      case 'checkForUpdates':
        vscode.commands.executeCommand('activate-framework.checkForUpdates');
        break;
      case 'openLogFile': {
        const logPath = this._telemetryLogPath;
        if (!logPath) {
          vscode.window.showWarningMessage('Telemetry log path not available.');
          break;
        }
        vscode.commands.executeCommand('vscode.open', vscode.Uri.file(logPath)).then(
          () => {},
          () => vscode.window.showWarningMessage(`Could not open ${logPath}`),
        );
        break;
      }
    }
  }

  // ── HTML ──────────────────────────────────────────────

  _getNoCliHtml() {
    return `<!DOCTYPE html>
<html><head><style>
  body { font-family: var(--vscode-font-family); color: var(--vscode-foreground); padding: 16px; text-align: center; }
  .icon { font-size: 48px; margin: 24px 0 12px; }
  h3 { margin: 0 0 8px; }
  p { color: var(--vscode-descriptionForeground); font-size: 13px; margin: 0 0 20px; }
  button { padding: 8px 16px; background: var(--vscode-button-background); color: var(--vscode-button-foreground);
    border: none; border-radius: 4px; cursor: pointer; font-size: 13px; }
  button:hover { background: var(--vscode-button-hoverBackground); }
  code { background: var(--vscode-textCodeBlock-background); padding: 2px 6px; border-radius: 3px; font-size: 12px; }
</style></head><body>
  <div class="icon">⚠️</div>
  <h3>CLI Not Installed</h3>
  <p>The Activate CLI is required for this extension to work. Click below to install it.</p>
  <button onclick="send('installCLI')">Install Activate CLI</button>
  <script>
    const vscode = acquireVsCodeApi();
    function send(command) { vscode.postMessage({ command }); }
  </script>
</body></html>`;
  }

  _getHtml({ tier, tierLabel, isActive, manifestName, manifestCount, installedFiles, availableFiles, outsideTierFiles, excludedFiles, versionMap, fileOverrides, skippedVersions }) {
    const installAction = isActive ? 'removeFromWorkspace' : 'addToWorkspace';
    const installButtonLabel = isActive ? '− Remove' : '+ Install';

    /** Build HTML for one file card */
    const fileCard = (f, installed) => {
      const name = esc(displayName(f));
      const desc = esc(f.description || '');
      const tierBadge = esc(f.tier);
      const json = esc(JSON.stringify(f));

      // File override badge (pinned / excluded) — from daemon FileStatus
      const override = f.override || '';
      const overrideBadge = override === 'pinned'
        ? '<span class="override-badge pinned" title="Pinned — always included">📌</span>'
        : override === 'excluded'
          ? '<span class="override-badge excluded" title="Excluded — never installed">🚫</span>'
          : '';

      // Version info for installed files
      let versionHtml = '';
      let outdated = false;
      if (installed && versionMap) {
        const vi = versionMap.get(f.dest);
        if (vi) {
          const iv = vi.installed || '?';
          const bv = vi.bundled || '?';
          // A file is outdated only if versions differ AND the user
          // has not explicitly skipped the bundled version.
          const skippedVer = skippedVersions?.[f.dest];
          outdated = vi.installed && vi.bundled && vi.installed !== vi.bundled
            && skippedVer !== vi.bundled;
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
      } else if (override === 'excluded') {
        // Excluded files should not show install/uninstall buttons
        actionButtons = '';
      } else {
        actionButtons = `
          <button class="icon-btn" title="Install" onclick="event.stopPropagation(); send('installFile', ${json})">↓</button>`;
      }

      const openClick = installed
        ? `onclick="send('openFile', ${json})"` : '';
      const cursorClass = installed ? 'clickable' : '';

      // Override action buttons
      let overrideButtons = '';
      if (override === 'pinned') {
        overrideButtons = `
          <button class="icon-btn" title="Remove pin" onclick="event.stopPropagation(); send('setOverride', { file: '${esc(f.dest)}', override: '' })">📌✕</button>`;
      } else if (override === 'excluded') {
        overrideButtons = `
          <button class="icon-btn" title="Remove exclusion" onclick="event.stopPropagation(); send('setOverride', { file: '${esc(f.dest)}', override: '' })">🚫✕</button>`;
      } else {
        overrideButtons = `
          <button class="icon-btn" title="Pin (always include)" onclick="event.stopPropagation(); send('setOverride', { file: '${esc(f.dest)}', override: 'pinned' })">📌</button>
          <button class="icon-btn" title="Exclude (never install)" onclick="event.stopPropagation(); send('setOverride', { file: '${esc(f.dest)}', override: 'excluded' })">🚫</button>`;
      }

      return `
        <div class="file-card ${installed ? 'installed' : 'available'}${outdated ? ' outdated-card' : ''}">
          <div class="file-main ${cursorClass}" ${openClick}>
            <span class="file-status">${installed ? (outdated ? '⬆' : '✓') : '○'}</span>
            <div class="file-info">
              <span class="file-name">${name} ${versionHtml} ${overrideBadge}</span>
              <span class="file-desc">${desc}</span>
            </div>
          </div>
          <div class="file-actions">
            <span class="file-tier">${tierBadge}</span>
            ${actionButtons}
            ${overrideButtons}
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

    // Group files by category (using daemon-provided category metadata)
    const categories = this._categories || [];
    const installedGroups = groupByCategory(installedFiles, categories);
    const availableGroups = groupByCategory(availableFiles, categories);
    const outsideTierGroups = groupByCategory(outsideTierFiles || [], categories);
    const excludedGroups = groupByCategory(excludedFiles || [], categories);

    const installedHtml = installedGroups
      .map((g) => categorySection(g.label, CATEGORY_ICONS_DEFAULT[g.category] || '📄', g.files, true, 'installed'))
      .join('');

    const availableHtml = availableGroups
      .map((g) => categorySection(g.label, CATEGORY_ICONS_DEFAULT[g.category] || '📄', g.files, false, 'available'))
      .join('');

    const outsideTierHtml = outsideTierGroups
      .map((g) => categorySection(g.label, CATEGORY_ICONS_DEFAULT[g.category] || '📄', g.files, false, 'outside'))
      .join('');

    const excludedHtml = excludedGroups
      .map((g) => categorySection(g.label, CATEGORY_ICONS_DEFAULT[g.category] || '📄', g.files, false, 'excluded'))
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
    .status-bar .spacer {
      flex-grow: 1;
    }
    .status-bar .gear-btn {
      cursor: pointer;
      opacity: 0.6;
      font-size: 20px;
      padding: 2px 4px;
      border-radius: 3px;
      transition: opacity 0.15s, background 0.15s;
    }
    .status-bar .gear-btn:hover {
      opacity: 1;
      background: var(--vscode-toolbar-hoverBackground);
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

    /* ── Override badges ── */
    .override-badge {
      font-size: 10px;
      margin-left: 4px;
      vertical-align: middle;
    }

    /* ── Outside tier / Excluded sections ── */
    .dim-section-label {
      opacity: 0.5;
    }
    .dim-section-hint {
      font-size: 11px;
      opacity: 0.4;
      font-style: italic;
      padding: 0 0 6px 0;
    }
  </style>
</head>
<body>
  <div class="status-bar">
    <span class="badge">${esc(tierLabel)}</span>
    <span class="dot">·</span>
    <span class="badge">${esc(manifestName)}</span>
    <span class="dot">·</span>
    <span class="ws-status">${isActive ? '✓' : '○'} Installed</span>
    <span class="spacer"></span>
    <span class="gear-btn" onclick="send('showSettings')" title="Settings">⚙</span>
  </div>

  <div class="button-row">
    <button class="secondary" onclick="send('changeTier')">◆ Tier</button>
    ${manifestCount > 1 ? `<button class="secondary" onclick="send('changeManifest')">⇋ Manifest</button>` : ''}
    <button class="secondary" onclick="send('${installAction}')">${esc(installButtonLabel)}</button>
    <button class="primary" onclick="send('updateAll')">↻ Update</button>
    <button class="secondary" onclick="send('showUsage')">📊 Usage</button>
  </div>

  <hr>

  <div class="section-label">Installed · ${installedFiles.length}</div>
  ${installedHtml || '<div class="empty">No files installed</div>'}

  <div class="section-label">Available · ${availableFiles.length}</div>
  ${availableHtml || '<div class="empty">All tier files installed</div>'}

  ${outsideTierFiles && outsideTierFiles.length > 0 ? `
  <div class="section-label dim-section-label">Outside Tier · ${outsideTierFiles.length}</div>
  <div class="dim-section-hint">Switch to a higher tier to access these files</div>
  ${outsideTierHtml}
  ` : ''}

  ${excludedFiles && excludedFiles.length > 0 ? `
  <div class="section-label dim-section-label">Excluded · ${excludedFiles.length}</div>
  <div class="dim-section-hint">These files are excluded and will not be installed</div>
  ${excludedHtml}
  ` : ''}

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

  // ── Usage page HTML ─────────────────────────────────────

  _getUsageHtml(entries, telemetryEnabled) {
    // Sort entries by date descending (most recent first)
    const sorted = [...entries].sort((a, b) => (b.date || '').localeCompare(a.date || ''));

    // Deduplicate by date (keep last entry per day — most recent timestamp)
    const byDate = new Map();
    for (const e of [...entries].sort((a, b) => (a.timestamp || '').localeCompare(b.timestamp || ''))) {
      byDate.set(e.date, e);
    }
    const daily = [...byDate.values()].sort((a, b) => (b.date || '').localeCompare(a.date || ''));

    // Latest entry summary
    const latest = daily[0];
    const entitlement = latest?.premium_entitlement;
    const remaining = latest?.premium_remaining;
    const used = latest?.premium_used;
    const resetDate = latest?.quota_reset_date_utc;

    const pctUsed = entitlement && used != null ? Math.round((used / entitlement) * 100) : null;

    // Sparkline data for last 30 days (usage values)
    const sparkData = daily.slice(0, 30).reverse();
    const maxUsed = Math.max(...sparkData.map((e) => e.premium_used ?? 0), 1);

    // Table rows
    const rows = daily.map((e) => {
      const pct = e.premium_entitlement && e.premium_used != null
        ? Math.round((e.premium_used / e.premium_entitlement) * 100)
        : '—';
      return `
        <tr>
          <td>${esc(e.date || '—')}</td>
          <td class="num">${e.premium_used ?? '—'}</td>
          <td class="num">${e.premium_remaining ?? '—'}</td>
          <td class="num">${e.premium_entitlement ?? '—'}</td>
          <td class="num">${typeof pct === 'number' ? pct + '%' : pct}</td>
        </tr>`;
    }).join('');

    // Colour for usage percentage
    const usageColor = pctUsed == null ? 'inherit'
      : pctUsed >= 90 ? 'var(--vscode-errorForeground, #f48771)'
      : pctUsed >= 70 ? 'var(--vscode-editorWarning-foreground, #cca700)'
      : 'var(--vscode-testing-iconPassed, #73c991)';

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

    /* ── Header / nav ── */
    .usage-header {
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 0 4px;
    }
    .usage-header h2 {
      font-size: 14px;
      font-weight: 600;
      flex: 1;
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

    hr {
      border: none;
      border-top: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, #333));
      margin: 2px 0 8px;
    }

    /* ── Summary card ── */
    .summary-card {
      background: var(--vscode-editor-background);
      border: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, #333));
      border-radius: 6px;
      padding: 12px;
      margin-bottom: 12px;
    }
    .summary-card .title {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      opacity: 0.6;
      margin-bottom: 6px;
      font-weight: 600;
    }
    .summary-card .big-number {
      font-size: 28px;
      font-weight: 700;
      line-height: 1.1;
    }
    .summary-card .detail {
      font-size: 12px;
      opacity: 0.7;
      margin-top: 4px;
    }
    .summary-row {
      display: flex;
      gap: 10px;
    }
    .summary-row .summary-card {
      flex: 1;
      min-width: 0;
    }

    /* ── Progress bar ── */
    .progress-bar-container {
      background: var(--vscode-progressBar-background, #333);
      border-radius: 4px;
      height: 8px;
      margin-top: 8px;
      overflow: hidden;
      opacity: 0.6;
    }
    .progress-bar-fill {
      height: 100%;
      border-radius: 4px;
      transition: width 0.3s;
    }

    /* ── Sparkline ── */
    .sparkline-container {
      margin: 8px 0 12px;
      display: flex;
      align-items: flex-end;
      gap: 2px;
      height: 40px;
    }
    .spark-bar {
      flex: 1;
      min-width: 3px;
      max-width: 12px;
      background: var(--vscode-button-background);
      border-radius: 2px 2px 0 0;
      opacity: 0.7;
      position: relative;
    }
    .spark-bar:hover {
      opacity: 1;
    }
    .spark-bar:hover::after {
      content: attr(data-label);
      position: absolute;
      bottom: 100%;
      left: 50%;
      transform: translateX(-50%);
      background: var(--vscode-editorWidget-background);
      border: 1px solid var(--vscode-widget-border);
      padding: 2px 6px;
      border-radius: 3px;
      font-size: 10px;
      white-space: nowrap;
      z-index: 10;
    }

    /* ── Table ── */
    .section-label {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      opacity: 0.6;
      margin: 10px 0 4px;
      font-weight: 600;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 12px;
    }
    th, td {
      padding: 4px 6px;
      text-align: left;
      border-bottom: 1px solid var(--vscode-widget-border, var(--vscode-panel-border, #222));
    }
    th {
      font-weight: 600;
      font-size: 11px;
      opacity: 0.7;
      text-transform: uppercase;
      letter-spacing: 0.3px;
    }
    td.num { text-align: right; font-variant-numeric: tabular-nums; }
    th.num { text-align: right; }
    tr:hover { background: var(--vscode-list-hoverBackground); }

    .empty {
      opacity: 0.5;
      font-style: italic;
      padding: 16px 0;
      font-size: 12px;
      text-align: center;
    }
  </style>
</head>
<body>
  <div class="usage-header">
    <h2>📊 Copilot Usage</h2>
  </div>

  <div class="button-row">
    <button class="secondary" onclick="send('backToMain')">← Back</button>
    <button class="primary" onclick="send('refreshUsage')" ${telemetryEnabled ? '' : 'disabled'}>↻ Refresh</button>
    <button class="secondary" onclick="send('openLogFile')">📄 Open Log</button>
    <button class="secondary" onclick="send('toggleTelemetry', { enabled: ${!telemetryEnabled} })">${telemetryEnabled ? '⏸ Disable' : '▶ Enable'} Telemetry</button>
  </div>

  ${!telemetryEnabled ? '<div class="empty">Telemetry is disabled. Click Enable to start tracking Copilot usage.</div><hr>' : '<hr>'}

  ${latest ? `
  <div class="summary-row">
    <div class="summary-card">
      <div class="title">Used Today</div>
      <div class="big-number" style="color: ${usageColor}">${used ?? '—'}</div>
      <div class="detail">of ${entitlement ?? '?'} premium requests</div>
      ${pctUsed != null ? `
      <div class="progress-bar-container">
        <div class="progress-bar-fill" style="width: ${Math.min(pctUsed, 100)}%; background: ${usageColor};"></div>
      </div>` : ''}
    </div>
    <div class="summary-card">
      <div class="title">Remaining</div>
      <div class="big-number">${remaining ?? '—'}</div>
      <div class="detail">${resetDate ? 'Resets ' + esc(resetDate.split('T')[0]) : ''}</div>
    </div>
  </div>
  ` : '<div class="empty">No telemetry data yet. Click Refresh to log now.</div>'}

  ${sparkData.length > 1 ? `
  <div class="section-label">Last ${sparkData.length} days</div>
  <div class="sparkline-container">
    ${sparkData.map((e) => {
      const h = e.premium_used != null ? Math.max(2, Math.round((e.premium_used / maxUsed) * 40)) : 2;
      const label = `${e.date}: ${e.premium_used ?? 0} used`;
      return `<div class="spark-bar" style="height: ${h}px" data-label="${esc(label)}"></div>`;
    }).join('')}
  </div>
  ` : ''}

  ${daily.length > 0 ? `
  <div class="section-label">Daily Log · ${daily.length} entries</div>
  <table>
    <thead>
      <tr>
        <th>Date</th>
        <th class="num">Used</th>
        <th class="num">Left</th>
        <th class="num">Quota</th>
        <th class="num">%</th>
      </tr>
    </thead>
    <tbody>
      ${rows}
    </tbody>
  </table>
  ` : ''}

  <script>
    const vscode = acquireVsCodeApi();
    function send(command, data) {
      vscode.postMessage({ command, ...data });
    }
  </script>
</body>
</html>`;
  }

  // ── Settings page HTML ─────────────────────────────────

  _getSettingsHtml(state, globalCfg, projectCfg) {
    const resolved = state?.config || {};
    const global = globalCfg || {};
    const project = projectCfg || {};
    const tiers = state?.tiers || [];
    const telemetryEnabled = resolved.telemetryEnabled === true;

    const manifestLabel = resolved.manifest || '—';
    const tierLabel = (tiers.find((t) => t.id === resolved.tier) || {}).label || resolved.tier || '—';
    const repoLabel = resolved.repo || 'peregrine-digital/activate-framework';
    const branchLabel = resolved.branch || 'main';

    // Helper: show where a value comes from
    const source = (field) => {
      if (project[field] != null && project[field] !== '') return 'project';
      if (global[field] != null && global[field] !== '') return 'global';
      return 'default';
    };

    const manifestSrc = source('manifest');
    const tierSrc = source('tier');
    const repoSrc = source('repo');
    const branchSrc = source('branch');
    const telemetrySrc = project.telemetryEnabled != null ? 'project'
      : global.telemetryEnabled != null ? 'global' : 'default';

    const srcBadge = (s) => `<span class="source-badge ${s}">${s}</span>`;

    return `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <style>
    body {
      font-family: var(--vscode-font-family);
      font-size: var(--vscode-font-size);
      color: var(--vscode-foreground);
      padding: 0 12px 20px 12px;
      line-height: 1.5;
    }
    h2 { font-size: 14px; margin: 12px 0 8px; }
    hr {
      border: none;
      border-top: 1px solid var(--vscode-panel-border, rgba(128,128,128,0.2));
      margin: 10px 0;
    }
    .button-row {
      display: flex;
      gap: 6px;
      margin: 8px 0;
      flex-wrap: wrap;
    }
    button {
      font-family: inherit;
      font-size: 11px;
      padding: 4px 10px;
      border-radius: 3px;
      cursor: pointer;
      border: 1px solid var(--vscode-button-border, transparent);
    }
    .primary {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
    }
    .secondary {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    .setting-row {
      display: flex;
      align-items: center;
      justify-content: space-between;
      padding: 6px 0;
      border-bottom: 1px solid var(--vscode-panel-border, rgba(128,128,128,0.1));
    }
    .setting-label {
      font-size: 12px;
      font-weight: 600;
    }
    .setting-value {
      font-size: 12px;
      display: flex;
      align-items: center;
      gap: 6px;
    }
    .source-badge {
      font-size: 10px;
      padding: 1px 5px;
      border-radius: 3px;
      opacity: 0.8;
    }
    .source-badge.project {
      background: var(--vscode-badge-background);
      color: var(--vscode-badge-foreground);
    }
    .source-badge.global {
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    .source-badge.default {
      opacity: 0.4;
      font-style: italic;
    }
    .toggle-btn {
      font-size: 11px;
      padding: 2px 8px;
      border-radius: 3px;
      cursor: pointer;
      border: 1px solid var(--vscode-button-border, transparent);
      background: var(--vscode-button-secondaryBackground);
      color: var(--vscode-button-secondaryForeground);
    }
    .toggle-btn.active {
      background: var(--vscode-button-background);
      color: var(--vscode-button-foreground);
    }
    .section-label {
      font-size: 11px;
      text-transform: uppercase;
      letter-spacing: 0.5px;
      opacity: 0.6;
      margin: 14px 0 4px;
    }
    .path-display {
      font-size: 11px;
      opacity: 0.5;
      word-break: break-all;
      padding: 2px 0;
    }
  </style>
</head>
<body>
  <div class="button-row">
    <button class="secondary" onclick="send('backToMain')">← Back</button>
    <h2 style="margin: 0; flex: 1;">⚙ Settings</h2>
  </div>

  <hr>

  <div class="section-label">Configuration</div>

  <div class="setting-row">
    <span class="setting-label">Manifest</span>
    <span class="setting-value">
      ${esc(manifestLabel)} ${srcBadge(manifestSrc)}
    </span>
  </div>

  <div class="setting-row">
    <span class="setting-label">Tier</span>
    <span class="setting-value">
      ${esc(tierLabel)} ${srcBadge(tierSrc)}
    </span>
  </div>

  <div class="setting-row">
    <span class="setting-label">Repository</span>
    <span class="setting-value">
      ${esc(repoLabel)}
      <button class="toggle-btn" onclick="send('editRepo', { current: '${esc(resolved.repo || '')}', scope: 'project' })" title="Change repository">✎</button>
      ${srcBadge(repoSrc)}
    </span>
  </div>

  <div class="setting-row">
    <span class="setting-label">Branch</span>
    <span class="setting-value">
      ${esc(branchLabel)}
      <button class="toggle-btn" onclick="send('editBranch', { current: '${esc(resolved.branch || '')}', scope: 'project' })" title="Change branch">✎</button>
      ${srcBadge(branchSrc)}
    </span>
  </div>

  <div class="setting-row">
    <span class="setting-label">Telemetry</span>
    <span class="setting-value">
      <button class="toggle-btn ${telemetryEnabled ? 'active' : ''}"
        onclick="send('toggleTelemetry', { enabled: ${!telemetryEnabled} })">
        ${telemetryEnabled ? '● Enabled' : '○ Disabled'}
      </button>
      ${srcBadge(telemetrySrc)}
    </span>
  </div>

  <hr>

  <div class="section-label">Global Defaults</div>
  <div class="path-display">${esc(state?.projectDir ? '~/.activate/config.json' : '')}</div>

  <div class="setting-row">
    <span class="setting-label">Manifest</span>
    <span class="setting-value">${esc(global.manifest || '(not set)')}</span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Tier</span>
    <span class="setting-value">${esc(global.tier || '(not set)')}</span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Repository</span>
    <span class="setting-value">${esc(global.repo || '(not set)')}</span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Branch</span>
    <span class="setting-value">${esc(global.branch || '(not set)')}</span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Telemetry</span>
    <span class="setting-value">${global.telemetryEnabled === true ? 'Enabled' : global.telemetryEnabled === false ? 'Disabled' : '(not set)'}</span>
  </div>

  <hr>

  <div class="section-label">Project Overrides</div>
  <div class="path-display">~/.activate/repos/&lt;hash&gt;/config.json</div>

  <div class="setting-row">
    <span class="setting-label">Manifest</span>
    <span class="setting-value">
      ${esc(project.manifest || '(not set)')}
      ${project.manifest ? `<button class="toggle-btn" onclick="send('clearProjectOverride', { updates: { manifest: '__clear__' } })">✕</button>` : ''}
    </span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Tier</span>
    <span class="setting-value">
      ${esc(project.tier || '(not set)')}
      ${project.tier ? `<button class="toggle-btn" onclick="send('clearProjectOverride', { updates: { tier: '__clear__' } })">✕</button>` : ''}
    </span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Repository</span>
    <span class="setting-value">
      ${esc(project.repo || '(not set)')}
      ${project.repo ? `<button class="toggle-btn" onclick="send('clearProjectOverride', { updates: { repo: '__clear__' } })">✕</button>` : ''}
    </span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Branch</span>
    <span class="setting-value">
      ${esc(project.branch || '(not set)')}
      ${project.branch ? `<button class="toggle-btn" onclick="send('clearProjectOverride', { updates: { branch: '__clear__' } })">✕</button>` : ''}
    </span>
  </div>

  ${Object.keys(project.fileOverrides || {}).length > 0 ? `
  <div class="setting-row">
    <span class="setting-label">File Overrides</span>
    <span class="setting-value">${Object.keys(project.fileOverrides).length} file(s)</span>
  </div>
  ` : ''}

  ${Object.keys(project.skippedVersions || {}).length > 0 ? `
  <div class="setting-row">
    <span class="setting-label">Skipped Updates</span>
    <span class="setting-value">${Object.keys(project.skippedVersions).length} file(s)</span>
  </div>
  ` : ''}

  <hr>

  <div class="section-label">Updates</div>
  <div class="setting-row">
    <span class="setting-label">CLI Version</span>
    <span class="setting-value">${esc(this._client?.serverVersion || '—')}</span>
  </div>
  <div class="setting-row">
    <span class="setting-label">Extension Version</span>
    <span class="setting-value">${esc(this._extensionVersion || '—')}</span>
  </div>
  ${this._lastUpdateCheck ? `
  <div class="setting-row">
    <span class="setting-label">Last Checked</span>
    <span class="setting-value last-checked">${esc(formatTimestamp(this._lastUpdateCheck))}</span>
  </div>
  ` : ''}
  <div style="padding: 4px 0;">
    <button class="primary" onclick="send('checkForUpdates')">🔄 Check for Updates</button>
  </div>

  <script>
    const vscode = acquireVsCodeApi();
    function send(command, data) {
      vscode.postMessage({ command, ...data });
    }
  </script>
</body>
</html>`;
  }
}

/** Use daemon-provided displayName, or fall back to dest path */
function displayName(f) {
  return f.displayName || f.dest.split('/').pop().replace(/\.md$/, '');
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

/** Format an ISO timestamp as a human-readable relative/absolute string. */
function formatTimestamp(iso) {
  if (!iso) return '';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return String(iso);
  const now = Date.now();
  const diffMs = now - d.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

// ── Tier helpers (dynamic — reads from daemon state) ───────────

// ── Category grouping (uses daemon-provided categories) ────────

/** Default category icons (presentation only). */
const CATEGORY_ICONS_DEFAULT = {
  instructions: '📝',
  prompts: '💬',
  skills: '🛠',
  agents: '🤖',
  'mcp-servers': '🔌',
  other: '📄',
};

/**
 * Group files by their `category` field.
 * @param {object[]} files - FileStatus items with `category`
 * @param {object[]} categories - [{id, label}] from daemon state
 */
function groupByCategory(files, categories) {
  const groups = {};
  for (const f of files) {
    const cat = f.category || 'other';
    if (!groups[cat]) groups[cat] = [];
    groups[cat].push(f);
  }

  // Use daemon category order if available, otherwise natural order
  const order = categories && categories.length > 0
    ? categories.map((c) => c.id)
    : Object.keys(groups);

  const labelMap = {};
  if (categories) {
    for (const c of categories) {
      labelMap[c.id] = c.label;
    }
  }

  const result = [];
  for (const cat of order) {
    if (groups[cat]) {
      result.push({
        category: cat,
        label: labelMap[cat] || cat,
        files: groups[cat],
      });
    }
  }
  // Include any categories not in the daemon list
  for (const cat of Object.keys(groups)) {
    if (!result.some((g) => g.category === cat)) {
      result.push({ category: cat, label: cat, files: groups[cat] });
    }
  }
  return result;
}

module.exports = { ControlPanelProvider };
