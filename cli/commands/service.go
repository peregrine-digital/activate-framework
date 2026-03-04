package commands

import (
	"fmt"

	"github.com/peregrine-digital/activate-framework/cli/engine"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// ActivateAPI defines the contract for all domain operations.
type ActivateAPI interface {
	Initialize(projectDir string)
	GetState() StateResult
	GetConfig(scope string) (*model.Config, error)
	SetConfig(scope string, updates *model.Config) (*SetConfigResult, error)
	ListManifests() []model.Manifest
	ListFiles(manifestID, tierID, category string) (*ListFilesResult, error)
	RepoAdd() (*RepoAddResult, error)
	RepoRemove() error
	Sync() (*SyncResult, error)
	Update() (*UpdateResult, error)
	InstallFile(file string) (*FileResult, error)
	UninstallFile(file string) (*FileResult, error)
	DiffFile(file string) (*DiffResult, error)
	SkipUpdate(file string) (*FileResult, error)
	SetOverride(file, override string) (*FileResult, error)
	RunTelemetry(token string) (*TelemetryRunResult, error)
	ReadTelemetryLog() ([]model.TelemetryEntry, error)
	RefreshConfig()
	CurrentConfig() model.Config
	CurrentManifests() []model.Manifest
	CurrentProjectDir() string
}

// Compile-time interface check.
var _ ActivateAPI = (*ActivateService)(nil)

// ActivateService is the single API surface for all domain operations.
type ActivateService struct {
	ProjectDir     string
	Manifests      []model.Manifest
	Config         model.Config
	remoteVersions map[string]string // cached remote file versions (srcPath → version)
}

// NewService creates a fully initialized service instance.
func NewService(projectDir string, manifests []model.Manifest, cfg model.Config) *ActivateService {
	return &ActivateService{
		ProjectDir: projectDir,
		Manifests:  manifests,
		Config:     cfg,
	}
}

func (s *ActivateService) refreshConfig() {
	s.Config = storage.ResolveConfig(s.ProjectDir, nil)
}

func (s *ActivateService) Initialize(projectDir string) {
	if projectDir != "" {
		s.ProjectDir = projectDir
		s.refreshConfig()
	}

	// Discover manifests if not already loaded
	if len(s.Manifests) == 0 {
		s.discoverManifests()
	}
}

// discoverManifests fetches manifests from GitHub, falling back to local cache.
func (s *ActivateService) discoverManifests() {
	repo := s.Config.Repo
	branch := s.Config.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	m, err := engine.DiscoverRemoteManifests(repo, branch)
	if err == nil && len(m) > 0 {
		s.Manifests = m
		// Update cache for offline fallback
		if s.ProjectDir != "" {
			_ = storage.WriteManifestCache(s.ProjectDir, m)
		}
		s.refreshRemoteVersions(repo, branch)
		return
	}

	// Fall back to cached manifests
	if s.ProjectDir != "" {
		if cached, cacheErr := storage.ReadManifestCache(s.ProjectDir); cacheErr == nil && len(cached) > 0 {
			s.Manifests = cached
		}
	}
}

// refreshRemoteVersions fetches the remote frontmatter version for every file
// in the active manifest and caches the results so that GetState does not need
// to make per-file HTTP calls.
func (s *ActivateService) refreshRemoteVersions(repo, branch string) {
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return
	}
	s.remoteVersions = engine.FetchRemoteVersions(*chosen, repo, branch)
}

func (s *ActivateService) RefreshConfig()                { s.refreshConfig() }
func (s *ActivateService) CurrentConfig() model.Config   { return s.Config }
func (s *ActivateService) CurrentManifests() []model.Manifest { return s.Manifests }
func (s *ActivateService) CurrentProjectDir() string     { return s.ProjectDir }

// ── Result types ───────────────────────────────────────────────

type CategoryInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type StateResult struct {
	ProjectDir       string               `json:"projectDir"`
	InstallDir       string               `json:"installDir"`
	TelemetryLogPath string               `json:"telemetryLogPath,omitempty"`
	State            model.InstallState   `json:"state"`
	Config           model.Config         `json:"config"`
	Tiers            []model.ResolvedTier `json:"tiers,omitempty"`
	Categories       []CategoryInfo       `json:"categories,omitempty"`
	Files            []model.FileStatus   `json:"files,omitempty"`
}

type SetConfigResult struct {
	OK    bool   `json:"ok"`
	Scope string `json:"scope"`
}

type ListFilesResult struct {
	Manifest   string               `json:"manifest"`
	Tier       string               `json:"tier"`
	Categories []model.CategoryGroup `json:"categories"`
	TotalFiles int                  `json:"totalFiles"`
}

type RepoAddResult struct {
	Manifest string `json:"manifest"`
	Tier     string `json:"tier"`
	Count    int    `json:"count"`
}

