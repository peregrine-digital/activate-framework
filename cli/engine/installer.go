package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// InstallFiles copies files from the local bundle to the target directory.
func InstallFiles(files []model.ManifestFile, basePath, targetDir, version, manifestID string) error {
	for _, f := range files {
		src := filepath.Join(basePath, f.Src)
		dest := filepath.Join(targetDir, f.Dest)

		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", src, err)
		}
		if err := os.WriteFile(dest, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
		fmt.Printf("  ✓  %s\n", f.Dest)
	}

	return nil
}

// ResolveBundleDir locates the manifest bundle directory starting from startDir.
func ResolveBundleDir(startDir string) (string, error) {
	if hasManifests(startDir) {
		return startDir, nil
	}

	dir := filepath.Dir(startDir)
	for {
		parent := filepath.Dir(dir)
		if hasManifests(dir) {
			return dir, nil
		}
		if dir == parent {
			break
		}
		dir = parent
	}

	pluginDir := filepath.Join(startDir, "plugins", "activate-framework")
	if hasManifests(pluginDir) {
		return pluginDir, nil
	}

	return "", fmt.Errorf("could not locate manifests/ or manifest.json from %s", startDir)
}

func hasManifests(dir string) bool {
	entries, err := os.ReadDir(filepath.Join(dir, "manifests"))
	if err == nil {
		for _, e := range entries {
			if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
				return true
			}
		}
	}
	_, err = os.Stat(filepath.Join(dir, "manifest.json"))
	return err == nil
}
