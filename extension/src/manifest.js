/** Maps tier name to the set of manifest tiers included */
const TIER_MAP = {
  minimal: new Set(['core']),
  standard: new Set(['core', 'ad-hoc']),
  advanced: new Set(['core', 'ad-hoc', 'ad-hoc-advanced']),
};

/** Filter manifest files to those included in the chosen tier */
function selectFiles(files, tier) {
  const allowed = TIER_MAP[tier] ?? TIER_MAP.standard;
  return files.filter((f) => allowed.has(f.tier));
}

module.exports = { TIER_MAP, selectFiles };
