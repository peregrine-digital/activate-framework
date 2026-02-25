const vscode = require('vscode');
const { discoverAvailableTiers, TIER_LABELS } = require('../manifest');
const { injectFiles, readInjectedVersion } = require('../injector');
const { resolveConfig, writeProjectConfig } = require('../config');
const { readBundledManifestById } = require('../installer');

async function changeTierCommand(context) {
  const cfg = await resolveConfig();
  const currentTier = cfg.tier;

  // Get the current manifest to discover available tiers
  const injectedInfo = await readInjectedVersion();
  const manifestId = cfg.manifest || injectedInfo?.manifest || 'activate-framework';

  let availableTiers;
  try {
    const manifest = await readBundledManifestById(context, manifestId);
    availableTiers = discoverAvailableTiers(manifest.files, manifest.tiers);
  } catch {
    // Fallback to default tiers if manifest can't be read
    availableTiers = [
      { id: 'minimal', label: TIER_LABELS.minimal },
      { id: 'standard', label: TIER_LABELS.standard },
      { id: 'advanced', label: TIER_LABELS.advanced },
    ];
  }

  const tierItems = availableTiers.map((tier) => ({
    label: tier.label,
    description: tier.id === currentTier ? '(current)' : '',
    tierId: tier.id,
  }));

  const pick = await new Promise((resolve) => {
    const qp = vscode.window.createQuickPick();
    qp.items = tierItems;
    qp.title = 'Peregrine Activate — Change Tier';
    qp.placeholder = `Current tier: ${TIER_LABELS[currentTier] || currentTier}`;
    // Pre-select the current tier
    const active = tierItems.find((i) => i.tierId === currentTier);
    if (active) qp.activeItems = [active];
    qp.onDidAccept(() => { resolve(qp.selectedItems[0]); qp.dispose(); });
    qp.onDidHide(() => { resolve(undefined); qp.dispose(); });
    qp.show();
  });
  if (!pick || pick.tierId === currentTier) return;

  const newTier = pick.tierId;
  const newTierLabel = pick.label;

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Peregrine Activate',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: `Switching to ${newTierLabel} tier…` });

      // Persist the tier choice
      await writeProjectConfig({ tier: newTier });

      const result = await injectFiles(context, newTier, manifestId);
      vscode.window.showInformationMessage(
        `Peregrine Activate switched to ${newTierLabel} tier (${result.injected.length} files).`,
      );
    },
  );
}

module.exports = { changeTierCommand };
