import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import readline from 'node:readline';
import { fileURLToPath } from 'node:url';
import { selectFiles, TIER_MAP, discoverManifests, loadManifest, formatManifestList } from './core.mjs';
import {
  resolveConfig,
  writeProjectConfig,
  ensureGitExclude,
  PROJECT_CONFIG_FILENAME,
} from './config.mjs';

const ASSISTANT_TARGET_MAP = {
  'github-copilot': '~/.copilot',
  'vs-code': '~/.copilot',
};

function expandHome(target, homeDir = process.env.HOME) {
  return target.startsWith('~/') ? path.join(homeDir ?? '', target.slice(2)) : target;
}

export function resolveAssistant(rawChoice) {
  const choice = rawChoice.trim().toLowerCase();
  if (!choice || choice.includes('github') || choice.includes('copilot')) return 'github-copilot';
  if (choice.includes('vs') || choice.includes('code')) return 'vs-code';
  return 'github-copilot';
}

export function resolveTargetDir(rawTarget, { assistant, homeDir = process.env.HOME, cwd = process.cwd() } = {}) {
  const resolvedInput = rawTarget.trim();
  if (resolvedInput) return path.resolve(cwd, expandHome(resolvedInput, homeDir));
  return path.resolve(expandHome(ASSISTANT_TARGET_MAP[assistant] ?? ASSISTANT_TARGET_MAP['github-copilot'], homeDir));
}

export async function resolveBundleDir(startDir) {
  const { readdir } = await import('node:fs/promises');

  /**
   * Check a directory for manifests/ or legacy manifest.json.
   * Returns the directory if found, null otherwise.
   */
  async function probe(dir) {
    try {
      const entries = await readdir(path.join(dir, 'manifests'));
      if (entries.some((e) => e.endsWith('.json'))) return dir;
    } catch { /* no manifests/ dir here */ }
    try {
      await readFile(path.join(dir, 'manifest.json'));
      return dir;
    } catch { /* no manifest.json here */ }
    return null;
  }

  // 1. Check startDir itself
  const direct = await probe(startDir);
  if (direct) return direct;

  // 2. Walk up parent directories (e.g. manifests/ at repo root)
  let dir = path.dirname(startDir);
  const root = path.parse(dir).root;
  while (dir !== root) {
    const found = await probe(dir);
    if (found) return found;
    dir = path.dirname(dir);
  }

  // 3. Legacy: check nested plugins directory from startDir
  const pluginDir = path.join(startDir, 'plugins', 'activate-framework');
  const pluginResult = await probe(pluginDir);
  if (pluginResult) return pluginResult;

  throw new Error(`Could not locate manifests/ or manifest.json from ${startDir}`);
}

export async function installFiles({ files, bundleDir, basePath, targetDir, version, manifestId }) {
  const sourceDir = basePath || bundleDir;
  for (const f of files) {
    const src = path.join(sourceDir, f.src);
    const dest = path.join(targetDir, f.dest);
    await mkdir(path.dirname(dest), { recursive: true });
    await copyFile(src, dest);
    console.log(`  ✓  ${f.dest}`);
  }
  const versionFile = path.join(targetDir, '.github', '.activate-version');
  await mkdir(path.dirname(versionFile), { recursive: true });
  await writeFile(versionFile, JSON.stringify({ manifest: manifestId, version }, null, 2));
}

async function prompt(rl, question) {
  return new Promise((resolve) => rl.question(question, resolve));
}

function parseArgs(argv) {
  const args = { manifest: null, tier: null, target: null, assistant: null, list: false, help: false };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === '--manifest' && argv[i + 1]) { args.manifest = argv[++i]; continue; }
    if (argv[i] === '--tier' && argv[i + 1]) { args.tier = argv[++i]; continue; }
    if (argv[i] === '--target' && argv[i + 1]) { args.target = argv[++i]; continue; }
    if (argv[i] === '--assistant' && argv[i + 1]) { args.assistant = argv[++i]; continue; }
    if (argv[i] === '--list') { args.list = true; continue; }
    if (argv[i] === '--help' || argv[i] === '-h') { args.help = true; continue; }
  }
  return args;
}

