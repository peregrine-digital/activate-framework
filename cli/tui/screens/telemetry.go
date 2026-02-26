package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/peregrine-digital/activate-framework/cli/commands"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/tui/style"
)

// ── Telemetry screen ────────────────────────────────────────────

type telemetryValues struct {
	action string
}

type telemetryModel struct {
	svc    commands.ActivateAPI
	vals   *telemetryValues
	form   *huh.Form
	mode   string // "menu", "text"
	width  int
	height int

	textTitle string
	textBody  string
	done      bool
}

// RunTelemetryScreen launches the telemetry screen as a fullscreen program.
func RunTelemetryScreen(svc commands.ActivateAPI) error {
	m := newTelemetryModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newTelemetryModel(svc commands.ActivateAPI) telemetryModel {
	vals := &telemetryValues{}
	svc.RefreshConfig()
	cfg := svc.CurrentConfig()
	enabled := cfg.TelemetryEnabled != nil && *cfg.TelemetryEnabled

	form := buildTelemetryMenuForm(enabled, &vals.action)
	return telemetryModel{
		svc:  svc,
		vals: vals,
		form: form,
		mode: "menu",
	}
}

func (m telemetryModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m telemetryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			if m.mode == "text" {
				return m.switchToMenu()
			}
			m.done = true
			return m, tea.Quit
		case "enter", "q":
			if m.mode == "text" {
				return m.switchToMenu()
			}
		}
	}

	if m.mode == "text" {
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
		return m.handleAction()
	}

	return m, cmd
}

