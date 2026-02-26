package main

import (
	"fmt"
	"path/filepath"
)

// ActivateService is the single API surface for all domain operations.
// Both the TUI and the JSON-RPC daemon call through this struct.
type ActivateService struct {
	ProjectDir string
	Manifests  []Manifest
	Config     Config
	UseRemote  bool
	Repo       string
	Branch     string
}

// NewService creates a fully initialized service instance.
func NewService(projectDir string, manifests []Manifest, cfg Config, useRemote bool, repo, branch string) *ActivateService {
	return &ActivateService{
		ProjectDir: projectDir,
		Manifests:  manifests,
		Config:     cfg,
		UseRemote:  useRemote,
		Repo:       repo,
		Branch:     branch,
	}
}

// refreshConfig re-reads the resolved config from disk.
func (s *ActivateService) refreshConfig() {
	s.Config = ResolveConfig(s.ProjectDir, nil)
}

// ── State queries ──────────────────────────────────────────────

// CategoryInfo describes a file category for UI rendering.
type CategoryInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// StateResult is the full state snapshot returned by GetState.
type StateResult struct {
	ProjectDir       string         `json:"projectDir"`
	InstallDir       string         `json:"installDir"`
	TelemetryLogPath string         `json:"telemetryLogPath,omitempty"`
	State            InstallState   `json:"state"`
	Config           Config         `json:"config"`
	Tiers            []ResolvedTier `json:"tiers,omitempty"`
	Categories       []CategoryInfo `json:"categories,omitempty"`
	Files            []FileStatus   `json:"files,omitempty"`
}

// GetState returns the full install/config state for the project.
func (s *ActivateService) GetState() StateResult {
	state := DetectInstallState(s.ProjectDir)
	sidecar, _ := readRepoSidecar(s.ProjectDir)
	chosen := findManifestByID(s.Manifests, s.Config.Manifest)

	// Build category list from known categories
	cats := make([]CategoryInfo, len(categoryOrder))
	for i, id := range categoryOrder {
		label := categoryLabels[id]
		if label == "" {
			label = id
		}
		cats[i] = CategoryInfo{ID: id, Label: label}
	}

	result := StateResult{
		ProjectDir:       s.ProjectDir,
		InstallDir:       ".github",
		TelemetryLogPath: telemetryLogPath(),
		State:            state,
		Config:           s.Config,
		Categories:       cats,
	}
	if chosen != nil {
		result.Tiers = DiscoverAvailableTiers(*chosen)
		result.Files = ComputeFileStatuses(*chosen, sidecar, s.Config, s.ProjectDir)
	}
	return result
}

// ── Config operations ──────────────────────────────────────────

// GetConfig returns config at the requested scope.
func (s *ActivateService) GetConfig(scope string) (*Config, error) {
	switch scope {
	case "global":
		cfg, _ := ReadGlobalConfig()
		if cfg == nil {
			cfg = &Config{}
		}
		return cfg, nil
	case "project":
		cfg, _ := ReadProjectConfig(s.ProjectDir)
		if cfg == nil {
			cfg = &Config{}
		}
		return cfg, nil
	case "resolved", "":
		cfg := ResolveConfig(s.ProjectDir, nil)
		return &cfg, nil
	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
}

// SetConfigResult is the result of a config set operation.
type SetConfigResult struct {
	OK    bool   `json:"ok"`
	Scope string `json:"scope"`
}

// SetConfig writes config values at the requested scope.
// When changing manifest, automatically resets tier to the first available
// tier in the new manifest if the current tier is not valid.
func (s *ActivateService) SetConfig(scope string, updates *Config) (*SetConfigResult, error) {
	if scope == "" {
		scope = "project"
	}
	switch scope {
	case "global":
		if err := WriteGlobalConfig(updates); err != nil {
			return nil, err
		}
	case "project":
		if err := WriteProjectConfig(s.ProjectDir, updates); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid scope for set: %s (use project|global)", scope)
	}
	s.refreshConfig()

	// When manifest changes, validate the tier is valid for the new manifest
	if updates.Manifest != "" {
		chosen := findManifestByID(s.Manifests, s.Config.Manifest)
		if chosen != nil {
			tiers := DiscoverAvailableTiers(*chosen)
			tierValid := false
			for _, t := range tiers {
				if t.ID == s.Config.Tier {
					tierValid = true
					break
				}
			}
			if !tierValid && len(tiers) > 0 {
				tierUpdate := &Config{Tier: tiers[0].ID}
				switch scope {
				case "global":
					_ = WriteGlobalConfig(tierUpdate)
				case "project":
					_ = WriteProjectConfig(s.ProjectDir, tierUpdate)
				}
				s.refreshConfig()
			}
		}
	}

	return &SetConfigResult{OK: true, Scope: scope}, nil
}

// ── Manifest queries ───────────────────────────────────────────

// ListManifests returns all discovered manifests.
func (s *ActivateService) ListManifests() []Manifest {
	return s.Manifests
}

// ListFilesResult groups files by category.
type ListFilesResult struct {
	Manifest   string          `json:"manifest"`
	Tier       string          `json:"tier"`
	Categories []CategoryGroup `json:"categories"`
	TotalFiles int             `json:"totalFiles"`
}

// ListFiles returns files for a manifest, optionally filtered by tier and category.
func (s *ActivateService) ListFiles(manifestID, tierID, category string) (*ListFilesResult, error) {
	if manifestID == "" {
		manifestID = s.Config.Manifest
	}
	if tierID == "" {
		tierID = s.Config.Tier
	}
	chosen := findManifestByID(s.Manifests, manifestID)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", manifestID)
	}

	groups := ListByCategory(chosen.Files, *chosen, tierID, category)
	total := 0
	for _, g := range groups {
		total += len(g.Files)
	}

	return &ListFilesResult{
		Manifest:   chosen.ID,
		Tier:       tierID,
		Categories: groups,
		TotalFiles: total,
	}, nil
}

