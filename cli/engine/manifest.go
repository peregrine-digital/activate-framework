package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

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

	known := []string{"adhoc", "ironarch"}
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
	return model.Manifest{
		ID:          id,
		Name:        name,
		Description: raw.Description,
		BasePath:    raw.BasePath,
		Tiers:       raw.Tiers,
		Files:       raw.Files,
	}, nil
}

// ── Preset discovery ────────────────────────────────────────────

// presetJSON is the raw shape of a preset JSON file.
type presetJSON struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Extends     string          `json:"extends,omitempty"`
	Files       json.RawMessage `json:"files"` // array of strings or {src,dest} objects
}

// DiscoverRemotePresets fetches all preset metadata from GitHub.
// It autodiscovers plugins by listing plugins/ subdirectories,
// then lists presets/*.json inside each plugin.
func DiscoverRemotePresets(repo, branch string) ([]model.Preset, error) {
	// 1. Discover plugins by listing plugins/ directory.
	plugins, err := storage.ListDirEntries("plugins", repo, branch, "dir")
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}

	var results []model.Preset
	for _, plugin := range plugins {
		// 2. List preset files in plugins/<plugin>/presets/.
		files, err := storage.ListDirEntries(
			fmt.Sprintf("plugins/%s/presets", plugin), repo, branch, "file",
		)
		if err != nil {
			continue // plugin may not have presets
		}

		// 3. Load each .json preset file.
		for _, f := range files {
			name := strings.TrimSuffix(f, ".json")
			if name == f {
				continue // skip non-JSON files
			}
			p, err := loadRemotePreset(plugin, name, repo, branch)
			if err != nil {
				continue
			}
			results = append(results, p)
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no presets found in %s@%s", repo, branch)
	}
	return results, nil
}

// loadRemotePreset fetches and parses a single preset file.
func loadRemotePreset(plugin, name, repo, branch string) (model.Preset, error) {
	filePath := fmt.Sprintf("plugins/%s/presets/%s.json", plugin, name)
	var raw presetJSON
	if err := storage.FetchJSON(filePath, repo, branch, &raw); err != nil {
		return model.Preset{}, err
	}

	displayName := raw.Name
	if displayName == "" {
		displayName = name
	}

	// Parse files array — each entry is either a string or {"src":"...","dest":"..."}.
	files, err := parsePresetFiles(plugin, raw.Files)
	if err != nil {
		return model.Preset{}, err
	}

	return model.Preset{
		ID:          plugin + "/" + name,
		Name:        displayName,
		Description: raw.Description,
		Plugin:      plugin,
		Extends:     raw.Extends,
		Files:       files,
	}, nil
}

// parsePresetFiles parses the files array from a preset JSON.
// Each entry is either:
//   - a string like "instructions/foo.md" (relative to plugin dir)
//   - a string like "@otherplugin/path" (cross-plugin reference)
//   - an object {"src": "...", "dest": "..."}
func parsePresetFiles(plugin string, raw json.RawMessage) ([]model.PresetFile, error) {
	if raw == nil {
		return nil, nil
	}
	var entries []json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}

	var files []model.PresetFile
	for _, entry := range entries {
		var s string
		if err := json.Unmarshal(entry, &s); err == nil {
			f := resolvePresetFileEntry(plugin, s)
			files = append(files, f)
			continue
		}
		var obj struct {
			Src  string `json:"src"`
			Dest string `json:"dest"`
		}
		if err := json.Unmarshal(entry, &obj); err != nil {
			return nil, fmt.Errorf("invalid file entry: %s", string(entry))
		}
		files = append(files, model.PresetFile{
			Src:  obj.Src,
			Dest: obj.Dest,
		})
	}
	return files, nil
}

// resolvePresetFileEntry resolves a string file entry to a PresetFile.
func resolvePresetFileEntry(plugin, entry string) model.PresetFile {
	if strings.HasPrefix(entry, "@") {
		// Cross-plugin: @otherplugin/path/to/file
		rest := entry[1:] // strip @
		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			return model.PresetFile{Src: entry, Dest: entry}
		}
		refPlugin := rest[:slashIdx]
		refPath := rest[slashIdx+1:]
		return model.PresetFile{
			Src:  "plugins/" + refPlugin + "/" + refPath,
			Dest: refPath,
		}
	}
	// Local: path relative to plugin dir.
	return model.PresetFile{
		Src:  "plugins/" + plugin + "/" + entry,
		Dest: entry,
	}
}

// ResolvePresetInheritance resolves the full file list for a preset by walking the extends chain.
// Returns a new Preset with all inherited files included (parent first).
func ResolvePresetInheritance(presets []model.Preset, presetID string) (*model.Preset, error) {
	target := model.FindPresetByID(presets, presetID)
	if target == nil {
		return nil, fmt.Errorf("unknown preset: %s", presetID)
	}

	// Walk extends chain, collecting presets from root to leaf.
	chain := []*model.Preset{target}
	visited := map[string]bool{presetID: true}
	current := target
	for current.Extends != "" {
		if visited[current.Extends] {
			return nil, fmt.Errorf("circular extends: %s", current.Extends)
		}
		visited[current.Extends] = true
		parent := model.FindPresetByID(presets, current.Extends)
		if parent == nil {
			return nil, fmt.Errorf("preset %s extends unknown preset %s", current.ID, current.Extends)
		}
		chain = append(chain, parent)
		current = parent
	}

	// Build file list: root first, then each child appends.
	var allFiles []model.PresetFile
	seen := make(map[string]bool)
	for i := len(chain) - 1; i >= 0; i-- {
		for _, f := range chain[i].Files {
			if !seen[f.Dest] {
				allFiles = append(allFiles, f)
				seen[f.Dest] = true
			}
		}
	}

	resolved := *target
	resolved.Files = allFiles
	return &resolved, nil
}

// InstallFilesFromRemote downloads and writes files from GitHub.
func InstallFilesFromRemote(files []model.ManifestFile, basePath, targetDir, repo, branch string) error {
	for _, f := range files {
		srcPath := f.Src
		if basePath != "" {
			srcPath = path.Clean(basePath + "/" + f.Src)
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
		fmt.Fprintf(os.Stderr, "  ✓  %s\n", f.Dest)
	}

	return nil
}
