package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// manifestJSON is the raw shape of a manifest JSON file.
type manifestJSON struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Version     string             `json:"version"`
	BasePath    string             `json:"basePath"`
	Tiers       []model.TierDef    `json:"tiers,omitempty"`
	Files       []model.ManifestFile `json:"files"`
}

// DiscoverManifests searches for manifests starting from baseDir.
func DiscoverManifests(baseDir string) ([]model.Manifest, error) {
	if ms, err := loadManifestsFromDir(filepath.Join(baseDir, "manifests"), baseDir); err == nil && len(ms) > 0 {
		return ms, nil
	}

	dir := filepath.Dir(baseDir)
	for {
		parent := filepath.Dir(dir)
		if ms, err := loadManifestsFromDir(filepath.Join(dir, "manifests"), dir); err == nil && len(ms) > 0 {
			return ms, nil
		}
		if dir == parent {
			break
		}
		dir = parent
	}

	return loadLegacyManifest(baseDir)
}

func loadManifestsFromDir(manifestsDir, repoRoot string) ([]model.Manifest, error) {
	entries, err := os.ReadDir(manifestsDir)
	if err != nil {
		return nil, err
	}

	var jsonFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonFiles = append(jsonFiles, e.Name())
		}
	}
	if len(jsonFiles) == 0 {
		return nil, fmt.Errorf("no JSON manifests in %s", manifestsDir)
	}
	sort.Strings(jsonFiles)

	var manifests []model.Manifest
	for _, file := range jsonFiles {
		data, err := os.ReadFile(filepath.Join(manifestsDir, file))
		if err != nil {
			continue
		}
		var raw manifestJSON
		if err := json.Unmarshal(data, &raw); err != nil {
			continue
		}
		id := strings.TrimSuffix(file, ".json")
		basePath := repoRoot
		if raw.BasePath != "" {
			basePath = filepath.Join(repoRoot, raw.BasePath)
		}
		name := raw.Name
		if name == "" {
			name = id
		}
		version := raw.Version
		if version == "" {
			version = "unknown"
		}
		manifests = append(manifests, model.Manifest{
			ID:          id,
			Name:        name,
			Description: raw.Description,
			Version:     version,
			BasePath:    basePath,
			Tiers:       raw.Tiers,
			Files:       raw.Files,
		})
	}
	return manifests, nil
}

func loadLegacyManifest(baseDir string) ([]model.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(baseDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var raw manifestJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	version := raw.Version
	if version == "" {
		version = "unknown"
	}
	return []model.Manifest{{
		ID:       "activate-framework",
		Name:     "Activate Framework",
		Version:  version,
		BasePath: baseDir,
		Tiers:    raw.Tiers,
		Files:    raw.Files,
	}}, nil
}

// DiscoverRemoteManifests fetches manifest metadata from GitHub.
func DiscoverRemoteManifests(repo, branch string) ([]model.Manifest, error) {
	var index struct {
		Manifests []string `json:"manifests"`
	}
	if err := storage.FetchJSON("manifests/index.json", repo, branch, &index); err == nil && len(index.Manifests) > 0 {
		var results []model.Manifest
		for _, id := range index.Manifests {
			m, err := LoadRemoteManifest(id, repo, branch)
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
		m, err := LoadRemoteManifest(id, repo, branch)
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

// LoadRemoteManifest fetches a single manifest by ID from GitHub.
func LoadRemoteManifest(id, repo, branch string) (model.Manifest, error) {
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
		destPath := filepath.Join(targetDir, f.Dest)

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
