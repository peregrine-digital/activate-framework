package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// ── helpers ────────────────────────────────────────────────────

// serveRemoteFiles creates an httptest server serving files from a map.
// Returns the test server, repo, and branch strings. Sets storage.RawBase.
func serveRemoteFiles(t *testing.T, files map[string]string) (*httptest.Server, string, string) {
	t.Helper()
	repo := "test/repo"
	branch := "main"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + repo + "/" + branch + "/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			key := strings.TrimPrefix(r.URL.Path, prefix)
			if content, ok := files[key]; ok {
				w.Write([]byte(content))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	origRaw := storage.RawBase
	origToken := os.Getenv("GITHUB_TOKEN")
	storage.RawBase = raw.URL
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() {
		storage.RawBase = origRaw
		if origToken != "" {
			os.Setenv("GITHUB_TOKEN", origToken)
		}
		raw.Close()
	})

	return raw, repo, branch
}

// mutableFiles is a thread-safe map for test server content that can be updated mid-test.
type mutableFiles struct {
	files map[string]string
}

// serveRemoteMutableFiles creates an httptest server serving files from a mutable map.
func serveRemoteMutableFiles(t *testing.T, mf *mutableFiles) (*httptest.Server, string, string) {
	t.Helper()
	repo := "test/repo"
	branch := "main"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + repo + "/" + branch + "/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			key := strings.TrimPrefix(r.URL.Path, prefix)
			if content, ok := mf.files[key]; ok {
				w.Write([]byte(content))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	origRaw := storage.RawBase
	origToken := os.Getenv("GITHUB_TOKEN")
	storage.RawBase = raw.URL
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() {
		storage.RawBase = origRaw
		if origToken != "" {
			os.Setenv("GITHUB_TOKEN", origToken)
		}
		raw.Close()
	})

	return raw, repo, branch
}

// setupBundle creates a temp project dir and httptest server with one source file.
// Returns the manifest, projectDir, repo, branch, and a mutableFiles reference for updating content.
func setupBundle(t *testing.T) (model.Manifest, string, string, string, *mutableFiles) {
	t.Helper()
	homeDir := t.TempDir()
	old := storage.ActivateBaseDir
	storage.ActivateBaseDir = homeDir
	t.Cleanup(func() { storage.ActivateBaseDir = old })

	projectDir := t.TempDir()

	// .git/info/exclude so sidecar writes succeed
	excludeDir := filepath.Join(projectDir, ".git", "info")
	if err := os.MkdirAll(excludeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	basePath := "plugins/test"
	srcRel := "instructions/test.instructions.md"
	content := "---\nversion: '1.0.0'\n---\n# Test\n"

	mf := &mutableFiles{files: map[string]string{
		basePath + "/" + srcRel: content,
	}}
	_, repo, branch := serveRemoteMutableFiles(t, mf)

	m := model.Manifest{
		ID:       "test-manifest",
		Name:     "Test Manifest",
		Version:  "1.0.0",
		BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: srcRel, Dest: srcRel, Tier: "core", Category: "instructions"},
		},
	}
	return m, projectDir, repo, branch, mf
}

func newTestService(m model.Manifest, projectDir, repo, branch string) *ActivateService {
	cfg := storage.ResolveConfig(projectDir, nil)
	cfg.Manifest = m.ID
	cfg.Tier = "minimal"
	cfg.Repo = repo
	cfg.Branch = branch
	// Write repo/branch to project config so refreshConfig() preserves them
	_ = storage.WriteProjectConfig(projectDir, &model.Config{Repo: repo, Branch: branch})
	return NewService(projectDir, []model.Manifest{m}, cfg)
}

// ── TestNewService ─────────────────────────────────────────────

func TestServiceNewService(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	if svc.ProjectDir != projectDir {
		t.Fatalf("ProjectDir = %q, want %q", svc.ProjectDir, projectDir)
	}
	if len(svc.Manifests) != 1 || svc.Manifests[0].ID != "test-manifest" {
		t.Fatalf("Manifests not set correctly: %+v", svc.Manifests)
	}
	if svc.Config.Manifest != "test-manifest" {
		t.Fatalf("Config.Manifest = %q, want test-manifest", svc.Config.Manifest)
	}
}

