package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// InstallState captures config and install status for state-aware boot flow.
type InstallState struct {
	HasGlobalConfig  bool   `json:"hasGlobalConfig"`
	HasProjectConfig bool   `json:"hasProjectConfig"`
	HasInstallMarker bool   `json:"hasInstallMarker"`
	InstalledManifest string `json:"installedManifest,omitempty"`
	InstalledVersion  string `json:"installedVersion,omitempty"`
}

type installMarker struct {
	Manifest string `json:"manifest"`
	Version  string `json:"version"`
}

func DetectInstallState(projectDir string) InstallState {
	state := InstallState{}

	if _, err := os.Stat(globalConfigPath()); err == nil {
		state.HasGlobalConfig = true
	}
	if _, err := os.Stat(projectConfigPath(projectDir)); err == nil {
		state.HasProjectConfig = true
	}

	markerPath := filepath.Join(projectDir, ".github", ".activate-version")
	if data, err := os.ReadFile(markerPath); err == nil {
		var marker installMarker
		if json.Unmarshal(data, &marker) == nil {
			state.HasInstallMarker = true
			state.InstalledManifest = marker.Manifest
			state.InstalledVersion = marker.Version
		}
	}

	return state
}
