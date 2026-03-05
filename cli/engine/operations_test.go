package engine

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

// setupTestStore isolates all activate state to a temp directory.
func setupTestStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := storage.ActivateBaseDir
	storage.ActivateBaseDir = dir
	t.Cleanup(func() { storage.ActivateBaseDir = old })
	return dir
}

// serveRemoteFiles creates an httptest server that serves files from the given map.
// Keys are relative paths (e.g. "plugins/test/instructions/general.md").
// Returns the server (caller must defer Close) and the test repo/branch to use.
func serveRemoteFiles(t *testing.T, files map[string]string) (*httptest.Server, string, string) {
	t.Helper()
	repo := "test/repo"
	branch := "main"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL path format: /<repo>/<branch>/<filePath>
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
	origResolver := storage.TokenResolver
	storage.RawBase = raw.URL
	storage.TokenResolver = func() string { return "" }
	storage.ResetTokenCache()
	t.Cleanup(func() {
		storage.RawBase = origRaw
		storage.TokenResolver = origResolver
		storage.ResetTokenCache()
		raw.Close()
	})

	return raw, repo, branch
}

// ── UpdateFiles tests ───────────────────────────────────────────

func TestUpdateFilesReinstallsTrackedFiles(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	basePath := "plugins/test"

	// Serve bundled source (newer) from remote
	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/instructions/general.md": "---\nversion: '0.5.0'\n---\n# Updated",
	})

	// Create installed file (older)
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Old"), 0644)

	// Create sidecar
	scPath := storage.SidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(model.RepoSidecar{
		Manifest: "test", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	// Setup git exclude so sidecar write doesn't fail
	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "skills/test.md", Dest: "skills/test.md", Tier: "ad-hoc"}, // not tracked
		},
	}
	sidecar := &model.RepoSidecar{
		Manifest: "test", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	}
	cfg := model.Config{Repo: repo, Branch: branch}

	updated, skipped, err := UpdateFiles(manifest, sidecar, cfg, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(updated) != 1 || updated[0] != "instructions/general.md" {
		t.Fatalf("expected 1 updated file, got %v", updated)
	}
	if len(skipped) != 0 {
		t.Fatalf("expected 0 skipped, got %v", skipped)
	}

	// Verify file was actually updated
	data, _ := os.ReadFile(installedPath)
	if !strings.Contains(string(data), "Updated") {
		t.Fatalf("expected updated content, got %q", string(data))
	}
}

func TestUpdateFilesRespectsSkippedVersions(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	basePath := "plugins/test"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/a.md": "---\nversion: '0.5.0'\n---\n# New",
	})

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Old"), 0644)

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	scPath := storage.SidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(model.RepoSidecar{
		Manifest: "test", Tier: "minimal",
		Files: []string{".github/a.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}
	sidecar := &model.RepoSidecar{
		Manifest: "test", Tier: "minimal",
		Files: []string{".github/a.md"},
	}
	cfg := model.Config{
		Repo: repo, Branch: branch,
		SkippedVersions: map[string]string{"a.md": "0.5.0"},
	}

	updated, skipped, err := UpdateFiles(manifest, sidecar, cfg, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	if len(updated) != 0 {
		t.Fatalf("expected 0 updated, got %v", updated)
	}
	if len(skipped) != 1 || skipped[0] != "a.md" {
		t.Fatalf("expected 1 skipped, got %v", skipped)
	}

	// Verify file was NOT updated
	data, _ := os.ReadFile(installedPath)
	if strings.Contains(string(data), "New") {
		t.Fatal("file should not have been updated")
	}
}

func TestUpdateFilesNilSidecar(t *testing.T) {
	_, _, err := UpdateFiles(model.Manifest{}, nil, model.Config{}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for nil sidecar")
	}
}

// ── InstallSingleFile tests ─────────────────────────────────────

func TestInstallSingleFile(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/skills/test.md": "# Test Skill",
	})

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	manifest := model.Manifest{ID: "m1", BasePath: basePath}
	file := model.ManifestFile{Src: "skills/test.md", Dest: "skills/test.md", Tier: "core"}
	cfg := model.Config{Repo: repo, Branch: branch}

	if err := InstallSingleFile(file, manifest, projectDir, cfg); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	data, err := os.ReadFile(filepath.Join(projectDir, ".github", "skills", "test.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "# Test Skill" {
		t.Fatalf("unexpected content: %q", string(data))
	}

	// Verify sidecar was updated
	sc, err := storage.ReadRepoSidecar(projectDir)
	if err != nil || sc == nil {
		t.Fatal("expected sidecar to exist")
	}
	if !model.ContainsString(sc.Files, ".github/skills/test.md") {
		t.Fatalf("expected file in sidecar, got %v", sc.Files)
	}
}

func TestInstallSingleFileIdempotent(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/a.md": "content",
	})

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	manifest := model.Manifest{ID: "m1", BasePath: basePath}
	file := model.ManifestFile{Src: "a.md", Dest: "a.md", Tier: "core"}
	cfg := model.Config{Repo: repo, Branch: branch}

	// Install twice
	InstallSingleFile(file, manifest, projectDir, cfg)
	InstallSingleFile(file, manifest, projectDir, cfg)

	sc, _ := storage.ReadRepoSidecar(projectDir)
	count := 0
	for _, f := range sc.Files {
		if f == ".github/a.md" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected file listed once in sidecar, got %d times", count)
	}
}

