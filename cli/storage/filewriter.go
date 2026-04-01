package storage

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// WriteManifestFile fetches a manifest file from GitHub and writes it to destPath.
func WriteManifestFile(f model.ManifestFile, basePath, destPath, repo, branch string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	srcPath := f.Src
	if basePath != "" {
		srcPath = path.Clean(basePath + "/" + f.Src)
	}
	data, err := FetchFile(srcPath, repo, branch)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", f.Src, err)
	}
	return os.WriteFile(destPath, data, 0644)
}

// WritePresetFile fetches a preset file from GitHub and writes it to destPath.
// Unlike WriteManifestFile, the Src field is already a full repo-relative path.
func WritePresetFile(f model.PresetFile, destPath, repo, branch string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}
	data, err := FetchFile(f.Src, repo, branch)
	if err != nil {
		return fmt.Errorf("fetch %s: %w", f.Src, err)
	}
	return os.WriteFile(destPath, data, 0644)
}
