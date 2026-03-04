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

	repo, branch, cleanup := serveVersionFiles(t, map[string]string{
		basePath + "/instructions/general.md": "---\nversion: '0.5.0'\n---\n# General",
	})
	defer cleanup()

	// Create installed file with older version
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# General"), 0644)

	manifest := model.Manifest{
		ID: "test", Version: "0.5.0", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "skills/test.md", Dest: "skills/test.md", Tier: "ad-hoc"},
		},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/instructions/general.md"}}
	cfg := model.Config{Repo: repo, Branch: branch}

	statuses := ComputeFileStatuses(manifest, sidecar, cfg, projectDir)
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

	repo, branch, cleanup := serveVersionFiles(t, map[string]string{
		basePath + "/instructions/sec.md": "---\nversion: '0.5.0'\n---\n# Sec",
	})
	defer cleanup()

	installedPath := filepath.Join(projectDir, ".github", "instructions", "sec.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Sec"), 0644)

	manifest := model.Manifest{
		ID: "test", Version: "0.5.0", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "instructions/sec.md", Dest: "instructions/sec.md", Tier: "core"}},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/instructions/sec.md"}}
	cfg := model.Config{
		Repo: repo, Branch: branch,
		SkippedVersions: map[string]string{"instructions/sec.md": "0.5.0"},
	}

	statuses := ComputeFileStatuses(manifest, sidecar, cfg, projectDir)
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
		ID: "test", Version: "1.0.0", BasePath: basePath,
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

	statuses := ComputeFileStatuses(manifest, nil, cfg, projectDir)
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
		ID: "test", Version: "1.0.0", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}

	statuses := ComputeFileStatuses(manifest, nil, model.Config{Repo: repo, Branch: branch}, projectDir)
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
		ID: "test", Version: "1.0.0", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}
	sidecar := &model.RepoSidecar{Files: []string{".github/a.md"}}

	statuses := ComputeFileStatuses(manifest, sidecar, model.Config{Repo: repo, Branch: branch}, projectDir)
	if statuses[0].UpdateAvailable {
		t.Fatal("expected no update when versions match")
	}
}
