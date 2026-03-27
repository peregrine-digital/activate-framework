package model

// Deprecated: ResolvedTier is part of the old manifest+tier system, will be removed.
type ResolvedTier struct {
	ID       string   `json:"id"`
	Label    string   `json:"label"`
	Includes []string `json:"includes"`
}

// Deprecated: DefaultTiers is part of the old manifest+tier system, will be removed.
var DefaultTiers = []ResolvedTier{
	{ID: "minimal", Label: "Minimal", Includes: []string{"core"}},
	{ID: "standard", Label: "Standard", Includes: []string{"core", "ad-hoc"}},
	{ID: "advanced", Label: "Advanced", Includes: []string{"core", "ad-hoc", "ad-hoc-advanced"}},
}

// Deprecated: GetManifestTiers is part of the old manifest+tier system, will be removed.
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

// Deprecated: DiscoverAvailableTiers is part of the old manifest+tier system, will be removed.
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

// Deprecated: GetAllowedFileTiers is part of the old manifest+tier system, will be removed.
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

// Deprecated: SelectFiles is part of the old manifest+tier system, will be removed.
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

// CategoryLabels maps category IDs to display labels.
var CategoryLabels = map[string]string{
	"instructions": "Instructions",
	"prompts":      "Prompts",
	"skills":       "Skills",
	"agents":       "Agents",
	"mcp-servers":  "MCP Servers",
	"other":        "Other",
}

// CategoryOrder defines the display order for categories.
var CategoryOrder = []string{"instructions", "prompts", "skills", "agents", "mcp-servers", "other"}

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

// PresetCategoryGroup holds a group of preset files under one category.
type PresetCategoryGroup struct {
	Category string
	Label    string
	Files    []PresetFile
}

// ListPresetFilesByCategory groups PresetFiles by category, optionally filtering to a single category.
func ListPresetFilesByCategory(files []PresetFile, category string) []PresetCategoryGroup {
	groups := make(map[string][]PresetFile)
	for _, f := range files {
		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Dest)
		}
		if category != "" && cat != category {
			continue
		}
		groups[cat] = append(groups[cat], f)
	}

	var result []PresetCategoryGroup
	for _, cat := range CategoryOrder {
		if fs, ok := groups[cat]; ok {
			label := CategoryLabels[cat]
			if label == "" {
				label = cat
			}
			result = append(result, PresetCategoryGroup{Category: cat, Label: label, Files: fs})
		}
	}
	return result
}

// Deprecated: use ListPresetFilesByCategory instead.
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
	for _, cat := range CategoryOrder {
		if fs, ok := groups[cat]; ok {
			label := CategoryLabels[cat]
			if label == "" {
				label = cat
			}
			result = append(result, CategoryGroup{Category: cat, Label: label, Files: fs})
		}
	}
	return result
}
