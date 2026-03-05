package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/peregrine-digital/activate-framework/cli/commands"
	"github.com/peregrine-digital/activate-framework/cli/engine"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
	"github.com/peregrine-digital/activate-framework/cli/tui/style"
)

// ── Installer wizard (first-run flow) ───────────────────────────

type phase int

const (
	phaseManifest  phase = iota
	phaseConfigure
)

type installerValues struct {
	manifestID string
	tierID     string
	targetDir  string
	confirm    bool
}

type installerModel struct {
	phase  phase
	form   *huh.Form
	width  int
	height int

	quitting bool

	manifests []model.Manifest
	cfg       model.Config
	vals      *installerValues
	chosen    model.Manifest
}

func initialModel(manifests []model.Manifest, cfg model.Config) installerModel {
	vals := &installerValues{}
	m := installerModel{
		manifests: manifests,
		cfg:       cfg,
		vals:      vals,
	}

	vals.manifestID = cfg.Manifest
	found := false
	for _, man := range manifests {
		if man.ID == vals.manifestID {
			found = true
			break
		}
	}
	if !found {
		vals.manifestID = manifests[0].ID
	}

	if len(manifests) == 1 {
		vals.manifestID = manifests[0].ID
		m.chosen = manifests[0]
		m.phase = phaseConfigure
		m.form = m.buildConfigureForm()
	} else {
		m.phase = phaseManifest
		m.form = m.buildManifestForm()
	}

	return m
}

