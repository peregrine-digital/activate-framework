#!/usr/bin/env node

/**
 * List available Activate framework files by category.
 *
 * Usage:
 *   node list.mjs                     # list all manifests overview
 *   node list.mjs --manifest activate-framework
 *   node list.mjs --manifest activate-framework --tier advanced
 *   node list.mjs --category prompts  # list only prompts (first manifest)
 *   node list.mjs --json              # machine-readable output
 */

import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { listByCategory, formatList, discoverManifests, formatManifestList } from './core.mjs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

function parseArgs(argv) {
  const args = { tier: null, category: null, manifest: null, json: false };
  for (let i = 2; i < argv.length; i++) {
    if (argv[i] === '--tier' && argv[i + 1]) { args.tier = argv[++i]; continue; }
    if (argv[i] === '--category' && argv[i + 1]) { args.category = argv[++i]; continue; }
    if (argv[i] === '--manifest' && argv[i + 1]) { args.manifest = argv[++i]; continue; }
    if (argv[i] === '--json') { args.json = true; continue; }
    if (argv[i] === '--help' || argv[i] === '-h') {
      console.log(`Usage: node list.mjs [--manifest <id>] [--tier minimal|standard|advanced] [--category instructions|prompts|skills|agents] [--json]`);
      process.exit(0);
    }
  }
  return args;
}

async function main() {
  const args = parseArgs(process.argv);
  const manifests = await discoverManifests(__dirname);

  if (manifests.length === 0) {
    console.error('No manifests found.');
    process.exit(1);
  }

  // If no manifest specified, show overview of all manifests
  if (!args.manifest && !args.tier && !args.category) {
    if (args.json) {
      console.log(JSON.stringify({ manifests: manifests.map(({ id, name, description, version, files }) => ({ id, name, description, version, fileCount: files.length })) }, null, 2));
      return;
    }
    console.log('\nAvailable manifests:\n');
    console.log(formatManifestList(manifests));
    console.log('\nUse --manifest <id> to see files for a specific manifest.\n');
    return;
  }

  // Pick the manifest to display
  const chosen = args.manifest
    ? manifests.find((m) => m.id === args.manifest)
    : manifests[0];

  if (!chosen) {
    console.error(`Unknown manifest: ${args.manifest}`);
    console.error(`Available: ${manifests.map((m) => m.id).join(', ')}`);
    process.exit(1);
  }

  const groups = listByCategory(chosen.files, {
    tier: args.tier || undefined,
    category: args.category || undefined,
  });

  if (args.json) {
    console.log(JSON.stringify({ id: chosen.id, name: chosen.name, version: chosen.version, groups }, null, 2));
    return;
  }

  const tierLabel = args.tier || 'all tiers';
  console.log(`\n${chosen.name} v${chosen.version} — ${tierLabel}`);
  console.log(formatList(groups));
  console.log();
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
