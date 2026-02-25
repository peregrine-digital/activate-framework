package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsTelemetryEnabled(t *testing.T) {
	// Default: disabled (opt-in)
	if IsTelemetryEnabled(Config{}) {
		t.Fatal("expected disabled by default")
	}

	// Explicitly false
	f := false
	if IsTelemetryEnabled(Config{TelemetryEnabled: &f}) {
		t.Fatal("expected disabled when false")
	}

	// Explicitly true
	tr := true
	if !IsTelemetryEnabled(Config{TelemetryEnabled: &tr}) {
		t.Fatal("expected enabled when true")
	}
}

func TestExtractPremiumQuota(t *testing.T) {
	data := map[string]interface{}{
		"quota_snapshots": map[string]interface{}{
			"premium": map[string]interface{}{
				"quota_id":    "premium_interactions",
				"entitlement": float64(300),
				"remaining":   float64(142),
			},
		},
	}

	ent, rem := ExtractPremiumQuota(data)
	if ent == nil || *ent != 300 {
		t.Fatalf("expected entitlement 300, got %v", ent)
	}
	if rem == nil || *rem != 142 {
		t.Fatalf("expected remaining 142, got %v", rem)
	}
}

func TestExtractPremiumQuotaUnlimited(t *testing.T) {
	data := map[string]interface{}{
		"quota_snapshots": map[string]interface{}{
			"premium": map[string]interface{}{
				"quota_id":  "premium_interactions",
				"unlimited": true,
			},
		},
	}

	ent, rem := ExtractPremiumQuota(data)
	if ent != nil || rem != nil {
		t.Fatal("expected nil for unlimited quota")
	}
}

func TestExtractPremiumQuotaMissing(t *testing.T) {
	ent, rem := ExtractPremiumQuota(map[string]interface{}{})
	if ent != nil || rem != nil {
		t.Fatal("expected nil for missing data")
	}
}

func TestBuildTelemetryEntry(t *testing.T) {
	data := map[string]interface{}{
		"quota_snapshots": map[string]interface{}{
			"premium": map[string]interface{}{
				"quota_id":    "premium_interactions",
				"entitlement": float64(300),
				"remaining":   float64(142),
			},
		},
		"quota_reset_date_utc": "2026-03-01T00:00:00Z",
	}

	entry := BuildTelemetryEntry(data)
	if entry.PremiumEntitlement == nil || *entry.PremiumEntitlement != 300 {
		t.Fatalf("expected entitlement 300, got %v", entry.PremiumEntitlement)
	}
	if entry.PremiumUsed == nil || *entry.PremiumUsed != 158 {
		t.Fatalf("expected used 158, got %v", entry.PremiumUsed)
	}
	if entry.QuotaResetDateUTC != "2026-03-01T00:00:00Z" {
		t.Fatalf("expected reset date, got %q", entry.QuotaResetDateUTC)
	}
	if entry.Source != "github_copilot_internal" {
		t.Fatalf("expected source, got %q", entry.Source)
	}
	if entry.Version != 1 {
		t.Fatalf("expected version 1, got %d", entry.Version)
	}
	if entry.Date == "" || entry.Timestamp == "" {
		t.Fatal("expected date and timestamp")
	}
}

func TestBuildTelemetryEntryNoQuota(t *testing.T) {
	entry := BuildTelemetryEntry(map[string]interface{}{})
	if entry.PremiumEntitlement != nil {
		t.Fatal("expected nil entitlement")
	}
	if entry.PremiumUsed != nil {
		t.Fatal("expected nil used")
	}
}

func TestAppendAndReadTelemetryLog(t *testing.T) {
	homeDir := t.TempDir()
	old := activateBaseDir
	activateBaseDir = homeDir
	t.Cleanup(func() { activateBaseDir = old })

	ent := 300
	rem := 200
	used := 100
	entry := TelemetryEntry{
		Date:               "2026-02-25",
		Timestamp:          "2026-02-25T14:00:00.000Z",
		PremiumEntitlement: &ent,
		PremiumRemaining:   &rem,
		PremiumUsed:        &used,
		Source:             "github_copilot_internal",
		Version:            1,
	}

	if err := AppendTelemetryEntry(entry); err != nil {
		t.Fatal(err)
	}
	// Append a second entry
	if err := AppendTelemetryEntry(entry); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadTelemetryLog()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if *entries[0].PremiumEntitlement != 300 {
		t.Fatalf("expected entitlement 300, got %d", *entries[0].PremiumEntitlement)
	}
}

