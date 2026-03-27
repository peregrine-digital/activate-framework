package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

const (
	activateDirName  = ".activate"
	globalConfigFile = "config.json"
)

// ActivateBaseDir overrides the base store path for testing.
// Empty means use ~/.activate. Also respects ACTIVATE_BASE env var.
var ActivateBaseDir string

// StoreBase returns the root of all activate state (~/.activate or test override).
func StoreBase() string {
	if ActivateBaseDir != "" {
		return ActivateBaseDir
	}
	if env := os.Getenv("ACTIVATE_BASE"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, activateDirName)
}

// RepoStorePath returns ~/.activate/repos/<sha256>/ for a project directory.
func RepoStorePath(projectDir string) string {
	abs, _ := filepath.Abs(projectDir)
	hash := sha256.Sum256([]byte(abs))
	return filepath.Join(StoreBase(), "repos", hex.EncodeToString(hash[:]))
}

type repoMeta struct {
	Path string `json:"path"`
}

// EnsureRepoMeta writes repo.json metadata alongside the per-repo config.
func EnsureRepoMeta(projectDir string) error {
	dir := RepoStorePath(projectDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	metaPath := filepath.Join(dir, "repo.json")
	if _, err := os.Stat(metaPath); err == nil {
		return nil
	}
	abs, _ := filepath.Abs(projectDir)
	data, _ := json.MarshalIndent(repoMeta{Path: abs}, "", "  ")
	return os.WriteFile(metaPath, append(data, '\n'), 0644)
}

// GlobalConfigPath returns ~/.activate/config.json.
func GlobalConfigPath() string {
	return filepath.Join(StoreBase(), globalConfigFile)
}

// ProjectConfigPath returns ~/.activate/repos/<hash>/config.json.
func ProjectConfigPath(projectDir string) string {
	return filepath.Join(RepoStorePath(projectDir), "config.json")
}

// readJSONConfig reads and parses a JSON config file. Returns nil if missing.
func readJSONConfig(path string) (*model.Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}
	var cfg model.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, nil
	}
	return &cfg, nil
}

// ReadGlobalConfig reads ~/.activate/config.json.
func ReadGlobalConfig() (*model.Config, error) {
	return readJSONConfig(GlobalConfigPath())
}

// ReadProjectConfig reads per-repo config from ~/.activate/repos/<hash>/config.json.
func ReadProjectConfig(projectDir string) (*model.Config, error) {
	return readJSONConfig(ProjectConfigPath(projectDir))
}

// ResolveConfig merges defaults < global < project < overrides.
func ResolveConfig(projectDir string, overrides *model.Config) model.Config {
	result := model.Config{
		Repo:            DefaultRepo,
		Branch:          DefaultBranch,
		Manifest:        model.DefaultManifest,
		Tier:            model.DefaultTier,
		Preset:          model.DefaultPreset,
		FileOverrides:   make(map[string]string),
		SkippedVersions: make(map[string]string),
	}

	if g, _ := ReadGlobalConfig(); g != nil {
		model.MergeConfig(&result, g)
	}

	if projectDir != "" {
		if p, _ := ReadProjectConfig(projectDir); p != nil {
			model.MergeConfig(&result, p)
		}
	}

	if overrides != nil {
		model.MergeConfig(&result, overrides)
	}

	return result
}

// WriteProjectConfig writes (merge-update) per-repo config.
func WriteProjectConfig(projectDir string, updates *model.Config) error {
	if err := EnsureRepoMeta(projectDir); err != nil {
		return err
	}
	path := ProjectConfigPath(projectDir)
	existing, _ := ReadProjectConfig(projectDir)
	base := &model.Config{
		FileOverrides:   make(map[string]string),
		SkippedVersions: make(map[string]string),
	}
	if existing != nil {
		base = existing
	}
	model.MergeConfig(base, updates)
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// WriteGlobalConfig writes (merge-update) ~/.activate/config.json.
func WriteGlobalConfig(updates *model.Config) error {
	path := GlobalConfigPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	existing, _ := ReadGlobalConfig()
	base := &model.Config{
		FileOverrides:   make(map[string]string),
		SkippedVersions: make(map[string]string),
	}
	if existing != nil {
		base = existing
	}
	model.MergeConfig(base, updates)
	data, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

// SetFileOverride sets a file override ("pinned" or "excluded") in project config.
func SetFileOverride(projectDir, dest, override string) error {
	return WriteProjectConfig(projectDir, &model.Config{
		FileOverrides: map[string]string{dest: override},
	})
}

// SetSkippedVersion marks a file's version as skipped in project config.
func SetSkippedVersion(projectDir, dest, version string) error {
	return WriteProjectConfig(projectDir, &model.Config{
		SkippedVersions: map[string]string{dest: version},
	})
}

// ClearSkippedVersion removes a skip entry for a file.
func ClearSkippedVersion(projectDir, dest string) error {
	return WriteProjectConfig(projectDir, &model.Config{
		SkippedVersions: map[string]string{dest: ""},
	})
}

// ReadFileVersion reads a file and extracts its frontmatter version.
func ReadFileVersion(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return model.ParseFrontmatterVersion(data), nil
}

// ReadFileVersionRemote fetches a file from GitHub and extracts its frontmatter version.
func ReadFileVersionRemote(filePath, repo, branch string) (string, error) {
	data, err := FetchFile(filePath, repo, branch)
	if err != nil {
		return "", err
	}
	return model.ParseFrontmatterVersion(data), nil
}
