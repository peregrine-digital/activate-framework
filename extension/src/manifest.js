/**
 * Default tiers for backward compatibility with manifests that don't define their own.
 * Maps legacy UI tier names to the file-level tier tags they include.
 */
const DEFAULT_TIERS = [
  { id: 'minimal', label: 'Minimal', includes: ['core'] },
  { id: 'standard', label: 'Standard', includes: ['core', 'ad-hoc'] },
  { id: 'advanced', label: 'Advanced', includes: ['core', 'ad-hoc', 'ad-hoc-advanced'] },
];

/**
 * Get the tier definitions for a manifest.
 * If the manifest defines its own tiers, use those (cumulative).
 * Otherwise fall back to DEFAULT_TIERS for backward compatibility.
 *
 * @param {object} manifest - The manifest object (must have files, may have tiers)
 * @returns {Array<{id: string, label: string, includes: string[]}>}
 */
function getManifestTiers(manifest) {
  if (manifest.tiers && Array.isArray(manifest.tiers) && manifest.tiers.length > 0) {
    // Manifest defines custom tiers - make them cumulative
    // Each tier includes its own files + all files from previous tiers
    const result = [];
    const cumulativeIncludes = [];
    for (const tier of manifest.tiers) {
      cumulativeIncludes.push(tier.id);
      result.push({
        id: tier.id,
        label: tier.label || tier.id,
        includes: [...cumulativeIncludes],
      });
    }
    return result;
  }

  // Fall back to default tiers for backward compatibility
  return DEFAULT_TIERS;
}

/**
 * Discover which tiers are available for a manifest.
 * Returns only tiers that have at least one matching file.
 *
 * @param {object} manifest - The manifest object (files + optional tiers)
 * @returns {Array<{id: string, label: string, includes: string[]}>}
 */
function discoverAvailableTiers(manifest) {
  const tiers = getManifestTiers(manifest);
  const presentFileTiers = new Set(manifest.files.map((f) => f.tier).filter(Boolean));

  // Return only tiers that include at least one file tier present in the manifest
  return tiers.filter((tier) =>
    tier.includes.some((fileTier) => presentFileTiers.has(fileTier)),
  );
}

/**
 * Build a Set of allowed file tiers for a given tier selection.
 * Works with both manifest-defined tiers and legacy default tiers.
 *
 * @param {object} manifest - The manifest object
 * @param {string} tierId - The selected tier ID
 * @returns {Set<string>} Set of file tier values that should be included
 */
function getAllowedFileTiers(manifest, tierId) {
  const tiers = getManifestTiers(manifest);
  const tier = tiers.find((t) => t.id === tierId);
  if (tier) {
    return new Set(tier.includes);
  }
  // Fallback: if tier not found, use standard tier or first available
  const fallback = tiers.find((t) => t.id === 'standard') || tiers[0];
  return fallback ? new Set(fallback.includes) : new Set(['core']);
}

/** Ordered list of categories for display */
const CATEGORY_ORDER = ['instructions', 'prompts', 'skills', 'agents', 'mcp-servers', 'other'];

const CATEGORY_LABELS = {
  instructions: 'Instructions',
  prompts: 'Prompts',
  skills: 'Skills',
  agents: 'Agents',
  'mcp-servers': 'MCP Servers',
  other: 'Other',
};

/**
 * Filter manifest files to those included in the chosen tier.
 * @param {Array} files - The files array from the manifest
 * @param {string} tier - The selected tier ID
 * @param {object} [manifest] - Optional full manifest object for custom tier support
 * @returns {Array}
 */
function selectFiles(files, tier, manifest) {
  const allowed = manifest
    ? getAllowedFileTiers(manifest, tier)
    : getAllowedFileTiers({ files }, tier);
  return files.filter((f) => allowed.has(f.tier));
}

/** Infer category from file path if not set in manifest */
function inferCategory(filePath) {
  if (filePath.startsWith('instructions/')) return 'instructions';
  if (filePath.startsWith('prompts/')) return 'prompts';
  if (filePath.startsWith('skills/')) return 'skills';
  if (filePath.startsWith('agents/')) return 'agents';
  if (filePath.startsWith('mcp-servers/')) return 'mcp-servers';
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
 * @returns {{id: string, name: string, description: string, version: string, files: Array, tiers: Array|undefined}}
 */
function parseManifestData(id, data) {
  return {
    id,
    name: data.name || id,
    description: data.description || '',
    version: data.version || 'unknown',
    files: data.files || [],
    tiers: data.tiers || undefined,
  };
}

/**
 * Get a tier's label from a manifest.
 * @param {object} manifest - The manifest object
 * @param {string} tierId - The tier ID
 * @returns {string}
 */
function getTierLabel(manifest, tierId) {
  const tiers = getManifestTiers(manifest);
  const tier = tiers.find((t) => t.id === tierId);
  return tier ? tier.label : tierId;
}

module.exports = {
  DEFAULT_TIERS,
  CATEGORY_ORDER,
  selectFiles,
  inferCategory,
  listByCategory,
  parseManifestData,
  discoverAvailableTiers,
  getManifestTiers,
  getAllowedFileTiers,
  getTierLabel,
};
