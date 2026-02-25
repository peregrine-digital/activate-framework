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
 * Discover all manifests in a `manifests/` directory.
 * Each *.json file becomes a manifest whose id is derived from the filename.
 * Falls back to a single root `manifest.json` for backward compatibility.
 *
 * @param {string} baseDir - The bundle directory (e.g. plugins/activate-framework)
 * @returns {Promise<Array<{id: string, name: string, description: string, version: string, files: Array}>>}
 */
export async function discoverManifests(baseDir) {
  const manifestsDir = path.join(baseDir, 'manifests');

  try {
    const entries = await readdir(manifestsDir);
    const jsonFiles = entries.filter((e) => e.endsWith('.json')).sort();

    if (jsonFiles.length === 0) {
      return loadLegacyManifest(baseDir);
    }

    const manifests = [];
    for (const file of jsonFiles) {
      const raw = await readFile(path.join(manifestsDir, file), 'utf8');
      const data = JSON.parse(raw);
      const id = file.replace(/\.json$/, '');
      manifests.push({
        id,
        name: data.name || id,
        description: data.description || '',
        version: data.version || 'unknown',
        files: data.files || [],
      });
    }
    return manifests;
  } catch {
    // manifests/ directory does not exist — fall back to legacy
    return loadLegacyManifest(baseDir);
  }
}

/**
 * Load a single manifest by id from the manifests/ directory.
 * Falls back to legacy manifest.json if the directory doesn't exist.
 *
 * @param {string} baseDir
 * @param {string} manifestId
 * @returns {Promise<{id: string, name: string, description: string, version: string, files: Array}>}
 */
export async function loadManifest(baseDir, manifestId) {
  const manifestPath = path.join(baseDir, 'manifests', `${manifestId}.json`);
  try {
    const raw = await readFile(manifestPath, 'utf8');
    const data = JSON.parse(raw);
    return {
      id: manifestId,
      name: data.name || manifestId,
      description: data.description || '',
      version: data.version || 'unknown',
      files: data.files || [],
    };
  } catch {
    // Fall back to legacy manifest.json if requested id is not found
    const legacy = await loadLegacyManifest(baseDir);
    if (legacy.length > 0 && (legacy[0].id === manifestId || manifestId === 'default')) {
      return legacy[0];
    }
    throw new Error(`Manifest "${manifestId}" not found in ${baseDir}/manifests/`);
  }
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
