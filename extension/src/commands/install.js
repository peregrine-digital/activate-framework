const vscode = require('vscode');
const { selectFiles } = require('../manifest');
const { readBundledManifest, readBundledVersion, installFiles } = require('../installer');

async function installCommand(context) {
  // 1. Require an open workspace
  const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
  if (!workspaceFolder) {
    vscode.window.showErrorMessage('Peregrine Activate: Open a workspace folder first.');
    return;
  }

  // 2. Pick tier
  const config = vscode.workspace.getConfiguration('activate-framework');
  const defaultTier = config.get('defaultTier', 'standard');

  const tierItems = [
    {
      label: 'minimal',
      description: 'Core workflow guidance (AGENTS.md, instructions, prompts)',
      picked: defaultTier === 'minimal',
    },
    {
      label: 'standard',
      description: 'Core + ad-hoc instructions, skills, and agents',
      picked: defaultTier === 'standard',
    },
    {
      label: 'advanced',
      description: 'Standard + advanced tooling',
      picked: defaultTier === 'advanced',
    },
  ];

  const tierPick = await vscode.window.showQuickPick(tierItems, {
    placeHolder: 'Select installation tier',
    title: 'Peregrine Activate — Installation Tier',
  });
  if (!tierPick) return;

  const tier = tierPick.label;

  // 3. Pick target subdirectory
  const targetSubdir = config.get('targetSubdir', '.github');

  const targetItems = [
    {
      label: '.github',
      description: 'Standard location for Copilot customization files',
      picked: targetSubdir === '.github',
    },
    {
      label: '.copilot',
      description: 'Alternative Copilot configuration directory',
      picked: targetSubdir === '.copilot',
    },
    {
      label: 'Custom…',
      description: 'Choose a custom subdirectory',
    },
  ];

  const targetPick = await vscode.window.showQuickPick(targetItems, {
    placeHolder: 'Where should files be installed?',
    title: 'Peregrine Activate — Target Directory',
  });
  if (!targetPick) return;

  let finalTarget = targetPick.label;
  if (targetPick.label === 'Custom…') {
    const custom = await vscode.window.showInputBox({
      prompt: 'Enter subdirectory relative to workspace root',
      value: '.github',
      validateInput: (v) => (v.trim() ? null : 'Cannot be empty'),
    });
    if (!custom) return;
    finalTarget = custom.trim();
  }

  // 4. Read manifest and select files
  const manifest = await readBundledManifest(context);
  const version = await readBundledVersion(context);
  const files = selectFiles(manifest.files, tier);

  // 5. Confirm
  const confirm = await vscode.window.showInformationMessage(
    `Install ${files.length} files (${tier} tier) into ${workspaceFolder.name}/${finalTarget}?`,
    { modal: true },
    'Install',
  );
  if (confirm !== 'Install') return;

  // 6. Install with progress
  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Peregrine Activate',
      cancellable: false,
    },
    async (progress) => {
      progress.report({ message: `Installing ${files.length} files…` });

      const result = await installFiles(context, workspaceFolder.uri, finalTarget, files, version);

      const msg = `Peregrine Activate ${result.version} (${tier}) — ${result.installed.length} files installed.`;
      vscode.window.showInformationMessage(msg);

      // Log details to output channel
      const channel = vscode.window.createOutputChannel('Peregrine Activate');
      channel.appendLine(msg);
      result.installed.forEach((f) => channel.appendLine(`  ✓ ${f}`));
      channel.show(true);
    },
  );
}

module.exports = { installCommand };
