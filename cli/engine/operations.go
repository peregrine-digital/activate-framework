package engine

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// UpdateFiles re-installs only currently-tracked files, respecting skipped versions.
func UpdateFiles(m model.Manifest, sidecar *model.RepoSidecar, cfg model.Config, projectDir string) (updated []string, skipped []string, err error) {
	if sidecar == nil {
		return nil, nil, fmt.Errorf("no sidecar found; nothing to update")
	}

	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
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
			srcPath := f.Src
			if m.BasePath != "" {
				srcPath = path.Clean(m.BasePath + "/" + f.Src)
			}
			bv, _ := storage.ReadFileVersionRemote(srcPath, repo, branch)
			if sv == bv {
				skipped = append(skipped, f.Dest)
				continue
			}
		}

		destPath := filepath.Join(projectDir, destRel)
		if writeErr := storage.WriteManifestFile(f, m.BasePath, destPath, repo, branch); writeErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, writeErr)
			continue
		}

		updated = append(updated, f.Dest)
	}

	if len(mcpFiles) > 0 || len(sidecar.McpServers) > 0 {
		names, mcpErr := storage.InjectMcpFromManifest(mcpFiles, m.BasePath, projectDir, sidecar.McpServers, repo, branch)
		if mcpErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", mcpErr)
		} else {
			sidecar.McpServers = names
		}
	}

	if err := storage.WriteRepoSidecar(projectDir, *sidecar); err != nil {
		return updated, skipped, err
	}

	return updated, skipped, nil
}

// InstallSingleFile installs one manifest file and updates the sidecar.
func InstallSingleFile(f model.ManifestFile, m model.Manifest, projectDir string, cfg model.Config) error {
	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	destRel := ".github/" + f.Dest
	destPath := filepath.Join(projectDir, destRel)

	if err := storage.WriteManifestFile(f, m.BasePath, destPath, repo, branch); err != nil {
		return err
	}

	sidecar, _ := storage.ReadRepoSidecar(projectDir)
	if sidecar == nil {
		sidecar = &model.RepoSidecar{Manifest: m.ID, Tier: ""}
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

// DiffFile produces a unified diff between remote source and installed versions.
func DiffFile(f model.ManifestFile, m model.Manifest, projectDir string, cfg model.Config) (string, error) {
	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	srcPath := f.Src
	if m.BasePath != "" {
		srcPath = path.Clean(m.BasePath + "/" + f.Src)
	}
	bundled, err := storage.FetchFile(srcPath, repo, branch)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", f.Src, err)
	}

	destRel := ".github/" + f.Dest
	installedPath := filepath.Join(projectDir, destRel)
	installed, err := os.ReadFile(installedPath)
	if err != nil {
		return "", fmt.Errorf("read installed %s: %w", destRel, err)
	}

	return unifiedDiff(string(bundled), string(installed), "remote/"+f.Src, "installed/"+destRel), nil
}

// SyncNeeded checks if the installed state differs from the desired state.
func SyncNeeded(m model.Manifest, sidecar *model.RepoSidecar, tier string) bool {
	if sidecar == nil {
		return false
	}
	return sidecar.Manifest != m.ID || sidecar.Tier != tier
}

// ── Preset-aware operations ─────────────────────────────────────

// PresetUpdateFiles re-installs only currently-tracked files for a preset.
func PresetUpdateFiles(p model.Preset, sidecar *model.RepoSidecar, cfg model.Config, projectDir string) (updated []string, skipped []string, err error) {
	if sidecar == nil {
		return nil, nil, fmt.Errorf("no sidecar found; nothing to update")
	}

	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	installedSet := make(map[string]bool)
	for _, f := range sidecar.Files {
		installedSet[f] = true
	}

	var mcpFiles []model.PresetFile
	for _, f := range p.Files {
		if f.IsDir {
			continue
		}
		cat := f.Category
		if cat == "" {
			cat = model.InferCategory(f.Dest)
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
			bv, _ := storage.ReadFileVersionRemote(f.Src, repo, branch)
			if sv == bv {
				skipped = append(skipped, f.Dest)
				continue
			}
		}

		destPath := filepath.Join(projectDir, destRel)
		if writeErr := storage.WritePresetFile(f, destPath, repo, branch); writeErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, writeErr)
			continue
		}
		updated = append(updated, f.Dest)
	}

	// Handle MCP
	var mcpManifestFiles []model.ManifestFile
	for _, f := range mcpFiles {
		mcpManifestFiles = append(mcpManifestFiles, model.ManifestFile{Src: f.Src, Dest: f.Dest})
	}
	if len(mcpManifestFiles) > 0 || len(sidecar.McpServers) > 0 {
		names, mcpErr := storage.InjectMcpFromManifest(mcpManifestFiles, "", projectDir, sidecar.McpServers, repo, branch)
		if mcpErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", mcpErr)
		} else {
			sidecar.McpServers = names
		}
	}

	if err := storage.WriteRepoSidecar(projectDir, *sidecar); err != nil {
		return updated, skipped, err
	}
	return updated, skipped, nil
}

// PresetInstallSingleFile installs one preset file and updates the sidecar.
func PresetInstallSingleFile(f model.PresetFile, presetID, projectDir string, cfg model.Config) error {
	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	destRel := ".github/" + f.Dest
	destPath := filepath.Join(projectDir, destRel)

	if err := storage.WritePresetFile(f, destPath, repo, branch); err != nil {
		return err
	}

	sidecar, _ := storage.ReadRepoSidecar(projectDir)
	if sidecar == nil {
		sidecar = &model.RepoSidecar{Preset: presetID}
	}
	if !model.ContainsString(sidecar.Files, destRel) {
		sidecar.Files = append(sidecar.Files, destRel)
	}
	return storage.WriteRepoSidecar(projectDir, *sidecar)
}

// PresetDiffFile produces a unified diff for a preset file.
func PresetDiffFile(f model.PresetFile, projectDir string, cfg model.Config) (string, error) {
	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	bundled, err := storage.FetchFile(f.Src, repo, branch)
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", f.Src, err)
	}

	destRel := ".github/" + f.Dest
	installedPath := filepath.Join(projectDir, destRel)
	installed, err := os.ReadFile(installedPath)
	if err != nil {
		return "", fmt.Errorf("read installed %s: %w", destRel, err)
	}

	return unifiedDiff(string(bundled), string(installed), "remote/"+f.Src, "installed/"+destRel), nil
}

// PresetSyncNeeded checks if the installed preset differs from the desired preset.
func PresetSyncNeeded(sidecar *model.RepoSidecar, presetID string) bool {
	if sidecar == nil {
		return false
	}
	if sidecar.Preset != "" {
		return sidecar.Preset != presetID
	}
	// Legacy: check old manifest+tier
	if sidecar.Manifest != "" {
		legacy := model.MigrateManifestTierToPreset(sidecar.Manifest, sidecar.Tier)
		return legacy != presetID
	}
	return false
}