type SyncResult struct {
	Action           string   `json:"action"`
	PreviousVersion  string   `json:"previousVersion,omitempty"`
	AvailableVersion string   `json:"availableVersion,omitempty"`
	Updated          []string `json:"updated,omitempty"`
	Skipped          []string `json:"skipped,omitempty"`
	Reason           string   `json:"reason,omitempty"`
}

type FileResult struct {
	OK   bool   `json:"ok"`
	File string `json:"file"`
}

type DiffResult struct {
	File      string `json:"file"`
	Diff      string `json:"diff"`
	Identical bool   `json:"identical"`
}

type UpdateResult struct {
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
}

type TelemetryRunResult struct {
	OK    bool                  `json:"ok"`
	Entry *model.TelemetryEntry `json:"entry,omitempty"`
}

// ── Service methods ────────────────────────────────────────────

func (s *ActivateService) GetState() StateResult {
	state := engine.DetectInstallState(s.ProjectDir)
	sidecar, _ := storage.ReadRepoSidecar(s.ProjectDir)
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)

	cats := make([]CategoryInfo, len(model.CategoryOrder))
	for i, id := range model.CategoryOrder {
		label := model.CategoryLabels[id]
		if label == "" {
			label = id
		}
		cats[i] = CategoryInfo{ID: id, Label: label}
	}

	result := StateResult{
		ProjectDir:       s.ProjectDir,
		InstallDir:       ".github",
		TelemetryLogPath: engine.TelemetryLogPath(),
		State:            state,
		Config:           s.Config,
		Categories:       cats,
	}
	if chosen != nil {
		result.Tiers = model.DiscoverAvailableTiers(*chosen)
		result.Files = engine.ComputeFileStatuses(*chosen, sidecar, s.Config, s.ProjectDir, s.remoteVersions)
	}
	return result
}

func (s *ActivateService) GetConfig(scope string) (*model.Config, error) {
	switch scope {
	case "global":
		cfg, _ := storage.ReadGlobalConfig()
		if cfg == nil {
			cfg = &model.Config{}
		}
		return cfg, nil
	case "project":
		cfg, _ := storage.ReadProjectConfig(s.ProjectDir)
		if cfg == nil {
			cfg = &model.Config{}
		}
		return cfg, nil
	case "resolved", "":
		cfg := storage.ResolveConfig(s.ProjectDir, nil)
		return &cfg, nil
	default:
		return nil, fmt.Errorf("invalid scope: %s", scope)
	}
}

func (s *ActivateService) SetConfig(scope string, updates *model.Config) (*SetConfigResult, error) {
	if scope == "" {
		scope = "project"
	}
	switch scope {
	case "global":
		if err := storage.WriteGlobalConfig(updates); err != nil {
			return nil, err
		}
	case "project":
		if err := storage.WriteProjectConfig(s.ProjectDir, updates); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("invalid scope for set: %s (use project|global)", scope)
	}
	s.refreshConfig()

	// Re-discover manifests if repo or branch changed
	if updates.Repo != "" || updates.Branch != "" {
		s.Manifests = nil
		s.discoverManifests()
	}

	if updates.Manifest != "" {
		chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
		if chosen != nil {
			tiers := model.DiscoverAvailableTiers(*chosen)
			tierValid := false
			for _, t := range tiers {
				if t.ID == s.Config.Tier {
					tierValid = true
					break
				}
			}
			if !tierValid && len(tiers) > 0 {
				tierUpdate := &model.Config{Tier: tiers[0].ID}
				switch scope {
				case "global":
					_ = storage.WriteGlobalConfig(tierUpdate)
				case "project":
					_ = storage.WriteProjectConfig(s.ProjectDir, tierUpdate)
				}
				s.refreshConfig()
			}
		}
		// Refresh version cache for the newly selected manifest
		repo := s.Config.Repo
		branch := s.Config.Branch
		if repo == "" {
			repo = storage.DefaultRepo
		}
		if branch == "" {
			branch = storage.DefaultBranch
		}
		s.refreshRemoteVersions(repo, branch)
	}

	return &SetConfigResult{OK: true, Scope: scope}, nil
}

func (s *ActivateService) ListManifests() []model.Manifest {
	return s.Manifests
}

func (s *ActivateService) ListFiles(manifestID, tierID, category string) (*ListFilesResult, error) {
	if manifestID == "" {
		manifestID = s.Config.Manifest
	}
	if tierID == "" {
		tierID = s.Config.Tier
	}
	chosen := model.FindManifestByID(s.Manifests, manifestID)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", manifestID)
	}

	groups := model.ListByCategory(chosen.Files, *chosen, tierID, category)
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

func (s *ActivateService) RepoAdd() (*RepoAddResult, error) {
	if err := engine.RepoAdd(s.Manifests, s.Config, s.ProjectDir); err != nil {
		return nil, err
	}
	s.refreshConfig()
	return &RepoAddResult{
		Manifest: s.Config.Manifest,
		Tier:     s.Config.Tier,
	}, nil
}

