package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// DiscoverRemoteManifests fetches manifest metadata from GitHub.
func DiscoverRemoteManifests(repo, branch string) ([]model.Manifest, error) {
	var index struct {
		Manifests []string `json:"manifests"`
	}
	if err := storage.FetchJSON("manifests/index.json", repo, branch, &index); err == nil && len(index.Manifests) > 0 {
		var results []model.Manifest
		for _, id := range index.Manifests {
			m, err := loadRemoteManifest(id, repo, branch)
			if err != nil {
				continue
			}
			results = append(results, m)
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	known := []string{"activate-framework", "ironarch"}
	var results []model.Manifest
	for _, id := range known {
		m, err := loadRemoteManifest(id, repo, branch)
		if err != nil {
			continue
		}
		results = append(results, m)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no manifests found in %s@%s", repo, branch)
	}
	return results, nil
}

// manifestJSON is the raw shape of a manifest JSON file.
type manifestJSON struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Version     string               `json:"version"`
	BasePath    string               `json:"basePath"`
	Tiers       []model.TierDef      `json:"tiers,omitempty"`
	Files       []model.ManifestFile `json:"files"`
}

// loadRemoteManifest fetches a single manifest by ID from GitHub.
func loadRemoteManifest(id, repo, branch string) (model.Manifest, error) {
	var raw manifestJSON
	if err := storage.FetchJSON(fmt.Sprintf("manifests/%s.json", id), repo, branch, &raw); err != nil {
		return model.Manifest{}, err
	}
	name := raw.Name
	if name == "" {
		name = id
	}
	version := raw.Version
	if version == "" {
		version = "unknown"
	}
	return model.Manifest{
		ID:          id,
		Name:        name,
		Description: raw.Description,
		Version:     version,
		BasePath:    raw.BasePath,
		Tiers:       raw.Tiers,
		Files:       raw.Files,
	}, nil
}

// InstallFilesFromRemote downloads and writes files from GitHub.
func InstallFilesFromRemote(files []model.ManifestFile, basePath, targetDir, version, manifestID, repo, branch string) error {
	for _, f := range files {
		srcPath := f.Src
		if basePath != "" {
			srcPath = basePath + "/" + f.Src
		}
		destPath := targetDir + "/" + f.Dest

		data, err := storage.FetchFile(srcPath, repo, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, err)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
		fmt.Printf("  ✓  %s\n", f.Dest)
	}

	return nil
}
