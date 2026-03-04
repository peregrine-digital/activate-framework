package engine

import (
	"os"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// ComputeFileStatuses builds a status list for every file in the manifest.
func ComputeFileStatuses(m model.Manifest, sidecar *model.RepoSidecar, cfg model.Config, projectDir string) []model.FileStatus {
	allowedTiers := model.GetAllowedFileTiers(m, cfg.Tier)

	installedSet := make(map[string]bool)
	if sidecar != nil {
		for _, f := range sidecar.Files {
			installedSet[f] = true
		}
	}

	result := make([]model.FileStatus, 0, len(m.Files))
	for _, f := range m.Files {
		destRel := ".github/" + f.Dest

		cat := f.Category
		if cat == "" {
			cat = model.InferCategory(f.Src)
		}

		fs := model.FileStatus{
			Dest:        f.Dest,
			DisplayName: model.FileDisplayName(f.Dest),
			Category:    cat,
			Tier:        f.Tier,
			InTier:      allowedTiers[f.Tier],
		}
		if f.Description != "" {
			fs.Description = f.Description
		}

		if ov, ok := cfg.FileOverrides[f.Dest]; ok {
			fs.Override = ov
		}

		fs.Installed = installedSet[destRel]

		srcPath := f.Src
		if m.BasePath != "" {
			srcPath = m.BasePath + "/" + f.Src
		}
		bv, _ := storage.ReadFileVersionRemote(srcPath, cfg.Repo, cfg.Branch)
		fs.BundledVersion = bv

		if fs.Installed {
			iv, _ := storage.ReadFileVersion(projectDir + "/" + destRel)
			fs.InstalledVersion = iv
		}

		if fs.Installed && fs.BundledVersion != "" && fs.InstalledVersion != "" && fs.BundledVersion != fs.InstalledVersion {
			fs.UpdateAvailable = true
		}

		if sv, ok := cfg.SkippedVersions[f.Dest]; ok && sv == fs.BundledVersion {
			fs.Skipped = true
			fs.UpdateAvailable = false
		}

		result = append(result, fs)
	}
	return result
}

// DetectInstallState checks config and install status for state-aware boot flow.
func DetectInstallState(projectDir string) model.InstallState {
	state := model.InstallState{}

	if _, err := os.Stat(storage.GlobalConfigPath()); err == nil {
		state.HasGlobalConfig = true
	}
	if _, err := os.Stat(storage.ProjectConfigPath(projectDir)); err == nil {
		state.HasProjectConfig = true
	}

	if sc, _ := storage.ReadRepoSidecar(projectDir); sc != nil {
		state.HasInstallMarker = true
		state.InstalledManifest = sc.Manifest
		state.InstalledVersion = sc.Version
	}

	return state
}
