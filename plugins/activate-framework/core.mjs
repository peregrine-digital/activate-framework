/**
 * Shared core logic for Activate framework.
 * Used by both the interactive install script and the VS Code extension.
 */

import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';

/** Maps tier name to the set of manifest tiers included */
export const TIER_MAP = {
  minimal: new Set(['core']),
  standard: new Set(['core', 'ad-hoc']),
  advanced: new Set(['core', 'ad-hoc', 'ad-hoc-advanced']),
};

/** Category display labels */
const CATEGORY_LABELS = {
  instructions: 'Instructions',
  prompts: 'Prompts',
  skills: 'Skills',
  agents: 'Agents',
  other: 'Other',
};

/** Ordered list of categories for display */
const CATEGORY_ORDER = ['instructions', 'prompts', 'skills', 'agents', 'other'];

/**
 * Filter manifest files to those included in the chosen tier.
 * @param {Array<{src: string, dest: string, tier: string}>} files
 * @param {string} tier
 * @returns {Array}
 */
export function selectFiles(files, tier) {
  const allowed = TIER_MAP[tier] ?? TIER_MAP.standard;
  return files.filter((f) => allowed.has(f.tier));
}

/**
 * Group manifest files by category, optionally filtered by tier.
 * Returns an ordered array of { category, label, files } objects.
 *
 * @param {Array<{src: string, dest: string, tier: string, category?: string, description?: string}>} files
 * @param {object} [options]
 * @param {string} [options.tier] - If set, filter to this tier first
 * @param {string} [options.category] - If set, return only this category
 * @returns {Array<{category: string, label: string, files: Array}>}
 */
export function listByCategory(files, { tier, category } = {}) {
  let filtered = tier ? selectFiles(files, tier) : files;

  const groups = new Map();
  for (const f of filtered) {
    const cat = f.category || inferCategory(f.src);
    if (category && cat !== category) continue;
    if (!groups.has(cat)) groups.set(cat, []);
    groups.get(cat).push(f);
  }

  return CATEGORY_ORDER
    .filter((cat) => groups.has(cat))
    .map((cat) => ({
      category: cat,
      label: CATEGORY_LABELS[cat] || cat,
      files: groups.get(cat),
    }));
}

/**
 * Infer category from file path if not explicitly set in manifest.
 * @param {string} filePath
 * @returns {string}
 */
export function inferCategory(filePath) {
  if (filePath.startsWith('instructions/')) return 'instructions';
  if (filePath.startsWith('prompts/')) return 'prompts';
  if (filePath.startsWith('skills/')) return 'skills';
  if (filePath.startsWith('agents/')) return 'agents';
  return 'other';
}

/**
 * Format a grouped file list for human-readable terminal output.
 * @param {Array<{category: string, label: string, files: Array}>} groups
 * @returns {string}
 */
export function formatList(groups) {
  const lines = [];
  for (const { label, files } of groups) {
    lines.push(`\n${label} (${files.length})`);
    lines.push('─'.repeat(40));
    for (const f of files) {
      const name = f.dest.split('/').pop().replace(/\.(instructions|prompt|agent)\.md$/, '').replace(/^SKILL\.md$/, f.dest.split('/').slice(-2, -1)[0]);
      const desc = f.description || '';
      const tier = f.tier || '';
      lines.push(`  ${name}`);
      if (desc) lines.push(`    ${desc}`);
      lines.push(`    tier: ${tier}  →  ${f.dest}`);
    }
  }
  return lines.join('\n');
}

// ── Multi-manifest discovery ──────────────────────────────────────────

/**
 * Discover all manifests.
 *
 * Search order:
 *   1. `baseDir/manifests/` — new canonical location (root manifests/)
 *   2. Walk up from `baseDir` to find a parent `manifests/` directory
 *   3. `baseDir/manifest.json` — legacy single file
 *
 * Each manifest may include a `basePath` (relative to the repo root) that
 * tells callers where source files live.  If omitted, basePath defaults to
 * `baseDir` (backward compatible).
 *
 * @param {string} baseDir - Starting directory for discovery
 * @returns {Promise<Array<{id: string, name: string, description: string, version: string, basePath: string, files: Array}>>}
 */
