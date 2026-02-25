/**
 * Copies the plugin bundle (manifests, referenced files, .activate-version)
 * from plugins/activate-framework/ into extension/assets/ at build time.
 *
 * Supports both the new multi-manifest layout (manifests/*.json) and
 * the legacy single manifest.json for backward compatibility.
 *
 * Run: node scripts/prepare-assets.mjs
 */
import { copyFile, mkdir, readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(__dirname, '..');
const pluginDir = path.join(repoRoot, '..', 'plugins', 'activate-framework');
const assetsDir = path.join(repoRoot, 'assets');

async function main() {
  await mkdir(assetsDir, { recursive: true });

  // Collect all manifest files (from manifests/ or legacy manifest.json)
  const allFiles = new Set();
  const manifestsDir = path.join(pluginDir, 'manifests');
  let hasManifestsDir = false;

  try {
    const entries = await readdir(manifestsDir);
    const jsonFiles = entries.filter((e) => e.endsWith('.json'));
    if (jsonFiles.length > 0) {
      hasManifestsDir = true;
      const assetsManifestsDir = path.join(assetsDir, 'manifests');
      await mkdir(assetsManifestsDir, { recursive: true });

      for (const file of jsonFiles) {
        const src = path.join(manifestsDir, file);
        const dest = path.join(assetsManifestsDir, file);
        await copyFile(src, dest);
        console.log(`  ✓ manifests/${file}`);

        // Collect referenced files
        const manifest = JSON.parse(await readFile(src, 'utf8'));
        for (const f of manifest.files || []) {
          allFiles.add(f.src);
        }
      }
    }
  } catch {
    // No manifests/ directory
  }

  // Legacy fallback: copy manifest.json if no manifests/ dir
  if (!hasManifestsDir) {
    const manifestPath = path.join(pluginDir, 'manifest.json');
    const manifest = JSON.parse(await readFile(manifestPath, 'utf8'));
    await copyFile(manifestPath, path.join(assetsDir, 'manifest.json'));
    console.log('  ✓ manifest.json');

    for (const f of manifest.files || []) {
      allFiles.add(f.src);
    }
  }

  // Copy .activate-version if it exists
  try {
    const versionSrc = path.join(pluginDir, '.activate-version');
    await copyFile(versionSrc, path.join(assetsDir, '.activate-version'));
    console.log('  ✓ .activate-version');
  } catch {
    console.log('  ℹ No .activate-version file (version in manifest)');
  }

  // Copy each unique file referenced across all manifests
  let copied = 0;
  let skipped = 0;
  for (const fileSrc of [...allFiles].sort()) {
    const src = path.join(pluginDir, fileSrc);
    const dest = path.join(assetsDir, fileSrc);
    try {
      await mkdir(path.dirname(dest), { recursive: true });
      await copyFile(src, dest);
      console.log(`  ✓ ${fileSrc}`);
      copied++;
    } catch {
      console.warn(`  ⚠ ${fileSrc} — skipped (not found)`);
      skipped++;
    }
  }

  const extraCount = hasManifestsDir ? 0 : 1; // manifest.json counts as extra if legacy
  console.log(`\nDone. ${copied + extraCount + (hasManifestsDir ? [...allFiles].length === copied ? 0 : 0 : 1)} assets copied to extension/assets/`);
  if (skipped > 0) {
    console.warn(`${skipped} manifest entries skipped (source files missing).`);
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
