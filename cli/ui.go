package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── Brand colors ────────────────────────────────────────────────
const (
	colorGold   = "#E8C228" // Peregrine falcon gold
	colorDim    = "#666666"
	colorBright = "#FFFFFF"
	colorGreen  = "#04B575"
	colorRed    = "#FF4672"
	colorPurple = "#7B61FF"
)

// ── Styles ──────────────────────────────────────────────────────
var (
	goldStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGold))
	brightStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorBright))
	dimStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorPurple))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorGreen))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color(colorRed))

	bannerBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGold)).
			Padding(1, 3)

	summaryBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGreen)).
			Padding(1, 3).
			MarginTop(1)

	resultBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGreen)).
			Padding(1, 3).
			MarginTop(1).
			MarginBottom(1)
)

// ── Logo ────────────────────────────────────────────────────────

// falconArtRaw stores the imported pixel map for the peregrine mark.
const falconArtRaw = `
●●●●●●●●●●◐◐◐◐◐●●●●●●●●●●
●●●●●●●●◐◐◐◐◐◐◐◐◐◐●●●●●●●
●●●●●◐●●●●●●●●●●●◐◐◐●●●●●
●●●●●◐◐●●●●●●●●●●●●◐◐●●●●
●●◐●●●◐◐●●●●●●●●●●●●◐◐●●●
●●●◐●●●◐◐◐●●●●●●●●●●●◐◐●●
●●●◐◐◐●◐◐◐◐●●●●●●●●●●●◐●●
●●●●◐◐◐◐◐◐◐◐◐●●●●●●●●●◐◐●
●◐◐●●◐◐◐◐◐◐◐◐◐◐●●●●●●●●◐●
●●◐◐◐●◐◐◐◐◐◐◐◐◐◐◐◐●●●●●◐●
●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●◐●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐
●◐●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●
●◐◐●●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●
●●◐●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●
●●◐◐●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●●
●●●◐●●●●◐◐◐◐◐◐◐◐◐◐◐◐◐◐◐●●
●●●◐◐●●●●●◐◐◐◐◐◐◐◐◐◐◐◐●●●
●●●●◐◐◐●●●●●●●●●●●◐◐◐●●●●
●●●●●◐◐◐●●●●●●●●●◐◐◐●●●●●
●●●●●●●◐◐◐◐◐◐◐◐◐◐◐●●●●●●●
●●●●●●●●●●◐◐◐◐◐●●●●●●●●●●`

func renderFalconLogo() string {
	raw := strings.Trim(falconArtRaw, "\n")
	lines := strings.Split(raw, "\n")

	// Preserve source shape while correcting terminal cell stretch by
	// packing two source rows into one display row with half-block glyphs.
	packed := make([]string, 0, (len(lines)+1)/2)
	for i := 0; i < len(lines); i += 2 {
		upper := []rune(lines[i])
		var lower []rune
		if i+1 < len(lines) {
			lower = []rune(lines[i+1])
		}

		maxLen := len(upper)
		if len(lower) > maxLen {
			maxLen = len(lower)
		}

		row := make([]rune, maxLen)
		for j := 0; j < maxLen; j++ {
			upOn := j < len(upper) && upper[j] == '◐'
			dnOn := j < len(lower) && lower[j] == '◐'

			switch {
			case upOn && dnOn:
				row[j] = '█'
			case upOn:
				row[j] = '▀'
			case dnOn:
				row[j] = '▄'
			default:
				row[j] = ' '
			}
		}

		packed = append(packed, string(row))
	}

	return goldStyle.Render(strings.Join(packed, "\n"))
}

// wordmarkArt is the "PEREGRINE" title in a clean ASCII style.
const wordmarkArt = `
██████  ███████ ██████  ███████  ██████  ██████  ██ ███    ██ ███████
██   ██ ██      ██   ██ ██      ██       ██   ██ ██ ████   ██ ██
██████  █████   ██████  █████   ██   ███ ██████  ██ ██ ██  ██ █████
██      ██      ██   ██ ██      ██    ██ ██   ██ ██ ██  ██ ██ ██
██      ███████ ██   ██ ███████  ██████  ██   ██ ██ ██   ████ ███████`

