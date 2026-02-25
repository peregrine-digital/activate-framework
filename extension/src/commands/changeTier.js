const vscode = require('vscode');
const { TIER_MAP } = require('../manifest');
const { syncFiles, addWorkspaceRoot, readInstalledVersion } = require('../installer');

async function changeTierCommand(context) {
  const config = vscode.workspace.getConfiguration('activate-framework');
  const currentTier = config.get('defaultTier', 'standard');

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

      // Update the setting
      await config.update('defaultTier', newTier, vscode.ConfigurationTarget.Global);

      // Preserve active manifest when switching tiers
      const installedInfo = await readInstalledVersion(context);
      const manifestId = installedInfo?.manifest || undefined;

      // Re-sync files for the new tier
      const result = await syncFiles(context, newTier, manifestId);
      addWorkspaceRoot(context);

      vscode.window.showInformationMessage(
        `Peregrine Activate switched to ${newTier} tier (${result.installed.length} files).`,
      );
    },
  );
}

module.exports = { changeTierCommand };
