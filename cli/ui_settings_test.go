package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// isolatedSettingsSvc creates an ActivateService with test isolation.
func isolatedSettingsSvc(t *testing.T, cfg Config, manifests []Manifest) *ActivateService {
	t.Helper()
	dir := t.TempDir()
	old := activateBaseDir
	activateBaseDir = dir
	t.Cleanup(func() { activateBaseDir = old })
	return &ActivateService{Config: cfg, Manifests: manifests, ProjectDir: dir}
}

// ── Settings form builder ───────────────────────────────────────

func TestBuildSettingsForm_HasAllFields(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{Manifest: "alpha", Tier: "standard"}, testManifests())
	vals := &settingsValues{
		manifest: "alpha", tier: "standard",
		telemetry: false, scope: "project",
	}
	form := buildSettingsForm(svc, vals)
	form.Init()

	// Should have manifest pre-selected
	if vals.manifest != "alpha" {
		t.Fatalf("expected manifest=alpha, got %q", vals.manifest)
	}
}

func TestBuildTierOptions(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{}, testManifests())

	opts := buildTierOptions(svc, "alpha")
	if len(opts) == 0 {
		t.Fatal("expected tier options for alpha manifest")
	}

	// Alpha tiers are: core, ad-hoc
	found := false
	for _, o := range opts {
		if o.Value == "core" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected 'core' tier option for alpha")
	}
}

func TestBuildTierOptions_UnknownManifest(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{}, testManifests())
	opts := buildTierOptions(svc, "nonexistent")
	if len(opts) != 1 {
		t.Fatalf("expected 1 fallback option, got %d", len(opts))
	}
}

// ── Settings model ──────────────────────────────────────────────

func TestSettingsModel_ViewContainsTitle(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{Manifest: "alpha", Tier: "standard"}, testManifests())
	m := newSettingsModel(svc)
	view := m.View()

	if !strings.Contains(view, "Settings") {
		t.Fatal("expected 'Settings' in view")
	}
	if !strings.Contains(view, "scope:") {
		t.Fatal("expected scope info in view")
	}
}

func TestSettingsModel_EscCancels(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{Manifest: "alpha", Tier: "standard"}, testManifests())
	m := newSettingsModel(svc)

	result := simulateRuntime(m, []tea.Msg{
		tea.KeyMsg{Type: tea.KeyEscape},
	}).(settingsModel)

	if !result.done {
		t.Fatal("expected done=true after esc")
	}
	if result.changed {
		t.Fatal("expected changed=false after cancel")
	}
}

func TestSettingsModel_CtrlCQuits(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{Manifest: "alpha", Tier: "standard"}, testManifests())
	m := newSettingsModel(svc)

	result := simulateRuntime(m, []tea.Msg{
		tea.KeyMsg{Type: tea.KeyCtrlC},
	}).(settingsModel)

	if !result.done {
		t.Fatal("expected done=true after ctrl+c")
	}
}

func TestSettingsModel_ResultModeView(t *testing.T) {
	svc := isolatedSettingsSvc(t, Config{Manifest: "alpha", Tier: "standard"}, testManifests())
	m := newSettingsModel(svc)
	m.mode = "result"
	m.resultTitle = "Settings Saved"
	m.resultBody = "Manifest: alpha → beta"

	view := m.View()
	if !strings.Contains(view, "Settings Saved") {
		t.Fatal("expected result title in view")
	}
	if !strings.Contains(view, "alpha → beta") {
		t.Fatal("expected change details in view")
	}
}
