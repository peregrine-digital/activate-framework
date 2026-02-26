package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── UpdateFiles tests ───────────────────────────────────────────

func TestUpdateFilesReinstallsTrackedFiles(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	// Create bundled source (newer)
	srcPath := filepath.Join(bundleDir, "instructions", "general.md")
	os.MkdirAll(filepath.Dir(srcPath), 0755)
	os.WriteFile(srcPath, []byte("---\nversion: '0.5.0'\n---\n# Updated"), 0644)

	// Create installed file (older)
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Old"), 0644)

	// Create sidecar
	scPath := sidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(repoSidecar{
		Manifest: "test", Version: "0.4.0", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	// Setup git exclude so sidecar write doesn't fail
	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	manifest := Manifest{
		ID: "test", Version: "0.5.0", BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "skills/test.md", Dest: "skills/test.md", Tier: "ad-hoc"}, // not tracked
		},
	}
	sidecar := &repoSidecar{
		Manifest: "test", Version: "0.4.0", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	}

	updated, skipped, err := UpdateFiles(manifest, sidecar, Config{}, projectDir, false, "", "")
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
	bundleDir := t.TempDir()

	srcPath := filepath.Join(bundleDir, "a.md")
	os.WriteFile(srcPath, []byte("---\nversion: '0.5.0'\n---\n# New"), 0644)

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Old"), 0644)

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	scPath := sidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(repoSidecar{
		Manifest: "test", Version: "0.4.0", Tier: "minimal",
		Files: []string{".github/a.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	manifest := Manifest{
		ID: "test", Version: "0.5.0", BasePath: bundleDir,
		Files: []ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}
	sidecar := &repoSidecar{
		Manifest: "test", Version: "0.4.0", Tier: "minimal",
		Files: []string{".github/a.md"},
	}
	cfg := Config{SkippedVersions: map[string]string{"a.md": "0.5.0"}}

	updated, skipped, err := UpdateFiles(manifest, sidecar, cfg, projectDir, false, "", "")
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
	_, _, err := UpdateFiles(Manifest{}, nil, Config{}, t.TempDir(), false, "", "")
	if err == nil {
		t.Fatal("expected error for nil sidecar")
	}
}

// ── InstallSingleFile tests ─────────────────────────────────────

func TestInstallSingleFile(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	srcPath := filepath.Join(bundleDir, "skills", "test.md")
	os.MkdirAll(filepath.Dir(srcPath), 0755)
	os.WriteFile(srcPath, []byte("# Test Skill"), 0644)

	manifest := Manifest{ID: "m1", Version: "1.0.0", BasePath: bundleDir}
	file := ManifestFile{Src: "skills/test.md", Dest: "skills/test.md", Tier: "core"}

	if err := InstallSingleFile(file, manifest, projectDir, false, "", ""); err != nil {
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
	sc, err := readRepoSidecar(projectDir)
	if err != nil || sc == nil {
		t.Fatal("expected sidecar to exist")
	}
	if !containsString(sc.Files, ".github/skills/test.md") {
		t.Fatalf("expected file in sidecar, got %v", sc.Files)
	}
}

func TestInstallSingleFileIdempotent(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	srcPath := filepath.Join(bundleDir, "a.md")
	os.WriteFile(srcPath, []byte("content"), 0644)

	manifest := Manifest{ID: "m1", Version: "1.0.0", BasePath: bundleDir}
	file := ManifestFile{Src: "a.md", Dest: "a.md", Tier: "core"}

	// Install twice
	InstallSingleFile(file, manifest, projectDir, false, "", "")
	InstallSingleFile(file, manifest, projectDir, false, "", "")

	sc, _ := readRepoSidecar(projectDir)
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
	scPath := sidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(repoSidecar{
		Manifest: "m1", Version: "1.0.0", Tier: "minimal",
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
	sc, _ := readRepoSidecar(projectDir)
	if containsString(sc.Files, ".github/instructions/test.md") {
		t.Fatal("expected file removed from sidecar")
	}
	if !containsString(sc.Files, ".github/other.md") {
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
	bundleDir := t.TempDir()

	content := "---\nversion: '1.0.0'\n---\n# Same"
	srcPath := filepath.Join(bundleDir, "a.md")
	os.WriteFile(srcPath, []byte(content), 0644)

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte(content), 0644)

	file := ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := Manifest{BasePath: bundleDir}

	diff, err := DiffFile(file, manifest, projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if diff != "" {
		t.Fatalf("expected empty diff for identical files, got:\n%s", diff)
	}
}

func TestDiffFileDifferent(t *testing.T) {
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	srcPath := filepath.Join(bundleDir, "a.md")
	os.WriteFile(srcPath, []byte("line1\nline2\nline3\n"), 0644)

	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("line1\nchanged\nline3\n"), 0644)

	file := ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := Manifest{BasePath: bundleDir}

	diff, err := DiffFile(file, manifest, projectDir)
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
	bundleDir := t.TempDir()
	srcPath := filepath.Join(bundleDir, "a.md")
	os.WriteFile(srcPath, []byte("content"), 0644)

	file := ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := Manifest{BasePath: bundleDir}

	_, err := DiffFile(file, manifest, t.TempDir())
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
	if !containsString([]string{"a", "b"}, "b") {
		t.Fatal("expected true")
	}
	if containsString([]string{"a", "b"}, "c") {
		t.Fatal("expected false")
	}
	if containsString(nil, "a") {
		t.Fatal("expected false for nil")
	}
}

func TestFindManifestFile(t *testing.T) {
	files := []ManifestFile{
		{Src: "a.md", Dest: "a.md"},
		{Src: "b.md", Dest: "b.md"},
	}
	if f := findManifestFile(files, "b.md"); f == nil || f.Src != "b.md" {
		t.Fatal("expected to find b.md")
	}
	if f := findManifestFile(files, "nope"); f != nil {
		t.Fatal("expected nil for missing file")
	}
}

// ── UpdateFiles MCP-aware filtering ─────────────────────────────

func TestUpdateFilesMcpAware(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	// Create bundled non-MCP source
	srcPath := filepath.Join(bundleDir, "instructions", "general.md")
	os.MkdirAll(filepath.Dir(srcPath), 0755)
	os.WriteFile(srcPath, []byte("---\nversion: '0.5.0'\n---\n# Updated"), 0644)

	// Create installed non-MCP file
	installedPath := filepath.Join(projectDir, ".github", "instructions", "general.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("---\nversion: '0.4.0'\n---\n# Old"), 0644)

	// Setup git exclude
	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	// Create sidecar
	scPath := sidecarPath(projectDir)
	os.MkdirAll(filepath.Dir(scPath), 0755)
	scData, _ := json.Marshal(repoSidecar{
		Manifest: "test", Version: "0.4.0", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	})
	os.WriteFile(scPath, scData, 0644)

	manifest := Manifest{
		ID: "test", Version: "0.5.0", BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
			{Src: "mcp-servers/server.json", Dest: "mcp-servers/server.json", Tier: "core", Category: "mcp-servers"},
		},
	}
	sidecar := &repoSidecar{
		Manifest: "test", Version: "0.4.0", Tier: "minimal",
		Files: []string{".github/instructions/general.md"},
	}

	updated, _, err := UpdateFiles(manifest, sidecar, Config{}, projectDir, false, "", "")
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
	bundleDir := t.TempDir()

	// Create installed file but NOT the bundled source
	installedPath := filepath.Join(projectDir, ".github", "a.md")
	os.MkdirAll(filepath.Dir(installedPath), 0755)
	os.WriteFile(installedPath, []byte("content"), 0644)

	file := ManifestFile{Src: "a.md", Dest: "a.md"}
	manifest := Manifest{BasePath: bundleDir}

	_, err := DiffFile(file, manifest, projectDir)
	if err == nil {
		t.Fatal("expected error when bundled source is missing")
	}
	if !strings.Contains(err.Error(), "read bundled") {
		t.Fatalf("expected 'read bundled' in error, got: %s", err)
	}
}

// ── SyncNeeded tests ────────────────────────────────────────────

func TestSyncNeeded(t *testing.T) {
	m := Manifest{Version: "1.1.0"}

	// Versions differ → sync needed
	sc := &repoSidecar{Version: "1.0.0"}
	if !SyncNeeded(m, sc) {
		t.Fatal("expected SyncNeeded=true when versions differ")
	}

	// Versions same → no sync needed
	sc.Version = "1.1.0"
	if SyncNeeded(m, sc) {
		t.Fatal("expected SyncNeeded=false when versions match")
	}
}

func TestSyncNeededNilSidecar(t *testing.T) {
	m := Manifest{Version: "1.0.0"}
	if SyncNeeded(m, nil) {
		t.Fatal("expected SyncNeeded=false for nil sidecar")
	}
}
