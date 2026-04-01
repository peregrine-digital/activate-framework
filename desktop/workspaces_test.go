package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ── isTestPath Tests ───────────────────────────────────────────

func TestIsTestPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"macOS temp dir", "/var/folders/xx/yy/T/test123", true},
		{"macOS private temp dir", "/private/var/folders/xx/yy/T/test456", true},
		{"Linux tmp dir", "/tmp/test-workspace", true},
		{"normal home path", "/Users/dev/projects/myapp", false},
		{"linux home path", "/home/user/workspace", false},
		{"root path", "/opt/myapp", false},
		{"empty string", "", false},
		{"tmp-like but not prefix", "/data/tmp/workspace", false},
		{"var-like but not folders", "/var/log/app", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isTestPath(tt.path)
			if got != tt.want {
				t.Errorf("isTestPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// ── ListWorkspaces Tests ───────────────────────────────────────

func TestListWorkspaces_EmptyReposDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// No .activate/repos/ directory at all
	app := &App{}
	ws := app.ListWorkspaces()
	if ws != nil {
		t.Errorf("expected nil for missing repos dir, got %v", ws)
	}
}

func TestListWorkspaces_EmptyDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	reposDir := filepath.Join(home, ".activate", "repos")
	os.MkdirAll(reposDir, 0755)

	app := &App{}
	ws := app.ListWorkspaces()
	if len(ws) != 0 {
		t.Errorf("expected 0 workspaces, got %d", len(ws))
	}
}

func TestListWorkspaces_ValidWorkspace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Use a well-known system dir that exists and won't be filtered by isTestPath.
	// t.TempDir() returns /var/folders/... which isTestPath filters out.
	projectDir := "/usr/local"

	// Create the repos entry
	reposDir := filepath.Join(home, ".activate", "repos")
	hashDir := filepath.Join(reposDir, "abc123")
	os.MkdirAll(hashDir, 0755)

	// repo.json with path to existing directory
	repoJSON, _ := json.Marshal(map[string]string{"path": projectDir})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	// installed.json with manifest info
	installedJSON, _ := json.Marshal(map[string]interface{}{
		"manifest": "activate-framework",
		"tier":     "standard",
		"files":    []string{"file1.yml", "file2.yml", "file3.yml"},
	})
	os.WriteFile(filepath.Join(hashDir, "installed.json"), installedJSON, 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(ws))
	}

	w := ws[0]
	if w.Path != projectDir {
		t.Errorf("path = %q, want %q", w.Path, projectDir)
	}
	if w.Name != "local" {
		t.Errorf("name = %q, want %q", w.Name, "local")
	}
	if !w.Exists {
		t.Error("exists = false, want true")
	}
	if w.Manifest != "activate-framework" {
		t.Errorf("manifest = %q, want %q", w.Manifest, "activate-framework")
	}
	if w.Tier != "standard" {
		t.Errorf("tier = %q, want %q", w.Tier, "standard")
	}
	if w.FileCount != 3 {
		t.Errorf("fileCount = %d, want %d", w.FileCount, 3)
	}
}

func TestListWorkspaces_MissingDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	reposDir := filepath.Join(home, ".activate", "repos")
	hashDir := filepath.Join(reposDir, "def456")
	os.MkdirAll(hashDir, 0755)

	// Use a path that won't be filtered by isTestPath but doesn't exist
	nonExistent := "/Users/nonexistent-test-user/gone/project"
	repoJSON, _ := json.Marshal(map[string]string{"path": nonExistent})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(ws))
	}
	if ws[0].Exists {
		t.Error("exists = true, want false for missing directory")
	}
	if ws[0].Name != "project" {
		t.Errorf("name = %q, want %q", ws[0].Name, "project")
	}
}

func TestListWorkspaces_FiltersTestPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	reposDir := filepath.Join(home, ".activate", "repos")

	// Create an entry with a test-like path
	hashDir := filepath.Join(reposDir, "test-hash")
	os.MkdirAll(hashDir, 0755)
	repoJSON, _ := json.Marshal(map[string]string{"path": "/var/folders/xx/yy/T/GoTest12345"})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	// Create a normal entry (use a real non-temp path)
	normalPath := "/usr/local"
	hashDir2 := filepath.Join(reposDir, "real-hash")
	os.MkdirAll(hashDir2, 0755)
	repoJSON2, _ := json.Marshal(map[string]string{"path": normalPath})
	os.WriteFile(filepath.Join(hashDir2, "repo.json"), repoJSON2, 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace (test path filtered), got %d", len(ws))
	}
	if ws[0].Path != normalPath {
		t.Errorf("remaining workspace path = %q, want %q", ws[0].Path, normalPath)
	}
}

func TestListWorkspaces_InvalidRepoJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	reposDir := filepath.Join(home, ".activate", "repos")

	// Entry with invalid JSON
	hashDir := filepath.Join(reposDir, "bad-json")
	os.MkdirAll(hashDir, 0755)
	os.WriteFile(filepath.Join(hashDir, "repo.json"), []byte(`{invalid json`), 0644)

	// Entry with empty path
	hashDir2 := filepath.Join(reposDir, "empty-path")
	os.MkdirAll(hashDir2, 0755)
	os.WriteFile(filepath.Join(hashDir2, "repo.json"), []byte(`{"path":""}`), 0644)

	// Entry with missing repo.json
	hashDir3 := filepath.Join(reposDir, "no-repo-json")
	os.MkdirAll(hashDir3, 0755)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 0 {
		t.Errorf("expected 0 workspaces (all invalid), got %d", len(ws))
	}
}

