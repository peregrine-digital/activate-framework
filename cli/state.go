package main

import (
	"os"
)

// InstallState captures config and install status for state-aware boot flow.
type InstallState struct {
	HasGlobalConfig   bool   `json:"hasGlobalConfig"`
	HasProjectConfig  bool   `json:"hasProjectConfig"`
	HasInstallMarker  bool   `json:"hasInstallMarker"`
	InstalledManifest string `json:"installedManifest,omitempty"`
	InstalledVersion  string `json:"installedVersion,omitempty"`
}

func DetectInstallState(projectDir string) InstallState {
	state := InstallState{}

	if _, err := os.Stat(globalConfigPath()); err == nil {
		state.HasGlobalConfig = true
	}
	if _, err := os.Stat(projectConfigPath(projectDir)); err == nil {
		state.HasProjectConfig = true
	}

	if sc, _ := readRepoSidecar(projectDir); sc != nil {
		state.HasInstallMarker = true
		state.InstalledManifest = sc.Manifest
		state.InstalledVersion = sc.Version
	}

	return state
}
