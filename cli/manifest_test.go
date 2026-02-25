package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── DiscoverManifests tests ─────────────────────────────────────

func TestDiscoverManifestsFindsJSONFiles(t *testing.T) {
	root := t.TempDir()
	manifestsDir := filepath.Join(root, "manifests")
	os.MkdirAll(manifestsDir, 0755)

	raw := manifestJSON{
		Name:    "Alpha",
		Version: "1.0.0",
		Files: []ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "core"},
		},
	}
	data, _ := json.Marshal(raw)
	os.WriteFile(filepath.Join(manifestsDir, "alpha.json"), data, 0644)

	raw2 := manifestJSON{
		Name:    "Beta",
		Version: "2.0.0",
		Files: []ManifestFile{
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
		Files:    []ManifestFile{{Src: "a.md", Dest: "a.md", Tier: "core"}},
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

// ── FormatManifestList tests ────────────────────────────────────

func TestFormatManifestListMultiple(t *testing.T) {
	manifests := []Manifest{
		{ID: "alpha", Name: "Alpha Plugin", Version: "1.0.0", Description: "First plugin", Files: []ManifestFile{{Src: "a.md"}, {Src: "b.md"}}},
		{ID: "beta", Name: "Beta Plugin", Version: "2.0.0", Files: []ManifestFile{{Src: "c.md"}}},
	}

	out := FormatManifestList(manifests)
	if !strings.Contains(out, "alpha") {
		t.Fatal("expected alpha in output")
	}
	if !strings.Contains(out, "Alpha Plugin (v1.0.0) — 2 files") {
		t.Fatalf("expected formatted alpha line, got:\n%s", out)
	}
	if !strings.Contains(out, "First plugin") {
		t.Fatal("expected description in output")
	}
	if !strings.Contains(out, "Beta Plugin (v2.0.0) — 1 files") {
		t.Fatalf("expected formatted beta line, got:\n%s", out)
	}
}

// ── InferCategory tests ─────────────────────────────────────────

func TestInferCategoryPaths(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"instructions/general.md", "instructions"},
		{"prompts/ask.md", "prompts"},
		{"agents/planner.md", "agents"},
		{"skills/test.md", "skills"},
		{"mcp-servers/config.json", "mcp-servers"},
		{"random/file.md", "other"},
		{"readme.md", "other"},
	}
	for _, tc := range cases {
		got := InferCategory(tc.path)
		if got != tc.want {
			t.Errorf("InferCategory(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// ── ListByCategory tests ────────────────────────────────────────

func TestListByCategoryGroupsFiles(t *testing.T) {
	files := []ManifestFile{
		{Src: "instructions/a.md", Dest: "instructions/a.md", Tier: "core"},
		{Src: "skills/b.md", Dest: "skills/b.md", Tier: "core"},
		{Src: "instructions/c.md", Dest: "instructions/c.md", Tier: "core"},
		{Src: "agents/d.md", Dest: "agents/d.md", Tier: "core"},
	}
	m := Manifest{Files: files}

	groups := ListByCategory(files, m, "", "")
	if len(groups) != 3 {
		t.Fatalf("expected 3 category groups, got %d", len(groups))
	}

	// instructions should come first, then skills, then agents (per categoryOrder)
	if groups[0].Category != "instructions" || len(groups[0].Files) != 2 {
		t.Fatalf("expected instructions with 2 files, got %s with %d", groups[0].Category, len(groups[0].Files))
	}
	if groups[1].Category != "skills" || len(groups[1].Files) != 1 {
		t.Fatalf("expected skills with 1 file, got %s with %d", groups[1].Category, len(groups[1].Files))
	}
	if groups[2].Category != "agents" || len(groups[2].Files) != 1 {
		t.Fatalf("expected agents with 1 file, got %s with %d", groups[2].Category, len(groups[2].Files))
	}
}

// ── GetAllowedFileTiers tests ───────────────────────────────────

func TestGetAllowedFileTiersDefaultTiers(t *testing.T) {
	m := Manifest{} // no custom tiers → DefaultTiers

	minimal := GetAllowedFileTiers(m, "minimal")
	if !minimal["core"] || minimal["ad-hoc"] {
		t.Fatalf("minimal should include core only, got %v", minimal)
	}

	standard := GetAllowedFileTiers(m, "standard")
	if !standard["core"] || !standard["ad-hoc"] || standard["ad-hoc-advanced"] {
		t.Fatalf("standard should include core+ad-hoc, got %v", standard)
	}

	advanced := GetAllowedFileTiers(m, "advanced")
	if !advanced["core"] || !advanced["ad-hoc"] || !advanced["ad-hoc-advanced"] {
		t.Fatalf("advanced should include all three, got %v", advanced)
	}
}

func TestGetAllowedFileTiersCustomTiers(t *testing.T) {
	m := Manifest{
		Tiers: []TierDef{
			{ID: "base", Label: "Base"},
			{ID: "full", Label: "Full"},
		},
	}

	base := GetAllowedFileTiers(m, "base")
	if !base["base"] || base["full"] {
		t.Fatalf("base should include only base, got %v", base)
	}

	full := GetAllowedFileTiers(m, "full")
	if !full["base"] || !full["full"] {
		t.Fatalf("full should include base+full (cumulative), got %v", full)
	}
}
