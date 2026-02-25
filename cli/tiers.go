package main

// ── Default tiers (backward compat) ────────────────────────────

// ResolvedTier is a tier definition with cumulative includes.
type ResolvedTier struct {
	ID       string
	Label    string
	Includes []string
}

// DefaultTiers for manifests that don't define their own.
var DefaultTiers = []ResolvedTier{
	{ID: "minimal", Label: "Minimal", Includes: []string{"core"}},
	{ID: "standard", Label: "Standard", Includes: []string{"core", "ad-hoc"}},
	{ID: "advanced", Label: "Advanced", Includes: []string{"core", "ad-hoc", "ad-hoc-advanced"}},
}

// GetManifestTiers returns the tier definitions for a manifest.
// If the manifest defines custom tiers, they are made cumulative.
// Otherwise returns DefaultTiers.
func GetManifestTiers(m Manifest) []ResolvedTier {
	if len(m.Tiers) > 0 {
		var result []ResolvedTier
		var cumulative []string
		for _, t := range m.Tiers {
			cumulative = append(cumulative, t.ID)
			inc := make([]string, len(cumulative))
			copy(inc, cumulative)
			label := t.Label
			if label == "" {
				label = t.ID
			}
			result = append(result, ResolvedTier{ID: t.ID, Label: label, Includes: inc})
		}
		return result
	}
	return DefaultTiers
}

// DiscoverAvailableTiers returns only tiers that have at least one file.
func DiscoverAvailableTiers(m Manifest) []ResolvedTier {
	presentTiers := make(map[string]bool)
	for _, f := range m.Files {
		if f.Tier != "" {
			presentTiers[f.Tier] = true
		}
	}
	tiers := GetManifestTiers(m)
	var result []ResolvedTier
	for _, t := range tiers {
		for _, inc := range t.Includes {
			if presentTiers[inc] {
				result = append(result, t)
				break
			}
		}
	}
	return result
}

// GetAllowedFileTiers returns the set of file-tier values allowed for a given tier ID.
func GetAllowedFileTiers(m Manifest, tierID string) map[string]bool {
	tiers := GetManifestTiers(m)
	for _, t := range tiers {
		if t.ID == tierID {
			s := make(map[string]bool, len(t.Includes))
			for _, inc := range t.Includes {
				s[inc] = true
			}
			return s
		}
	}
	// Fallback to "standard" or first tier
	for _, t := range tiers {
		if t.ID == "standard" {
			s := make(map[string]bool, len(t.Includes))
			for _, inc := range t.Includes {
				s[inc] = true
			}
			return s
		}
	}
	if len(tiers) > 0 {
		s := make(map[string]bool, len(tiers[0].Includes))
		for _, inc := range tiers[0].Includes {
			s[inc] = true
		}
		return s
	}
	return map[string]bool{"core": true}
}

// SelectFiles filters manifest files to those included in the chosen tier.
func SelectFiles(files []ManifestFile, m Manifest, tierID string) []ManifestFile {
	allowed := GetAllowedFileTiers(m, tierID)
	var result []ManifestFile
	for _, f := range files {
		if allowed[f.Tier] {
			result = append(result, f)
		}
	}
	return result
}

// ── Categories ──────────────────────────────────────────────────

var categoryLabels = map[string]string{
	"instructions": "Instructions",
	"prompts":      "Prompts",
	"skills":       "Skills",
	"agents":       "Agents",
	"mcp-servers":  "MCP Servers",
	"other":        "Other",
}

var categoryOrder = []string{"instructions", "prompts", "skills", "agents", "mcp-servers", "other"}

// CategoryGroup holds a group of files under one category.
type CategoryGroup struct {
	Category string
	Label    string
	Files    []ManifestFile
}

// InferCategory guesses the category from a file path.
func InferCategory(filePath string) string {
	prefixes := []string{"instructions/", "prompts/", "skills/", "agents/", "mcp-servers/"}
	cats := []string{"instructions", "prompts", "skills", "agents", "mcp-servers"}
	for i, p := range prefixes {
		if len(filePath) >= len(p) && filePath[:len(p)] == p {
			return cats[i]
		}
	}
	return "other"
}

// ListByCategory groups files by category, optionally filtering by tier and/or category.
func ListByCategory(files []ManifestFile, m Manifest, tierID, category string) []CategoryGroup {
	var filtered []ManifestFile
	if tierID != "" {
		filtered = SelectFiles(files, m, tierID)
	} else {
		filtered = files
	}

	groups := make(map[string][]ManifestFile)
	for _, f := range filtered {
		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Src)
		}
		if category != "" && cat != category {
			continue
		}
		groups[cat] = append(groups[cat], f)
	}

	var result []CategoryGroup
	for _, cat := range categoryOrder {
		if fs, ok := groups[cat]; ok {
			label := categoryLabels[cat]
			if label == "" {
				label = cat
			}
			result = append(result, CategoryGroup{Category: cat, Label: label, Files: fs})
		}
	}
	return result
}
