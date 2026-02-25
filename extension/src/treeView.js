const vscode = require('vscode');
const { listByCategory } = require('./manifest');
const { discoverBundledManifests, readBundledManifestById, readInstalledVersion, getActivateRoot, isFileInstalled } = require('./installer');

const CATEGORY_ICONS = {
  instructions: 'book',
  prompts: 'comment-discussion',
  skills: 'tools',
  agents: 'hubot',
  other: 'file',
};

/**
 * TreeDataProvider for the Peregrine Activate sidebar file browser.
 *
 * Status/actions live in the WebviewView control panel above.
 * This tree shows only Installed and Available file sections.
 *
 * Tree structure:
 *   ▸ Installed (15)
 *       ▸ Instructions (3)
 *           ✓ general              [🗑]
 *       ▸ Prompts (3)
 *       ...
 *   ▸ Available (7)
 *       ▸ Skills (4)
 *           ⬇ ato-compliant        [⬇]
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
        // Determine which manifest is active
        const installedInfo = await readInstalledVersion(this._context);
        const manifestId = installedInfo?.manifest || 'activate-framework';
        const chosen = await readBundledManifestById(this._context, manifestId);
        this._manifest = { files: chosen.files };
      } catch {
        // Fall back to discovering all and using the first
        try {
          const all = await discoverBundledManifests(this._context);
          this._manifest = all.length > 0 ? { files: all[0].files } : { files: [] };
        } catch {
          this._manifest = { files: [] };
        }
      }
    }
    return this._manifest;
  }

  getTreeItem(element) {
    return element;
  }

  async getChildren(element) {
    if (!element) return this._getRootSections();
    if (typeof element.getChildren === 'function') return element.getChildren();
    if (element.childItems) return element.childItems;
    return [];
  }

  async _getRootSections() {
    const manifest = await this._getManifest();
    const root = getActivateRoot(this._context);

    // Scan which files are actually on disk
    const installedSet = new Set();
    for (const f of manifest.files) {
      if (await isFileInstalled(this._context, f)) {
        installedSet.add(f.dest);
      }
    }

    const installedFiles = manifest.files.filter((f) => installedSet.has(f.dest));
    const availableFiles = manifest.files.filter((f) => !installedSet.has(f.dest));

    // ── Installed section ──
    const installedSection = this._section(
      `Installed`,
      'folder-opened',
      installedFiles.length > 0
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None,
    );
    installedSection.description = `${installedFiles.length} files`;
    installedSection.childItems = this._buildCategoryGroups(installedFiles, root, true);

    // ── Available section ──
    const availableSection = this._section(
      `Available`,
      'cloud',
      availableFiles.length > 0
        ? vscode.TreeItemCollapsibleState.Collapsed
        : vscode.TreeItemCollapsibleState.None,
    );
    availableSection.description = `${availableFiles.length} files`;
    availableSection.childItems = this._buildCategoryGroups(availableFiles, root, false);

    return [installedSection, availableSection];
  }

  _buildCategoryGroups(files, root, isInstalled) {
    if (files.length === 0) return [];

    const groups = listByCategory(files);
    return groups.map(({ category, label, files: groupFiles }) => {
      const catItem = new vscode.TreeItem(
        `${label}`,
        vscode.TreeItemCollapsibleState.Collapsed,
      );
      catItem.description = `${groupFiles.length}`;
      catItem.iconPath = new vscode.ThemeIcon(CATEGORY_ICONS[category] || 'file');
      catItem.contextValue = 'category';
      catItem.childItems = groupFiles.map((f) =>
        isInstalled ? this._installedFileItem(f, root) : this._availableFileItem(f),
      );
      return catItem;
    });
  }

  _installedFileItem(f, root) {
    const item = new vscode.TreeItem(this._displayName(f));
    item.description = f.description || '';
    item.tooltip = new vscode.MarkdownString(
      `**${f.dest}**\n\n${f.description || ''}\n\nTier: \`${f.tier}\`\n\n*Click to open*`,
    );
    item.iconPath = new vscode.ThemeIcon('pass-filled', new vscode.ThemeColor('testing.iconPassed'));
    item.contextValue = 'installed-file';
    item.fileData = f;

    const fileUri = vscode.Uri.joinPath(root, '.github', f.dest);
    item.command = {
      command: 'vscode.open',
      title: 'Open file',
      arguments: [fileUri],
    };

    return item;
  }

  _availableFileItem(f) {
    const item = new vscode.TreeItem(this._displayName(f));
    item.description = f.description || '';
    item.tooltip = new vscode.MarkdownString(
      `**${f.dest}**\n\n${f.description || ''}\n\nTier: \`${f.tier}\`\n\n*Click to install*`,
    );
    item.iconPath = new vscode.ThemeIcon('circle-outline');
    item.contextValue = 'available-file';
    item.fileData = f;

    item.command = {
      command: 'activate-framework.installFile',
      title: 'Install',
      arguments: [f],
    };

    return item;
  }

  _section(label, iconId, collapsibleState) {
    const item = new vscode.TreeItem(label, collapsibleState);
    item.iconPath = new vscode.ThemeIcon(iconId);
    item.contextValue = 'section';
    return item;
  }

  _displayName(f) {
    const parts = f.dest.split('/');
    const filename = parts[parts.length - 1];
    if (filename === 'SKILL.md' && parts.length >= 2) {
      return parts[parts.length - 2];
    }
    return filename
      .replace(/\.(instructions|prompt|agent)\.md$/, '')
      .replace(/\.md$/, '');
  }
}

module.exports = { ActivateTreeProvider };
