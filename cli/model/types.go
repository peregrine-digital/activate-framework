package model

// RepoSidecar tracks installed files and their metadata.
type RepoSidecar struct {
	Manifest   string   `json:"manifest,omitempty"`   // Deprecated: use Preset
	Tier       string   `json:"tier,omitempty"`        // Deprecated: use Preset
	Preset     string   `json:"preset,omitempty"`
	Files      []string `json:"files"`
	McpServers []string `json:"mcpServers,omitempty"`
	Source     string   `json:"source,omitempty"`
}

// InstallState captures config and install status for state-aware boot flow.
type InstallState struct {
	HasGlobalConfig   bool   `json:"hasGlobalConfig"`
	HasProjectConfig  bool   `json:"hasProjectConfig"`
	HasInstallMarker  bool   `json:"hasInstallMarker"`
	InstalledManifest string `json:"installedManifest,omitempty"` // Deprecated: kept for compat
	InstalledPreset   string `json:"installedPreset,omitempty"`
}

// FileStatus describes the install/version state of a single file.
type FileStatus struct {
	Dest             string `json:"dest"`
	DisplayName      string `json:"displayName"`
	Category         string `json:"category"`
	Tier             string `json:"tier,omitempty"`              // Deprecated: kept for compat
	Description      string `json:"description,omitempty"`
	Installed        bool   `json:"installed"`
	InTier           bool   `json:"inTier"`                     // Deprecated: kept for compat
	InPreset         bool   `json:"inPreset"`
	BundledVersion   string `json:"bundledVersion,omitempty"`
	InstalledVersion string `json:"installedVersion,omitempty"`
	UpdateAvailable  bool   `json:"updateAvailable"`
	Skipped          bool   `json:"skipped"`
	Override         string `json:"override,omitempty"`
}

// TelemetryEntry is a single quota log entry.
type TelemetryEntry struct {
	Date               string `json:"date"`
	Timestamp          string `json:"timestamp"`
	PremiumEntitlement *int   `json:"premium_entitlement"`
	PremiumRemaining   *int   `json:"premium_remaining"`
	PremiumUsed        *int   `json:"premium_used"`
	QuotaResetDateUTC  string `json:"quota_reset_date_utc,omitempty"`
	Source             string `json:"source"`
	Version            int    `json:"version"`
}