export async function discoverManifests(baseDir) {
  // Try baseDir/manifests/ directly
  const localManifestsDir = path.join(baseDir, 'manifests');
  const result = await _loadManifestsFromDir(localManifestsDir, baseDir);
  if (result.length > 0) return result;

  // Walk up to find a manifests/ directory (e.g. repo root)
  let dir = path.dirname(baseDir);
  const root = path.parse(dir).root;
  while (dir !== root) {
    const candidate = path.join(dir, 'manifests');
    const found = await _loadManifestsFromDir(candidate, dir);
    if (found.length > 0) return found;
    dir = path.dirname(dir);
  }

  // Legacy fallback
  return loadLegacyManifest(baseDir);
}

/**
 * Try to load all *.json manifests from a directory.
 * @param {string} manifestsDir - Absolute path to the manifests/ directory
 * @param {string} repoRoot - Repo root used to resolve basePath
 * @returns {Promise<Array>}
 */
async function _loadManifestsFromDir(manifestsDir, repoRoot) {
  try {
    const entries = await readdir(manifestsDir);
    const jsonFiles = entries.filter((e) => e.endsWith('.json')).sort();
    if (jsonFiles.length === 0) return [];

    const manifests = [];
    for (const file of jsonFiles) {
      const raw = await readFile(path.join(manifestsDir, file), 'utf8');
      const data = JSON.parse(raw);
      const id = file.replace(/\.json$/, '');
      const basePath = data.basePath
        ? path.resolve(repoRoot, data.basePath)
        : repoRoot;
      manifests.push({
        id,
        name: data.name || id,
        description: data.description || '',
        version: data.version || 'unknown',
        basePath,
        files: data.files || [],
      });
    }
    return manifests;
  } catch {
    return [];
  }
}

/**
 * Load a single manifest by id.
 * Searches the same locations as discoverManifests.
 *
 * @param {string} baseDir
 * @param {string} manifestId
 * @returns {Promise<{id: string, name: string, description: string, version: string, basePath: string, files: Array}>}
 */
export async function loadManifest(baseDir, manifestId) {
  const all = await discoverManifests(baseDir);
  const found = all.find((m) => m.id === manifestId);
  if (found) return found;

  // Legacy fallback
  const legacy = await loadLegacyManifest(baseDir);
  if (legacy.length > 0 && (legacy[0].id === manifestId || manifestId === 'default')) {
    return legacy[0];
  }
  throw new Error(`Manifest "${manifestId}" not found`);
}

/**
 * Load the legacy single manifest.json from a bundle directory.
 * Returns an array with one manifest entry for consistency with discoverManifests.
 *
 * @param {string} baseDir
 * @returns {Promise<Array<{id: string, name: string, description: string, version: string, files: Array}>>}
 */
async function loadLegacyManifest(baseDir) {
  try {
    const raw = await readFile(path.join(baseDir, 'manifest.json'), 'utf8');
    const data = JSON.parse(raw);
    let version = data.version || 'unknown';
    // Try reading .activate-version for legacy bundles
    try {
      version = (await readFile(path.join(baseDir, '.activate-version'), 'utf8')).trim();
    } catch { /* use manifest version */ }
    return [{
      id: 'activate-framework',
      name: 'Activate Framework',
      description: '',
      version,
      basePath: baseDir,
      files: data.files || [],
    }];
  } catch {
    return [];
  }
}

/**
 * Format a manifest summary for display in the terminal.
 * @param {Array<{id: string, name: string, description: string, version: string, files: Array}>} manifests
 * @returns {string}
 */
export function formatManifestList(manifests) {
  const lines = [];
  for (const m of manifests) {
    lines.push(`  ${m.id}`);
    lines.push(`    ${m.name} (v${m.version}) — ${m.files.length} files`);
    if (m.description) lines.push(`    ${m.description}`);
  }
  return lines.join('\n');
}
