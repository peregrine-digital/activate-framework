package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	globalConfigDir      = ".activate"
	globalConfigFile     = "config.json"
	projectConfigFile    = ".activate.json"
	defaultManifest      = "activate-framework"
	defaultTier          = "standard"
)

// Config is the unified configuration shape used at both layers.
type Config struct {
	Manifest        string            `json:"manifest"`
	Tier            string            `json:"tier"`
	FileOverrides   map[string]string `json:"fileOverrides,omitempty"`
	SkippedVersions map[string]string `json:"skippedVersions,omitempty"`
}

// globalConfigPath returns ~/.activate/config.json.
func globalConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, globalConfigDir, globalConfigFile)
}

// projectConfigPath returns <projectDir>/.activate.json.
func projectConfigPath(projectDir string) string {
	return filepath.Join(projectDir, projectConfigFile)
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

// ReadProjectConfig reads .activate.json from projectDir.
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

// WriteProjectConfig writes (merge-update) .activate.json.
func WriteProjectConfig(projectDir string, updates *Config) error {
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
func mergeInto(dst, src *Config) {
	if src.Manifest != "" {
		dst.Manifest = src.Manifest
	}
	if src.Tier != "" {
		dst.Tier = src.Tier
	}
	if src.FileOverrides != nil {
		if dst.FileOverrides == nil {
			dst.FileOverrides = make(map[string]string)
		}
		for k, v := range src.FileOverrides {
			dst.FileOverrides[k] = v
		}
	}
	if src.SkippedVersions != nil {
		if dst.SkippedVersions == nil {
			dst.SkippedVersions = make(map[string]string)
		}
		for k, v := range src.SkippedVersions {
			dst.SkippedVersions[k] = v
		}
	}
}