func renderBanner() string {
	falcon := renderFalconLogo()

	// Render the large ASCII text
	wordmark := brightStyle.Render(strings.Trim(wordmarkArt, "\n"))
	
	// Subtitle shown under the wordmark
	subtitle := dimStyle.Render("                     DIGITAL SERVICES")

	text := lipgloss.JoinVertical(lipgloss.Left,
		wordmark,
		subtitle,
	)

	// Combine logo and text with padding
	logo := lipgloss.JoinHorizontal(lipgloss.Center, falcon, "    ", text)
	return bannerBox.Render(logo)
}

// ── Bubble Tea model ────────────────────────────────────────────

type phase int

const (
	phaseManifest  phase = iota // select manifest (skipped if one)
	phaseConfigure              // select tier + target dir + confirm
)

// model is the top-level Bubble Tea model for the interactive installer.
type model struct {
	phase  phase
	form   *huh.Form
	width  int
	height int

	quitting bool

	// data
	manifests []Manifest
	cfg       Config

	// form-bound values
	manifestID string
	tierID     string
	targetDir  string
	confirm    bool

	// resolved after manifest selection
	chosen Manifest
}

// fullscreenFormModel renders a huh.Form inside the branded full-screen shell.
type fullscreenFormModel struct {
	form     *huh.Form
	width    int
	height   int
	title    string
	subtitle string
}

// fullscreenTextModel renders read-only content in the same branded shell.
type fullscreenTextModel struct {
	width    int
	height   int
	title    string
	subtitle string
	body     string
}

type mainMenuModel struct {
	form      *huh.Form
	width     int
	height    int
	manifests []Manifest
	cfg       Config
	state     InstallState
	projectDir string

	choice string
	action string

	mode     string // "menu" | "text"
	textTitle string
	textBody  string
}

func initialModel(manifests []Manifest, cfg Config) model {
	m := model{
		manifests: manifests,
		cfg:       cfg,
	}

	// Resolve manifest default from config
	m.manifestID = cfg.Manifest
	found := false
	for _, man := range manifests {
		if man.ID == m.manifestID {
			found = true
			break
		}
	}
	if !found {
		m.manifestID = manifests[0].ID
	}

	if len(manifests) == 1 {
		// Skip manifest phase
		m.manifestID = manifests[0].ID
		m.chosen = manifests[0]
		m.phase = phaseConfigure
		m.form = m.buildConfigureForm()
	} else {
		m.phase = phaseManifest
		m.form = m.buildManifestForm()
	}

	return m
}

