package model

import (
	"encoding/json"
	"testing"
)

// --- MergeConfig ---

func TestConfig_mergeInto_EmptyStringDoesNotOverwrite(t *testing.T) {
	dst := &Config{Manifest: "original", Tier: "full"}
	src := &Config{Manifest: "", Tier: ""}
	MergeConfig(dst, src)
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
	MergeConfig(dst, src)
	if dst.Manifest != "new" || dst.Tier != "new" {
		t.Fatalf("expected new values, got %+v", dst)
	}
}

func TestConfig_mergeInto_ClearValueUnsetsField(t *testing.T) {
	dst := &Config{Manifest: "my-manifest", Tier: "advanced"}
	src := &Config{Manifest: ClearValue, Tier: ClearValue}
	MergeConfig(dst, src)
	if dst.Manifest != "" {
		t.Fatalf("expected Manifest cleared, got %q", dst.Manifest)
	}
	if dst.Tier != "" {
		t.Fatalf("expected Tier cleared, got %q", dst.Tier)
	}
}

func TestConfig_mergeInto_NilMapDoesNotOverwrite(t *testing.T) {
	dst := &Config{
		FileOverrides: map[string]string{"a": "pinned"},
	}
	src := &Config{FileOverrides: nil}
	MergeConfig(dst, src)
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
	MergeConfig(dst, src)
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
	MergeConfig(dst, src)
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
	MergeConfig(dst, src)
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
		MergeConfig(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != true {
			t.Fatal("nil *bool overwrote existing value")
		}
	})

	t.Run("true overwrites nil", func(t *testing.T) {
		tr := true
		dst := &Config{}
		src := &Config{TelemetryEnabled: &tr}
		MergeConfig(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != true {
			t.Fatal("*bool true not applied")
		}
	})

	t.Run("false overwrites true", func(t *testing.T) {
		tr := true
		fa := false
		dst := &Config{TelemetryEnabled: &tr}
		src := &Config{TelemetryEnabled: &fa}
		MergeConfig(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != false {
			t.Fatal("*bool false did not overwrite true")
		}
	})

	t.Run("false overwrites nil", func(t *testing.T) {
		fa := false
		dst := &Config{}
		src := &Config{TelemetryEnabled: &fa}
		MergeConfig(dst, src)
		if dst.TelemetryEnabled == nil || *dst.TelemetryEnabled != false {
			t.Fatal("*bool false not applied to nil dst")
		}
	})
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
