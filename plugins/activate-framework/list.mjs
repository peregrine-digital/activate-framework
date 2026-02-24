#!/usr/bin/env node

/**
 * List available Activate framework files by category.
 *
 * Usage:
 *   node list.mjs                     # list all files (standard tier)
 *   node list.mjs --tier advanced     # list all files for advanced tier
 *   node list.mjs --category prompts  # list only prompts
 *   node list.mjs --category skills --tier minimal
 *   node list.mjs --json              # machine-readable output
 */

import { readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { listByCategory, formatList } from './core.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function parseArgs(argv) {
  const args = { tier: null, category: null, json: false };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === '--tier' && argv[i + 1]) { args.tier = argv[++i]; continue; }
    if (argv[i] === '--category' && argv[i + 1]) { args.category = argv[++i]; continue; }
    if (argv[i] === '--json') { args.json = true; continue; }
    if (argv[i] === '--help' || argv[i] === '-h') {
      console.log(`Usage: node list.mjs [--tier minimal|standard|advanced] [--category instructions|prompts|skills|agents] [--json]`);
      process.exit(0);
    }
  }
  return args;
}

async function main() {
  const args = parseArgs(process.argv);

  const manifestPath = path.join(__dirname, 'manifest.json');
  const manifest = JSON.parse(await readFile(manifestPath, 'utf8'));
  const version = (await readFile(path.join(__dirname, '.activate-version'), 'utf8')).trim();

  const groups = listByCategory(manifest.files, {
    tier: args.tier || undefined,
    category: args.category || undefined,
  });

  if (args.json) {
    console.log(JSON.stringify({ version, groups }, null, 2));
    return;
  }

  const tierLabel = args.tier || 'all tiers';
  console.log(`\nActivate Framework ${version} — ${tierLabel}`);
  console.log(formatList(groups));
  console.log();
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
