package model

import (
	"strings"
	"testing"
)

// ── FormatManifestList tests ────────────────────────────────────

func TestFormatManifestListMultiple(t *testing.T) {
	manifests := []Manifest{
		{ID: "alpha", Name: "Alpha Plugin", Description: "First plugin", Files: []ManifestFile{{Src: "a.md"}, {Src: "b.md"}}},
		{ID: "beta", Name: "Beta Plugin", Files: []ManifestFile{{Src: "c.md"}}},
	}

	out := FormatManifestList(manifests)
	if !strings.Contains(out, "alpha") {
		t.Fatal("expected alpha in output")
	}
	if !strings.Contains(out, "Alpha Plugin — 2 files") {
		t.Fatalf("expected formatted alpha line, got:\n%s", out)
	}
	if !strings.Contains(out, "First plugin") {
		t.Fatal("expected description in output")
	}
	if !strings.Contains(out, "Beta Plugin — 1 files") {
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
