package transport

import (
	"github.com/peregrine-digital/activate-framework/cli/model"
)

// ── Protocol: JSON-RPC method names and typed params/results ───

// Method name constants for the JSON-RPC protocol.
const (
	MethodInitialize    = "activate/initialize"
	MethodShutdown      = "activate/shutdown"
	MethodStateGet      = "activate/state"
	MethodConfigGet     = "activate/configGet"
	MethodConfigSet     = "activate/configSet"
	MethodManifestList  = "activate/manifestList"
	MethodManifestFiles = "activate/manifestFiles"
	MethodRepoAdd       = "activate/repoAdd"
	MethodRepoRemove    = "activate/repoRemove"
	MethodSync          = "activate/sync"
	MethodUpdate        = "activate/update"
	MethodFileInstall   = "activate/fileInstall"
	MethodFileUninstall = "activate/fileUninstall"
	MethodFileDiff      = "activate/fileDiff"
	MethodFileSkip      = "activate/fileSkip"
	MethodFileOverride  = "activate/fileOverride"
	MethodTelemetryRun  = "activate/telemetryRun"
	MethodTelemetryLog  = "activate/telemetryLog"

	// Notification methods (server → client)
	NotifyStateChanged = "activate/stateChanged"
)

// ── Param types ────────────────────────────────────────────────

// InitializeParams is sent by the client on startup.
type InitializeParams struct {
	ProjectDir string `json:"projectDir,omitempty"`
}

// InitializeResult is returned on initialize.
type InitializeResult struct {
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

// ConfigGetParams specifies which config scope to read.
type ConfigGetParams struct {
	Scope string `json:"scope,omitempty"` // "global", "project", "resolved" (default)
}

// ConfigSetParams specifies config updates and scope.
type ConfigSetParams struct {
	Scope            string       `json:"scope,omitempty"` // "project" (default), "global"
	Manifest         string       `json:"manifest,omitempty"`
	Tier             string       `json:"tier,omitempty"`
	TelemetryEnabled *bool        `json:"telemetryEnabled,omitempty"`
	Updates          *model.Config `json:"updates,omitempty"` // for full config patches
}

// ManifestFilesParams specifies which manifest/tier/category to list files for.
type ManifestFilesParams struct {
	Manifest string `json:"manifest,omitempty"`
	Tier     string `json:"tier,omitempty"`
	Category string `json:"category,omitempty"`
}

// FileParams identifies a single file for operations.
type FileParams struct {
	File string `json:"file"`
}

// FileOverrideParams sets an override on a file.
type FileOverrideParams struct {
	File     string `json:"file"`
	Override string `json:"override"` // "pinned", "excluded", "" (clear)
}

// TelemetryRunParams optionally provides a token.
type TelemetryRunParams struct {
	Token string `json:"token,omitempty"`
}

// StateChangedNotification creates a state-changed notification.
func StateChangedNotification() *Notification {
	return &Notification{
		Method: NotifyStateChanged,
	}
}
