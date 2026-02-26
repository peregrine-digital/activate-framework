package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	activateDirName  = ".activate"
	globalConfigFile = "config.json"
	defaultManifest  = "activate-framework"
	defaultTier      = "standard"

	// ClearValue is a sentinel passed via configSet to unset a string field.
	ClearValue = "__clear__"
)

// activateBaseDir overrides the base store path for testing.
// Empty means use ~/.activate.
var activateBaseDir string

// storeBase returns the root of all activate state (~/.activate or test override).
func storeBase() string {
	if activateBaseDir != "" {
		return activateBaseDir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, activateDirName)
}

// repoStorePath returns ~/.activate/repos/<sha256>/ for a project directory.
func repoStorePath(projectDir string) string {
	abs, _ := filepath.Abs(projectDir)
	hash := sha256.Sum256([]byte(abs))
	return filepath.Join(storeBase(), "repos", hex.EncodeToString(hash[:]))
}

type repoMeta struct {
	Path string `json:"path"`
}

// ensureRepoMeta writes repo.json metadata alongside the per-repo config.
func ensureRepoMeta(projectDir string) error {
	dir := repoStorePath(projectDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	metaPath := filepath.Join(dir, "repo.json")
	if _, err := os.Stat(metaPath); err == nil {
		return nil // already exists
	}
	abs, _ := filepath.Abs(projectDir)
	data, _ := json.MarshalIndent(repoMeta{Path: abs}, "", "  ")
	return os.WriteFile(metaPath, append(data, '\n'), 0644)
}

// Config is the unified configuration shape used at both layers.
type Config struct {
	Manifest         string            `json:"manifest"`
	Tier             string            `json:"tier"`
	FileOverrides    map[string]string `json:"fileOverrides,omitempty"`
	SkippedVersions  map[string]string `json:"skippedVersions,omitempty"`
	TelemetryEnabled *bool             `json:"telemetryEnabled,omitempty"`
}

// globalConfigPath returns ~/.activate/config.json.
func globalConfigPath() string {
	return filepath.Join(storeBase(), globalConfigFile)
}

// projectConfigPath returns ~/.activate/repos/<hash>/config.json.
func projectConfigPath(projectDir string) string {
	return filepath.Join(repoStorePath(projectDir), "config.json")
}

// readJSONConfig reads and parses a JSON config file. Returns nil if missing.
func readJSONConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil // file doesn't exist — not an error
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, nil // invalid JSON — treat as absent
	}
	return &cfg, nil
}

// ReadGlobalConfig reads ~/.activate/config.json.
func ReadGlobalConfig() (*Config, error) {
	return readJSONConfig(globalConfigPath())
}

// ReadProjectConfig reads per-repo config from ~/.activate/repos/<hash>/config.json.
func ReadProjectConfig(projectDir string) (*Config, error) {
	return readJSONConfig(projectConfigPath(projectDir))
}

// ResolveConfig merges defaults < global < project < overrides.
func ResolveConfig(projectDir string, overrides *Config) Config {
	result := Config{
		Manifest:        defaultManifest,
		Tier:            defaultTier,
		FileOverrides:   make(map[string]string),
		SkippedVersions: make(map[string]string),
	}

	// Global layer
	if g, _ := ReadGlobalConfig(); g != nil {
		mergeInto(&result, g)
	}

	// Project layer
	if projectDir != "" {
		if p, _ := ReadProjectConfig(projectDir); p != nil {
			mergeInto(&result, p)
		}
	}

	// Explicit overrides
	if overrides != nil {
		mergeInto(&result, overrides)
	}

	return result
}

// WriteProjectConfig writes (merge-update) per-repo config to ~/.activate/repos/<hash>/config.json.
func WriteProjectConfig(projectDir string, updates *Config) error {
	if err := ensureRepoMeta(projectDir); err != nil {
		return err
	}
	path := projectConfigPath(projectDir)
	existing, _ := ReadProjectConfig(projectDir)
	base := &Config{
		FileOverrides:   make(map[string]string),
		SkippedVersions: make(map[string]string),
	}
	if existing != nil {
		base = existing
	}
	mergeInto(base, updates)
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// WriteGlobalConfig writes (merge-update) ~/.activate/config.json.
func WriteGlobalConfig(updates *Config) error {
	path := globalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	existing, _ := ReadGlobalConfig()
	base := &Config{
		FileOverrides:   make(map[string]string),
		SkippedVersions: make(map[string]string),
	}
	if existing != nil {
		base = existing
	}
	mergeInto(base, updates)
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// mergeInto applies non-zero fields from src onto dst.
// Use ClearValue ("__clear__") to explicitly unset a string field.
func mergeInto(dst, src *Config) {
	if src.Manifest == ClearValue {
		dst.Manifest = ""
	} else if src.Manifest != "" {
		dst.Manifest = src.Manifest
	}
	if src.Tier == ClearValue {
		dst.Tier = ""
	} else if src.Tier != "" {
		dst.Tier = src.Tier
	}
	if src.FileOverrides != nil {
		if dst.FileOverrides == nil {
			dst.FileOverrides = make(map[string]string)
		}
		for k, v := range src.FileOverrides {
			if v == "" {
				delete(dst.FileOverrides, k) // empty string = clear override
			} else {
				dst.FileOverrides[k] = v
			}
		}
	}
	if src.SkippedVersions != nil {
		if dst.SkippedVersions == nil {
			dst.SkippedVersions = make(map[string]string)
		}
		for k, v := range src.SkippedVersions {
			if v == "" {
				delete(dst.SkippedVersions, k) // empty string = clear skip
			} else {
				dst.SkippedVersions[k] = v
			}
		}
	}
	if src.TelemetryEnabled != nil {
		dst.TelemetryEnabled = src.TelemetryEnabled
	}
}

// SetFileOverride sets a file override ("pinned" or "excluded") in project config.
// Pass empty value to clear the override.
func SetFileOverride(projectDir, dest, override string) error {
	return WriteProjectConfig(projectDir, &Config{
		FileOverrides: map[string]string{dest: override},
	})
}

// SetSkippedVersion marks a file's version as skipped in project config.
func SetSkippedVersion(projectDir, dest, version string) error {
	return WriteProjectConfig(projectDir, &Config{
		SkippedVersions: map[string]string{dest: version},
	})
}

// ClearSkippedVersion removes a skip entry for a file.
func ClearSkippedVersion(projectDir, dest string) error {
	return WriteProjectConfig(projectDir, &Config{
		SkippedVersions: map[string]string{dest: ""},
	})
}
