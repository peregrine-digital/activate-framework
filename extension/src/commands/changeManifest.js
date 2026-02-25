const vscode = require('vscode');
const { discoverBundledManifests } = require('../installer');
const { injectFiles, readInjectedVersion } = require('../injector');
const { resolveConfig, writeProjectConfig } = require('../config');

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

  const cfg = await resolveConfig();
  const injectedInfo = await readInjectedVersion();
  const activeId = cfg.manifest || injectedInfo?.manifest || manifests[0].id;

  const items = manifests.map((m) => ({
    label: m.name,
    description: `v${m.version} · ${m.files.length} files`,
    detail: m.description || undefined,
    manifestId: m.id,
  }));

  const pick = await new Promise((resolve) => {
    const qp = vscode.window.createQuickPick();
    qp.items = items;
    qp.title = 'Switch Manifest';
    qp.placeholder = 'Select a manifest';
    // Pre-select the active manifest
    const active = items.find((i) => i.manifestId === activeId);
    if (active) qp.activeItems = [active];
    qp.onDidAccept(() => { resolve(qp.selectedItems[0]); qp.dispose(); });
    qp.onDidHide(() => { resolve(undefined); qp.dispose(); });
    qp.show();
  });

  if (!pick || pick.manifestId === activeId) return false;

  // Persist the manifest choice
  await writeProjectConfig({ manifest: pick.manifestId });

  await injectFiles(context, cfg.tier, pick.manifestId);

  vscode.window.showInformationMessage(
    `Switched to ${pick.label} (${cfg.tier}).`,
  );
  return true;
}

module.exports = { changeManifestCommand };