func (m telemetryModel) View() string {
	var sections []string
	sections = append(sections, style.RenderBanner())
	sections = append(sections, "")
	sections = append(sections, style.DimStyle.Render("  Telemetry"))

	m.svc.RefreshConfig()
	cfg := m.svc.CurrentConfig()
	enabled := cfg.TelemetryEnabled != nil && *cfg.TelemetryEnabled
	if enabled {
		sections = append(sections, style.DimStyle.Render("  status: enabled"))
	} else {
		sections = append(sections, style.DimStyle.Render("  status: disabled"))
	}

	if m.mode == "menu" {
		summary := m.buildSummary()
		if summary != "" {
			sections = append(sections, "")
			box := lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color(style.ColorGold)).
				Padding(0, 2).
				Render(summary)
			sections = append(sections, box)
		}
	}

	sections = append(sections, "")

	switch m.mode {
	case "menu":
		sections = append(sections, m.form.View())
		sections = append(sections, "")
		sections = append(sections, style.DimStyle.Render("  ↑/↓ navigate · enter select · esc back · ctrl+c quit"))
	case "text":
		body := strings.TrimSpace(m.textBody)
		if body == "" {
			body = "(no output)"
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(style.ColorGold)).
			Padding(1, 2).
			Render(m.textTitle + "\n\n" + body)
		sections = append(sections, box)
		sections = append(sections, "")
		sections = append(sections, style.DimStyle.Render("  enter/esc to return · ctrl+c quit"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return style.CenterContent(content, m.height)
}

// ── Actions ─────────────────────────────────────────────────────

func (m telemetryModel) switchToMenu() (tea.Model, tea.Cmd) {
	m.svc.RefreshConfig()
	cfg := m.svc.CurrentConfig()
	enabled := cfg.TelemetryEnabled != nil && *cfg.TelemetryEnabled
	m.vals.action = ""
	m.mode = "menu"
	m.form = buildTelemetryMenuForm(enabled, &m.vals.action)
	return m, m.form.Init()
}

func (m telemetryModel) handleAction() (tea.Model, tea.Cmd) {
	switch m.vals.action {
	case "back":
		m.done = true
		return m, tea.Quit

	case "run":
		result, err := m.svc.RunTelemetry("")
		if err != nil {
			m.mode = "text"
			m.textTitle = "Telemetry Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Telemetry Run Complete"
		if result.Entry != nil {
			m.textBody = formatTelemetryEntry(*result.Entry)
		} else {
			m.textBody = "Run completed but no entry was recorded."
		}
		return m, nil

	case "toggle":
		m.svc.RefreshConfig()
		currentCfg := m.svc.CurrentConfig()
		current := currentCfg.TelemetryEnabled != nil && *currentCfg.TelemetryEnabled
		newVal := !current
		updates := &model.Config{TelemetryEnabled: &newVal}
		_, err := m.svc.SetConfig("global", updates)
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Telemetry Updated"
		if newVal {
			m.textBody = "Telemetry is now enabled."
		} else {
			m.textBody = "Telemetry is now disabled."
		}
		return m, nil

	case "log":
		entries, err := m.svc.ReadTelemetryLog()
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Telemetry Log"
		if len(entries) == 0 {
			m.textBody = "No telemetry entries recorded yet."
		} else {
			m.textBody = formatTelemetryLog(entries)
		}
		return m, nil
	}

	return m.switchToMenu()
}

func (m telemetryModel) buildSummary() string {
	entries, err := m.svc.ReadTelemetryLog()
	if err != nil || len(entries) == 0 {
		return ""
	}
	latest := entries[len(entries)-1]
	return formatTelemetrySummary(latest)
}

// ── Formatting ──────────────────────────────────────────────────

func formatTelemetrySummary(e model.TelemetryEntry) string {
	var lines []string

	if e.PremiumUsed != nil && e.PremiumEntitlement != nil && *e.PremiumEntitlement > 0 {
		used := *e.PremiumUsed
		total := *e.PremiumEntitlement
		remaining := 0
		if e.PremiumRemaining != nil {
			remaining = *e.PremiumRemaining
		}
		pct := float64(used) / float64(total) * 100

		lines = append(lines,
			fmt.Sprintf("Used today:  %d / %d (%.1f%%)", used, total, pct),
			fmt.Sprintf("Remaining:   %d", remaining),
		)
	}

	if e.QuotaResetDateUTC != "" {
		lines = append(lines, fmt.Sprintf("Resets:      %s", e.QuotaResetDateUTC))
	}

	if e.Date != "" {
		lines = append(lines, fmt.Sprintf("Last check:  %s", e.Date))
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func formatTelemetryEntry(e model.TelemetryEntry) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("Date:    %s", e.Date))

	if e.PremiumUsed != nil && e.PremiumEntitlement != nil {
		lines = append(lines,
			fmt.Sprintf("Used:    %d / %d", *e.PremiumUsed, *e.PremiumEntitlement),
		)
		if e.PremiumRemaining != nil {
			lines = append(lines, fmt.Sprintf("Left:    %d", *e.PremiumRemaining))
		}
	}

	if e.QuotaResetDateUTC != "" {
		lines = append(lines, fmt.Sprintf("Resets:  %s", e.QuotaResetDateUTC))
	}
	lines = append(lines, fmt.Sprintf("Source:  %s", e.Source))
	return strings.Join(lines, "\n")
}

func formatTelemetryLog(entries []model.TelemetryEntry) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("%-12s %6s %6s %6s %6s", "Date", "Used", "Left", "Quota", "%"))
	lines = append(lines, strings.Repeat("─", 44))

	start := 0
	if len(entries) > 14 {
		start = len(entries) - 14
	}

	for i := len(entries) - 1; i >= start; i-- {
		e := entries[i]
		used, total, remaining := "—", "—", "—"
		pct := "—"

		if e.PremiumUsed != nil {
			used = fmt.Sprintf("%d", *e.PremiumUsed)
		}
		if e.PremiumRemaining != nil {
			remaining = fmt.Sprintf("%d", *e.PremiumRemaining)
		}
		if e.PremiumEntitlement != nil {
			total = fmt.Sprintf("%d", *e.PremiumEntitlement)
			if e.PremiumUsed != nil && *e.PremiumEntitlement > 0 {
				pct = fmt.Sprintf("%.0f%%", float64(*e.PremiumUsed)/float64(*e.PremiumEntitlement)*100)
			}
		}

		lines = append(lines, fmt.Sprintf("%-12s %6s %6s %6s %6s",
			e.Date, used, remaining, total, pct))
	}

	return strings.Join(lines, "\n")
}

func buildTelemetryMenuForm(enabled bool, action *string) *huh.Form {
	var toggleLabel string
	if enabled {
		toggleLabel = "Disable telemetry"
	} else {
		toggleLabel = "Enable telemetry"
	}

	options := []huh.Option[string]{
		huh.NewOption("Run telemetry now", "run"),
		huh.NewOption(toggleLabel, "toggle"),
		huh.NewOption("View log", "log"),
		huh.NewOption("← Back to menu", "back"),
	}

	*action = ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Telemetry Actions").
				Options(options...).
				Value(action),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}