func (m *model) buildManifestForm() *huh.Form {
	var opts []huh.Option[string]
	for _, man := range m.manifests {
		desc := fmt.Sprintf("v%s · %d files", man.Version, len(man.Files))
		if man.Description != "" {
			desc += " — " + man.Description
		}
		opts = append(opts, huh.NewOption(
			fmt.Sprintf("%s  %s", man.Name, dimStyle.Render(desc)),
			man.ID,
		))
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select manifest").
				Description("Choose which collection to install").
				Options(opts...).
				Value(&m.manifestID),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

func (m *model) buildConfigureForm() *huh.Form {
	// Resolve chosen manifest
	for _, man := range m.manifests {
		if man.ID == m.manifestID {
			m.chosen = man
			break
		}
	}

	tiers := DiscoverAvailableTiers(m.chosen)

	// Tier default
	m.tierID = m.cfg.Tier
	tierFound := false
	for _, t := range tiers {
		if t.ID == m.tierID {
			tierFound = true
			break
		}
	}
	if !tierFound && len(tiers) > 0 {
		m.tierID = tiers[0].ID
	}

	var tierOpts []huh.Option[string]
	for _, t := range tiers {
		files := SelectFiles(m.chosen.Files, m.chosen, t.ID)
		tierOpts = append(tierOpts, huh.NewOption(
			fmt.Sprintf("%-12s %s", t.Label, dimStyle.Render(fmt.Sprintf("%d files", len(files)))),
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
				Value(&m.tierID),
		)
	} else if len(tiers) == 1 {
		m.tierID = tiers[0].ID
	}

	fields = append(fields,
		huh.NewInput().
			Title("Target directory").
			Description("Where to install files").
			Placeholder(defaultTarget).
			Value(&m.targetDir),
		huh.NewConfirm().
			Title("Install?").
			Affirmative("  Install  ").
			Negative("  Cancel  ").
			Value(&m.confirm),
	)

	return huh.NewForm(
		huh.NewGroup(fields...),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

// ── tea.Model implementation ────────────────────────────────────

func (m model) Init() tea.Cmd {
	return m.form.Init()
}

func (m fullscreenFormModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m fullscreenTextModel) Init() tea.Cmd {
	return nil
}

func (m mainMenuModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.quitting = true
			m.confirm = false
			return m, tea.Quit
		}
	}

	// Delegate to the embedded huh form
	updated, cmd := m.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.form = f

		if f.State == huh.StateAborted {
			m.quitting = true
			m.confirm = false
			return m, tea.Quit
		}

		if f.State == huh.StateCompleted {
			switch m.phase {
			case phaseManifest:
				// Advance to configure phase
				m.phase = phaseConfigure
				m.form = m.buildConfigureForm()
				return m, m.form.Init()

			case phaseConfigure:
				// Done — exit to stdout for install
				m.quitting = true
				return m, tea.Quit
			}
		}
	}

	return m, cmd
}

func (m fullscreenFormModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
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

func (m mainMenuModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.action = "exit"
			return m, tea.Quit
		case "esc", "q", "enter":
			if m.mode == "text" {
				m.mode = "menu"
				m.form = buildMainMenuForm(m.state, &m.choice)
				return m, m.form.Init()
			}
		}
	}

	if m.mode == "text" {
		return m, nil
	}

	updated, cmd := m.form.Update(msg)
	if f, ok := updated.(*huh.Form); ok {
		m.form = f
		if f.State == huh.StateAborted {
			m.action = "exit"
			return m, tea.Quit
		}
		if f.State == huh.StateCompleted {
			switch m.choice {
			case "list":
				m.mode = "text"
				m.textTitle = "Frameworks"
				m.textBody = strings.TrimSpace(FormatManifestList(m.manifests))
				m.form = buildMainMenuForm(m.state, &m.choice)
				return m, nil
			case "state":
				m.mode = "text"
				m.textTitle = "Current State"
				m.textBody = m.stateBody()
				m.form = buildMainMenuForm(m.state, &m.choice)
				return m, nil
			default:
				m.action = m.choice
				return m, tea.Quit
			}
		}
	}

	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}

	var sections []string

	// Banner
	sections = append(sections, renderBanner())
	sections = append(sections, "")

	// Phase header
	switch m.phase {
	case phaseManifest:
		sections = append(sections, dimStyle.Render("  Step 1 of 2 · Select Manifest"))
	case phaseConfigure:
		header := fmt.Sprintf("  Step 2 of 2 · %s v%s",
			brightStyle.Render(m.chosen.Name), m.chosen.Version)
		sections = append(sections, header)
	}
	sections = append(sections, "")

	// Form
	sections = append(sections, m.form.View())

	// Footer
	sections = append(sections, "")
	sections = append(sections, dimStyle.Render("  ↑/↓ navigate · enter select · ctrl+c quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Vertically position in upper-third of terminal
	if m.height > 0 {
		contentLines := strings.Count(content, "\n") + 1
		topPad := (m.height - contentLines) / 4
		if topPad > 1 {
			content = strings.Repeat("\n", topPad) + content
		}
	}

	return content
}

func (m fullscreenFormModel) View() string {
	var sections []string

	sections = append(sections, renderBanner())
	sections = append(sections, "")

	if strings.TrimSpace(m.title) != "" {
		sections = append(sections, dimStyle.Render("  "+m.title))
	}
	if strings.TrimSpace(m.subtitle) != "" {
		sections = append(sections, dimStyle.Render("  "+m.subtitle))
	}
	sections = append(sections, "")

	sections = append(sections, m.form.View())
	sections = append(sections, "")
	sections = append(sections, dimStyle.Render("  ↑/↓ navigate · enter select · ctrl+c quit"))

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

func (m fullscreenTextModel) View() string {
	var sections []string

	sections = append(sections, renderBanner())
	sections = append(sections, "")

	if strings.TrimSpace(m.title) != "" {
		sections = append(sections, dimStyle.Render("  "+m.title))
	}
	if strings.TrimSpace(m.subtitle) != "" {
		sections = append(sections, dimStyle.Render("  "+m.subtitle))
	}
	sections = append(sections, "")

	body := strings.TrimSpace(m.body)
	if body == "" {
		body = "(no details)"
	}
	bodyBlock := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(colorGold)).
		Padding(1, 2).
		Render(body)
	sections = append(sections, bodyBlock)

	sections = append(sections, "")
	sections = append(sections, dimStyle.Render("  enter/esc to return · ctrl+c quit"))

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	if m.height > 0 {
		contentLines := strings.Count(content, "\n") + 1
		topPad := (m.height - contentLines) / 6
		if topPad > 1 {
			content = strings.Repeat("\n", topPad) + content
		}
	}

	return content
}

func runFullscreenForm(form *huh.Form, title, subtitle string) error {
	m := fullscreenFormModel{
		form:     form,
		title:    title,
		subtitle: subtitle,
	}
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

func (m mainMenuModel) stateText() string {
	text := ""
	if m.state.HasProjectConfig {
		text = "saved config detected"
	} else {
		text = "no project config detected"
	}
	if m.state.HasInstallMarker {
		text += fmt.Sprintf(" · installed %s v%s", m.state.InstalledManifest, m.state.InstalledVersion)
	}
	return text
}

func (m mainMenuModel) stateBody() string {
	return fmt.Sprintf(
		"Project: %s\nGlobal config:  %t\nProject config: %t\nInstall marker: %t\nInstalled: %s v%s\n\nEffective config:\n  manifest: %s\n  tier: %s",
		m.projectDir,
		m.state.HasGlobalConfig,
		m.state.HasProjectConfig,
		m.state.HasInstallMarker,
		m.state.InstalledManifest,
		m.state.InstalledVersion,
		m.cfg.Manifest,
		m.cfg.Tier,
	)
}

func buildMainMenuForm(state InstallState, choice *string) *huh.Form {
	type menuItem struct {
		label string
		value string
	}

	items := []menuItem{}
	if state.HasProjectConfig {
		if state.HasInstallMarker {
			items = append(items, menuItem{label: "Reinstall using saved settings", value: "quick-install"})
		} else {
			items = append(items, menuItem{label: "Install using saved settings", value: "quick-install"})
		}
		items = append(items, menuItem{label: "Change settings and install", value: "guided-install"})
	} else {
		items = append(items, menuItem{label: "New setup (choose settings and install)", value: "guided-install"})
	}
	items = append(items, menuItem{label: "Add managed files to repository", value: "repo-add"})
	if state.HasInstallMarker {
		items = append(items, menuItem{label: "Remove managed files from repository", value: "repo-remove"})
	}
	items = append(items,
		menuItem{label: "Show frameworks", value: "list"},
		menuItem{label: "Show current state", value: "state"},
		menuItem{label: "Exit", value: "exit"},
	)

	options := make([]huh.Option[string], 0, len(items))
	for _, item := range items {
		options = append(options, huh.NewOption(item.label, item.value))
	}

	*choice = ""
	stateText := ""
	if state.HasProjectConfig {
		stateText = "saved config detected"
	} else {
		stateText = "no project config detected"
	}
	if state.HasInstallMarker {
		stateText += fmt.Sprintf(" · installed %s v%s", state.InstalledManifest, state.InstalledVersion)
	}

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Peregrine Activate").
				Description(stateText).
				Options(options...).
				Value(choice),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

func (m mainMenuModel) View() string {
	var sections []string

	sections = append(sections, renderBanner())
	sections = append(sections, "")
	sections = append(sections, dimStyle.Render("  Main Menu"))
	sections = append(sections, dimStyle.Render("  "+m.stateText()))
	sections = append(sections, "")

	if m.mode == "text" {
		body := strings.TrimSpace(m.textBody)
		if body == "" {
			body = "(no details)"
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGold)).
			Padding(1, 2).
			Render(m.textTitle + "\n\n" + body)
		sections = append(sections, box)
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  enter/esc to return · ctrl+c quit"))
	} else {
		sections = append(sections, m.form.View())
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  ↑/↓ navigate · enter select · ctrl+c quit"))
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

// RunInteractiveMenu starts in a state-aware mode and offers actions based on
// whether config/install markers are already present.
func RunInteractiveMenu(svc *ActivateService) error {
	for {
		svc.refreshConfig()
		cfg := svc.Config
		state := DetectInstallState(svc.ProjectDir)

		menuModel := mainMenuModel{
			manifests: svc.Manifests,
			cfg:       cfg,
			state:     state,
			projectDir: svc.ProjectDir,
			mode:      "menu",
		}
		menuModel.form = buildMainMenuForm(menuModel.state, &menuModel.choice)

		p := tea.NewProgram(menuModel, tea.WithAltScreen())
		finalModel, err := p.Run()
		if err != nil {
			return err
		}
		result, ok := finalModel.(mainMenuModel)
		if !ok {
			return fmt.Errorf("unexpected TUI model type: %T", finalModel)
		}

		switch result.action {
		case "guided-install":
			if err := RunInteractiveInstall(svc); err != nil {
				_ = runFullscreenText("Action Failed", "Guided install", err.Error())
			}

		case "quick-install":
			target := defaultTargetDir()
			confirm := false

			quickForm := huh.NewForm(
				huh.NewGroup(
					huh.NewInput().
						Title("Target directory").
						Description("Where to install files").
						Placeholder(defaultTargetDir()).
						Value(&target),
					huh.NewConfirm().
						Title(fmt.Sprintf("Install with manifest=%s, tier=%s?", cfg.Manifest, cfg.Tier)).
						Affirmative("  Install  ").
						Negative("  Cancel  ").
						Value(&confirm),
				),
			).WithTheme(huh.ThemeCharm()).WithShowHelp(false)

			if err := runFullscreenForm(quickForm, "Quick Install", fmt.Sprintf("manifest=%s · tier=%s", cfg.Manifest, cfg.Tier)); err != nil {
				return err
			}
			if confirm {
				if err := installWithResolvedConfig(svc.Manifests, cfg, resolveTargetPath(target), svc.UseRemote, svc.Repo, svc.Branch); err != nil {
					_ = runFullscreenText("Action Failed", "Quick install", err.Error())
				}
			}

		case "repo-add":
			if _, err := svc.RepoAdd(); err != nil {
				_ = runFullscreenText("Action Failed", "Repo add", err.Error())
			}

		case "repo-remove":
			if err := svc.RepoRemove(); err != nil {
				_ = runFullscreenText("Action Failed", "Repo remove", err.Error())
			}

		default:
			return nil
		}
	}
}

// RunInteractiveInstall runs the full-screen TUI installer wizard, then
// performs the file installation with normal stdout output.
func RunInteractiveInstall(svc *ActivateService) error {
	m := initialModel(svc.Manifests, svc.Config)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result, ok := finalModel.(model)
	if !ok {
		return fmt.Errorf("unexpected TUI model type: %T", finalModel)
	}

	if !result.confirm {
		fmt.Println(dimStyle.Render("\n  Cancelled.\n"))
		return nil
	}

	// ── Resolve target ──────────────────────────────────────────
	target := resolveTargetPath(result.targetDir)

	// ── Show summary ────────────────────────────────────────────
	files := SelectFiles(result.chosen.Files, result.chosen, result.tierID)

	summary := fmt.Sprintf(
		"%s  %s v%s\n%s  %s\n%s  %d files\n%s  %s",
		dimStyle.Render("Manifest:"),
		brightStyle.Render(result.chosen.Name),
		result.chosen.Version,
		dimStyle.Render("Tier:    "),
		brightStyle.Render(result.tierID),
		dimStyle.Render("Files:   "),
		len(files),
		dimStyle.Render("Target:  "),
		brightStyle.Render(target),
	)
	fmt.Println(summaryBox.Render(summary))
	fmt.Println()

	// ── Install ─────────────────────────────────────────────────
	if svc.UseRemote {
		if err := InstallFilesFromRemote(files, result.chosen.BasePath, target, result.chosen.Version, result.chosen.ID, svc.Repo, svc.Branch); err != nil {
			return err
		}
	} else {
		if err := InstallFiles(files, result.chosen.BasePath, target, result.chosen.Version, result.chosen.ID); err != nil {
			return err
		}
	}

	// Persist config via service
	_, _ = svc.SetConfig("project", &Config{Manifest: result.chosen.ID, Tier: result.tierID})

	// ── Success ─────────────────────────────────────────────────
	resultMsg := fmt.Sprintf(
		"%s  %s v%s (%s) installed\n%s  %s",
		successStyle.Render("✓"),
		result.chosen.Name,
		result.chosen.Version,
		result.tierID,
		dimStyle.Render("→"),
		target,
	)
	fmt.Println(resultBox.Render(resultMsg))
	return nil
}

// ── List command (stdout, no Bubble Tea) ────────────────────────

// RunList displays manifests/files in human or JSON format.
func RunList(svc *ActivateService, manifestID, tierID, category string, jsonOutput bool) error {
	// Overview mode
	if manifestID == "" && tierID == "" && category == "" {
		manifests := svc.ListManifests()
		if jsonOutput {
			type summary struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
				Version     string `json:"version"`
				FileCount   int    `json:"fileCount"`
			}
			var items []summary
			for _, m := range manifests {
				items = append(items, summary{m.ID, m.Name, m.Description, m.Version, len(m.Files)})
			}
			return printJSON(map[string]interface{}{"manifests": items})
		}
		fmt.Println()
		fmt.Println(renderBanner())
		fmt.Println()
		fmt.Println(FormatManifestList(manifests))
		fmt.Println(dimStyle.Render("  Use --manifest <id> to see files for a specific manifest.\n"))
		return nil
	}

	// Detail mode via service
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
	chosen := findManifestByID(svc.Manifests, result.Manifest)
	name := result.Manifest
	ver := ""
	if chosen != nil {
		name = chosen.Name
		ver = chosen.Version
	}
	fmt.Println(titleStyle.Render(fmt.Sprintf("\n%s v%s — %s", name, ver, tierLabel)))
	fmt.Println(formatGroups(result.Categories))
	fmt.Println()
	return nil
}

// ── Helpers ─────────────────────────────────────────────────────

func formatGroups(groups []CategoryGroup) string {
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, "\n%s (%d)\n", g.Label, len(g.Files))
		b.WriteString(strings.Repeat("─", 40) + "\n")
		for _, f := range g.Files {
			name := fileDisplayName(f.Dest)
			fmt.Fprintf(&b, "  %s\n", name)
			if f.Description != "" {
				fmt.Fprintf(&b, "    %s\n", f.Description)
			}
			fmt.Fprintf(&b, "    tier: %s  →  %s\n", f.Tier, f.Dest)
		}
	}
	return b.String()
}