// ── TestServiceGetState ────────────────────────────────────────

func TestServiceGetState(t *testing.T) {
	t.Run("no sidecar", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result := svc.GetState()
		if result.ProjectDir != projectDir {
			t.Fatalf("ProjectDir = %q, want %q", result.ProjectDir, projectDir)
		}
		if result.State.HasInstallMarker {
			t.Fatal("expected no install marker")
		}
	})

	t.Run("with sidecar", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		// Install files to create sidecar
		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		result := svc.GetState()
		if !result.State.HasInstallMarker {
			t.Fatal("expected install marker after RepoAdd")
		}
		if result.State.InstalledManifest != "test-manifest" {
			t.Fatalf("InstalledManifest = %q, want test-manifest", result.State.InstalledManifest)
		}
		if len(result.Files) == 0 {
			t.Fatal("expected file statuses")
		}
	})
}

// ── TestServiceGetConfig ───────────────────────────────────────

func TestServiceGetConfig(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	// Write a project config
	if err := storage.WriteProjectConfig(projectDir, &model.Config{Manifest: "test-manifest", Tier: "minimal"}); err != nil {
		t.Fatal(err)
	}
	svc := newTestService(m, projectDir, repo, branch)

	t.Run("global", func(t *testing.T) {
		cfg, err := svc.GetConfig("global")
		if err != nil {
			t.Fatal(err)
		}
		if cfg == nil {
			t.Fatal("expected non-nil config")
		}
	})

	t.Run("project", func(t *testing.T) {
		cfg, err := svc.GetConfig("project")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Manifest != "test-manifest" {
			t.Fatalf("Manifest = %q, want test-manifest", cfg.Manifest)
		}
	})

	t.Run("resolved", func(t *testing.T) {
		cfg, err := svc.GetConfig("resolved")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Manifest != "test-manifest" {
			t.Fatalf("Manifest = %q, want test-manifest", cfg.Manifest)
		}
	})

	t.Run("empty scope defaults to resolved", func(t *testing.T) {
		cfg, err := svc.GetConfig("")
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Manifest != "test-manifest" {
			t.Fatalf("Manifest = %q, want test-manifest", cfg.Manifest)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		_, err := svc.GetConfig("bogus")
		if err == nil {
			t.Fatal("expected error for invalid scope")
		}
		if !strings.Contains(err.Error(), "invalid scope") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ── TestServiceSetConfig ───────────────────────────────────────

func TestServiceSetConfig(t *testing.T) {
	t.Run("project scope", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.SetConfig("project", &model.Config{Tier: "advanced"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Scope != "project" {
			t.Fatalf("unexpected result: %+v", result)
		}

		// Verify persisted
		cfg, _ := storage.ReadProjectConfig(projectDir)
		if cfg.Tier != "advanced" {
			t.Fatalf("Tier not persisted: got %q", cfg.Tier)
		}

		// Verify service config refreshed
		if svc.Config.Tier != "advanced" {
			t.Fatalf("service config not refreshed: Tier = %q", svc.Config.Tier)
		}
	})

	t.Run("global scope", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.SetConfig("global", &model.Config{Tier: "advanced"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Scope != "global" {
			t.Fatalf("unexpected result: %+v", result)
		}

		cfg, _ := storage.ReadGlobalConfig()
		if cfg.Tier != "advanced" {
			t.Fatalf("Tier not persisted globally: got %q", cfg.Tier)
		}
	})

	t.Run("empty scope defaults to project", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.SetConfig("", &model.Config{Tier: "advanced"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Scope != "project" {
			t.Fatalf("expected project scope, got %q", result.Scope)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		_, err := svc.SetConfig("bogus", &model.Config{Tier: "advanced"})
		if err == nil {
			t.Fatal("expected error for invalid scope")
		}
	})

	t.Run("changing manifest resets invalid tier", func(t *testing.T) {
		homeDir := t.TempDir()
		old := storage.ActivateBaseDir
		storage.ActivateBaseDir = homeDir
		t.Cleanup(func() { storage.ActivateBaseDir = old })

		projectDir := t.TempDir()
		excludeDir := filepath.Join(projectDir, ".git", "info")
		os.MkdirAll(excludeDir, 0755)
		os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

		bundleDir := t.TempDir()
		srcRel := "instructions/test.instructions.md"
		srcPath := filepath.Join(bundleDir, srcRel)
		os.MkdirAll(filepath.Dir(srcPath), 0755)
		os.WriteFile(srcPath, []byte("---\nversion: '1.0.0'\n---\n# Test\n"), 0644)

		// Manifest A has tier "alpha"
		mA := model.Manifest{
			ID: "manifest-a", Version: "1.0.0", BasePath: bundleDir,
			Files: []model.ManifestFile{{Src: srcRel, Dest: srcRel, Tier: "alpha", Category: "instructions"}},
			Tiers: []model.TierDef{{ID: "alpha", Label: "Alpha"}},
		}
		// Manifest B has tier "beta" — "alpha" is NOT valid here
		mB := model.Manifest{
			ID: "manifest-b", Version: "1.0.0", BasePath: bundleDir,
			Files: []model.ManifestFile{{Src: srcRel, Dest: srcRel, Tier: "beta", Category: "instructions"}},
			Tiers: []model.TierDef{{ID: "beta", Label: "Beta"}},
		}

		cfg := storage.ResolveConfig(projectDir, nil)
		cfg.Manifest = "manifest-a"
		cfg.Tier = "alpha"
		svc := NewService(projectDir, []model.Manifest{mA, mB}, cfg)

		// Switch to manifest-b — tier "alpha" is invalid for it
		result, err := svc.SetConfig("project", &model.Config{Manifest: "manifest-b"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK {
			t.Fatal("expected OK")
		}

		// Tier should have been auto-reset to "beta" (first tier of manifest-b)
		if svc.Config.Tier != "beta" {
			t.Fatalf("expected tier auto-reset to 'beta', got %q", svc.Config.Tier)
		}
	})
}

func TestServiceListManifests(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	manifests := svc.ListManifests()
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0].ID != "test-manifest" {
		t.Fatalf("ID = %q, want test-manifest", manifests[0].ID)
	}
}

// ── TestServiceListFiles ───────────────────────────────────────

func TestServiceListFiles(t *testing.T) {
	t.Run("defaults from config", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.ListFiles("", "", "")
		if err != nil {
			t.Fatal(err)
		}
		if result.Manifest != "test-manifest" {
			t.Fatalf("Manifest = %q, want test-manifest", result.Manifest)
		}
		if result.TotalFiles != 1 {
			t.Fatalf("TotalFiles = %d, want 1", result.TotalFiles)
		}
	})

	t.Run("explicit manifest and tier", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.ListFiles("test-manifest", "minimal", "instructions")
		if err != nil {
			t.Fatal(err)
		}
		if result.TotalFiles != 1 {
			t.Fatalf("TotalFiles = %d, want 1", result.TotalFiles)
		}
		if len(result.Categories) != 1 || result.Categories[0].Category != "instructions" {
			t.Fatalf("unexpected categories: %+v", result.Categories)
		}
	})

	t.Run("unknown manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		_, err := svc.ListFiles("no-such-manifest", "", "")
		if err == nil {
			t.Fatal("expected error for unknown manifest")
		}
		if !strings.Contains(err.Error(), "unknown manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ── TestServiceRepoAdd ─────────────────────────────────────────

func TestServiceRepoAdd(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	result, err := svc.RepoAdd()
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest != "test-manifest" {
		t.Fatalf("Manifest = %q, want test-manifest", result.Manifest)
	}

	// Verify file installed
	destPath := filepath.Join(projectDir, ".github", "instructions", "test.instructions.md")
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected installed file, err=%v", err)
	}
	if !strings.Contains(string(data), "version: '1.0.0'") {
		t.Fatalf("unexpected file content: %q", string(data))
	}

	// Verify sidecar exists
	if _, err := os.Stat(storage.SidecarPath(projectDir)); err != nil {
		t.Fatalf("expected sidecar, err=%v", err)
	}
}

// ── TestServiceRepoRemove ──────────────────────────────────────

func TestServiceRepoRemove(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	// Install first
	if _, err := svc.RepoAdd(); err != nil {
		t.Fatal(err)
	}
	destPath := filepath.Join(projectDir, ".github", "instructions", "test.instructions.md")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatal("file should exist after add")
	}

	// Remove
	if err := svc.RepoRemove(); err != nil {
		t.Fatal(err)
	}

	// Verify file removed
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, err=%v", err)
	}
	// Verify sidecar removed
	if _, err := os.Stat(storage.SidecarPath(projectDir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar removed, err=%v", err)
	}
}

// ── TestServiceSync ────────────────────────────────────────────

func TestServiceSync(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.Sync()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "none" || result.Reason != "not installed" {
			t.Fatalf("unexpected sync result: %+v", result)
		}
	})

	t.Run("up to date", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		result, err := svc.Sync()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "none" || result.Reason != "up to date" {
			t.Fatalf("expected up to date, got: %+v", result)
		}
	})

	t.Run("version mismatch triggers update", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		// Tamper sidecar version to simulate mismatch
		sc, _ := storage.ReadRepoSidecar(projectDir)
		sc.Version = "0.9.0"
		scData, _ := json.MarshalIndent(sc, "", "  ")
		if err := os.WriteFile(storage.SidecarPath(projectDir), append(scData, '\n'), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := svc.Sync()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "updated" {
			t.Fatalf("expected updated, got %q", result.Action)
		}
		if result.PreviousVersion != "0.9.0" {
			t.Fatalf("PreviousVersion = %q, want 0.9.0", result.PreviousVersion)
		}
		if result.AvailableVersion != "1.0.0" {
			t.Fatalf("AvailableVersion = %q, want 1.0.0", result.AvailableVersion)
		}
	})
	t.Run("tier change triggers reinstall", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		// Change the tier in config
		svc.Config.Tier = "standard"

		result, err := svc.Sync()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "reinstalled" {
			t.Fatalf("expected reinstalled, got %q", result.Action)
		}
		if !strings.Contains(result.Reason, "manifest/tier changed") {
			t.Fatalf("expected reason about tier change, got %q", result.Reason)
		}
	})

	t.Run("manifest change triggers reinstall", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		// Add a second manifest and switch to it (same basePath, same files served)
		m2 := model.Manifest{
			ID: "other-manifest", Version: "1.0.0", BasePath: m.BasePath,
			Files: []model.ManifestFile{
				{Src: "instructions/test.instructions.md", Dest: "instructions/test.instructions.md", Tier: "core", Category: "instructions"},
			},
		}
		svc.Manifests = append(svc.Manifests, m2)
		svc.Config.Manifest = "other-manifest"

		result, err := svc.Sync()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "reinstalled" {
			t.Fatalf("expected reinstalled, got %q", result.Action)
		}
	})
}

// ── TestServiceUpdate ──────────────────────────────────────────

func TestServiceUpdate(t *testing.T) {
	t.Run("updates installed files", func(t *testing.T) {
		m, projectDir, repo, branch, mf := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		// Change the remote source file content
		mf.files[m.BasePath+"/instructions/test.instructions.md"] = "---\nversion: '2.0.0'\n---\n# Updated\n"

		result, err := svc.Update()
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Updated) == 0 {
			t.Fatal("expected at least one updated file")
		}

		// Verify content was updated
		destPath := filepath.Join(projectDir, ".github", "instructions", "test.instructions.md")
		data, _ := os.ReadFile(destPath)
		if !strings.Contains(string(data), "version: '2.0.0'") {
			t.Fatalf("file not updated: %q", string(data))
		}
	})

	t.Run("no sidecar errors", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		_, err := svc.Update()
		if err == nil {
			t.Fatal("expected error when no sidecar")
		}
	})
}

// ── TestServiceInstallFile ─────────────────────────────────────

func TestServiceInstallFile(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	result, err := svc.InstallFile("instructions/test.instructions.md")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}
	if result.File != "instructions/test.instructions.md" {
		t.Fatalf("File = %q", result.File)
	}

	// Verify in sidecar
	sc, _ := storage.ReadRepoSidecar(projectDir)
	if sc == nil {
		t.Fatal("expected sidecar to exist")
	}
	found := false
	for _, f := range sc.Files {
		if f == ".github/instructions/test.instructions.md" {
			found = true
		}
	}
	if !found {
		t.Fatalf("file not in sidecar: %v", sc.Files)
	}
}

