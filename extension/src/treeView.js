const vscode = require('vscode');
const { listByCategory, selectFiles, CATEGORY_ORDER } = require('./manifest');
const { readBundledManifest, readInstalledVersion, getActivateRoot, findActivateWorkspaceFolder } = require('./installer');

const CATEGORY_ICONS = {
  instructions: 'book',
  prompts: 'comment-discussion',
  skills: 'tools',
  agents: 'hubot',
  other: 'file',
};

/**
 * TreeDataProvider for the Peregrine Activate sidebar.
 *
 * Tree structure:
 *   Status (root item — collapsible)
 *     Version: 0.5.0
 *     Tier: standard
 *     Workspace root: active
 *   Instructions (4)
 *     general — Universal coding conventions...
 *     security — Security guardrails...
 *   Prompts (3)
 *     ...
 *   Skills (5)
 *     ...
 *   Agents (1)
 *     ...
 *   ── Higher tier ──
 *     ...
 */
class ActivateTreeProvider {
  constructor(context) {
    this._context = context;
    this._onDidChangeTreeData = new vscode.EventEmitter();
    this.onDidChangeTreeData = this._onDidChangeTreeData.event;
    this._manifest = null;
  }

  refresh() {
    this._manifest = null;
    this._onDidChangeTreeData.fire();
  }

  async _getManifest() {
    if (!this._manifest) {
      try {
        this._manifest = await readBundledManifest(this._context);
      } catch {
        this._manifest = { files: [] };
      }
    }
    return this._manifest;
  }

  getTreeItem(element) {
    return element;
  }

  async getChildren(element) {
    // Root level — return status + category groups + higher tier section
    if (!element) {
      return this._getRootItems();
    }

    // Children of a category group
    if (element.contextValue === 'category') {
      return element.fileItems || [];
    }

    // Children of status
    if (element.contextValue === 'status') {
      return element.statusItems || [];
    }

    // Children of higher-tier section
    if (element.contextValue === 'higher-tier') {
      return element.fileItems || [];
    }

    return [];
  }

  async _getRootItems() {
    const manifest = await this._getManifest();
    const config = vscode.workspace.getConfiguration('activate-framework');
    const tier = config.get('defaultTier', 'standard');
    const version = this._context.extension.packageJSON.version ?? 'unknown';
    const installedVersion = await readInstalledVersion(this._context);
    const isActive = !!findActivateWorkspaceFolder();
    const root = getActivateRoot(this._context);

    const items = [];

    // Status section
    const statusItem = new vscode.TreeItem('Status', vscode.TreeItemCollapsibleState.Collapsed);
    statusItem.iconPath = new vscode.ThemeIcon('info');
    statusItem.contextValue = 'status';
    statusItem.statusItems = [
      this._infoItem('Version', installedVersion || version),
      this._infoItem('Tier', tier),
      this._infoItem('Workspace root', isActive ? 'active' : 'not active'),
      this._infoItem('Storage', root.fsPath),
    ];
    items.push(statusItem);

    // Installed file groups
    const groups = listByCategory(manifest.files, { tier });
    const installed = new Set();
    for (const f of selectFiles(manifest.files, tier)) {
      const fileUri = vscode.Uri.joinPath(root, '.github', f.dest);
      try {
        await vscode.workspace.fs.stat(fileUri);
        installed.add(f.dest);
      } catch {}
    }

    for (const { category, label, files } of groups) {
      const catItem = new vscode.TreeItem(
        `${label} (${files.length})`,
        vscode.TreeItemCollapsibleState.Expanded,
      );
      catItem.iconPath = new vscode.ThemeIcon(CATEGORY_ICONS[category] || 'file');
      catItem.contextValue = 'category';
      catItem.fileItems = files.map((f) => this._fileItem(f, installed.has(f.dest), root));
      items.push(catItem);
    }

    // Higher-tier files
    const currentFiles = selectFiles(manifest.files, tier);
    const currentDests = new Set(currentFiles.map((f) => f.dest));
    const higherFiles = manifest.files.filter((f) => !currentDests.has(f.dest));

    if (higherFiles.length > 0) {
      const higherItem = new vscode.TreeItem(
        `Available at higher tier (${higherFiles.length})`,
        vscode.TreeItemCollapsibleState.Collapsed,
      );
      higherItem.iconPath = new vscode.ThemeIcon('lock');
      higherItem.contextValue = 'higher-tier';
      higherItem.fileItems = higherFiles.map((f) => {
        const item = new vscode.TreeItem(this._displayName(f));
        item.description = `[${f.tier}]`;
        item.tooltip = f.description || f.dest;
        item.iconPath = new vscode.ThemeIcon('circle-outline');
        item.contextValue = 'locked-file';
        return item;
      });
      items.push(higherItem);
    }

    return items;
  }

  _fileItem(f, isSynced, root) {
    const item = new vscode.TreeItem(this._displayName(f));
    item.description = f.description || '';
    item.tooltip = new vscode.MarkdownString(
      `**${f.dest}**\n\n${f.description || ''}\n\nTier: \`${f.tier}\` | Synced: ${isSynced ? 'Yes' : 'No'}`,
    );
    item.iconPath = new vscode.ThemeIcon(isSynced ? 'check' : 'circle-outline');
    item.contextValue = isSynced ? 'synced-file' : 'missing-file';

    if (isSynced) {
      const fileUri = vscode.Uri.joinPath(root, '.github', f.dest);
      item.command = {
        command: 'vscode.open',
        title: 'Open file',
        arguments: [fileUri],
      };
    }

    return item;
  }

  _infoItem(label, value) {
    const item = new vscode.TreeItem(`${label}: ${value}`);
    item.contextValue = 'info';
    return item;
  }

  _displayName(f) {
    const parts = f.dest.split('/');
    const filename = parts[parts.length - 1];
    // For skills, use the directory name instead of SKILL.md
    if (filename === 'SKILL.md' && parts.length >= 2) {
      return parts[parts.length - 2];
    }
    return filename
      .replace(/\.(instructions|prompt|agent)\.md$/, '')
      .replace(/\.md$/, '');
  }
}

module.exports = { ActivateTreeProvider };
