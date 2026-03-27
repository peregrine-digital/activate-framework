/**
 * Prepares assets for extension development / packaging.
 *
 * 1. Rebuilds the CLI binary (cli/activate) so the dev-mode daemon always
 *    matches the current source.  Skipped if `go` is not available.
 * 2. Rebuilds the shared Svelte UI bundle (webview-dist/) so the panel
 *    always matches the current UI source.  Skipped if npm is not available.
 * 3. Copies install-cli.sh into extension/ so it ships in the VSIX.
 *
 * Run: node scripts/prepare-assets.mjs
 */
import { copyFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const extensionDir = path.resolve(__dirname, '..');
const repoRoot = path.resolve(extensionDir, '..');

async function main() {
  // Build CLI so the dev extension always uses a fresh binary
  const cliDir = path.join(repoRoot, 'cli');
  try {
    execFileSync('go', ['build', '-o', 'activate', '.'], {
      cwd: cliDir,
      stdio: 'inherit',
      timeout: 120_000,
    });
    console.log('  ✓ cli/activate rebuilt');
  } catch {
    console.warn('  ⚠ cli/activate — go build failed or go not available (skipped)');
  }

  // Build shared UI webview bundle so the panel matches current source
  const uiDir = path.join(repoRoot, 'ui');
  try {
    execFileSync('npm', ['run', 'build:webview'], {
      cwd: uiDir,
      stdio: 'inherit',
      timeout: 60_000,
    });
    console.log('  ✓ webview-dist/ rebuilt');
  } catch {
    console.warn('  ⚠ webview-dist/ — UI build failed or npm not available (skipped)');
  }

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
