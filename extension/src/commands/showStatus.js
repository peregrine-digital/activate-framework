const vscode = require('vscode');
const { readInstalledVersion, discoverBundledManifests, readBundledManifestById, getActivateRoot } = require('../installer');
const { readInjectedVersion, getWorkspaceRoot } = require('../injector');
const { listByCategory, selectFiles } = require('../manifest');

async function showStatusCommand(context) {
  const config = vscode.workspace.getConfiguration('peregrine-activate');
  const tier = config.get('defaultTier', 'standard');

  const injectedInfo = await readInjectedVersion();
  const installedVersion = injectedInfo?.version || null;
  const activeManifestId = injectedInfo?.manifest || 'activate-framework';
  const isActive = !!injectedInfo;
  const wsRoot = getWorkspaceRoot();
  const storagePath = wsRoot ? `${wsRoot.fsPath}/.github/` : '(no workspace)';

  const bundledVersion = context.extension.packageJSON.version ?? 'unknown';

  // Load active manifest
  let chosen;
  try {
    chosen = await readBundledManifestById(context, activeManifestId);
  } catch {
    const all = await discoverBundledManifests(context);
    chosen = all[0];
  }

  if (!chosen) {
    vscode.window.showErrorMessage('No manifest found.');
    return;
  }

  // Check which files exist
  const installed = new Set();
  for (const f of chosen.files) {
    const wsRoot = getWorkspaceRoot();
    const fileUri = wsRoot ? vscode.Uri.joinPath(wsRoot, '.github', f.dest) : null;
    if (!fileUri) continue;
    try {
      await vscode.workspace.fs.stat(fileUri);
      installed.add(f.dest);
    } catch {}
  }

  // Discover all available manifests for the overview
  const allManifests = await discoverBundledManifests(context);

  const channel = vscode.window.createOutputChannel('Peregrine Activate');
  channel.clear();
  channel.appendLine('Peregrine Activate — Status');
  channel.appendLine('═'.repeat(40));
  channel.appendLine(`Bundled version: ${bundledVersion}`);
  channel.appendLine(`Synced version:  ${installedVersion ?? 'not synced'}`);
  channel.appendLine(`Active manifest: ${chosen.name} (${chosen.id})`);
  channel.appendLine(`Manifest version:${chosen.version}`);
  channel.appendLine(`Tier:            ${tier}`);
  channel.appendLine(`Active:          ${isActive ? 'yes' : 'no'}`);
  channel.appendLine(`Storage:         ${storagePath}`);

  // Show available manifests
  if (allManifests.length > 1) {
    channel.appendLine('');
    channel.appendLine(`Available manifests (${allManifests.length})`);
    channel.appendLine('─'.repeat(40));
    for (const m of allManifests) {
      const active = m.id === chosen.id ? ' ← active' : '';
      channel.appendLine(`  ${m.id} — ${m.name} v${m.version} (${m.files.length} files)${active}`);
      if (m.description) channel.appendLine(`    ${m.description}`);
    }
  }

  // Show synced files grouped by category
  const groups = listByCategory(chosen.files, { tier });
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
  const currentFiles = selectFiles(chosen.files, tier);
  const currentDests = new Set(currentFiles.map((f) => f.dest));
  const higherTierFiles = chosen.files.filter((f) => !currentDests.has(f.dest));

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
