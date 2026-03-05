package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

func TestDetectInstallState(t *testing.T) {
	projectDir := t.TempDir()
	storeDir := setupTestStore(t)

	state := DetectInstallState(projectDir)
	if state.HasGlobalConfig || state.HasProjectConfig || state.HasInstallMarker {
		t.Fatalf("expected empty state, got %+v", state)
	}

	globalPath := filepath.Join(storeDir, "config.json")
	if err := os.MkdirAll(filepath.Dir(globalPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(globalPath, []byte(`{"manifest":"activate-framework","tier":"standard"}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := storage.WriteProjectConfig(projectDir, &model.Config{Manifest: "ironarch", Tier: "minimal"}); err != nil {
		t.Fatal(err)
	}

	// Write sidecar to signal install marker
	scPath := storage.SidecarPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(scPath), 0755); err != nil {
		t.Fatal(err)
	}
	scData, _ := json.Marshal(model.RepoSidecar{Manifest: "ironarch"})
	if err := os.WriteFile(scPath, scData, 0644); err != nil {
		t.Fatal(err)
	}

	state = DetectInstallState(projectDir)
	if !state.HasGlobalConfig || !state.HasProjectConfig || !state.HasInstallMarker {
		t.Fatalf("expected all state flags true, got %+v", state)
	}
	if state.InstalledManifest != "ironarch" {
		t.Fatalf("unexpected installed marker values: %+v", state)
	}
}
