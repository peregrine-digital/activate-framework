package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- readJSONConfig ---

func TestConfig_readJSONConfig_MissingFile(t *testing.T) {
	cfg, err := readJSONConfig("/no/such/path/config.json")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config, got %+v", cfg)
	}
}

func TestConfig_readJSONConfig_InvalidJSON(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "bad.json")
	if err := os.WriteFile(p, []byte(`{not json`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := readJSONConfig(p)
	if err != nil {
		t.Fatalf("expected nil error for invalid JSON, got %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil config for invalid JSON, got %+v", cfg)
	}
}

func TestConfig_readJSONConfig_ValidJSON(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "good.json")
	if err := os.WriteFile(p, []byte(`{"manifest":"ironarch","tier":"minimal"}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := readJSONConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.Manifest != "ironarch" || cfg.Tier != "minimal" {
		t.Fatalf("unexpected config values: %+v", cfg)
	}
}

func TestConfig_readJSONConfig_EmptyObject(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "empty.json")
	if err := os.WriteFile(p, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := readJSONConfig(p)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for empty object")
	}
	if cfg.Manifest != "" || cfg.Tier != "" {
		t.Fatalf("expected zero-value strings, got %+v", cfg)
	}
}

// --- ReadProjectConfig / WriteProjectConfig ---

func TestConfig_ReadProjectConfig_Missing(t *testing.T) {
	tmp := t.TempDir()
	cfg, err := ReadProjectConfig(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Fatalf("expected nil for missing project config, got %+v", cfg)
	}
}

func TestConfig_WriteProjectConfig_CreatesFile(t *testing.T) {
	tmp := t.TempDir()
	err := WriteProjectConfig(tmp, &Config{Manifest: "ironarch", Tier: "minimal"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cfg, err := ReadProjectConfig(tmp)
	if err != nil {
		t.Fatalf("unexpected error reading back: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config after write")
	}
	if cfg.Manifest != "ironarch" || cfg.Tier != "minimal" {
		t.Fatalf("unexpected values: %+v", cfg)
	}
}

func TestConfig_WriteProjectConfig_MergeUpdate(t *testing.T) {
	tmp := t.TempDir()
	// Write initial config
	if err := WriteProjectConfig(tmp, &Config{Manifest: "ironarch", Tier: "full"}); err != nil {
		t.Fatal(err)
	}
	// Merge-update with only tier change; manifest should survive
	if err := WriteProjectConfig(tmp, &Config{Tier: "minimal"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := ReadProjectConfig(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Manifest != "ironarch" {
		t.Fatalf("manifest clobbered: got %q", cfg.Manifest)
	}
	if cfg.Tier != "minimal" {
		t.Fatalf("tier not updated: got %q", cfg.Tier)
	}
}

func TestConfig_WriteProjectConfig_PreservesExistingMaps(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteProjectConfig(tmp, &Config{
		FileOverrides: map[string]string{"a.md": "pinned"},
	}); err != nil {
		t.Fatal(err)
	}
	// Add a second override without clobbering the first
	if err := WriteProjectConfig(tmp, &Config{
		FileOverrides: map[string]string{"b.md": "excluded"},
	}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.FileOverrides["a.md"] != "pinned" {
		t.Fatalf("existing override lost: %v", cfg.FileOverrides)
	}
	if cfg.FileOverrides["b.md"] != "excluded" {
		t.Fatalf("new override not set: %v", cfg.FileOverrides)
	}
}

// --- WriteGlobalConfig / ReadGlobalConfig ---

func TestConfig_WriteGlobalConfig_CreatesDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	err := WriteGlobalConfig(&Config{Manifest: "ironarch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Verify file exists
	p := filepath.Join(home, globalConfigDir, globalConfigFile)
	if _, err := os.Stat(p); os.IsNotExist(err) {
		t.Fatal("global config file was not created")
	}
	cfg, err := ReadGlobalConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || cfg.Manifest != "ironarch" {
		t.Fatalf("unexpected global config: %+v", cfg)
	}
}

func TestConfig_WriteGlobalConfig_MergeUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := WriteGlobalConfig(&Config{Manifest: "ironarch", Tier: "full"}); err != nil {
		t.Fatal(err)
	}
	// Update only tier
	if err := WriteGlobalConfig(&Config{Tier: "minimal"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadGlobalConfig()
	if cfg.Manifest != "ironarch" {
		t.Fatalf("manifest clobbered: got %q", cfg.Manifest)
	}
	if cfg.Tier != "minimal" {
		t.Fatalf("tier not updated: got %q", cfg.Tier)
	}
}

// --- ResolveConfig precedence ---

func TestConfig_ResolveConfig_DefaultsOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tmp := t.TempDir()

	cfg := ResolveConfig(tmp, nil)
	if cfg.Manifest != defaultManifest {
		t.Fatalf("expected default manifest %q, got %q", defaultManifest, cfg.Manifest)
	}
	if cfg.Tier != defaultTier {
		t.Fatalf("expected default tier %q, got %q", defaultTier, cfg.Tier)
	}
	if cfg.FileOverrides == nil {
		t.Fatal("expected initialized FileOverrides map")
	}
	if cfg.SkippedVersions == nil {
		t.Fatal("expected initialized SkippedVersions map")
	}
}

func TestConfig_ResolveConfig_GlobalOverridesDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tmp := t.TempDir()

	if err := WriteGlobalConfig(&Config{Tier: "full"}); err != nil {
		t.Fatal(err)
	}
	cfg := ResolveConfig(tmp, nil)
	if cfg.Manifest != defaultManifest {
		t.Fatalf("manifest should stay default, got %q", cfg.Manifest)
	}
	if cfg.Tier != "full" {
		t.Fatalf("global tier not applied: got %q", cfg.Tier)
	}
}

func TestConfig_ResolveConfig_ProjectOverridesGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tmp := t.TempDir()

	if err := WriteGlobalConfig(&Config{Manifest: "global-manifest", Tier: "full"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteProjectConfig(tmp, &Config{Manifest: "project-manifest"}); err != nil {
		t.Fatal(err)
	}
	cfg := ResolveConfig(tmp, nil)
	if cfg.Manifest != "project-manifest" {
		t.Fatalf("project manifest not applied: got %q", cfg.Manifest)
	}
	if cfg.Tier != "full" {
		t.Fatalf("global tier should survive: got %q", cfg.Tier)
	}
}

func TestConfig_ResolveConfig_OverridesWin(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	tmp := t.TempDir()

	if err := WriteGlobalConfig(&Config{Manifest: "global-manifest"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteProjectConfig(tmp, &Config{Manifest: "project-manifest"}); err != nil {
		t.Fatal(err)
	}
	cfg := ResolveConfig(tmp, &Config{Manifest: "override-manifest"})
	if cfg.Manifest != "override-manifest" {
		t.Fatalf("override should win: got %q", cfg.Manifest)
	}
}

func TestConfig_ResolveConfig_EmptyProjectDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := WriteGlobalConfig(&Config{Tier: "full"}); err != nil {
		t.Fatal(err)
	}
	cfg := ResolveConfig("", nil)
	if cfg.Tier != "full" {
		t.Fatalf("global tier should apply when projectDir is empty: got %q", cfg.Tier)
	}
}

// --- mergeInto ---

func TestConfig_mergeInto_EmptyStringDoesNotOverwrite(t *testing.T) {
	dst := &Config{Manifest: "original", Tier: "full"}
	src := &Config{Manifest: "", Tier: ""}
	mergeInto(dst, src)
	if dst.Manifest != "original" {
		t.Fatalf("empty string overwrote Manifest: got %q", dst.Manifest)
	}
	if dst.Tier != "full" {
		t.Fatalf("empty string overwrote Tier: got %q", dst.Tier)
	}
}

func TestConfig_mergeInto_NonEmptyOverwrites(t *testing.T) {
	dst := &Config{Manifest: "old", Tier: "old"}
	src := &Config{Manifest: "new", Tier: "new"}
	mergeInto(dst, src)
	if dst.Manifest != "new" || dst.Tier != "new" {
		t.Fatalf("expected new values, got %+v", dst)
	}
}

func TestConfig_mergeInto_NilMapDoesNotOverwrite(t *testing.T) {
	dst := &Config{
		FileOverrides: map[string]string{"a": "pinned"},
	}
	src := &Config{FileOverrides: nil}
	mergeInto(dst, src)
	if dst.FileOverrides["a"] != "pinned" {
		t.Fatal("nil src map overwrote dst map")
	}
}

func TestConfig_mergeInto_MapMerge(t *testing.T) {
	dst := &Config{
		FileOverrides: map[string]string{"a": "pinned"},
	}
	src := &Config{
		FileOverrides: map[string]string{"b": "excluded"},
	}
	mergeInto(dst, src)
	if dst.FileOverrides["a"] != "pinned" {
		t.Fatal("existing key lost during merge")
	}
	if dst.FileOverrides["b"] != "excluded" {
		t.Fatal("new key not added during merge")
	}
}

func TestConfig_mergeInto_MapEmptyValueDeletes(t *testing.T) {
	dst := &Config{
		FileOverrides:   map[string]string{"a": "pinned", "b": "excluded"},
		SkippedVersions: map[string]string{"x": "1.0.0"},
	}
	src := &Config{
		FileOverrides:   map[string]string{"a": ""},
		SkippedVersions: map[string]string{"x": ""},
	}
	mergeInto(dst, src)
	if _, ok := dst.FileOverrides["a"]; ok {
		t.Fatal("empty value did not delete FileOverrides key")
	}
	if dst.FileOverrides["b"] != "excluded" {
		t.Fatal("unrelated key was affected")
	}
	if _, ok := dst.SkippedVersions["x"]; ok {
		t.Fatal("empty value did not delete SkippedVersions key")
	}
}

func TestConfig_mergeInto_DstNilMapInitialized(t *testing.T) {
	dst := &Config{}
	src := &Config{
		FileOverrides:   map[string]string{"a": "pinned"},
		SkippedVersions: map[string]string{"x": "1.0.0"},
	}
	mergeInto(dst, src)
	if dst.FileOverrides["a"] != "pinned" {
		t.Fatal("FileOverrides not initialized on dst")
	}
	if dst.SkippedVersions["x"] != "1.0.0" {
		t.Fatal("SkippedVersions not initialized on dst")
	}
}

func TestConfig_mergeInto_BoolPointer(t *testing.T) {
	t.Run("nil does not overwrite", func(t *testing.T) {
		tr := true
		dst := &Config{TelemetryEnabled: &tr}
		src := &Config{TelemetryEnabled: nil}
		mergeInto(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != true {
			t.Fatal("nil *bool overwrote existing value")
		}
	})

	t.Run("true overwrites nil", func(t *testing.T) {
		tr := true
		dst := &Config{}
		src := &Config{TelemetryEnabled: &tr}
		mergeInto(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != true {
			t.Fatal("*bool true not applied")
		}
	})

	t.Run("false overwrites true", func(t *testing.T) {
		tr := true
		fa := false
		dst := &Config{TelemetryEnabled: &tr}
		src := &Config{TelemetryEnabled: &fa}
		mergeInto(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != false {
			t.Fatal("*bool false did not overwrite true")
		}
	})

	t.Run("false overwrites nil", func(t *testing.T) {
		fa := false
		dst := &Config{}
		src := &Config{TelemetryEnabled: &fa}
		mergeInto(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != false {
			t.Fatal("*bool false not applied to nil dst")
		}
	})
}

// --- SetFileOverride ---

func TestConfig_SetFileOverride(t *testing.T) {
	tmp := t.TempDir()
	if err := SetFileOverride(tmp, "agents/planner.md", "pinned"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.FileOverrides["agents/planner.md"] != "pinned" {
		t.Fatalf("override not set: %v", cfg.FileOverrides)
	}
}

func TestConfig_SetFileOverride_ClearsWithEmpty(t *testing.T) {
	tmp := t.TempDir()
	if err := SetFileOverride(tmp, "a.md", "pinned"); err != nil {
		t.Fatal(err)
	}
	if err := SetFileOverride(tmp, "a.md", ""); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if _, ok := cfg.FileOverrides["a.md"]; ok {
		t.Fatal("empty value did not clear override")
	}
}

func TestConfig_SetFileOverride_PreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteProjectConfig(tmp, &Config{Manifest: "ironarch", Tier: "full"}); err != nil {
		t.Fatal(err)
	}
	if err := SetFileOverride(tmp, "a.md", "excluded"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.Manifest != "ironarch" || cfg.Tier != "full" {
		t.Fatalf("SetFileOverride clobbered other fields: %+v", cfg)
	}
}

// --- SetSkippedVersion / ClearSkippedVersion ---

func TestConfig_SetSkippedVersion(t *testing.T) {
	tmp := t.TempDir()
	if err := SetSkippedVersion(tmp, "instructions/security.md", "0.5.0"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.SkippedVersions["instructions/security.md"] != "0.5.0" {
		t.Fatalf("skip not set: %v", cfg.SkippedVersions)
	}
}

func TestConfig_ClearSkippedVersion(t *testing.T) {
	tmp := t.TempDir()
	if err := SetSkippedVersion(tmp, "a.md", "1.0.0"); err != nil {
		t.Fatal(err)
	}
	if err := ClearSkippedVersion(tmp, "a.md"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if _, ok := cfg.SkippedVersions["a.md"]; ok {
		t.Fatal("ClearSkippedVersion did not remove entry")
	}
}

func TestConfig_SetSkippedVersion_PreservesOtherFields(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteProjectConfig(tmp, &Config{
		Manifest:      "ironarch",
		FileOverrides: map[string]string{"x.md": "pinned"},
	}); err != nil {
		t.Fatal(err)
	}
	if err := SetSkippedVersion(tmp, "a.md", "2.0.0"); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.Manifest != "ironarch" {
		t.Fatalf("manifest clobbered: %q", cfg.Manifest)
	}
	if cfg.FileOverrides["x.md"] != "pinned" {
		t.Fatalf("file override lost: %v", cfg.FileOverrides)
	}
}

// --- TelemetryEnabled round-trip ---

func TestConfig_TelemetryEnabled_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	fa := false
	if err := WriteProjectConfig(tmp, &Config{TelemetryEnabled: &fa}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.TelemetryEnabled == nil {
		t.Fatal("TelemetryEnabled lost on round-trip")
	}
	if *cfg.TelemetryEnabled != false {
		t.Fatal("TelemetryEnabled changed value on round-trip")
	}
}

func TestConfig_TelemetryEnabled_NilByDefault(t *testing.T) {
	tmp := t.TempDir()
	if err := WriteProjectConfig(tmp, &Config{Manifest: "test"}); err != nil {
		t.Fatal(err)
	}
	cfg, _ := ReadProjectConfig(tmp)
	if cfg.TelemetryEnabled != nil {
		t.Fatalf("TelemetryEnabled should be nil when not set, got %v", *cfg.TelemetryEnabled)
	}
}

// --- JSON serialization ---

func TestConfig_JSONOmitsEmptyMaps(t *testing.T) {
	cfg := Config{Manifest: "test", Tier: "standard"}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["fileOverrides"]; ok {
		t.Fatal("nil FileOverrides should be omitted from JSON")
	}
	if _, ok := raw["skippedVersions"]; ok {
		t.Fatal("nil SkippedVersions should be omitted from JSON")
	}
	if _, ok := raw["telemetryEnabled"]; ok {
		t.Fatal("nil TelemetryEnabled should be omitted from JSON")
	}
}
