import { copyFile, mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import readline from 'node:readline';
import { fileURLToPath } from 'node:url';
import { selectFiles, TIER_MAP } from './core.mjs';

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
  try {
    await readFile(path.join(startDir, 'manifest.json'));
    return startDir;
  } catch {}

  const pluginDir = path.join(startDir, 'plugins', 'activate-framework');
  try {
    await readFile(path.join(pluginDir, 'manifest.json'));
    return pluginDir;
  } catch {}

  throw new Error(`Could not locate manifest.json under ${startDir}`);
}

export async function installFiles({ files, bundleDir, targetDir, version }) {
  for (const f of files) {
    const src = path.join(bundleDir, f.src);
    const dest = path.join(targetDir, f.dest);
    await mkdir(path.dirname(dest), { recursive: true });
    await copyFile(src, dest);
    console.log(`  ✓  ${f.dest}`);
  }
  const versionFile = path.join(targetDir, '.github', '.activate-version');
  await mkdir(path.dirname(versionFile), { recursive: true });
  await writeFile(versionFile, version);
}

async function prompt(rl, question) {
  return new Promise((resolve) => rl.question(question, resolve));
}

async function main() {
  const bundleDir = await resolveBundleDir(path.dirname(fileURLToPath(import.meta.url)));
  const manifest = JSON.parse(await readFile(path.join(bundleDir, 'manifest.json'), 'utf8'));
  const version = (await readFile(path.join(bundleDir, '.activate-version'), 'utf8')).trim();

  const rl = readline.createInterface({ input: process.stdin, output: process.stdout });

  console.log(`\nActivate Copilot ${version} Installer\n`);
  console.log('Tiers:');
  console.log('  minimal   — Core workflow guidance (AGENTS.md, instructions, prompts)');
  console.log('  standard  — Core + ad-hoc instructions, skills, and agents');
  console.log('  advanced  — Standard + advanced tooling\n');

  const rawAssistant = await prompt(rl, 'Assistant? [GitHub Copilot/VS Code] (default: GitHub Copilot): ');
  const assistant = resolveAssistant(rawAssistant);
  const rawTier = (await prompt(rl, 'Which tier? [minimal/standard/advanced] (default: standard): ')).trim() || 'standard';
  const tier = Object.keys(TIER_MAP).includes(rawTier) ? rawTier : 'standard';
  const rawTarget = await prompt(rl, 'Target directory? (default: ~/.copilot): ');
  const targetDir = resolveTargetDir(rawTarget, { assistant });

  rl.close();

  const files = selectFiles(manifest.files, tier);
  console.log(`\nInstalling ${files.length} files to ${targetDir}:\n`);
  await installFiles({ files, bundleDir, targetDir, version });
  console.log(`\nDone. Activate ${version} (${tier}) installed.`);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await main();
}
