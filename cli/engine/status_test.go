package engine

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

func TestReadFileVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	if err := os.WriteFile(path, []byte("---\nversion: '0.3.0'\n---\n# Doc"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := storage.ReadFileVersion(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0.3.0" {
		t.Fatalf("expected 0.3.0, got %q", got)
	}
}

func TestReadFileVersionMissingFile(t *testing.T) {
	_, err := storage.ReadFileVersion("/nonexistent/file.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// serveVersionFiles creates an httptest server for ComputeFileStatuses tests.
func serveVersionFiles(t *testing.T, files map[string]string) (repo, branch string, cleanup func()) {
	t.Helper()
	repo = "test/repo"
	branch = "main"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + repo + "/" + branch + "/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			key := strings.TrimPrefix(r.URL.Path, prefix)
			if content, ok := files[key]; ok {
				w.Write([]byte(content))
				return
			}
		}
		http.NotFound(w, r)
	}))
	oldRaw := storage.RawBase
	oldResolver := storage.TokenResolver
	storage.RawBase = raw.URL
	storage.TokenResolver = func() string { return "" }
	storage.ResetTokenCache()
	cleanup = func() {
		raw.Close()
		storage.RawBase = oldRaw
		storage.TokenResolver = oldResolver
		storage.ResetTokenCache()
	}
	return
}

func TestComputeFileStatusesBasic(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	// Create installed file with older version
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# General"), 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "skills/test.md", Dest: "skills/test.md", Tier: "ad-hoc"},
		},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/instructions/general.md"}}
	cfg := model.Config{}

	remoteVersions := map[string]string{
		basePath + "/instructions/general.md": "0.5.0",
	}

	statuses := ComputeFileStatuses(manifest, sidecar, cfg, projectDir, remoteVersions)
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	s := statuses[0]
	if !s.Installed {
		t.Fatal("expected installed=true for general.md")
	}
	if s.BundledVersion != "0.5.0" {
		t.Fatalf("expected bundled 0.5.0, got %q", s.BundledVersion)
	}
	if s.InstalledVersion != "0.4.0" {
		t.Fatalf("expected installed 0.4.0, got %q", s.InstalledVersion)
	}
	if !s.UpdateAvailable {
		t.Fatal("expected updateAvailable=true")
	}
	if s.Category != "instructions" {
		t.Fatalf("expected category instructions, got %q", s.Category)
	}
	if s.DisplayName != "general" {
		t.Fatalf("expected displayName 'general', got %q", s.DisplayName)
	}

	s2 := statuses[1]
	if s2.Installed {
		t.Fatal("expected installed=false for test.md")
	}
	if s2.UpdateAvailable {
		t.Fatal("expected updateAvailable=false for uninstalled file")
	}
}

func TestComputeFileStatusesSkipped(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	installedPath := filepath.Join(projectDir, ".github", "instructions", "sec.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Sec"), 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "instructions/sec.md", Dest: "instructions/sec.md", Tier: "core"}},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/instructions/sec.md"}}
	cfg := model.Config{
		SkippedVersions: map[string]string{"instructions/sec.md": "0.5.0"},
	}

	remoteVersions := map[string]string{
		basePath + "/instructions/sec.md": "0.5.0",
	}

	statuses := ComputeFileStatuses(manifest, sidecar, cfg, projectDir, remoteVersions)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if !statuses[0].Skipped {
		t.Fatal("expected skipped=true")
	}
	if statuses[0].UpdateAvailable {
		t.Fatal("expected updateAvailable=false when skipped")
	}
}

func TestComputeFileStatusesOverrides(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	repo, branch, cleanup := serveVersionFiles(t, map[string]string{})
	defer cleanup()

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "core"},
			{Src: "b.md", Dest: "b.md", Tier: "core"},
		},
	}
	cfg := model.Config{
		Repo: repo, Branch: branch,
		FileOverrides: map[string]string{
			"a.md": "pinned",
			"b.md": "excluded",
		},
	}

	statuses := ComputeFileStatuses(manifest, nil, cfg, projectDir, nil)
	if statuses[0].Override != "pinned" {
		t.Fatalf("expected pinned override, got %q", statuses[0].Override)
	}
	if statuses[1].Override != "excluded" {
		t.Fatalf("expected excluded override, got %q", statuses[1].Override)
	}
}

func TestComputeFileStatusesNilSidecar(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	repo, branch, cleanup := serveVersionFiles(t, map[string]string{})
	defer cleanup()

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}

	statuses := ComputeFileStatuses(manifest, nil, model.Config{Repo: repo, Branch: branch}, projectDir, nil)
	if len(statuses) != 1 {
		t.Fatalf("expected 1, got %d", len(statuses))
	}
	if statuses[0].Installed {
		t.Fatal("expected not installed with nil sidecar")
	}
}

func TestComputeFileStatusesSameVersion(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	repo, branch, cleanup := serveVersionFiles(t, map[string]string{
		basePath + "/a.md": "---\nversion: '1.0.0'\n---\n",
	})
	defer cleanup()

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '1.0.0'\n---\n"), 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/a.md"}}

	statuses := ComputeFileStatuses(manifest, sidecar, model.Config{Repo: repo, Branch: branch}, projectDir, nil)
	if statuses[0].UpdateAvailable {
		t.Fatal("expected no update when versions match")
	}
}

