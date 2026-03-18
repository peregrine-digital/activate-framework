package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// WorkspaceInfo describes a known Activate workspace for the welcome page.
type WorkspaceInfo struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Manifest  string `json:"manifest,omitempty"`
	Tier      string `json:"tier,omitempty"`
	FileCount int    `json:"fileCount"`
	Exists    bool   `json:"exists"`
}

// ListWorkspaces scans ~/.activate/repos/ for known workspaces.
func (a *App) ListWorkspaces() []WorkspaceInfo {
	home, _ := os.UserHomeDir()
	reposDir := filepath.Join(home, ".activate", "repos")
	entries, err := os.ReadDir(reposDir)
	if err != nil {
		return nil
	}

	var workspaces []WorkspaceInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		hashDir := filepath.Join(reposDir, entry.Name())

		// Read repo.json to get the original path
		metaPath := filepath.Join(hashDir, "repo.json")
		metaData, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(metaData, &meta) != nil || meta.Path == "" {
			continue
		}

		ws := WorkspaceInfo{
			Path: meta.Path,
			Name: filepath.Base(meta.Path),
		}

		// Skip test/temp directories that leaked into the store
		if isTestPath(meta.Path) {
			continue
		}

		// Check if directory still exists
		if _, err := os.Stat(meta.Path); err == nil {
			ws.Exists = true
		}

		// Read installed.json for manifest/tier/file count
		sidecarPath := filepath.Join(hashDir, "installed.json")
		if data, err := os.ReadFile(sidecarPath); err == nil {
			var sidecar struct {
				Manifest string   `json:"manifest"`
				Tier     string   `json:"tier"`
				Files    []string `json:"files"`
			}
			if json.Unmarshal(data, &sidecar) == nil {
				ws.Manifest = sidecar.Manifest
				ws.Tier = sidecar.Tier
				ws.FileCount = len(sidecar.Files)
			}
		}

		workspaces = append(workspaces, ws)
	}

	// Sort: existing workspaces first, then by name
	sort.Slice(workspaces, func(i, j int) bool {
		if workspaces[i].Exists != workspaces[j].Exists {
			return workspaces[i].Exists
		}
		return workspaces[i].Name < workspaces[j].Name
	})

	return workspaces
}

// isTestPath returns true for paths that look like test temp directories.
func isTestPath(p string) bool {
	// macOS temp dirs from Go tests
	if strings.HasPrefix(p, "/var/folders/") || strings.HasPrefix(p, "/private/var/folders/") {
		return true
	}
	// Linux/generic temp dirs
	if strings.HasPrefix(p, "/tmp/") {
		return true
	}
	return false
}
