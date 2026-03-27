package model

const (
	DefaultManifest = "adhoc"    // Deprecated: use Preset field
	DefaultTier     = "standard" // Deprecated: use Preset field

	// ClearValue is a sentinel passed via configSet to unset a string field.
	ClearValue = "__clear__"
)

// Config is the unified configuration shape used at both layers.
type Config struct {
	Repo             string            `json:"repo,omitempty"`
	Branch           string            `json:"branch,omitempty"`
	Manifest         string            `json:"manifest,omitempty"`          // Deprecated: use Preset
	Tier             string            `json:"tier,omitempty"`              // Deprecated: use Preset
	Preset           string            `json:"preset,omitempty"`            // preset ID e.g. "adhoc/standard"
	FileOverrides    map[string]string `json:"fileOverrides,omitempty"`
	SkippedVersions  map[string]string `json:"skippedVersions,omitempty"`
	TelemetryEnabled *bool             `json:"telemetryEnabled,omitempty"`
}

// MergeConfig applies non-zero fields from src onto dst.
// Use ClearValue ("__clear__") to explicitly unset a string field.
func MergeConfig(dst, src *Config) {
	if src.Repo == ClearValue {
		dst.Repo = ""
	} else if src.Repo != "" {
		dst.Repo = src.Repo
	}
	if src.Branch == ClearValue {
		dst.Branch = ""
	} else if src.Branch != "" {
		dst.Branch = src.Branch
	}
	// Deprecated fields — kept for backward compat
	if src.Manifest == ClearValue {
		dst.Manifest = ""
	} else if src.Manifest != "" {
		dst.Manifest = src.Manifest
	}
	if src.Tier == ClearValue {
		dst.Tier = ""
	} else if src.Tier != "" {
		dst.Tier = src.Tier
	}
	// New preset field
	if src.Preset == ClearValue {
		dst.Preset = ""
	} else if src.Preset != "" {
		dst.Preset = src.Preset
	}
	if src.FileOverrides != nil {
		if dst.FileOverrides == nil {
			dst.FileOverrides = make(map[string]string)
		}
		for k, v := range src.FileOverrides {
			if v == "" {
				delete(dst.FileOverrides, k)
			} else {
				dst.FileOverrides[k] = v
			}
		}
	}
	if src.SkippedVersions != nil {
		if dst.SkippedVersions == nil {
			dst.SkippedVersions = make(map[string]string)
		}
		for k, v := range src.SkippedVersions {
			if v == "" {
				delete(dst.SkippedVersions, k)
			} else {
				dst.SkippedVersions[k] = v
			}
		}
	}
	if src.TelemetryEnabled != nil {
		dst.TelemetryEnabled = src.TelemetryEnabled
	}
}

// MigrateManifestTierToPreset converts legacy manifest+tier config to preset ID.
// Returns empty string if no migration needed.
func MigrateManifestTierToPreset(manifest, tier string) string {
	if manifest == "" && tier == "" {
		return ""
	}
	m := manifest
	if m == "" {
		m = DefaultManifest
	}
	t := tier
	if t == "" {
		t = DefaultTier
	}
	switch m {
	case "adhoc":
		switch t {
		case "minimal":
			return "adhoc/core"
		case "standard":
			return "adhoc/standard"
		case "advanced":
			return "adhoc/advanced"
		default:
			return m + "/" + t
		}
	case "ironarch":
		switch t {
		case "skills":
			return "ironarch/skills"
		case "workflow":
			return "ironarch/workflow"
		default:
			return m + "/" + t
		}
	default:
		return m + "/" + t
	}
}

// ResolvedPreset returns the effective preset ID from config,
// falling back to legacy manifest+tier migration. Returns "" if no preset is configured.
func (c *Config) ResolvedPreset() string {
	if c.Preset != "" {
		return c.Preset
	}
	if c.Manifest != "" || c.Tier != "" {
		return MigrateManifestTierToPreset(c.Manifest, c.Tier)
	}
	return ""
}