// ── UninstallSingleFile tests ───────────────────────────────────

func TestUninstallSingleFile(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	// Create installed file
	filePath := filepath.Join(projectDir, ".github", "instructions", "test.md")
	os.MkdirAll(filepath.Dir(filePath), 0755)
	os.WriteFile(filePath, []byte("content"), 0644)

	// Create sidecar tracking the file
	scPath := storage.SidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(model.RepoSidecar{
		Manifest: "m1", Tier: "minimal",
		Files: []string{".github/instructions/test.md", ".github/other.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	if err := UninstallSingleFile("instructions/test.md", projectDir); err != nil {
		t.Fatal(err)
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("expected file to be removed")
	}

	// Verify sidecar updated
	sc, _ := storage.ReadRepoSidecar(projectDir)
	if model.ContainsString(sc.Files, ".github/instructions/test.md") {
		t.Fatal("expected file removed from sidecar")
	}
	if !model.ContainsString(sc.Files, ".github/other.md") {
		t.Fatal("expected other file to remain in sidecar")
	}
}

func TestUninstallSingleFileNoSidecar(t *testing.T) {
	err := UninstallSingleFile("foo.md", t.TempDir())
	if err == nil {
		t.Fatal("expected error with no sidecar")
	}
}

// ── DiffFile tests ──────────────────────────────────────────────

func TestDiffFileIdentical(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	content := "---\nversion: '1.0.0'\n---\n# Same"
	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/a.md": content,
	})

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte(content), 0644)

	file := model.ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := model.Manifest{BasePath: basePath}
	cfg := model.Config{Repo: repo, Branch: branch}

	diff, err := DiffFile(file, manifest, projectDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Fatalf("expected empty diff for identical files, got:\n%s", diff)
	}
}

func TestDiffFileDifferent(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/a.md": "line1\nline2\nline3\n",
	})

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("line1\nchanged\nline3\n"), 0644)

	file := model.ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := model.Manifest{BasePath: basePath}
	cfg := model.Config{Repo: repo, Branch: branch}

	diff, err := DiffFile(file, manifest, projectDir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(diff, "---") || !strings.Contains(diff, "+++") {
		t.Fatalf("expected diff headers, got:\n%s", diff)
	}
	if !strings.Contains(diff, "-line2") || !strings.Contains(diff, "+changed") {
		t.Fatalf("expected diff content, got:\n%s", diff)
	}
}

func TestDiffFileMissingInstalled(t *testing.T) {
	basePath := "plugins/test"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/a.md": "content",
	})

	file := model.ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := model.Manifest{BasePath: basePath}
	cfg := model.Config{Repo: repo, Branch: branch}

	_, err := DiffFile(file, manifest, t.TempDir(), cfg)
	if err == nil {
		t.Fatal("expected error for missing installed file")
	}
}

// ── unifiedDiff tests ───────────────────────────────────────────

func TestUnifiedDiffIdentical(t *testing.T) {
	diff := unifiedDiff("hello\nworld\n", "hello\nworld\n", "a", "b")
	if diff != "" {
		t.Fatalf("expected empty diff, got %q", diff)
	}
}

func TestUnifiedDiffAddition(t *testing.T) {
	diff := unifiedDiff("a\nc\n", "a\nb\nc\n", "old", "new")
	if !strings.Contains(diff, "+b") {
		t.Fatalf("expected addition marker, got:\n%s", diff)
	}
}

func TestUnifiedDiffDeletion(t *testing.T) {
	diff := unifiedDiff("a\nb\nc\n", "a\nc\n", "old", "new")
	if !strings.Contains(diff, "-b") {
		t.Fatalf("expected deletion marker, got:\n%s", diff)
	}
}