func TestComputeFileStatusesCachedVersions(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	// No HTTP server needed – cached versions bypass remote fetch entirely.
	oldResolver := storage.TokenResolver
	storage.TokenResolver = func() string { return "" }
	storage.ResetTokenCache()
	defer func() {
		storage.TokenResolver = oldResolver
		storage.ResetTokenCache()
	}()

	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# General"), 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
		},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/instructions/general.md"}}

	cached := map[string]string{
		basePath + "/instructions/general.md": "0.5.0",
	}

	statuses := ComputeFileStatuses(manifest, sidecar, model.Config{}, projectDir, cached)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].BundledVersion != "0.5.0" {
		t.Fatalf("expected bundled 0.5.0 from cache, got %q", statuses[0].BundledVersion)
	}
	if !statuses[0].UpdateAvailable {
		t.Fatal("expected updateAvailable=true")
	}
}

func TestPrefetchManifestFiles(t *testing.T) {
	basePath := "plugins/test"
	repo, branch, cleanup := serveVersionFiles(t, map[string]string{
		basePath + "/a.md": "---\nversion: '1.0.0'\n---\ncontent a",
		basePath + "/b.md": "---\nversion: '2.0.0'\n---\ncontent b",
	})
	defer cleanup()

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "core"},
			{Src: "b.md", Dest: "b.md", Tier: "core"},
		},
	}

	cache := PrefetchManifestFiles(manifest, repo, branch)
	if data, ok := cache[basePath+"/a.md"]; !ok || !strings.Contains(string(data), "content a") {
		t.Fatalf("expected cached content for a.md, got %q", string(data))
	}
	if data, ok := cache[basePath+"/b.md"]; !ok || !strings.Contains(string(data), "content b") {
		t.Fatalf("expected cached content for b.md, got %q", string(data))
	}
	// Verify versions can be derived from cached content
	if v := model.ParseFrontmatterVersion(cache[basePath+"/a.md"]); v != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %q", v)
	}
}

func TestPrefetchManifestFilesRelativePaths(t *testing.T) {
	basePath := "plugins/ironarch"

	// Register files at the RESOLVED paths (what path.Clean produces)
	repo, branch, cleanup := serveVersionFiles(t, map[string]string{
		"skills/ci-debugger/SKILL.md":  "---\nversion: '1.0.0'\n---\nci debugger",
		"skills/pr-writing/SKILL.md":   "---\nversion: '2.0.0'\n---\npr writing",
		basePath + "/agents/planner.md": "---\nversion: '3.0.0'\n---\nplanner",
	})
	defer cleanup()

	manifest := model.Manifest{
		ID: "ironarch", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "../../skills/ci-debugger/SKILL.md", Dest: "skills/ci-debugger/SKILL.md", Tier: "skills"},
			{Src: "../../skills/pr-writing/SKILL.md", Dest: "skills/pr-writing/SKILL.md", Tier: "skills"},
			{Src: "agents/planner.md", Dest: "agents/planner.md", Tier: "core"},
		},
	}

	cache := PrefetchManifestFiles(manifest, repo, branch)

	// Relative paths should resolve and fetch successfully
	if _, ok := cache["skills/ci-debugger/SKILL.md"]; !ok {
		t.Fatal("expected cached content for ../../skills/ci-debugger/SKILL.md (resolved to skills/ci-debugger/SKILL.md)")
	}
	if _, ok := cache["skills/pr-writing/SKILL.md"]; !ok {
		t.Fatal("expected cached content for ../../skills/pr-writing/SKILL.md (resolved to skills/pr-writing/SKILL.md)")
	}
	// Non-relative path should still work
	if _, ok := cache[basePath+"/agents/planner.md"]; !ok {
		t.Fatal("expected cached content for agents/planner.md")
	}
	if len(cache) != 3 {
		t.Fatalf("expected 3 cached files, got %d", len(cache))
	}
}

func TestComputeFileStatusesRelativePaths(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/ironarch"

	// Install a file locally with an older version
	installedPath := filepath.Join(projectDir, ".github", "skills", "ci-debugger", "SKILL.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.9.0'\n---\n# CI Debugger"), 0644)

	manifest := model.Manifest{
		ID: "ironarch", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "../../skills/ci-debugger/SKILL.md", Dest: "skills/ci-debugger/SKILL.md", Tier: "skills"},
		},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/skills/ci-debugger/SKILL.md"}}

	// Remote versions keyed by RESOLVED path (what path.Clean produces)
	remoteVersions := map[string]string{
		"skills/ci-debugger/SKILL.md": "1.0.0",
	}

	statuses := ComputeFileStatuses(manifest, sidecar, model.Config{}, projectDir, remoteVersions)
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}
	if statuses[0].BundledVersion != "1.0.0" {
		t.Fatalf("expected bundled 1.0.0 from resolved path lookup, got %q", statuses[0].BundledVersion)
	}
	if statuses[0].InstalledVersion != "0.9.0" {
		t.Fatalf("expected installed 0.9.0, got %q", statuses[0].InstalledVersion)
	}
	if !statuses[0].UpdateAvailable {
		t.Fatal("expected updateAvailable=true for relative-path file with version mismatch")
	}
}
