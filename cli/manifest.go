package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ManifestFile represents a single file entry in a manifest.
type ManifestFile struct {
	Src         string `json:"src"`
	Dest        string `json:"dest"`
	Tier        string `json:"tier"`
	Category    string `json:"category,omitempty"`
	Description string `json:"description,omitempty"`
}

// TierDef is a tier definition within a manifest.
type TierDef struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
}

// manifestJSON is the raw shape of a manifest JSON file.
type manifestJSON struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Version     string         `json:"version"`
	BasePath    string         `json:"basePath"`
	Tiers       []TierDef      `json:"tiers,omitempty"`
	Files       []ManifestFile `json:"files"`
}

// Manifest is the fully resolved in-memory representation.
type Manifest struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Version     string         `json:"version"`
	BasePath    string         `json:"basePath"` // resolved absolute path (local) or relative prefix (remote)
	Tiers       []TierDef      `json:"tiers,omitempty"`
	Files       []ManifestFile `json:"files"`
}

// ── Discovery ───────────────────────────────────────────────────

// DiscoverManifests searches for manifests starting from baseDir.
// It walks up the directory tree looking for a manifests/ directory
// containing *.json files, then falls back to legacy manifest.json.
func DiscoverManifests(baseDir string) ([]Manifest, error) {
	// 1. Try baseDir/manifests/
	if ms, err := loadManifestsFromDir(filepath.Join(baseDir, "manifests"), baseDir); err == nil && len(ms) > 0 {
		return ms, nil
	}

	// 2. Walk up parents
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

	// 3. Legacy fallback
	return loadLegacyManifest(baseDir)
}

func loadManifestsFromDir(manifestsDir, repoRoot string) ([]Manifest, error) {
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

	var manifests []Manifest
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
		manifests = append(manifests, Manifest{
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

func loadLegacyManifest(baseDir string) ([]Manifest, error) {
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
	return []Manifest{{
		ID:       "activate-framework",
		Name:     "Activate Framework",
		Version:  version,
		BasePath: baseDir,
		Tiers:    raw.Tiers,
		Files:    raw.Files,
	}}, nil
}

// FormatManifestList produces a human-readable summary of manifests.
func FormatManifestList(manifests []Manifest) string {
	var b strings.Builder
	for _, m := range manifests {
		fmt.Fprintf(&b, "  %s\n", m.ID)
		fmt.Fprintf(&b, "    %s (v%s) — %d files\n", m.Name, m.Version, len(m.Files))
		if m.Description != "" {
			fmt.Fprintf(&b, "    %s\n", m.Description)
		}
	}
	return b.String()
}
