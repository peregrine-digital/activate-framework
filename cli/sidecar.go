package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type repoSidecar struct {
	Manifest   string   `json:"manifest"`
	Version    string   `json:"version"`
	Tier       string   `json:"tier"`
	Files      []string `json:"files"`
	McpServers []string `json:"mcpServers,omitempty"`
	Source     string   `json:"source,omitempty"`
}

func sidecarPath(projectDir string) string {
	return filepath.Join(repoStorePath(projectDir), "installed.json")
}

func readRepoSidecar(projectDir string) (*repoSidecar, error) {
	data, err := os.ReadFile(sidecarPath(projectDir))
	if err != nil {
		return nil, nil
	}
	var sc repoSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, nil
	}
	return &sc, nil
}

func writeRepoSidecar(projectDir string, next repoSidecar) error {
	prev, _ := readRepoSidecar(projectDir)
	prevSet := make(map[string]struct{})
	nextSet := make(map[string]struct{})

	var prevFiles []string
	if prev != nil {
		prevFiles = prev.Files
	}

	for _, path := range prevFiles {
		prevSet[path] = struct{}{}
	}
	for _, path := range next.Files {
		nextSet[path] = struct{}{}
	}

	for oldPath := range prevSet {
		if _, exists := nextSet[oldPath]; exists {
			continue
		}
		_ = os.Remove(filepath.Join(projectDir, oldPath))
	}

	if err := ensureRepoMeta(projectDir); err != nil {
		return err
	}
	path := sidecarPath(projectDir)
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return err
	}

	return syncRepoGitExcludeIfPresent(projectDir, next.Files)
}

func deleteRepoSidecar(projectDir string) error {
	sc, _ := readRepoSidecar(projectDir)
	if sc != nil {
		for _, rel := range sc.Files {
			_ = os.Remove(filepath.Join(projectDir, rel))
		}
		// Clean up managed MCP servers from .vscode/mcp.json
		if len(sc.McpServers) > 0 {
			_ = RemoveMcpServers(projectDir, sc.McpServers)
		}
	}
	_ = os.Remove(sidecarPath(projectDir))
	return removeRepoGitExcludeBlockIfPresent(projectDir)
}