// ── containsString / findManifestFile ───────────────────────────

func TestContainsString(t *testing.T) {
	if !model.ContainsString([]string{"a", "b"}, "b") {
		t.Fatal("expected true")
	}
	if model.ContainsString([]string{"a", "b"}, "c") {
		t.Fatal("expected false")
	}
	if model.ContainsString(nil, "a") {
		t.Fatal("expected false for nil")
	}
}

func TestFindManifestFile(t *testing.T) {
	files := []model.ManifestFile{
		{Src: "a.md", Dest: "a.md"},
		{Src: "b.md", Dest: "b.md"},
	}
	if f := model.FindManifestFile(files, "b.md"); f == nil || f.Src != "b.md" {
		t.Fatal("expected to find b.md")
	}
	if f := model.FindManifestFile(files, "nope"); f != nil {
		t.Fatal("expected nil for missing file")
	}
}

// ── UpdateFiles MCP-aware filtering ─────────────────────────────

func TestUpdateFilesMcpAware(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	basePath := "plugins/test"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/instructions/general.md": "---\nversion: '0.5.0'\n---\n# Updated",
	})

	// Create installed non-MCP file
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Old"), 0644)

	// Setup git exclude
	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	// Create sidecar
	scPath := storage.SidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(model.RepoSidecar{
		Manifest: "test", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	manifest := model.Manifest{
		ID: "test", BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "mcp-servers/server.json", Dest: "mcp-servers/server.json", Tier: "core", Category: "mcp-servers"},
		},
	}
	sidecar := &model.RepoSidecar{
		Manifest: "test", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	}
	cfg := model.Config{Repo: repo, Branch: branch}

	updated, _, err := UpdateFiles(manifest, sidecar, cfg, projectDir)
	if err != nil {
		t.Fatal(err)
	}

	// MCP file should NOT appear in updated (it's handled separately)
	for _, u := range updated {
		if strings.Contains(u, "mcp-servers") {
			t.Fatalf("MCP file should be filtered out of regular updates, got %s", u)
		}
	}
	// Non-MCP tracked file should be updated
	if len(updated) != 1 || updated[0] != "instructions/general.md" {
		t.Fatalf("expected 1 non-MCP updated file, got %v", updated)
	}
}

// ── DiffFile missing bundled source ─────────────────────────────

func TestDiffFileMissingBundled(t *testing.T) {
	projectDir := t.TempDir()

	// Serve nothing — so the fetch will 404
	_, repo, branch := serveRemoteFiles(t, map[string]string{})

	// Create installed file but NOT the remote source
	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("content"), 0644)

	file := model.ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := model.Manifest{BasePath: "plugins/test"}
	cfg := model.Config{Repo: repo, Branch: branch}

	_, err := DiffFile(file, manifest, projectDir, cfg)
	if err == nil {
		t.Fatal("expected error when bundled source is missing")
	}
	if !strings.Contains(err.Error(), "fetch") {
		t.Fatalf("expected 'fetch' in error, got: %s", err)
	}
}

// ── SyncNeeded tests ────────────────────────────────────────────

func TestSyncNeeded(t *testing.T) {
	m := model.Manifest{ID: "test-manifest"}

	// Same manifest and tier → no sync needed
	sc := &model.RepoSidecar{Manifest: "test-manifest", Tier: "standard"}
	if SyncNeeded(m, sc, "standard") {
		t.Fatal("expected SyncNeeded=false when manifest and tier match")
	}
}

func TestSyncNeededManifestChanged(t *testing.T) {
	m := model.Manifest{ID: "new-manifest"}
	sc := &model.RepoSidecar{Manifest: "old-manifest", Tier: "standard"}
	if !SyncNeeded(m, sc, "standard") {
		t.Fatal("expected SyncNeeded=true when manifest ID changed")
	}
}

func TestSyncNeededTierChanged(t *testing.T) {
	m := model.Manifest{ID: "test-manifest"}
	sc := &model.RepoSidecar{Manifest: "test-manifest", Tier: "minimal"}
	if !SyncNeeded(m, sc, "standard") {
		t.Fatal("expected SyncNeeded=true when tier changed")
	}
}

func TestSyncNeededNilSidecar(t *testing.T) {
	m := model.Manifest{}
	if SyncNeeded(m, nil, "standard") {
		t.Fatal("expected SyncNeeded=false for nil sidecar")
	}
}
