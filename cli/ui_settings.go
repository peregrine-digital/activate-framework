package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── Settings screen ─────────────────────────────────────────────
//
// Full-screen settings editor that mirrors the extension's settings
// page. Allows editing manifest, tier, telemetry, and scope.
//
// Called from RunInteractiveMenu as a sub-screen.

// settingsValues holds heap-allocated form bindings.
type settingsValues struct {
	manifest  string
	tier      string
	telemetry bool
	scope     string
}

type settingsModel struct {
	svc    ActivateAPI
	vals   *settingsValues
	form   *huh.Form
	mode   string // "form", "result"
	width  int
	height int

	// result display
	resultTitle string
	resultBody  string

	// signal that we're done
	done    bool
	changed bool
}

// RunSettings launches the settings screen as a fullscreen Bubble Tea program.
// Returns true if settings were changed (caller should sync).
func RunSettings(svc ActivateAPI) (changed bool, err error) {
	m := newSettingsModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return false, err
	}
	if result, ok := final.(settingsModel); ok {
		return result.changed, nil
	}
	return false, nil
}

func newSettingsModel(svc ActivateAPI) settingsModel {
	svc.RefreshConfig()
	cfg := svc.CurrentConfig()
	telemetryOn := cfg.TelemetryEnabled != nil && *cfg.TelemetryEnabled

	vals := &settingsValues{
		manifest:  cfg.Manifest,
		tier:      cfg.Tier,
		telemetry: telemetryOn,
		scope:     "project",
	}

	form := buildSettingsForm(svc, vals)
	return settingsModel{
		svc:  svc,
		vals: vals,
		form: form,
		mode: "form",
	}
}

func (m settingsModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m settingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.done = true
			return m, tea.Quit
		case "esc":
			if m.mode == "result" {
				m.done = true
				return m, tea.Quit
			}
			// esc in form mode = cancel
			m.done = true
			return m, tea.Quit
		case "enter", "q":
			if m.mode == "result" {
				m.done = true
				return m, tea.Quit
			}
		}
	}

	if m.mode == "result" {
		return m, nil
	}

	updated, cmd := m.form.Update(msg)
	f, ok := updated.(*huh.Form)
	if !ok {
		return m, cmd
	}
	m.form = f

	if f.State == huh.StateAborted {
		m.done = true
		return m, tea.Quit
	}

	if f.State == huh.StateCompleted {
		return m.saveSettings()
	}

	return m, cmd
}

func (m settingsModel) View() string {
	var sections []string
	sections = append(sections, renderBanner())
	sections = append(sections, "")
	sections = append(sections, dimStyle.Render("  Settings"))

	switch m.mode {
	case "form":
		scopeLabel := "project"
		if m.vals.scope == "global" {
			scopeLabel = "global (~/.activate/config.json)"
		}
		sections = append(sections, dimStyle.Render("  scope: "+scopeLabel))
		sections = append(sections, "")
		sections = append(sections, m.form.View())
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  ↑/↓ navigate · enter confirm · esc cancel · ctrl+c quit"))

	case "result":
		sections = append(sections, "")
		body := strings.TrimSpace(m.resultBody)
		if body == "" {
			body = "(no changes)"
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGreen)).
			Padding(1, 2).
			Render(m.resultTitle + "\n\n" + body)
		sections = append(sections, box)
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  enter/esc to close · ctrl+c quit"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	if m.height > 0 {
		contentLines := strings.Count(content, "\n") + 1
		topPad := (m.height - contentLines) / 4
		if topPad > 1 {
			content = strings.Repeat("\n", topPad) + content
		}
	}
	return content
}

// ── Settings logic ──────────────────────────────────────────────

