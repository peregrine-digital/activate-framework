package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// ── DiscoverManifests tests ─────────────────────────────────────

func TestDiscoverManifestsFindsJSONFiles(t *testing.T) {
	root := t.TempDir()
	manifestsDir := filepath.Join(root, "manifests")
	os.MkdirAll(manifestsDir, 0755)

	raw := manifestJSON{
		Name:    "Alpha",
		Version: "1.0.0",
		Files: []model.ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "core"},
		},
	}
	data, _ := json.Marshal(raw)
	os.WriteFile(filepath.Join(manifestsDir, "alpha.json"), data, 0644)

	raw2 := manifestJSON{
		Name:    "Beta",
		Version: "2.0.0",
		Files: []model.ManifestFile{
			{Src: "b.md", Dest: "b.md", Tier: "core"},
		},
	}
	data2, _ := json.Marshal(raw2)
	os.WriteFile(filepath.Join(manifestsDir, "beta.json"), data2, 0644)

	manifests, err := DiscoverManifests(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}
	if manifests[0].ID != "alpha" || manifests[1].ID != "beta" {
		t.Fatalf("unexpected IDs: %s, %s", manifests[0].ID, manifests[1].ID)
	}
}

func TestDiscoverManifestsResolvesBasePath(t *testing.T) {
	root := t.TempDir()
	manifestsDir := filepath.Join(root, "manifests")
	os.MkdirAll(manifestsDir, 0755)

	raw := manifestJSON{
		Name:     "WithBase",
		Version:  "1.0.0",
		BasePath: "plugins/my-plugin",
		Files:    []model.ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
	}
	data, _ := json.Marshal(raw)
	os.WriteFile(filepath.Join(manifestsDir, "withbase.json"), data, 0644)

	manifests, err := DiscoverManifests(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	expected := filepath.Join(root, "plugins/my-plugin")
	if manifests[0].BasePath != expected {
		t.Fatalf("expected BasePath %q, got %q", expected, manifests[0].BasePath)
	}
}

func TestDiscoverManifestsReturnsEmptyForNoManifests(t *testing.T) {
	root := t.TempDir()

	manifests, err := DiscoverManifests(root)
	if err == nil && len(manifests) > 0 {
		t.Fatalf("expected no manifests, got %d", len(manifests))
	}
}