// ── Install operations ─────────────────────────────────────────

// RepoAddResult is the result of adding managed files to a repo.
type RepoAddResult struct {
	Manifest string `json:"manifest"`
	Tier     string `json:"tier"`
	Count    int    `json:"count"`
}

// RepoAdd installs managed files into the project directory.
func (s *ActivateService) RepoAdd() (*RepoAddResult, error) {
	if err := RepoAdd(s.Manifests, s.Config, s.ProjectDir, s.UseRemote, s.Repo, s.Branch); err != nil {
		return nil, err
	}
	s.refreshConfig()
	return &RepoAddResult{
		Manifest: s.Config.Manifest,
		Tier:     s.Config.Tier,
	}, nil
}

// RepoRemove removes all managed files from the project directory.
func (s *ActivateService) RepoRemove() error {
	return RepoRemove(s.ProjectDir)
}

// SyncResult is the result of a sync operation.
type SyncResult struct {
	Action           string   `json:"action"`
	PreviousVersion  string   `json:"previousVersion,omitempty"`
	AvailableVersion string   `json:"availableVersion,omitempty"`
	Updated          []string `json:"updated,omitempty"`
	Skipped          []string `json:"skipped,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

// Sync detects manifest/tier/version changes and re-injects if needed.
// If the manifest or tier changed, a full reinstall is performed (not just update).
func (s *ActivateService) Sync() (*SyncResult, error) {
	chosen := findManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	sidecar, _ := readRepoSidecar(s.ProjectDir)
	if sidecar == nil {
		return &SyncResult{Action: "none", Reason: "not installed"}, nil
	}

	if !SyncNeeded(*chosen, sidecar, s.Config.Tier) {
		return &SyncResult{
			Action:           "none",
			Reason:           "up to date",
			AvailableVersion: chosen.Version,
		}, nil
	}

	// If manifest or tier changed, do a full reinstall to pick up the correct file set
	if sidecar.Manifest != chosen.ID || sidecar.Tier != s.Config.Tier {
		if err := RepoAdd(s.Manifests, s.Config, s.ProjectDir, s.UseRemote, s.Repo, s.Branch); err != nil {
			return nil, err
		}
		return &SyncResult{
			Action:           "reinstalled",
			PreviousVersion:  sidecar.Version,
			AvailableVersion: chosen.Version,
			Reason:           fmt.Sprintf("manifest/tier changed: %s/%s → %s/%s", sidecar.Manifest, sidecar.Tier, chosen.ID, s.Config.Tier),
		}, nil
	}

	prevVersion := sidecar.Version
	updated, skipped, err := UpdateFiles(*chosen, sidecar, s.Config, s.ProjectDir, s.UseRemote, s.Repo, s.Branch)
	if err != nil {
		return nil, err
	}

	return &SyncResult{
		Action:           "updated",
		PreviousVersion:  prevVersion,
		AvailableVersion: chosen.Version,
		Updated:          updated,
		Skipped:          skipped,
	}, nil
}

// ── Per-file operations ────────────────────────────────────────

// FileResult is a simple ok + file result.
type FileResult struct {
	OK   bool   `json:"ok"`
	File string `json:"file"`
}

// InstallFile installs a single file by dest path.
func (s *ActivateService) InstallFile(file string) (*FileResult, error) {
	chosen := findManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	target := findManifestFile(chosen.Files, file)
	if target == nil {
		return nil, fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	if err := InstallSingleFile(*target, *chosen, s.ProjectDir, s.UseRemote, s.Repo, s.Branch); err != nil {
		return nil, err
	}

	// Clear skipped version on reinstall
	if _, ok := s.Config.SkippedVersions[target.Dest]; ok {
		_ = ClearSkippedVersion(s.ProjectDir, target.Dest)
		s.refreshConfig()
	}

	return &FileResult{OK: true, File: target.Dest}, nil
}

// UninstallFile removes a single file by dest path.
func (s *ActivateService) UninstallFile(file string) (*FileResult, error) {
	if err := UninstallSingleFile(file, s.ProjectDir); err != nil {
		return nil, err
	}
	return &FileResult{OK: true, File: file}, nil
}

// DiffResult holds a unified diff string.
type DiffResult struct {
	File      string `json:"file"`
	Diff      string `json:"diff"`
	Identical bool   `json:"identical"`
}

// DiffFile produces a unified diff between bundled and installed versions.
func (s *ActivateService) DiffFile(file string) (*DiffResult, error) {
	chosen := findManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	target := findManifestFile(chosen.Files, file)
	if target == nil {
		return nil, fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	diff, err := DiffFile(*target, *chosen, s.ProjectDir)
	if err != nil {
		return nil, err
	}

	return &DiffResult{
		File:      target.Dest,
		Diff:      diff,
		Identical: diff == "",
	}, nil
}

// SkipUpdate marks a file's current bundled version as skipped.
func (s *ActivateService) SkipUpdate(file string) (*FileResult, error) {
	chosen := findManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	target := findManifestFile(chosen.Files, file)
	if target == nil {
		return nil, fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	bundledVersion, _ := ReadFileVersion(filepath.Join(chosen.BasePath, target.Src))
	if bundledVersion == "" {
		return nil, fmt.Errorf("no version found in bundled file %s", target.Src)
	}

	if err := SetSkippedVersion(s.ProjectDir, target.Dest, bundledVersion); err != nil {
		return nil, err
	}
	s.refreshConfig()
	return &FileResult{OK: true, File: target.Dest}, nil
}

// SetOverride sets a file override (pinned, excluded, or empty to clear).
func (s *ActivateService) SetOverride(file, override string) (*FileResult, error) {
	if err := SetFileOverride(s.ProjectDir, file, override); err != nil {
		return nil, err
	}
	s.refreshConfig()
	return &FileResult{OK: true, File: file}, nil
}

// ── Update (re-inject tracked files) ───────────────────────────

// UpdateResult is the result of an update operation.
type UpdateResult struct {
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
}

// Update re-installs currently tracked files, respecting skips.
func (s *ActivateService) Update() (*UpdateResult, error) {
	chosen := findManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	sidecar, _ := readRepoSidecar(s.ProjectDir)
	if sidecar == nil {
		return nil, fmt.Errorf("no installed files found; run 'repo add' first")
	}

	updated, skipped, err := UpdateFiles(*chosen, sidecar, s.Config, s.ProjectDir, s.UseRemote, s.Repo, s.Branch)
	if err != nil {
		return nil, err
	}

	return &UpdateResult{Updated: updated, Skipped: skipped}, nil
}

// ── Telemetry ──────────────────────────────────────────────────

// TelemetryRunResult wraps a telemetry entry.
type TelemetryRunResult struct {
	OK    bool            `json:"ok"`
	Entry *TelemetryEntry `json:"entry,omitempty"`
}

// RunTelemetry performs a single telemetry log run using the provided token.
// If token is empty, resolves from env/gh CLI.
func (s *ActivateService) RunTelemetry(token string) (*TelemetryRunResult, error) {
	if !IsTelemetryEnabled(s.Config) {
		return nil, fmt.Errorf("telemetry is not enabled; set telemetryEnabled: true in config")
	}

	if token == "" {
		token = ResolveGitHubToken()
	}
	if token == "" {
		return nil, fmt.Errorf("no GitHub token available; set GITHUB_TOKEN or install gh CLI")
	}

	entry, err := RunTelemetry(token)
	if err != nil {
		return nil, err
	}

	return &TelemetryRunResult{OK: true, Entry: entry}, nil
}

// ReadTelemetryLog returns all telemetry log entries.
func (s *ActivateService) ReadTelemetryLog() ([]TelemetryEntry, error) {
	return ReadTelemetryLog()
}
