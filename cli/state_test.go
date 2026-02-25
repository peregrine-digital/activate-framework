package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectInstallState(t *testing.T) {
	projectDir := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	state := DetectInstallState(projectDir)
	if state.HasGlobalConfig || state.HasProjectConfig || state.HasInstallMarker {
		t.Fatalf("expected empty state, got %+v", state)
	}

	globalPath := filepath.Join(homeDir, ".activate", "config.json")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte(`{"manifest":"activate-framework","tier":"standard"}`), 0644); err != nil {
		t.Fatal(err)
	}

	projectCfgPath := filepath.Join(projectDir, ".activate.json")
	if err := os.WriteFile(projectCfgPath, []byte(`{"manifest":"ironarch","tier":"minimal"}`), 0644); err != nil {
		t.Fatal(err)
	}

	markerPath := filepath.Join(projectDir, ".github", ".activate-version")
	if err := os.MkdirAll(filepath.Dir(markerPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(markerPath, []byte(`{"manifest":"ironarch","version":"1.2.3"}`), 0644); err != nil {
		t.Fatal(err)
	}

	state = DetectInstallState(projectDir)
	if !state.HasGlobalConfig || !state.HasProjectConfig || !state.HasInstallMarker {
		t.Fatalf("expected all state flags true, got %+v", state)
	}
	if state.InstalledManifest != "ironarch" || state.InstalledVersion != "1.2.3" {
		t.Fatalf("unexpected installed marker values: %+v", state)
	}
}
