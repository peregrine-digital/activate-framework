const vscode = require('vscode');
const { readInstalledVersion, readBundledManifest, getActivateRoot, findActivateWorkspaceFolder } = require('../installer');
const { listByCategory, selectFiles } = require('../manifest');

async function showStatusCommand(context) {
  const installedVersion = await readInstalledVersion(context);
  const bundledVersion = context.extension.packageJSON.version ?? 'unknown';
  const config = vscode.workspace.getConfiguration('activate-framework');
  const tier = config.get('defaultTier', 'standard');
  const root = getActivateRoot(context);
  const isActive = !!findActivateWorkspaceFolder();

  const manifest = await readBundledManifest(context);

  // Check which files exist in the managed root
  const installed = new Set();
  for (const f of manifest.files) {
    const fileUri = vscode.Uri.joinPath(root, '.github', f.dest);
    try {
      await vscode.workspace.fs.stat(fileUri);
      installed.add(f.dest);
    } catch {}
  }

  const channel = vscode.window.createOutputChannel('Peregrine Activate');
  channel.clear();
  channel.appendLine('Peregrine Activate — Status');
  channel.appendLine('═'.repeat(40));
  channel.appendLine(`Bundled version: ${bundledVersion}`);
  channel.appendLine(`Synced version:  ${installedVersion ?? 'not synced'}`);
  channel.appendLine(`Tier:            ${tier}`);
  channel.appendLine(`Workspace root:  ${isActive ? 'active' : 'not active'}`);
  channel.appendLine(`Storage:         ${root.fsPath}`);

  // Show synced files grouped by category
  const groups = listByCategory(manifest.files, { tier });
  for (const { label, files } of groups) {
    channel.appendLine('');
    channel.appendLine(`${label} (${files.length})`);
    channel.appendLine('─'.repeat(40));
    for (const f of files) {
      const check = installed.has(f.dest) ? '✓' : '○';
      const desc = f.description ? `  ${f.description}` : '';
      channel.appendLine(`  ${check} ${f.dest}`);
      if (desc) channel.appendLine(`   ${desc}`);
    }
  }

  // Show files available at higher tiers
  const allFiles = manifest.files;
  const currentFiles = selectFiles(allFiles, tier);
  const currentDests = new Set(currentFiles.map((f) => f.dest));
  const higherTierFiles = allFiles.filter((f) => !currentDests.has(f.dest));

  if (higherTierFiles.length > 0) {
    const higherGroups = listByCategory(higherTierFiles);
    channel.appendLine('');
    channel.appendLine(`Available at higher tier (${higherTierFiles.length})`);
    channel.appendLine('═'.repeat(40));
    for (const { label, files } of higherGroups) {
      channel.appendLine(`  ${label}:`);
      for (const f of files) {
        channel.appendLine(`    ○ ${f.dest} [${f.tier}]`);
        if (f.description) channel.appendLine(`      ${f.description}`);
      }
    }
  }

  channel.show(true);
}

module.exports = { showStatusCommand };