func (m *installerModel) buildManifestForm() *huh.Form {
	var opts []huh.Option[string]
	for _, man := range m.manifests {
		desc := fmt.Sprintf("%d files", len(man.Files))
		if man.Description != "" {
			desc += " — " + man.Description
		}
		opts = append(opts, huh.NewOption(
			fmt.Sprintf("%s  %s", man.Name, style.DimStyle.Render(desc)),
			man.ID,
		))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select manifest").
				Description("Choose which collection to install").
				Options(opts...).
				Value(&m.vals.manifestID),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

func (m *installerModel) buildConfigureForm() *huh.Form {
	for _, man := range m.manifests {
		if man.ID == m.vals.manifestID {
			m.chosen = man
			break
		}
	}

	tiers := model.DiscoverAvailableTiers(m.chosen)

	m.vals.tierID = m.cfg.Tier
	tierFound := false
	for _, t := range tiers {
		if t.ID == m.vals.tierID {
			tierFound = true
			break
		}
	}
	if !tierFound && len(tiers) > 0 {
		m.vals.tierID = tiers[0].ID
	}

	var tierOpts []huh.Option[string]
	for _, t := range tiers {
		files := model.SelectFiles(m.chosen.Files, m.chosen, t.ID)
		tierOpts = append(tierOpts, huh.NewOption(
			fmt.Sprintf("%-12s %s", t.Label, style.DimStyle.Render(fmt.Sprintf("%d files", len(files)))),
			t.ID,
		))
	}

	home, _ := os.UserHomeDir()
	defaultTarget := filepath.Join(home, ".copilot")

	var fields []huh.Field
	if len(tiers) > 1 {
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Select tier").
				Description("Higher tiers include everything from lower tiers").
				Options(tierOpts...).
				Value(&m.vals.tierID),
		)
	} else if len(tiers) == 1 {
		m.vals.tierID = tiers[0].ID
	}

	fields = append(fields,
		huh.NewInput().
			Title("Target directory").
			Description("Where to install files").
			Placeholder(defaultTarget).
			Value(&m.vals.targetDir),
		huh.NewConfirm().
			Title("Install?").
			Affirmative("  Install  ").
			Negative("  Cancel  ").
			Value(&m.vals.confirm),
	)

	return huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

func (m installerModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m installerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			m.vals.confirm = false
			return m, tea.Quit
		}
	}

	updated, cmd := m.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.form = f

		if f.State == huh.StateAborted {
			m.quitting = true
			m.vals.confirm = false
			return m, tea.Quit
		}

		if f.State == huh.StateCompleted {
			switch m.phase {
			case phaseManifest:
				m.phase = phaseConfigure
				m.form = m.buildConfigureForm()
				return m, m.form.Init()
			case phaseConfigure:
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	return m, cmd
}

func (m installerModel) View() string {
	if m.quitting {
		return ""
	}

	var sections []string
	sections = append(sections, style.RenderBanner())
	sections = append(sections, "")

	switch m.phase {
	case phaseManifest:
		sections = append(sections, style.DimStyle.Render("  Step 1 of 2 · Select Manifest"))
	case phaseConfigure:
		header := fmt.Sprintf("  Step 2 of 2 · %s",
			style.BrightStyle.Render(m.chosen.Name))
		sections = append(sections, header)
	}
	sections = append(sections, "")
	sections = append(sections, m.form.View())
	sections = append(sections, "")
	sections = append(sections, style.DimStyle.Render("  ↑/↓ navigate · enter select · ctrl+c quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return style.CenterContent(content, m.height)
}

// ── Fullscreen helpers ──────────────────────────────────────────

type fullscreenFormModel struct {
	form     *huh.Form
	width    int
	height   int
	title    string
	subtitle string
}

type fullscreenTextModel struct {
	width    int
	height   int
	title    string
	subtitle string
	body     string
}

func (m fullscreenFormModel) Init() tea.Cmd  { return m.form.Init() }
func (m fullscreenTextModel) Init() tea.Cmd  { return nil }

func (m fullscreenFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	updated, cmd := m.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.form = f
		if f.State == huh.StateAborted || f.State == huh.StateCompleted {
			return m, tea.Quit
		}
	}
	return m, cmd
}

func (m fullscreenTextModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc", "enter":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m fullscreenFormModel) View() string {
	var sections []string
	sections = append(sections, style.RenderBanner())
	sections = append(sections, "")
	if strings.TrimSpace(m.title) != "" {
		sections = append(sections, style.DimStyle.Render("  "+m.title))
	}
	if strings.TrimSpace(m.subtitle) != "" {
		sections = append(sections, style.DimStyle.Render("  "+m.subtitle))
	}
	sections = append(sections, "")
	sections = append(sections, m.form.View())
	sections = append(sections, "")
	sections = append(sections, style.DimStyle.Render("  ↑/↓ navigate · enter select · ctrl+c quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return style.CenterContent(content, m.height)
}

func (m fullscreenTextModel) View() string {
	var sections []string
	sections = append(sections, style.RenderBanner())
	sections = append(sections, "")
	if strings.TrimSpace(m.title) != "" {
		sections = append(sections, style.DimStyle.Render("  "+m.title))
	}
	if strings.TrimSpace(m.subtitle) != "" {
		sections = append(sections, style.DimStyle.Render("  "+m.subtitle))
	}
	sections = append(sections, "")
	body := strings.TrimSpace(m.body)
	if body == "" {
		body = "(no details)"
	}
	bodyBlock := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(style.ColorGold)).
		Padding(1, 2).
		Render(body)
	sections = append(sections, bodyBlock)
	sections = append(sections, "")
	sections = append(sections, style.DimStyle.Render("  enter/esc to return · ctrl+c quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return style.CenterContent(content, m.height)
}

func runFullscreenForm(form *huh.Form, title, subtitle string) error {
	m := fullscreenFormModel{form: form, title: title, subtitle: subtitle}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func runFullscreenText(title, subtitle, body string) error {
	m := fullscreenTextModel{title: title, subtitle: subtitle, body: body}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// ── RunInteractiveInstall ───────────────────────────────────────

func defaultTargetDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".copilot")
}

func resolveTargetPath(target string) string {
	if strings.TrimSpace(target) == "" {
		target = defaultTargetDir()
	}
	if strings.HasPrefix(target, "~/") {
		home, _ := os.UserHomeDir()
		target = filepath.Join(home, target[2:])
	}
	abs, err := filepath.Abs(target)
	if err == nil {
		return abs
	}
	return target
}

// RunInteractiveInstall runs the full-screen TUI installer wizard.
func RunInteractiveInstall(svc commands.ActivateAPI) error {
	m := initialModel(svc.CurrentManifests(), svc.CurrentConfig())

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result, ok := finalModel.(installerModel)
	if !ok {
		return fmt.Errorf("unexpected TUI model type: %T", finalModel)
	}

	if !result.vals.confirm {
		fmt.Println(style.DimStyle.Render("\n  Cancelled.\n"))
		return nil
	}

	target := resolveTargetPath(result.vals.targetDir)

	files := model.SelectFiles(result.chosen.Files, result.chosen, result.vals.tierID)

	summary := fmt.Sprintf(
		"%s  %s\n%s  %s\n%s  %d files\n%s  %s",
		style.DimStyle.Render("Manifest:"),
		style.BrightStyle.Render(result.chosen.Name),
		style.DimStyle.Render("Tier:    "),
		style.BrightStyle.Render(result.vals.tierID),
		style.DimStyle.Render("Files:   "),
		len(files),
		style.DimStyle.Render("Target:  "),
		style.BrightStyle.Render(target),
	)
	fmt.Println(style.SummaryBox.Render(summary))
	fmt.Println()

	if err := engine.InstallFilesFromRemote(files, result.chosen.BasePath, target, svc.CurrentConfig().Repo, svc.CurrentConfig().Branch); err != nil {
		return err
	}

	_, _ = svc.SetConfig("project", &model.Config{Manifest: result.chosen.ID, Tier: result.vals.tierID})

	resultMsg := fmt.Sprintf(
		"%s  %s (%s) installed\n%s  %s",
		style.SuccessStyle.Render("✓"),
		result.chosen.Name,
		result.vals.tierID,
		style.DimStyle.Render("→"),
		target,
	)
	fmt.Println(style.ResultBox.Render(resultMsg))
	return nil
}

// FormatUpdateResult formats an update result for display.
func FormatUpdateResult(result *commands.UpdateResult) string {
	var lines []string

	if len(result.Updated) > 0 {
		lines = append(lines, fmt.Sprintf("Updated %d files:", len(result.Updated)))
		for _, f := range result.Updated {
			lines = append(lines, "  ⬆ "+f)
		}
	}

	if len(result.Skipped) > 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, fmt.Sprintf("Skipped %d files:", len(result.Skipped)))
		for _, f := range result.Skipped {
			lines = append(lines, "  ⏭ "+f)
		}
	}

	if len(result.Updated) == 0 && len(result.Skipped) == 0 {
		lines = append(lines, "All files are up to date.")
	}

	return strings.Join(lines, "\n")
}

// ── RunList ─────────────────────────────────────────────────────

// RunList displays manifests/files in human or JSON format.
func RunList(svc commands.ActivateAPI, manifestID, tierID, category string, jsonOutput bool, printJSON func(v interface{}) error) error {
	if manifestID == "" && tierID == "" && category == "" {
		manifests := svc.ListManifests()
		if jsonOutput {
			type summary struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				FileCount   int    `json:"fileCount"`
			}
			var items []summary
			for _, m := range manifests {
				items = append(items, summary{m.ID, m.Name, m.Description, len(m.Files)})
			}
			return printJSON(map[string]interface{}{"manifests": items})
		}
		fmt.Println()
		fmt.Println(style.RenderBanner())
		fmt.Println()
		fmt.Println(model.FormatManifestList(manifests))
		fmt.Println(style.DimStyle.Render("  Use --manifest <id> to see files for a specific manifest.\n"))
		return nil
	}

	result, err := svc.ListFiles(manifestID, tierID, category)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}

	tierLabel := result.Tier
	if tierLabel == "" {
		tierLabel = "all tiers"
	}
	chosen := model.FindManifestByID(svc.CurrentManifests(), result.Manifest)
	name := result.Manifest
	if chosen != nil {
		name = chosen.Name
	}
	fmt.Println(style.TitleStyle.Render(fmt.Sprintf("\n%s — %s", name, tierLabel)))
	fmt.Println(formatGroups(result.Categories))
	fmt.Println()
	return nil
}

func formatGroups(groups []model.CategoryGroup) string {
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, "\n%s (%d)\n", g.Label, len(g.Files))
		b.WriteString(strings.Repeat("─", 40) + "\n")
		for _, f := range g.Files {
			name := model.FileDisplayName(f.Dest)
			fmt.Fprintf(&b, "  %s\n", name)
			if f.Description != "" {
				fmt.Fprintf(&b, "    %s\n", f.Description)
			}
			fmt.Fprintf(&b, "    tier: %s  →  %s\n", f.Tier, f.Dest)
		}
	}
	return b.String()
}

// InstallWithResolvedConfig performs installation using pre-resolved settings.
func InstallWithResolvedConfig(manifests []model.Manifest, cfg model.Config, target string) error {
	chosen := model.FindManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	files := model.SelectFiles(chosen.Files, *chosen, cfg.Tier)
	return engine.InstallFilesFromRemote(files, chosen.BasePath, target, repo, branch)
}
