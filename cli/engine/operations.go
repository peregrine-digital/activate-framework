package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// UpdateFiles re-installs only currently-tracked files, respecting skipped versions.
func UpdateFiles(m model.Manifest, sidecar *model.RepoSidecar, cfg model.Config, projectDir string, useRemote bool, repo, branch string) (updated []string, skipped []string, err error) {
	if sidecar == nil {
		return nil, nil, fmt.Errorf("no sidecar found; nothing to update")
	}

	installedSet := make(map[string]bool)
	for _, f := range sidecar.Files {
		installedSet[f] = true
	}

	var mcpFiles []model.ManifestFile

	for _, f := range m.Files {
		cat := f.Category
		if cat == "" {
			cat = model.InferCategory(f.Src)
		}
		if cat == "mcp-servers" {
			mcpFiles = append(mcpFiles, f)
			continue
		}

		destRel := ".github/" + f.Dest
		if !installedSet[destRel] {
			continue
		}

		if sv, ok := cfg.SkippedVersions[f.Dest]; ok {
			bv := ""
			if m.BasePath != "" {
				bv, _ = storage.ReadFileVersion(filepath.Join(m.BasePath, f.Src))
			}
			if sv == bv {
				skipped = append(skipped, f.Dest)
				continue
			}
		}

		destPath := filepath.Join(projectDir, destRel)
		if writeErr := storage.WriteManifestFile(f, m.BasePath, destPath, useRemote, repo, branch); writeErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, writeErr)
			continue
		}

		updated = append(updated, f.Dest)
	}

	if len(mcpFiles) > 0 || len(sidecar.McpServers) > 0 {
		names, mcpErr := storage.InjectMcpFromManifest(mcpFiles, m.BasePath, projectDir, sidecar.McpServers)
		if mcpErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", mcpErr)
		} else {
			sidecar.McpServers = names
		}
	}

	sidecar.Version = m.Version
	if err := storage.WriteRepoSidecar(projectDir, *sidecar); err != nil {
		return updated, skipped, err
	}

	return updated, skipped, nil
}

// InstallSingleFile installs one manifest file and updates the sidecar.
func InstallSingleFile(f model.ManifestFile, m model.Manifest, projectDir string, useRemote bool, repo, branch string) error {
	destRel := ".github/" + f.Dest
	destPath := filepath.Join(projectDir, destRel)

	if err := storage.WriteManifestFile(f, m.BasePath, destPath, useRemote, repo, branch); err != nil {
		return err
	}

	sidecar, _ := storage.ReadRepoSidecar(projectDir)
	if sidecar == nil {
		sidecar = &model.RepoSidecar{Manifest: m.ID, Version: m.Version, Tier: ""}
	}
	if !model.ContainsString(sidecar.Files, destRel) {
		sidecar.Files = append(sidecar.Files, destRel)
	}
	return storage.WriteRepoSidecar(projectDir, *sidecar)
}

// UninstallSingleFile removes one file and updates the sidecar.
func UninstallSingleFile(dest string, projectDir string) error {
	destRel := dest
	if !strings.HasPrefix(destRel, ".github/") {
		destRel = ".github/" + destRel
	}

	sidecar, _ := storage.ReadRepoSidecar(projectDir)
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
	return storage.WriteRepoSidecar(projectDir, *sidecar)
}

// DiffFile produces a unified diff between bundled and installed versions.
func DiffFile(f model.ManifestFile, m model.Manifest, projectDir string) (string, error) {
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

	return UnifiedDiff(string(bundled), string(installed), "bundled/"+f.Src, "installed/"+destRel), nil
}

// SyncNeeded checks if the installed state differs from the desired state.
func SyncNeeded(m model.Manifest, sidecar *model.RepoSidecar, tier string) bool {
	if sidecar == nil {
		return false
	}
	return sidecar.Version != m.Version || sidecar.Manifest != m.ID || sidecar.Tier != tier
}
