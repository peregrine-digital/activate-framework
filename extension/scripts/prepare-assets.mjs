/**
 * Copies the plugin bundle (manifest.json + all referenced files + .activate-version)
 * from plugins/activate-framework/ into extension/assets/ at build time.
 *
 * Run: node scripts/prepare-assets.mjs
 */
import { copyFile, mkdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');
const pluginDir = path.join(repoRoot, '..', 'plugins', 'activate-framework');
const assetsDir = path.join(repoRoot, 'assets');

async function main() {
  // Read manifest to know which files to copy
  const manifestPath = path.join(pluginDir, 'manifest.json');
  const manifest = JSON.parse(await readFile(manifestPath, 'utf8'));

  // Copy manifest.json itself
  await mkdir(assetsDir, { recursive: true });
  await copyFile(manifestPath, path.join(assetsDir, 'manifest.json'));
  console.log('  ✓ manifest.json');

  // Copy .activate-version
  const versionSrc = path.join(pluginDir, '.activate-version');
  await copyFile(versionSrc, path.join(assetsDir, '.activate-version'));
  console.log('  ✓ .activate-version');

  // Copy each file referenced in the manifest
  let copied = 0;
  let skipped = 0;
  for (const f of manifest.files) {
    const src = path.join(pluginDir, f.src);
    const dest = path.join(assetsDir, f.src);
    try {
      await mkdir(path.dirname(dest), { recursive: true });
      await copyFile(src, dest);
      console.log(`  ✓ ${f.src}`);
      copied++;
    } catch (err) {
      console.warn(`  ⚠ ${f.src} — skipped (not found)`);
      skipped++;
    }
  }

  console.log(`\nDone. ${copied + 2} assets copied to extension/assets/`);
  if (skipped > 0) {
    console.warn(`${skipped} manifest entries skipped (source files missing).`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