func TestListWorkspaces_InvalidInstalledJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Use a real non-temp path
	projectDir := "/usr/local"

	reposDir := filepath.Join(home, ".activate", "repos")
	hashDir := filepath.Join(reposDir, "bad-installed")
	os.MkdirAll(hashDir, 0755)

	repoJSON, _ := json.Marshal(map[string]string{"path": projectDir})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	// Write invalid installed.json
	os.WriteFile(filepath.Join(hashDir, "installed.json"), []byte(`{not valid`), 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(ws))
	}
	// Should still have workspace info, just without manifest/tier
	if ws[0].Manifest != "" {
		t.Errorf("manifest should be empty for invalid installed.json, got %q", ws[0].Manifest)
	}
	if ws[0].FileCount != 0 {
		t.Errorf("fileCount should be 0 for invalid installed.json, got %d", ws[0].FileCount)
	}
}

func TestListWorkspaces_Sorting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	reposDir := filepath.Join(home, ".activate", "repos")

	// Use real system paths for "exists" and fake non-temp paths for "missing".
	// t.TempDir() returns /var/folders/... which isTestPath filters out.
	entries := []struct {
		hash string
		path string // workspace path stored in repo.json
	}{
		{"hash1", "/usr/local"},                                 // exists, name="local" → sorts as "local"
		{"hash2", "/Users/nonexistent-test-user/alpha-project"}, // missing, name="alpha-project"
		{"hash3", "/usr/bin"},                                   // exists, name="bin" → sorts as "bin"
	}

	for _, e := range entries {
		hashDir := filepath.Join(reposDir, e.hash)
		os.MkdirAll(hashDir, 0755)
		repoJSON, _ := json.Marshal(map[string]string{"path": e.path})
		os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)
	}

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 3 {
		t.Fatalf("expected 3 workspaces, got %d", len(ws))
	}

	// Existing first sorted alphabetically (bin, local), then non-existing (alpha-project)
	if ws[0].Name != "bin" {
		t.Errorf("ws[0].Name = %q, want %q", ws[0].Name, "bin")
	}
	if ws[1].Name != "local" {
		t.Errorf("ws[1].Name = %q, want %q", ws[1].Name, "local")
	}
	if ws[2].Name != "alpha-project" {
		t.Errorf("ws[2].Name = %q, want %q", ws[2].Name, "alpha-project")
	}

	// Verify exists flags
	if !ws[0].Exists || !ws[1].Exists {
		t.Error("first two workspaces should exist")
	}
	if ws[2].Exists {
		t.Error("last workspace should not exist")
	}
}

func TestListWorkspaces_PresetField(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := "/usr/local"

	reposDir := filepath.Join(home, ".activate", "repos")
	hashDir := filepath.Join(reposDir, "preset-hash")
	os.MkdirAll(hashDir, 0755)

	repoJSON, _ := json.Marshal(map[string]string{"path": projectDir})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	installedJSON, _ := json.Marshal(map[string]interface{}{
		"preset": "adhoc/standard",
		"files":  []string{"file1.yml", "file2.yml"},
	})
	os.WriteFile(filepath.Join(hashDir, "installed.json"), installedJSON, 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(ws))
	}

	w := ws[0]
	if w.Preset != "adhoc/standard" {
		t.Errorf("preset = %q, want %q", w.Preset, "adhoc/standard")
	}
	if w.Manifest != "" {
		t.Errorf("manifest should be empty for preset-only sidecar, got %q", w.Manifest)
	}
	if w.Tier != "" {
		t.Errorf("tier should be empty for preset-only sidecar, got %q", w.Tier)
	}
	if w.FileCount != 2 {
		t.Errorf("fileCount = %d, want %d", w.FileCount, 2)
	}
}

func TestListWorkspaces_PresetWithLegacyFields(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	projectDir := "/usr/local"

	reposDir := filepath.Join(home, ".activate", "repos")
	hashDir := filepath.Join(reposDir, "preset-legacy-hash")
	os.MkdirAll(hashDir, 0755)

	repoJSON, _ := json.Marshal(map[string]string{"path": projectDir})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	// Sidecar with both preset and legacy manifest/tier fields
	installedJSON, _ := json.Marshal(map[string]interface{}{
		"manifest": "activate-framework",
		"tier":     "standard",
		"preset":   "adhoc/standard",
		"files":    []string{"file1.yml"},
	})
	os.WriteFile(filepath.Join(hashDir, "installed.json"), installedJSON, 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(ws))
	}

	w := ws[0]
	if w.Preset != "adhoc/standard" {
		t.Errorf("preset = %q, want %q", w.Preset, "adhoc/standard")
	}
	if w.Manifest != "activate-framework" {
		t.Errorf("manifest = %q, want %q", w.Manifest, "activate-framework")
	}
	if w.Tier != "standard" {
		t.Errorf("tier = %q, want %q", w.Tier, "standard")
	}
}

func TestListWorkspaces_SkipsFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	reposDir := filepath.Join(home, ".activate", "repos")
	os.MkdirAll(reposDir, 0755)

	// Create a file (not directory) in repos/
	os.WriteFile(filepath.Join(reposDir, "stray-file.json"), []byte("{}"), 0644)

	// Create a valid directory entry pointing to a real non-temp path
	hashDir := filepath.Join(reposDir, "valid")
	os.MkdirAll(hashDir, 0755)
	repoJSON, _ := json.Marshal(map[string]string{"path": "/usr/local"})
	os.WriteFile(filepath.Join(hashDir, "repo.json"), repoJSON, 0644)

	app := &App{}
	ws := app.ListWorkspaces()

	if len(ws) != 1 {
		t.Errorf("expected 1 workspace (file entry skipped), got %d", len(ws))
	}
}
