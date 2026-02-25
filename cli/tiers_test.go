package main

import (
	"testing"
)

func TestGetManifestTiers_DefaultTiers(t *testing.T) {
	m := Manifest{}
	tiers := GetManifestTiers(m)
	if len(tiers) != len(DefaultTiers) {
		t.Fatalf("expected %d default tiers, got %d", len(DefaultTiers), len(tiers))
	}
	if tiers[0].ID != "minimal" || tiers[1].ID != "standard" || tiers[2].ID != "advanced" {
		t.Fatalf("unexpected default tier IDs: %v", tiers)
	}
}

func TestGetManifestTiers_CustomTiersCumulative(t *testing.T) {
	m := Manifest{
		Tiers: []TierDef{
			{ID: "base", Label: "Base"},
			{ID: "pro", Label: "Pro"},
			{ID: "enterprise", Label: "Enterprise"},
		},
	}
	tiers := GetManifestTiers(m)
	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(tiers))
	}
	// Verify cumulative includes
	if len(tiers[0].Includes) != 1 || tiers[0].Includes[0] != "base" {
		t.Fatalf("tier 0 includes wrong: %v", tiers[0].Includes)
	}
	if len(tiers[1].Includes) != 2 || tiers[1].Includes[0] != "base" || tiers[1].Includes[1] != "pro" {
		t.Fatalf("tier 1 includes wrong: %v", tiers[1].Includes)
	}
	if len(tiers[2].Includes) != 3 {
		t.Fatalf("tier 2 includes wrong: %v", tiers[2].Includes)
	}
}

func TestGetManifestTiers_CustomTierMissingLabel(t *testing.T) {
	m := Manifest{Tiers: []TierDef{{ID: "only"}}}
	tiers := GetManifestTiers(m)
	if tiers[0].Label != "only" {
		t.Fatalf("expected ID as label fallback, got %q", tiers[0].Label)
	}
}

func TestDiscoverAvailableTiers_FiltersEmpty(t *testing.T) {
	m := Manifest{
		Files: []ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "core"},
		},
	}
	tiers := DiscoverAvailableTiers(m)
	// "minimal" includes "core", so it should appear.
	// "standard" includes "core" too.
	// "advanced" includes "core" too.
	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers with core files, got %d", len(tiers))
	}
}

func TestDiscoverAvailableTiers_OnlyMatchingTiers(t *testing.T) {
	m := Manifest{
		Files: []ManifestFile{
			{Src: "a.md", Dest: "a.md", Tier: "ad-hoc"},
		},
	}
	tiers := DiscoverAvailableTiers(m)
	// "minimal" only includes "core" → excluded
	// "standard" includes "ad-hoc" → included
	// "advanced" includes "ad-hoc" → included
	if len(tiers) != 2 {
		t.Fatalf("expected 2 tiers (standard+advanced), got %d", len(tiers))
	}
	if tiers[0].ID != "standard" || tiers[1].ID != "advanced" {
		t.Fatalf("unexpected tiers: %v", tiers)
	}
}

func TestSelectFiles_FiltersByTier(t *testing.T) {
	m := Manifest{}
	files := []ManifestFile{
		{Src: "a.md", Dest: "a.md", Tier: "core"},
		{Src: "b.md", Dest: "b.md", Tier: "ad-hoc"},
		{Src: "c.md", Dest: "c.md", Tier: "ad-hoc-advanced"},
	}
	minimal := SelectFiles(files, m, "minimal")
	if len(minimal) != 1 || minimal[0].Src != "a.md" {
		t.Fatalf("minimal should have 1 core file, got %d", len(minimal))
	}
	standard := SelectFiles(files, m, "standard")
	if len(standard) != 2 {
		t.Fatalf("standard should have 2 files, got %d", len(standard))
	}
	advanced := SelectFiles(files, m, "advanced")
	if len(advanced) != 3 {
		t.Fatalf("advanced should have 3 files, got %d", len(advanced))
	}
}

func TestGetAllowedFileTiers_UnknownTierFallsBack(t *testing.T) {
	m := Manifest{}
	allowed := GetAllowedFileTiers(m, "nonexistent")
	// Should fall back to "standard"
	if !allowed["core"] || !allowed["ad-hoc"] {
		t.Fatalf("expected standard fallback, got %v", allowed)
	}
}

func TestGetAllowedFileTiers_EmptyTierDefs(t *testing.T) {
	m := Manifest{Tiers: []TierDef{}}
	allowed := GetAllowedFileTiers(m, "anything")
	if !allowed["core"] {
		t.Fatalf("expected core fallback for empty tiers, got %v", allowed)
	}
}

func TestInferCategory(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"instructions/general.md", "instructions"},
		{"prompts/coding.md", "prompts"},
		{"skills/build.md", "skills"},
		{"agents/planner.md", "agents"},
		{"mcp-servers/viewer.json", "mcp-servers"},
		{"random/thing.md", "other"},
		{"", "other"},
	}
	for _, tt := range tests {
		got := InferCategory(tt.path)
		if got != tt.want {
			t.Errorf("InferCategory(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestListByCategory_GroupsCorrectly(t *testing.T) {
	m := Manifest{}
	files := []ManifestFile{
		{Src: "instructions/a.md", Dest: "instructions/a.md", Tier: "core"},
		{Src: "prompts/b.md", Dest: "prompts/b.md", Tier: "core"},
		{Src: "instructions/c.md", Dest: "instructions/c.md", Tier: "core"},
	}
	groups := ListByCategory(files, m, "minimal", "")
	if len(groups) != 2 {
		t.Fatalf("expected 2 category groups, got %d", len(groups))
	}
	// Instructions should come first (categoryOrder)
	if groups[0].Category != "instructions" || len(groups[0].Files) != 2 {
		t.Fatalf("expected 2 instruction files, got %v", groups[0])
	}
	if groups[1].Category != "prompts" || len(groups[1].Files) != 1 {
		t.Fatalf("expected 1 prompt file, got %v", groups[1])
	}
}

func TestListByCategory_FiltersByCategory(t *testing.T) {
	m := Manifest{}
	files := []ManifestFile{
		{Src: "instructions/a.md", Dest: "instructions/a.md", Tier: "core"},
		{Src: "prompts/b.md", Dest: "prompts/b.md", Tier: "core"},
	}
	groups := ListByCategory(files, m, "", "prompts")
	if len(groups) != 1 || groups[0].Category != "prompts" {
		t.Fatalf("expected only prompts group, got %v", groups)
	}
}

func TestListByCategory_UsesExplicitCategory(t *testing.T) {
	m := Manifest{}
	files := []ManifestFile{
		{Src: "other/thing.md", Dest: "other/thing.md", Tier: "core", Category: "skills"},
	}
	groups := ListByCategory(files, m, "minimal", "")
	if len(groups) != 1 || groups[0].Category != "skills" {
		t.Fatalf("expected explicit category override, got %v", groups)
	}
}
