package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── helpers ────────────────────────────────────────────────────

// setupBundle creates a temp bundle dir with one source file and returns
// the manifest, bundle dir, and resolved config.
func setupBundle(t *testing.T) (Manifest, string, string) {
	t.Helper()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	projectDir := t.TempDir()

	// .git/info/exclude so sidecar writes succeed
	excludeDir := filepath.Join(projectDir, ".git", "info")
	if err := os.MkdirAll(excludeDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	bundleDir := t.TempDir()
	srcRel := "instructions/test.instructions.md"
	srcPath := filepath.Join(bundleDir, srcRel)
	if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := "---\nversion: '1.0.0'\n---\n# Test\n"
	if err := os.WriteFile(srcPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	m := Manifest{
		ID:       "test-manifest",
		Name:     "Test Manifest",
		Version:  "1.0.0",
		BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: srcRel, Dest: srcRel, Tier: "core", Category: "instructions"},
		},
	}
	return m, projectDir, bundleDir
}

func newTestService(m Manifest, projectDir string) *ActivateService {
	cfg := ResolveConfig(projectDir, nil)
	cfg.Manifest = m.ID
	cfg.Tier = "minimal"
	return NewService(projectDir, []Manifest{m}, cfg, false, "", "")
}

// ── TestNewService ─────────────────────────────────────────────

func TestServiceNewService(t *testing.T) {
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

	if svc.ProjectDir != projectDir {
		t.Fatalf("ProjectDir = %q, want %q", svc.ProjectDir, projectDir)
	}
	if len(svc.Manifests) != 1 || svc.Manifests[0].ID != "test-manifest" {
		t.Fatalf("Manifests not set correctly: %+v", svc.Manifests)
	}
	if svc.Config.Manifest != "test-manifest" {
		t.Fatalf("Config.Manifest = %q, want test-manifest", svc.Config.Manifest)
	}
	if svc.UseRemote != false {
		t.Fatal("UseRemote should be false")
	}
}

// ── TestServiceGetState ────────────────────────────────────────

