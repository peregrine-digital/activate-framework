package engine

import (
	"os"
	"sync"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// PrefetchManifestFiles downloads all file contents for a manifest
// concurrently and returns them keyed by srcPath.  Callers can derive
// version strings via model.ParseFrontmatterVersion and pass cached
// bytes to RepoAdd so tier/manifest changes need zero HTTP calls.
func PrefetchManifestFiles(m model.Manifest, repo, branch string) map[string][]byte {
	type fetchResult struct {
		srcPath string
		data    []byte
	}
	results := make([]fetchResult, len(m.Files))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i, f := range m.Files {
		srcPath := f.Src
		if m.BasePath != "" {
			srcPath = m.BasePath + "/" + f.Src
		}
		wg.Add(1)
		go func(idx int, sp string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			data, _ := storage.FetchFile(sp, repo, branch)
			results[idx] = fetchResult{srcPath: sp, data: data}
		}(i, srcPath)
	}
	wg.Wait()

	cache := make(map[string][]byte, len(m.Files))
	for _, r := range results {
		if r.data != nil {
			cache[r.srcPath] = r.data
		}
	}
	return cache
}

// ComputeFileStatuses builds a status list for every file in the manifest.
//
// If remoteVersions is non-nil it is used as a cache – no HTTP calls are
// made.  Pass nil to fetch each version from the remote (legacy behaviour,
// useful in CLI one-shot commands where latency is acceptable).
func ComputeFileStatuses(m model.Manifest, sidecar *model.RepoSidecar, cfg model.Config, projectDir string, remoteVersions map[string]string) []model.FileStatus {
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

		// Use cached version if available, otherwise fetch from remote.
		if remoteVersions != nil {
			fs.BundledVersion = remoteVersions[srcPath]
		} else {
			repo := cfg.Repo
			branch := cfg.Branch
			if repo == "" {
				repo = storage.DefaultRepo
			}
			if branch == "" {
				branch = storage.DefaultBranch
			}
			bv, _ := storage.ReadFileVersionRemote(srcPath, repo, branch)
			fs.BundledVersion = bv
		}

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
	}

	return state
}