func (s *ActivateService) RepoRemove() error {
	return engine.RepoRemove(s.ProjectDir)
}

func (s *ActivateService) Sync() (*SyncResult, error) {
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	sidecar, _ := storage.ReadRepoSidecar(s.ProjectDir)
	if sidecar == nil {
		return &SyncResult{Action: "none", Reason: "not installed"}, nil
	}

	if !engine.SyncNeeded(*chosen, sidecar, s.Config.Tier) {
		return &SyncResult{
			Action:           "none",
			Reason:           "up to date",
			AvailableVersion: chosen.Version,
		}, nil
	}

	if sidecar.Manifest != chosen.ID || sidecar.Tier != s.Config.Tier {
		if err := engine.RepoAdd(s.Manifests, s.Config, s.ProjectDir); err != nil {
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
	updated, skipped, err := engine.UpdateFiles(*chosen, sidecar, s.Config, s.ProjectDir)
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

func (s *ActivateService) InstallFile(file string) (*FileResult, error) {
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	target := model.FindManifestFile(chosen.Files, file)
	if target == nil {
		return nil, fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	if err := engine.InstallSingleFile(*target, *chosen, s.ProjectDir, s.Config); err != nil {
		return nil, err
	}

	if _, ok := s.Config.SkippedVersions[target.Dest]; ok {
		_ = storage.ClearSkippedVersion(s.ProjectDir, target.Dest)
		s.refreshConfig()
	}

	return &FileResult{OK: true, File: target.Dest}, nil
}

func (s *ActivateService) UninstallFile(file string) (*FileResult, error) {
	if err := engine.UninstallSingleFile(file, s.ProjectDir); err != nil {
		return nil, err
	}
	return &FileResult{OK: true, File: file}, nil
}

func (s *ActivateService) DiffFile(file string) (*DiffResult, error) {
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	target := model.FindManifestFile(chosen.Files, file)
	if target == nil {
		return nil, fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	diff, err := engine.DiffFile(*target, *chosen, s.ProjectDir, s.Config)
	if err != nil {
		return nil, err
	}

	return &DiffResult{
		File:      target.Dest,
		Diff:      diff,
		Identical: diff == "",
	}, nil
}

func (s *ActivateService) SkipUpdate(file string) (*FileResult, error) {
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	target := model.FindManifestFile(chosen.Files, file)
	if target == nil {
		return nil, fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	srcPath := target.Src
	if chosen.BasePath != "" {
		srcPath = chosen.BasePath + "/" + target.Src
	}
	repo := s.Config.Repo
	branch := s.Config.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}
	bundledVersion, _ := storage.ReadFileVersionRemote(srcPath, repo, branch)
	if bundledVersion == "" {
		return nil, fmt.Errorf("no version found in bundled file %s", target.Src)
	}

	if err := storage.SetSkippedVersion(s.ProjectDir, target.Dest, bundledVersion); err != nil {
		return nil, err
	}
	s.refreshConfig()
	return &FileResult{OK: true, File: target.Dest}, nil
}

func (s *ActivateService) SetOverride(file, override string) (*FileResult, error) {
	if err := storage.SetFileOverride(s.ProjectDir, file, override); err != nil {
		return nil, err
	}
	s.refreshConfig()
	return &FileResult{OK: true, File: file}, nil
}

func (s *ActivateService) Update() (*UpdateResult, error) {
	chosen := model.FindManifestByID(s.Manifests, s.Config.Manifest)
	if chosen == nil {
		return nil, fmt.Errorf("unknown manifest: %s", s.Config.Manifest)
	}

	sidecar, _ := storage.ReadRepoSidecar(s.ProjectDir)
	if sidecar == nil {
		return nil, fmt.Errorf("no installed files found; run 'repo add' first")
	}

	updated, skipped, err := engine.UpdateFiles(*chosen, sidecar, s.Config, s.ProjectDir)
	if err != nil {
		return nil, err
	}

	return &UpdateResult{Updated: updated, Skipped: skipped}, nil
}

func (s *ActivateService) RunTelemetry(token string) (*TelemetryRunResult, error) {
	if !engine.IsTelemetryEnabled(s.Config) {
		return nil, fmt.Errorf("telemetry is not enabled; set telemetryEnabled: true in config")
	}

	if token == "" {
		token = engine.ResolveGitHubToken()
	}
	if token == "" {
		return nil, fmt.Errorf("no GitHub token available; set GITHUB_TOKEN or install gh CLI")
	}

	entry, err := engine.RunTelemetry(token)
	if err != nil {
		return nil, err
	}

	return &TelemetryRunResult{OK: true, Entry: entry}, nil
}

func (s *ActivateService) ReadTelemetryLog() ([]model.TelemetryEntry, error) {
	return engine.ReadTelemetryLog()
}