async function main() {
  const args = parseArgs(process.argv);

  if (args.help) {
    console.log(`Usage: node install.mjs [options]

Options:
  --manifest <id>     Select manifest by id (skip interactive prompt)
  --tier <tier>       Select tier: minimal, standard, advanced (default: standard)
  --target <dir>      Target directory (default: ~/.copilot)
  --assistant <name>  Assistant type: github-copilot, vs-code
  --list              List available manifests and exit
  -h, --help          Show this help message
`);
    process.exit(0);
  }

  const bundleDir = await resolveBundleDir(path.dirname(fileURLToPath(import.meta.url)));
  const manifests = await discoverManifests(bundleDir);

  if (manifests.length === 0) {
    console.error('No manifests found.');
    process.exit(1);
  }

  // --list: show available manifests and exit
  if (args.list) {
    console.log('\nAvailable manifests:\n');
    console.log(formatManifestList(manifests));
    console.log();
    return;
  }

  // Read persisted config; CLI flags take highest precedence
  const targetDir_cwd = process.cwd();
  const cfg = await resolveConfig(targetDir_cwd, {
    manifest: args.manifest ?? undefined,
    tier: args.tier ?? undefined,
  });

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

  // ── Manifest selection ──
  let chosen;
  if (args.manifest || cfg.manifest) {
    const lookupId = args.manifest || cfg.manifest;
    chosen = manifests.find((m) => m.id === lookupId);
    if (!chosen && args.manifest) {
      console.error(`Unknown manifest: ${args.manifest}`);
      console.error(`Available: ${manifests.map((m) => m.id).join(', ')}`);
      rl.close();
      process.exit(1);
    }
  }

  if (!chosen && manifests.length === 1) {
    chosen = manifests[0];
  }

  if (!chosen) {
    console.log('\nAvailable manifests:\n');
    console.log(formatManifestList(manifests));
    console.log();
    const ids = manifests.map((m) => m.id);
    const rawManifest = (await prompt(rl, `Which manifest? [${ids.join('/')}] (default: ${ids[0]}): `)).trim() || ids[0];
    chosen = manifests.find((m) => m.id === rawManifest);
    if (!chosen) {
      console.error(`Unknown manifest: ${rawManifest}`);
      rl.close();
      process.exit(1);
    }
  }

  console.log(`\n${chosen.name} v${chosen.version} Installer\n`);

  // ── Tier selection ──
  let tier, assistant, targetDir;

  if (args.tier && args.target) {
    // Fully non-interactive
    tier = Object.keys(TIER_MAP).includes(args.tier) ? args.tier : 'standard';
    assistant = resolveAssistant(args.assistant || '');
    targetDir = resolveTargetDir(args.target, { assistant });
  } else {
    console.log('Tiers:');
    console.log('  minimal   — Core workflow guidance (AGENTS.md, instructions, prompts)');
    console.log('  standard  — Core + ad-hoc instructions, skills, and agents');
    console.log('  advanced  — Standard + advanced tooling\n');

    const rawAssistant = args.assistant || await prompt(rl, 'Assistant? [GitHub Copilot/VS Code] (default: GitHub Copilot): ');
    assistant = resolveAssistant(rawAssistant);
    const defaultTier = cfg.tier || 'standard';
    const rawTier = args.tier || (await prompt(rl, `Which tier? [minimal/standard/advanced] (default: ${defaultTier}): `)).trim() || defaultTier;
    tier = Object.keys(TIER_MAP).includes(rawTier) ? rawTier : 'standard';
    const rawTarget = args.target || await prompt(rl, 'Target directory? (default: ~/.copilot): ');
    targetDir = resolveTargetDir(rawTarget, { assistant });
  }

  rl.close();

  const files = selectFiles(chosen.files, tier);
  console.log(`\nInstalling ${files.length} files to ${targetDir}:\n`);
  await installFiles({ files, bundleDir, basePath: chosen.basePath, targetDir, version: chosen.version, manifestId: chosen.id });

  // Persist choices to project config (if running from a project dir)
  try {
    await writeProjectConfig(targetDir_cwd, { manifest: chosen.id, tier });
    await ensureGitExclude(targetDir_cwd);
  } catch {
    // Not a git repo or can't write — that's OK
  }

  console.log(`\nDone. ${chosen.name} v${chosen.version} (${tier}) installed.`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await main();
}