func TestReadTelemetryLogMissing(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	t.Cleanup(func() { activateBaseDir = old })
	entries, err := ReadTelemetryLog()
	if err != nil {
		t.Fatal(err)
	}
	if entries != nil {
		t.Fatalf("expected nil, got %v", entries)
	}
}

func TestArchiveLogIfNeeded(t *testing.T) {
	homeDir := t.TempDir()
	old := activateBaseDir
	activateBaseDir = homeDir
	t.Cleanup(func() { activateBaseDir = old })

	// Create active log
	os.MkdirAll(homeDir, 0755)
	activePath := filepath.Join(homeDir, telemetryLogFile)
	os.WriteFile(activePath, []byte(`{"date":"2026-02-24"}`+"\n"), 0644)

	archivePath, err := ArchiveLogIfNeeded("2026-03-01T00:00:00Z", "2026-02-01T00:00:00Z")
	if err != nil {
		t.Fatal(err)
	}
	if archivePath == "" {
		t.Fatal("expected archive path")
	}
	if !strings.Contains(archivePath, "copilot-telemetry-2026-02-01.jsonl") {
		t.Fatalf("unexpected archive name: %s", archivePath)
	}

	// Active log should be gone
	if _, err := os.Stat(activePath); !os.IsNotExist(err) {
		t.Fatal("expected active log removed after archive")
	}

	// Archive should exist
	if _, err := os.Stat(archivePath); err != nil {
		t.Fatal("expected archive file to exist")
	}
}

func TestArchiveLogIfNeededSameDate(t *testing.T) {
	archivePath, err := ArchiveLogIfNeeded("2026-02-01", "2026-02-01")
	if err != nil {
		t.Fatal(err)
	}
	if archivePath != "" {
		t.Fatalf("expected no archive for same date, got %s", archivePath)
	}
}

func TestArchiveLogIfNeededNoPrevious(t *testing.T) {
	archivePath, err := ArchiveLogIfNeeded("2026-03-01", "")
	if err != nil {
		t.Fatal(err)
	}
	if archivePath != "" {
		t.Fatalf("expected no archive for empty previous, got %s", archivePath)
	}
}

func TestTelemetryEnabledInConfig(t *testing.T) {
	homeDir := t.TempDir()
	old := activateBaseDir
	activateBaseDir = homeDir
	t.Cleanup(func() { activateBaseDir = old })

	tr := true
	if err := WriteGlobalConfig(&Config{TelemetryEnabled: &tr}); err != nil {
		t.Fatal(err)
	}

	cfg, err := ReadGlobalConfig()
	if err != nil || cfg == nil {
		t.Fatal("expected config")
	}
	if cfg.TelemetryEnabled == nil || !*cfg.TelemetryEnabled {
		t.Fatal("expected telemetry enabled in persisted config")
	}
}

func TestTelemetryLogJSONLFormat(t *testing.T) {
	homeDir := t.TempDir()
	old := activateBaseDir
	activateBaseDir = homeDir
	t.Cleanup(func() { activateBaseDir = old })

	ent := 300
	entry := TelemetryEntry{
		Date:               "2026-02-25",
		Timestamp:          "2026-02-25T14:00:00Z",
		PremiumEntitlement: &ent,
		Source:             "github_copilot_internal",
		Version:            1,
	}
	AppendTelemetryEntry(entry)

	// Read raw file and verify it's valid JSONL
	data, _ := os.ReadFile(filepath.Join(homeDir, telemetryLogFile))
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("expected valid JSON line, got error: %s", err)
	}
	if parsed["source"] != "github_copilot_internal" {
		t.Fatalf("unexpected source: %v", parsed["source"])
	}
}
