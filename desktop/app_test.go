package main

import (
	"os"
	"path/filepath"
	"testing"
)

// ── findBinary Tests ───────────────────────────────────────────

func TestFindBinary_StandardLocation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, ".activate", "bin")
	os.MkdirAll(binDir, 0755)

	binPath := filepath.Join(binDir, "activate")
	os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0755)

	result := findBinary()
	if result != binPath {
		t.Errorf("findBinary() = %q, want %q", result, binPath)
	}
}

func TestFindBinary_FallbackToPATH(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No .activate/bin/activate in home

	// Create a fake binary on PATH
	pathDir := t.TempDir()
	fakeBin := filepath.Join(pathDir, "activate")
	os.WriteFile(fakeBin, []byte("#!/bin/sh\n"), 0755)

	t.Setenv("PATH", pathDir)

	result := findBinary()
	if result != fakeBin {
		t.Errorf("findBinary() = %q, want %q", result, fakeBin)
	}
}

func TestFindBinary_NotFound(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir()) // empty dir on PATH

	result := findBinary()
	if result != "" {
		t.Errorf("findBinary() = %q, want empty string", result)
	}
}

// ── Version Tests ──────────────────────────────────────────────

func TestVersion(t *testing.T) {
	app := NewApp()
	v := app.Version()
	// The version variable defaults to "dev" when not set by ldflags
	if v != version {
		t.Errorf("Version() = %q, want %q", v, version)
	}
	if v == "" {
		t.Error("version should not be empty")
	}
}

// ── CLIFound Tests ─────────────────────────────────────────────

func TestCLIFound_WithBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	binDir := filepath.Join(home, ".activate", "bin")
	os.MkdirAll(binDir, 0755)
	os.WriteFile(filepath.Join(binDir, "activate"), []byte("#!/bin/sh\n"), 0755)

	app := NewApp()
	if !app.CLIFound() {
		t.Error("CLIFound() = false, want true when binary exists")
	}
}

func TestCLIFound_WithoutBinary(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("PATH", t.TempDir())

	app := NewApp()
	if app.CLIFound() {
		t.Error("CLIFound() = true, want false when binary not found")
	}
}

// ── NewApp Tests ───────────────────────────────────────────────

func TestNewApp(t *testing.T) {
	app := NewApp()
	if app == nil {
		t.Fatal("NewApp() returned nil")
	}
}

// ── requireDaemon Tests ────────────────────────────────────────

func TestRequireDaemon_NoDaemon(t *testing.T) {
	app := NewApp()
	err := app.requireDaemon()
	if err == nil {
		t.Fatal("requireDaemon should return error when daemon is nil")
	}
}

func TestRequireDaemon_WithDaemon(t *testing.T) {
	app := NewApp()
	app.daemon = &daemonClient{}
	err := app.requireDaemon()
	if err != nil {
		t.Errorf("requireDaemon should return nil when daemon exists: %v", err)
	}
}

// ── RPC Forwarding: requireDaemon guard ────────────────────────

func TestRPCMethods_RequireDaemon(t *testing.T) {
	app := NewApp()

	tests := []struct {
		name string
		fn   func() error
	}{
		{"GetState", func() error { _, err := app.GetState(); return err }},
		{"GetConfig", func() error { _, err := app.GetConfig("project"); return err }},
		{"SetConfig", func() error { _, err := app.SetConfig(nil); return err }},
		{"InstallFile", func() error { _, err := app.InstallFile("f"); return err }},
		{"UninstallFile", func() error { _, err := app.UninstallFile("f"); return err }},
		{"DiffFile", func() error { _, err := app.DiffFile("f"); return err }},
		{"SkipUpdate", func() error { _, err := app.SkipUpdate("f"); return err }},
		{"SetOverride", func() error { _, err := app.SetOverride("f", "pinned"); return err }},
		{"UpdateAll", func() error { _, err := app.UpdateAll(); return err }},
		{"AddToWorkspace", func() error { _, err := app.AddToWorkspace(); return err }},
		{"RemoveFromWorkspace", func() error { _, err := app.RemoveFromWorkspace(); return err }},
		{"ListManifests", func() error { _, err := app.ListManifests(); return err }},
		{"ListBranches", func() error { _, err := app.ListBranches(); return err }},
		{"RunTelemetry", func() error { _, err := app.RunTelemetry(); return err }},
		{"ReadTelemetryLog", func() error { _, err := app.ReadTelemetryLog(); return err }},
		{"CheckForUpdates", func() error { _, err := app.CheckForUpdates(); return err }},
		{"SyncManifests", func() error { _, err := app.SyncManifests(); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if err == nil {
				t.Errorf("%s should error without daemon", tt.name)
			}
		})
	}
}

// ── OpenFile Tests ─────────────────────────────────────────────

func TestOpenFile_NoProjectDir(t *testing.T) {
	app := NewApp()
	err := app.OpenFile("test.yml")
	if err != nil {
		t.Errorf("OpenFile with no projectDir should return nil, got: %v", err)
	}
}

// ── CloseWorkspace Tests ───────────────────────────────────────

func TestCloseWorkspace_NoDaemon(t *testing.T) {
	app := NewApp()
	// Should not panic when daemon is nil
	app.CloseWorkspace()
	if app.projectDir != "" {
		t.Errorf("projectDir should be empty after close, got %q", app.projectDir)
	}
}