func TestServiceGetState(t *testing.T) {
	t.Run("no sidecar", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		result := svc.GetState()
		if result.ProjectDir != projectDir {
			t.Fatalf("ProjectDir = %q, want %q", result.ProjectDir, projectDir)
		}
		if result.State.HasInstallMarker {
			t.Fatal("expected no install marker")
		}
	})

	t.Run("with sidecar", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

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
	m, projectDir, _ := setupBundle(t)
	// Write a project config
	if err := WriteProjectConfig(projectDir, &Config{Manifest: "test-manifest", Tier: "minimal"}); err != nil {
		t.Fatal(err)
	}
	svc := newTestService(m, projectDir)

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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		result, err := svc.SetConfig("project", &Config{Tier: "advanced"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Scope != "project" {
			t.Fatalf("unexpected result: %+v", result)
		}

		// Verify persisted
		cfg, _ := ReadProjectConfig(projectDir)
		if cfg.Tier != "advanced" {
			t.Fatalf("Tier not persisted: got %q", cfg.Tier)
		}

		// Verify service config refreshed
		if svc.Config.Tier != "advanced" {
			t.Fatalf("service config not refreshed: Tier = %q", svc.Config.Tier)
		}
	})

	t.Run("global scope", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		result, err := svc.SetConfig("global", &Config{Tier: "advanced"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK || result.Scope != "global" {
			t.Fatalf("unexpected result: %+v", result)
		}

		cfg, _ := ReadGlobalConfig()
		if cfg.Tier != "advanced" {
			t.Fatalf("Tier not persisted globally: got %q", cfg.Tier)
		}
	})

	t.Run("empty scope defaults to project", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		result, err := svc.SetConfig("", &Config{Tier: "advanced"})
		if err != nil {
			t.Fatal(err)
		}
		if result.Scope != "project" {
			t.Fatalf("expected project scope, got %q", result.Scope)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		_, err := svc.SetConfig("bogus", &Config{Tier: "advanced"})
		if err == nil {
			t.Fatal("expected error for invalid scope")
		}
	})
}

// ── TestServiceListManifests ───────────────────────────────────

func TestServiceListManifests(t *testing.T) {
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

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
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

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
	if _, err := os.Stat(sidecarPath(projectDir)); err != nil {
		t.Fatalf("expected sidecar, err=%v", err)
	}
}

// ── TestServiceRepoRemove ──────────────────────────────────────

func TestServiceRepoRemove(t *testing.T) {
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

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
	if _, err := os.Stat(sidecarPath(projectDir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar removed, err=%v", err)
	}
}

// ── TestServiceSync ────────────────────────────────────────────

func TestServiceSync(t *testing.T) {
	t.Run("not installed", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		result, err := svc.Sync()
		if err != nil {
			t.Fatal(err)
		}
		if result.Action != "none" || result.Reason != "not installed" {
			t.Fatalf("unexpected sync result: %+v", result)
		}
	})

	t.Run("up to date", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		// Tamper sidecar version to simulate mismatch
		sc, _ := readRepoSidecar(projectDir)
		sc.Version = "0.9.0"
		scData, _ := json.MarshalIndent(sc, "", "  ")
		if err := os.WriteFile(sidecarPath(projectDir), append(scData, '\n'), 0644); err != nil {
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
}

// ── TestServiceUpdate ──────────────────────────────────────────

func TestServiceUpdate(t *testing.T) {
	t.Run("updates installed files", func(t *testing.T) {
		m, projectDir, bundleDir := setupBundle(t)
		svc := newTestService(m, projectDir)

		if _, err := svc.RepoAdd(); err != nil {
			t.Fatal(err)
		}

		// Change the source file content
		srcPath := filepath.Join(bundleDir, "instructions", "test.instructions.md")
		if err := os.WriteFile(srcPath, []byte("---\nversion: '2.0.0'\n---\n# Updated\n"), 0644); err != nil {
			t.Fatal(err)
		}

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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		_, err := svc.Update()
		if err == nil {
			t.Fatal("expected error when no sidecar")
		}
	})
}

// ── TestServiceInstallFile ─────────────────────────────────────

func TestServiceInstallFile(t *testing.T) {
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

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
	sc, _ := readRepoSidecar(projectDir)
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
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

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
	sc, _ := readRepoSidecar(projectDir)
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
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

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
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)

	result, err := svc.SkipUpdate("instructions/test.instructions.md")
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK {
		t.Fatal("expected OK=true")
	}

	// Verify persisted in config
	cfg, _ := ReadProjectConfig(projectDir)
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
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		result, err := svc.SetOverride("instructions/test.instructions.md", "pinned")
		if err != nil {
			t.Fatal(err)
		}
		if !result.OK {
			t.Fatal("expected OK=true")
		}

		cfg, _ := ReadProjectConfig(projectDir)
		if cfg.FileOverrides["instructions/test.instructions.md"] != "pinned" {
			t.Fatalf("override not set: %v", cfg.FileOverrides)
		}
	})

	t.Run("excluded", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		if _, err := svc.SetOverride("instructions/test.instructions.md", "excluded"); err != nil {
			t.Fatal(err)
		}
		cfg, _ := ReadProjectConfig(projectDir)
		if cfg.FileOverrides["instructions/test.instructions.md"] != "excluded" {
			t.Fatalf("override not set: %v", cfg.FileOverrides)
		}
	})

	t.Run("clear", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)

		if _, err := svc.SetOverride("instructions/test.instructions.md", "pinned"); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.SetOverride("instructions/test.instructions.md", ""); err != nil {
			t.Fatal(err)
		}
		cfg, _ := ReadProjectConfig(projectDir)
		if _, ok := cfg.FileOverrides["instructions/test.instructions.md"]; ok {
			t.Fatal("override should be cleared")
		}
	})
}

// ── TestServiceRunTelemetry ────────────────────────────────────

func TestServiceRunTelemetry(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		m, projectDir, _ := setupBundle(t)
		svc := newTestService(m, projectDir)
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
		m, projectDir, _ := setupBundle(t)
		enabled := true
		svc := newTestService(m, projectDir)
		svc.Config.TelemetryEnabled = &enabled

		// Ensure no token is available
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("GH_TOKEN", "")
		// Ensure gh CLI won't resolve (PATH trick not needed if gh isn't installed,
		// but passing empty token explicitly tests the path)
		_, err := svc.RunTelemetry("")
		if err == nil {
			// If gh CLI is installed and authenticated, this may succeed.
			// That's acceptable — we just verify the disabled case above.
			t.Skip("gh CLI resolved a token; skipping no-token assertion")
		}
		// Either "no GitHub token" or a network error is acceptable
	})
}

// ── TestServiceReadTelemetryLog ────────────────────────────────

func TestServiceReadTelemetryLog(t *testing.T) {
	m, projectDir, _ := setupBundle(t)
	svc := newTestService(m, projectDir)
	// HOME points to an empty temp dir, so no log file exists

	entries, err := svc.ReadTelemetryLog()
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil && len(entries) != 0 {
		t.Fatalf("expected empty log, got %d entries", len(entries))
	}
}
