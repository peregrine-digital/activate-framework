package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// WriteManifestFile copies a single manifest file to its destination,
// resolving the source from either a local bundle or a remote GitHub repo.
func WriteManifestFile(f model.ManifestFile, basePath, destPath string, useRemote bool, repo, branch string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	if useRemote {
		srcPath := f.Src
		if basePath != "" {
			srcPath = basePath + "/" + f.Src
		}
		data, err := FetchFile(srcPath, repo, branch)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", f.Src, err)
		}
		return os.WriteFile(destPath, data, 0644)
	}

	srcPath := filepath.Join(basePath, f.Src)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", srcPath, err)
	}
	return os.WriteFile(destPath, data, 0644)
}