// ── TestServiceUninstallFile ───────────────────────────────────

func TestServiceUninstallFile(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	// Install first
	if _, err := svc.InstallFile("instructions/test.instructions.md"); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(projectDir, ".github", "instructions", "test.instructions.md")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatal("file should exist after install")
	}

	// Uninstall
	result, err := svc.UninstallFile("instructions/test.instructions.md")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}

	// Verify file removed
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("expected file removed, err=%v", err)
	}

	// Verify removed from sidecar
	sc, _ := storage.ReadRepoSidecar(projectDir)
	if sc != nil {
		for _, f := range sc.Files {
			if strings.Contains(f, "test.instructions.md") {
				t.Fatal("file still in sidecar after uninstall")
			}
		}
	}
}

// ── TestServiceDiffFile ────────────────────────────────────────

func TestServiceDiffFile(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	// Install the file
	if _, err := svc.InstallFile("instructions/test.instructions.md"); err != nil {
		t.Fatal(err)
	}

	t.Run("identical", func(t *testing.T) {
		result, err := svc.DiffFile("instructions/test.instructions.md")
		if err != nil {
			t.Fatal(err)
		}
		if !result.Identical {
			t.Fatalf("expected identical, got diff:\n%s", result.Diff)
		}
	})

	t.Run("modified", func(t *testing.T) {
		// Modify the installed file
		destPath := filepath.Join(projectDir, ".github", "instructions", "test.instructions.md")
		if err := os.WriteFile(destPath, []byte("---\nversion: '1.0.0'\n---\n# Modified locally\n"), 0644); err != nil {
			t.Fatal(err)
		}

		result, err := svc.DiffFile("instructions/test.instructions.md")
		if err != nil {
			t.Fatal(err)
		}
		if result.Identical {
			t.Fatal("expected non-identical after modification")
		}
		if result.Diff == "" {
			t.Fatal("expected non-empty diff")
		}
	})
}

