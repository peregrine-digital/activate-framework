/**
 * Copies manifests and their referenced source files into extension/assets/
 * at build time.
 *
 * Manifest discovery order:
 *   1. Root manifests/ directory (each *.json has a basePath for source files)
 *   2. Legacy plugins/activate-framework/manifests/ (basePath defaults to that dir)
 *   3. Legacy plugins/activate-framework/manifest.json (single-file fallback)
 *
 * Run: node scripts/prepare-assets.mjs
 */
import { copyFile, mkdir, readFile, readdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const extensionDir = path.resolve(__dirname, '..');
const repoRoot = path.resolve(extensionDir, '..');
const assetsDir = path.join(extensionDir, 'assets');

/** Default plugin directory (used when basePath is not specified) */
const defaultPluginDir = path.join(repoRoot, 'plugins', 'activate-framework');

async function main() {
  await mkdir(assetsDir, { recursive: true });

  // fileEntry → { baseDir, dest } for resolving source and destination
  // Map<srcRelative, { baseDir: absolutePath, dest: relativePath }>
  const fileSources = new Map();
  let hasManifests = false;

  // 1. Check root manifests/ directory
  const rootManifestsDir = path.join(repoRoot, 'manifests');
  try {
    const entries = await readdir(rootManifestsDir);
    const jsonFiles = entries.filter((e) => e.endsWith('.json'));
    if (jsonFiles.length > 0) {
      hasManifests = true;
      const assetsManifestsDir = path.join(assetsDir, 'manifests');
      await mkdir(assetsManifestsDir, { recursive: true });

      for (const file of jsonFiles) {
        const src = path.join(rootManifestsDir, file);
        const dest = path.join(assetsManifestsDir, file);
        await copyFile(src, dest);
        console.log(`  ✓ manifests/${file}`);

        const manifest = JSON.parse(await readFile(src, 'utf8'));
        const baseDir = manifest.basePath
          ? path.resolve(repoRoot, manifest.basePath)
          : defaultPluginDir;

        for (const f of manifest.files || []) {
          // Use dest for the target path (no relative escaping), src for source resolution
          fileSources.set(f.src, { baseDir, dest: f.dest });
        }
      }
    }
  } catch {
    // No root manifests/ directory
  }

  // 2. Legacy fallback: plugin manifests/ dir
  if (!hasManifests) {
    const pluginManifestsDir = path.join(defaultPluginDir, 'manifests');
    try {
      const entries = await readdir(pluginManifestsDir);
      const jsonFiles = entries.filter((e) => e.endsWith('.json'));
      if (jsonFiles.length > 0) {
        hasManifests = true;
        const assetsManifestsDir = path.join(assetsDir, 'manifests');
        await mkdir(assetsManifestsDir, { recursive: true });

        for (const file of jsonFiles) {
          const src = path.join(pluginManifestsDir, file);
          const dest = path.join(assetsManifestsDir, file);
          await copyFile(src, dest);
          console.log(`  ✓ manifests/${file}`);

          const manifest = JSON.parse(await readFile(src, 'utf8'));
          for (const f of manifest.files || []) {
            fileSources.set(f.src, { baseDir: defaultPluginDir, dest: f.dest });
          }
        }
      }
    } catch {
      // No plugin manifests/ dir
    }
  }

  // 3. Legacy fallback: single manifest.json
  if (!hasManifests) {
    const manifestPath = path.join(defaultPluginDir, 'manifest.json');
    const manifest = JSON.parse(await readFile(manifestPath, 'utf8'));
    await copyFile(manifestPath, path.join(assetsDir, 'manifest.json'));
    console.log('  ✓ manifest.json');

    for (const f of manifest.files || []) {
      fileSources.set(f.src, { baseDir: defaultPluginDir, dest: f.dest });
    }
  }

  // Copy .activate-version if it exists (legacy)
  try {
    const versionSrc = path.join(defaultPluginDir, '.activate-version');
    await copyFile(versionSrc, path.join(assetsDir, '.activate-version'));
    console.log('  ✓ .activate-version');
  } catch {
    console.log('  ℹ No .activate-version file (version in manifest)');
  }

  // Copy each unique file referenced across all manifests
  let copied = 0;
  let skipped = 0;
  for (const [fileSrc, { baseDir, dest: fileDest }] of [...fileSources.entries()].sort(([a], [b]) => a.localeCompare(b))) {
    const src = path.join(baseDir, fileSrc);
    // Use dest path (not src) to avoid relative paths escaping the assets dir
    const dest = path.join(assetsDir, fileDest);
    try {
      await mkdir(path.dirname(dest), { recursive: true });
      await copyFile(src, dest);
      console.log(`  ✓ ${fileDest}`);
      copied++;
    } catch {
      console.warn(`  ⚠ ${fileDest} — skipped (not found at ${fileSrc})`);
      skipped++;
    }
  }

  console.log(`\nDone. ${copied} source files + ${hasManifests ? fileSources.size > 0 ? 'manifests' : '0 manifests' : 'manifest.json'} copied to extension/assets/`);
  if (skipped > 0) {
    console.warn(`${skipped} manifest entries skipped (source files missing).`);
  }

  // Copy install.sh to extension root so it ships in the VSIX
  try {
    const installSrc = path.join(repoRoot, 'install.sh');
    const installDest = path.join(extensionDir, 'install.sh');
    await copyFile(installSrc, installDest);
    console.log('  ✓ install.sh');
  } catch {
    console.warn('  ⚠ install.sh — not found at repo root');
  }
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
