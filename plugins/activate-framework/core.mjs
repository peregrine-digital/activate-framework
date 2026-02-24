/**
 * Shared core logic for Activate framework.
 * Used by both the interactive install script and the VS Code extension.
 */

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
