const vscode = require('vscode');
const { TIER_MAP } = require('../manifest');
const { injectFiles, readInjectedVersion } = require('../injector');
const { resolveConfig, writeProjectConfig } = require('../config');

async function changeTierCommand(context) {
  const cfg = await resolveConfig();
  const currentTier = cfg.tier;

  const tierItems = Object.keys(TIER_MAP).map((tier) => ({
    label: tier,
    description: tier === currentTier ? '(current)' : '',
    picked: tier === currentTier,
  }));

  const pick = await vscode.window.showQuickPick(tierItems, {
    placeHolder: `Current tier: ${currentTier}`,
    title: 'Peregrine Activate — Change Tier',
  });
  if (!pick || pick.label === currentTier) return;

  const newTier = pick.label;

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Peregrine Activate',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: `Switching to ${newTier} tier…` });

      // Persist the tier choice
      await writeProjectConfig({ tier: newTier });

      const injectedInfo = await readInjectedVersion();
      const manifestId = cfg.manifest || injectedInfo?.manifest || undefined;
      const result = await injectFiles(context, newTier, manifestId);
      vscode.window.showInformationMessage(
        `Peregrine Activate switched to ${newTier} tier (${result.injected.length} files).`,
      );
    },
  );
}

module.exports = { changeTierCommand };
