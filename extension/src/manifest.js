/** Maps tier name to the set of manifest tiers included */
const TIER_MAP = {
  minimal: new Set(['core']),
  standard: new Set(['core', 'ad-hoc']),
  advanced: new Set(['core', 'ad-hoc', 'ad-hoc-advanced']),
};

/** Ordered list of categories for display */
const CATEGORY_ORDER = ['instructions', 'prompts', 'skills', 'agents', 'other'];

const CATEGORY_LABELS = {
  instructions: 'Instructions',
  prompts: 'Prompts',
  skills: 'Skills',
  agents: 'Agents',
  other: 'Other',
};

/** Filter manifest files to those included in the chosen tier */
function selectFiles(files, tier) {
  const allowed = TIER_MAP[tier] ?? TIER_MAP.standard;
  return files.filter((f) => allowed.has(f.tier));
}

/** Infer category from file path if not set in manifest */
function inferCategory(filePath) {
  if (filePath.startsWith('instructions/')) return 'instructions';
  if (filePath.startsWith('prompts/')) return 'prompts';
  if (filePath.startsWith('skills/')) return 'skills';
  if (filePath.startsWith('agents/')) return 'agents';
  return 'other';
}

/**
 * Group manifest files by category, optionally filtered by tier/category.
 * Returns an ordered array of { category, label, files } objects.
 */
function listByCategory(files, { tier, category } = {}) {
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
 * Parse a manifest JSON object into a normalized manifest entry.
 * @param {string} id - The manifest id (derived from filename)
 * @param {object} data - The parsed JSON object
 * @returns {{id: string, name: string, description: string, version: string, files: Array}}
 */
function parseManifestData(id, data) {
  return {
    id,
    name: data.name || id,
    description: data.description || '',
    version: data.version || 'unknown',
    files: data.files || [],
  };
}

module.exports = { TIER_MAP, CATEGORY_ORDER, selectFiles, inferCategory, listByCategory, parseManifestData };