// ── TestServiceSkipUpdate ──────────────────────────────────────

func TestServiceSkipUpdate(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	result, err := svc.SkipUpdate("instructions/test.instructions.md")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}

	// Verify persisted in config
	cfg, _ := storage.ReadProjectConfig(projectDir)
	if cfg == nil {
		t.Fatal("expected project config")
	}
	if cfg.SkippedVersions["instructions/test.instructions.md"] != "1.0.0" {
		t.Fatalf("skip not persisted: %v", cfg.SkippedVersions)
	}

	// Verify service config refreshed
	if svc.Config.SkippedVersions["instructions/test.instructions.md"] != "1.0.0" {
		t.Fatalf("service config not refreshed: %v", svc.Config.SkippedVersions)
	}
}

// ── TestServiceSetOverride ─────────────────────────────────────

func TestServiceSetOverride(t *testing.T) {
	t.Run("pinned", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		result, err := svc.SetOverride("instructions/test.instructions.md", "pinned")
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK {
			t.Fatal("expected OK=true")
		}

		cfg, _ := storage.ReadProjectConfig(projectDir)
		if cfg.FileOverrides["instructions/test.instructions.md"] != "pinned" {
			t.Fatalf("override not set: %v", cfg.FileOverrides)
		}
	})

	t.Run("excluded", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.SetOverride("instructions/test.instructions.md", "excluded"); err != nil {
			t.Fatal(err)
		}
		cfg, _ := storage.ReadProjectConfig(projectDir)
		if cfg.FileOverrides["instructions/test.instructions.md"] != "excluded" {
			t.Fatalf("override not set: %v", cfg.FileOverrides)
		}
	})

	t.Run("clear", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		if _, err := svc.SetOverride("instructions/test.instructions.md", "pinned"); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SetOverride("instructions/test.instructions.md", ""); err != nil {
			t.Fatal(err)
		}
		cfg, _ := storage.ReadProjectConfig(projectDir)
		if _, ok := cfg.FileOverrides["instructions/test.instructions.md"]; ok {
			t.Fatal("override should be cleared")
		}
	})
}

