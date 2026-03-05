/**
 * Copies install-cli.sh into extension/ at build time.
 *
 * Manifest and source files are fetched from GitHub at runtime (remote-only),
 * so this script only needs to bundle install-cli.sh for the auto-install flow.
 *
 * Run: node scripts/prepare-assets.mjs
 */
import { copyFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const extensionDir = path.resolve(__dirname, '..');
const repoRoot = path.resolve(extensionDir, '..');

async function main() {
  // Copy install-cli.sh to extension root so it ships in the VSIX
  try {
    const installSrc = path.join(repoRoot, 'install-cli.sh');
    const installDest = path.join(extensionDir, 'install-cli.sh');
    await copyFile(installSrc, installDest);
    console.log('  ✓ install-cli.sh');
  } catch {
    console.warn('  ⚠ install-cli.sh — not found at repo root');
  }

  console.log('\nDone.');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
