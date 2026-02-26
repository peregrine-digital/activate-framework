package main

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// isolatedSvc creates an ActivateService isolated from the real ~/.activate dir.
func isolatedTelemetrySvc(t *testing.T, cfg Config) *ActivateService {
	t.Helper()
	dir := t.TempDir()
	old := activateBaseDir
	activateBaseDir = dir
	t.Cleanup(func() { activateBaseDir = old })
	return &ActivateService{Config: cfg, ProjectDir: dir}
}

// ── Telemetry form builder ──────────────────────────────────────

func TestBuildTelemetryMenuForm_EnabledState(t *testing.T) {
	action := ""
	form := buildTelemetryMenuForm(true, &action)
	form.Init()

	// First option is "Run telemetry now"
	if action != "run" {
		t.Fatalf("expected first action=run, got %q", action)
	}

	// Navigate through all options
	expected := []string{"run", "toggle", "log", "back"}
	for i := 1; i < len(expected); i++ {
		form = sendKey(form, tea.KeyDown)
		if action != expected[i] {
			t.Fatalf("after down %d: expected %q, got %q", i, expected[i], action)
		}
	}
}

func TestBuildTelemetryMenuForm_DisabledState(t *testing.T) {
	action := ""
	form := buildTelemetryMenuForm(false, &action)
	form.Init()

	// Navigate to toggle
	form = sendKey(form, tea.KeyDown)
	if action != "toggle" {
		t.Fatalf("expected toggle, got %q", action)
	}
}

// ── Telemetry formatting ────────────────────────────────────────

func TestFormatTelemetrySummary(t *testing.T) {
	used := 847
	total := 1000
	remaining := 153
	entry := TelemetryEntry{
		Date:               "2026-02-25",
		PremiumUsed:        &used,
		PremiumEntitlement: &total,
		PremiumRemaining:   &remaining,
		QuotaResetDateUTC:  "2026-02-27",
	}

	summary := formatTelemetrySummary(entry)
	if !strings.Contains(summary, "847") {
		t.Fatal("expected used count in summary")
	}
	if !strings.Contains(summary, "153") {
		t.Fatal("expected remaining in summary")
	}
	if !strings.Contains(summary, "2026-02-27") {
		t.Fatal("expected reset date in summary")
	}
}

func TestFormatTelemetrySummary_Empty(t *testing.T) {
	entry := TelemetryEntry{Date: "2026-02-25"}
	summary := formatTelemetrySummary(entry)
	if !strings.Contains(summary, "2026-02-25") {
		t.Fatal("expected date in summary")
	}
}

func TestFormatTelemetryEntry(t *testing.T) {
	used := 500
	total := 1000
	remaining := 500
	entry := TelemetryEntry{
		Date:               "2026-02-25",
		PremiumUsed:        &used,
		PremiumEntitlement: &total,
		PremiumRemaining:   &remaining,
		Source:             "cli",
	}

	text := formatTelemetryEntry(entry)
	if !strings.Contains(text, "500 / 1000") {
		t.Fatal("expected usage in entry")
	}
	if !strings.Contains(text, "cli") {
		t.Fatal("expected source in entry")
	}
}

func TestFormatTelemetryLog(t *testing.T) {
	used1, used2 := 100, 200
	total := 1000
	remaining1, remaining2 := 900, 800

	entries := []TelemetryEntry{
		{Date: "2026-02-24", PremiumUsed: &used1, PremiumEntitlement: &total, PremiumRemaining: &remaining1},
		{Date: "2026-02-25", PremiumUsed: &used2, PremiumEntitlement: &total, PremiumRemaining: &remaining2},
	}

	log := formatTelemetryLog(entries)
	if !strings.Contains(log, "2026-02-25") {
		t.Fatal("expected most recent date first")
	}
	if !strings.Contains(log, "Date") {
		t.Fatal("expected header row")
	}
}

func TestFormatTelemetryLog_Empty(t *testing.T) {
	log := formatTelemetryLog([]TelemetryEntry{})
	if !strings.Contains(log, "Date") {
		t.Fatal("expected header even for empty log")
	}
}

func TestFormatTelemetryLog_LimitsTo14(t *testing.T) {
	used, total, remaining := 100, 1000, 900
	entries := make([]TelemetryEntry, 20)
	for i := range entries {
		entries[i] = TelemetryEntry{
			Date: "2026-02-" + strings.Repeat("0", 2-len(string(rune('0'+i%10))))[0:0] + string(rune('0'+i%10)),
			PremiumUsed: &used, PremiumEntitlement: &total, PremiumRemaining: &remaining,
		}
	}

	log := formatTelemetryLog(entries)
	lines := strings.Split(log, "\n")
	// Header + separator + 14 data lines = 16
	if len(lines) > 16 {
		t.Fatalf("expected max 16 lines, got %d", len(lines))
	}
}

// ── Telemetry model ─────────────────────────────────────────────

func TestTelemetryModel_ViewMenuMode(t *testing.T) {
	svc := isolatedTelemetrySvc(t, Config{})
	m := newTelemetryModel(svc)

	view := m.View()
	if !strings.Contains(view, "Telemetry") {
		t.Fatal("expected 'Telemetry' in view")
	}
	if !strings.Contains(view, "disabled") {
		t.Fatal("expected 'disabled' status in view")
	}
}

func TestTelemetryModel_ViewEnabledStatus(t *testing.T) {
	enabled := true
	svc := isolatedTelemetrySvc(t, Config{TelemetryEnabled: &enabled})
	// Persist config so refreshConfig picks it up
	_ = WriteGlobalConfig(&Config{TelemetryEnabled: &enabled})
	m := newTelemetryModel(svc)

	view := m.View()
	if !strings.Contains(view, "enabled") {
		t.Fatal("expected 'enabled' status in view")
	}
}

func TestTelemetryModel_EscQuits(t *testing.T) {
	svc := isolatedTelemetrySvc(t, Config{})
	m := newTelemetryModel(svc)

	result := simulateRuntime(m, []tea.Msg{
		tea.KeyMsg{Type: tea.KeyEscape},
	}).(telemetryModel)

	if !result.done {
		t.Fatal("expected done=true after esc")
	}
}

func TestTelemetryModel_NavigateToBack(t *testing.T) {
	svc := isolatedTelemetrySvc(t, Config{})
	m := newTelemetryModel(svc)

	// Menu: [run, toggle, log, back] — 3 downs + enter
	keys := []tea.Msg{
		tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyEnter},
	}

	result := simulateRuntime(m, keys).(telemetryModel)
	if !result.done {
		t.Fatal("expected done=true after selecting back")
	}
}

func TestTelemetryModel_TextModeView(t *testing.T) {
	svc := isolatedTelemetrySvc(t, Config{})
	m := newTelemetryModel(svc)
	m.mode = "text"
	m.textTitle = "Telemetry Log"
	m.textBody = "some log data"

	view := m.View()
	if !strings.Contains(view, "Telemetry Log") {
		t.Fatal("expected title in text view")
	}
	if !strings.Contains(view, "some log data") {
		t.Fatal("expected body in text view")
	}
}