func (m settingsModel) saveSettings() (tea.Model, tea.Cmd) {
	updates := &Config{}
	changes := []string{}

	m.svc.RefreshConfig()
	currentCfg := m.svc.CurrentConfig()

	if m.vals.manifest != currentCfg.Manifest {
		updates.Manifest = m.vals.manifest
		changes = append(changes, fmt.Sprintf("Manifest: %s → %s", currentCfg.Manifest, m.vals.manifest))
	}
	if m.vals.tier != currentCfg.Tier {
		updates.Tier = m.vals.tier
		changes = append(changes, fmt.Sprintf("Tier: %s → %s", currentCfg.Tier, m.vals.tier))
	}

	currentTelemetry := currentCfg.TelemetryEnabled != nil && *currentCfg.TelemetryEnabled
	if m.vals.telemetry != currentTelemetry {
		updates.TelemetryEnabled = &m.vals.telemetry
		if m.vals.telemetry {
			changes = append(changes, "Telemetry: off → on")
		} else {
			changes = append(changes, "Telemetry: on → off")
		}
	}

	if len(changes) == 0 {
		m.mode = "result"
		m.resultTitle = "No Changes"
		m.resultBody = "Settings are unchanged."
		return m, nil
	}

	_, err := m.svc.SetConfig(m.vals.scope, updates)
	if err != nil {
		m.mode = "result"
		m.resultTitle = "Error"
		m.resultBody = err.Error()
		return m, nil
	}

	// Sync if manifest or tier changed
	syncMsg := ""
	if updates.Manifest != "" || updates.Tier != "" {
		result, syncErr := m.svc.Sync()
		if syncErr != nil {
			syncMsg = "\n\nSync error: " + syncErr.Error()
		} else if result.Action != "none" && result.Action != "" {
			syncMsg = fmt.Sprintf("\n\nSynced: %s (%d files updated)",
				result.Action, len(result.Updated))
		}
	}

	m.mode = "result"
	m.changed = true
	m.resultTitle = "Settings Saved"
	m.resultBody = strings.Join(changes, "\n") + syncMsg
	return m, nil
}

// ── Form builder ────────────────────────────────────────────────

func buildSettingsForm(svc ActivateAPI, vals *settingsValues) *huh.Form {
	manifests := svc.ListManifests()
	manifestOpts := make([]huh.Option[string], 0, len(manifests))
	for _, m := range manifests {
		label := fmt.Sprintf("%s (v%s)", m.Name, m.Version)
		manifestOpts = append(manifestOpts, huh.NewOption(label, m.ID))
	}

	// Build tier options from the currently selected manifest
	tierOpts := buildTierOptions(svc, vals.manifest)

	scopeOpts := []huh.Option[string]{
		huh.NewOption("Project (this repo only)", "project"),
		huh.NewOption("Global (all repos)", "global"),
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Manifest").
				Description("Framework to install").
				Options(manifestOpts...).
				Value(&vals.manifest),
			huh.NewSelect[string]().
				Title("Tier").
				Description("Content tier level").
				Options(tierOpts...).
				Value(&vals.tier),
			huh.NewConfirm().
				Title("Telemetry").
				Description("Track Copilot usage quota").
				Affirmative("  Enabled  ").
				Negative("  Disabled  ").
				Value(&vals.telemetry),
			huh.NewSelect[string]().
				Title("Scope").
				Description("Where to save these settings").
				Options(scopeOpts...).
				Value(&vals.scope),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

func buildTierOptions(svc ActivateAPI, manifestID string) []huh.Option[string] {
	chosen := findManifestByID(svc.CurrentManifests(), manifestID)
	if chosen == nil {
		return []huh.Option[string]{huh.NewOption("(no tiers)", "")}
	}
	tiers := DiscoverAvailableTiers(*chosen)
	opts := make([]huh.Option[string], 0, len(tiers))
	for _, t := range tiers {
		opts = append(opts, huh.NewOption(t.Label, t.ID))
	}
	if len(opts) == 0 {
		return []huh.Option[string]{huh.NewOption("(no tiers)", "")}
	}
	return opts
}
