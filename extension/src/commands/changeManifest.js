const vscode = require('vscode');
const { discoverBundledManifests, syncFiles, readInstalledVersion } = require('../installer');

/**
 * Show a QuickPick to switch between available manifests.
 * Re-syncs installed files to the selected manifest.
 *
 * @param {vscode.ExtensionContext} context
 * @returns {Promise<boolean>} true if manifest was changed
 */
async function changeManifestCommand(context) {
  const manifests = await discoverBundledManifests(context);

  if (manifests.length === 0) {
    vscode.window.showWarningMessage('No manifests available.');
    return false;
  }

  if (manifests.length === 1) {
    vscode.window.showInformationMessage(`Only one manifest available: ${manifests[0].name}`);
    return false;
  }

  const installedInfo = await readInstalledVersion(context);
  const activeId = installedInfo?.manifest || manifests[0].id;

  const items = manifests.map((m) => ({
    label: m.name,
    description: `v${m.version} · ${m.files.length} files`,
    detail: m.description || undefined,
    picked: m.id === activeId,
    manifestId: m.id,
  }));

  const pick = await vscode.window.showQuickPick(items, {
    placeHolder: 'Select a manifest',
    title: 'Switch Manifest',
  });

  if (!pick || pick.manifestId === activeId) return false;

  const config = vscode.workspace.getConfiguration('activate-framework');
  const tier = config.get('defaultTier', 'standard');

  await syncFiles(context, tier, pick.manifestId);

  vscode.window.showInformationMessage(
    `Switched to ${pick.label} (${tier}).`,
  );
  return true;
}

module.exports = { changeManifestCommand };
