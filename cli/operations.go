package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── Update command ──────────────────────────────────────────────

// UpdateFiles re-installs only currently-tracked files, respecting skipped versions.
// Also refreshes MCP server entries from manifest.
func UpdateFiles(m Manifest, sidecar *repoSidecar, cfg Config, projectDir string, useRemote bool, repo, branch string) (updated []string, skipped []string, err error) {
	if sidecar == nil {
		return nil, nil, fmt.Errorf("no sidecar found; nothing to update")
	}

	installedSet := make(map[string]bool)
	for _, f := range sidecar.Files {
		installedSet[f] = true
	}

	// Collect MCP files for batch injection
	var mcpFiles []ManifestFile

	for _, f := range m.Files {
		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Src)
		}
		if cat == "mcp-servers" {
			mcpFiles = append(mcpFiles, f)
			continue
		}

		destRel := ".github/" + f.Dest
		if !installedSet[destRel] {
			continue
		}

		// Check for skipped version
		if sv, ok := cfg.SkippedVersions[f.Dest]; ok {
			bv := ""
			if m.BasePath != "" {
				bv, _ = ReadFileVersion(filepath.Join(m.BasePath, f.Src))
			}
			if sv == bv {
				skipped = append(skipped, f.Dest)
				continue
			}
		}

		destPath := filepath.Join(projectDir, destRel)
		if writeErr := writeManifestFile(f, m.BasePath, destPath, useRemote, repo, branch); writeErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, writeErr)
			continue
		}

		updated = append(updated, f.Dest)
	}

	// Re-inject MCP servers
	if len(mcpFiles) > 0 || len(sidecar.McpServers) > 0 {
		names, mcpErr := InjectMcpFromManifest(mcpFiles, m.BasePath, projectDir, sidecar.McpServers)
		if mcpErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", mcpErr)
		} else {
			sidecar.McpServers = names
		}
	}

	// Update sidecar version
	sidecar.Version = m.Version
	if err := writeRepoSidecar(projectDir, *sidecar); err != nil {
		return updated, skipped, err
	}

	return updated, skipped, nil
}

// ── Per-file install ────────────────────────────────────────────

// InstallSingleFile installs one manifest file and updates the sidecar.
func InstallSingleFile(f ManifestFile, m Manifest, projectDir string, useRemote bool, repo, branch string) error {
	destRel := ".github/" + f.Dest
	destPath := filepath.Join(projectDir, destRel)

	if err := writeManifestFile(f, m.BasePath, destPath, useRemote, repo, branch); err != nil {
		return err
	}

	// Update sidecar
	sidecar, _ := readRepoSidecar(projectDir)
	if sidecar == nil {
		sidecar = &repoSidecar{Manifest: m.ID, Version: m.Version, Tier: ""}
	}
	if !containsString(sidecar.Files, destRel) {
		sidecar.Files = append(sidecar.Files, destRel)
	}
	return writeRepoSidecar(projectDir, *sidecar)
}

// UninstallSingleFile removes one file and updates the sidecar.
func UninstallSingleFile(dest string, projectDir string) error {
	destRel := dest
	if !strings.HasPrefix(destRel, ".github/") {
		destRel = ".github/" + destRel
	}

	sidecar, _ := readRepoSidecar(projectDir)
	if sidecar == nil {
		return fmt.Errorf("no sidecar found; nothing to uninstall")
	}

	newFiles := make([]string, 0, len(sidecar.Files))
	for _, f := range sidecar.Files {
		if f != destRel {
			newFiles = append(newFiles, f)
		}
	}
	sidecar.Files = newFiles
	// writeRepoSidecar diffs old vs new and deletes removed files
	return writeRepoSidecar(projectDir, *sidecar)
}

// ── File diff ───────────────────────────────────────────────────

// DiffFile produces a unified diff between bundled and installed versions.
func DiffFile(f ManifestFile, m Manifest, projectDir string) (string, error) {
	srcPath := filepath.Join(m.BasePath, f.Src)
	bundled, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read bundled %s: %w", f.Src, err)
	}

	destRel := ".github/" + f.Dest
	installedPath := filepath.Join(projectDir, destRel)
	installed, err := os.ReadFile(installedPath)
	if err != nil {
		return "", fmt.Errorf("read installed %s: %w", destRel, err)
	}

	return unifiedDiff(string(bundled), string(installed), "bundled/"+f.Src, "installed/"+destRel), nil
}

// ── Sync ────────────────────────────────────────────────────────

// SyncNeeded checks if the installed state differs from the desired state.
func SyncNeeded(m Manifest, sidecar *repoSidecar, tier string) bool {
	if sidecar == nil {
		return false
	}
	return sidecar.Version != m.Version || sidecar.Manifest != m.ID || sidecar.Tier != tier
}
