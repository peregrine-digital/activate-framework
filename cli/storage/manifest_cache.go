package storage

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// ManifestCachePath returns the path to the cached manifest file for a project.
func ManifestCachePath(projectDir string) string {
	return filepath.Join(RepoStorePath(projectDir), "manifest-cache.json")
}

// WriteManifestCache saves manifests to disk for offline fallback.
func WriteManifestCache(projectDir string, manifests []model.Manifest) error {
	if err := EnsureRepoMeta(projectDir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifests, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ManifestCachePath(projectDir), append(data, '\n'), 0644)
}

// ReadManifestCache loads cached manifests from disk. Returns nil if missing.
func ReadManifestCache(projectDir string) ([]model.Manifest, error) {
	data, err := os.ReadFile(ManifestCachePath(projectDir))
	if err != nil {
		return nil, err
	}
	var manifests []model.Manifest
	if err := json.Unmarshal(data, &manifests); err != nil {
		return nil, err
	}
	return manifests, nil
}