// ── TestServiceRunTelemetry ────────────────────────────────────

func TestServiceRunTelemetry(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)
		// TelemetryEnabled is nil by default (disabled)

		_, err := svc.RunTelemetry("")
		if err == nil {
			t.Fatal("expected error when telemetry disabled")
		}
		if !strings.Contains(err.Error(), "telemetry is not enabled") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("no token", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		enabled := true
		svc := newTestService(m, projectDir, repo, branch)
		svc.Config.TelemetryEnabled = &enabled

		// Ensure no token is available
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")
		_, err := svc.RunTelemetry("")
		if err == nil {
			t.Skip("gh CLI resolved a token; skipping no-token assertion")
		}
	})
}

// ── TestServiceReadTelemetryLog ────────────────────────────────

func TestServiceReadTelemetryLog(t *testing.T) {
	m, projectDir, repo, branch, _ := setupBundle(t)
	svc := newTestService(m, projectDir, repo, branch)

	entries, err := svc.ReadTelemetryLog()
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil && len(entries) != 0 {
		t.Fatalf("expected empty log, got %d entries", len(entries))
	}
}

// ── Error Path Tests ───────────────────────────────────────────

func TestServiceInstallFileErrors(t *testing.T) {
	t.Run("unknown manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)
		svc.Config.Manifest = "nonexistent"

		_, err := svc.InstallFile("instructions/test.instructions.md")
		if err == nil {
			t.Fatal("expected error for unknown manifest")
		}
		if !strings.Contains(err.Error(), "unknown manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("file not in manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		_, err := svc.InstallFile("agents/nonexistent.md")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "not found in manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestServiceDiffFileErrors(t *testing.T) {
	t.Run("unknown manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)
		svc.Config.Manifest = "nonexistent"

		_, err := svc.DiffFile("instructions/test.instructions.md")
		if err == nil {
			t.Fatal("expected error for unknown manifest")
		}
		if !strings.Contains(err.Error(), "unknown manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("file not in manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		_, err := svc.DiffFile("agents/nonexistent.md")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "not found in manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestServiceSkipUpdateErrors(t *testing.T) {
	t.Run("unknown manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)
		svc.Config.Manifest = "nonexistent"

		_, err := svc.SkipUpdate("instructions/test.instructions.md")
		if err == nil {
			t.Fatal("expected error for unknown manifest")
		}
		if !strings.Contains(err.Error(), "unknown manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("file not in manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)

		_, err := svc.SkipUpdate("agents/nonexistent.md")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
		if !strings.Contains(err.Error(), "not found in manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestServiceUpdateErrors(t *testing.T) {
	t.Run("unknown manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)
		svc.Config.Manifest = "nonexistent"

		_, err := svc.Update()
		if err == nil {
			t.Fatal("expected error for unknown manifest")
		}
		if !strings.Contains(err.Error(), "unknown manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestServiceSyncErrors(t *testing.T) {
	t.Run("unknown manifest", func(t *testing.T) {
		m, projectDir, repo, branch, _ := setupBundle(t)
		svc := newTestService(m, projectDir, repo, branch)
		svc.Config.Manifest = "nonexistent"

		_, err := svc.Sync()
		if err == nil {
			t.Fatal("expected error for unknown manifest")
		}
		if !strings.Contains(err.Error(), "unknown manifest") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
