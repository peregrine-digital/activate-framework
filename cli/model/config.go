package model

const (
	DefaultManifest = "activate-framework"
	DefaultTier     = "standard"

	// ClearValue is a sentinel passed via configSet to unset a string field.
	ClearValue = "__clear__"
)

// Config is the unified configuration shape used at both layers.
type Config struct {
	Manifest         string            `json:"manifest"`
	Tier             string            `json:"tier"`
	FileOverrides    map[string]string `json:"fileOverrides,omitempty"`
	SkippedVersions  map[string]string `json:"skippedVersions,omitempty"`
	TelemetryEnabled *bool             `json:"telemetryEnabled,omitempty"`
}

// MergeConfig applies non-zero fields from src onto dst.
// Use ClearValue ("__clear__") to explicitly unset a string field.
func MergeConfig(dst, src *Config) {
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
