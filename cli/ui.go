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

// falconArt is a peregrine falcon in flight, front-facing with spread wings.
// Designed on a 25-column grid centered at column 12.
const falconArt = `          ▄███▄
         █▀   ▀█
          ▀▄ ▄▀
           ███
       ▄▄▀▀███▀▀▄▄
     ▄▀   █████   ▀▄
   ▄▀    ███████    ▀▄
  ▀     █████████     ▀
       ████   ████
        ██▀   ▀██
         ▀     ▀`

func renderBanner() string {
	falcon := goldStyle.Render(falconArt)

	wordmark := brightStyle.Render("P E R E G R I N E")
	subtitle := dimStyle.Render("D I G I T A L   S E R V I C E S")
	tagline := dimStyle.Render("─── Activate Framework ───")

	text := lipgloss.JoinVertical(lipgloss.Left,
		"",
		"",
		wordmark,
		subtitle,
		"",
		tagline,
	)

	logo := lipgloss.JoinHorizontal(lipgloss.Center, falcon, "   ", text)
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

// ── RunInteractiveInstall ───────────────────────────────────────

// RunInteractiveInstall runs the full-screen TUI installer wizard, then
// performs the file installation with normal stdout output.
func RunInteractiveInstall(manifests []Manifest, cfg Config, useRemote bool, repo, branch string) error {
	m := initialModel(manifests, cfg)

	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	result := finalModel.(model)

	if !result.confirm {
		fmt.Println(dimStyle.Render("\n  Cancelled.\n"))
		return nil
	}

	// ── Resolve target ──────────────────────────────────────────
	home, _ := os.UserHomeDir()
	target := result.targetDir
	if strings.TrimSpace(target) == "" {
		target = filepath.Join(home, ".copilot")
	}
	if strings.HasPrefix(target, "~/") {
		target = filepath.Join(home, target[2:])
	}
	target, _ = filepath.Abs(target)

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
	if useRemote {
		if err := InstallFilesFromRemote(files, result.chosen.BasePath, target, result.chosen.Version, result.chosen.ID, repo, branch); err != nil {
			return err
		}
	} else {
		if err := InstallFiles(files, result.chosen.BasePath, target, result.chosen.Version, result.chosen.ID); err != nil {
			return err
		}
	}

	// Persist config
	cwd, _ := os.Getwd()
	_ = WriteProjectConfig(cwd, &Config{Manifest: result.chosen.ID, Tier: result.tierID})
	_ = EnsureGitExclude(cwd)

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
func RunList(manifests []Manifest, manifestID, tierID, category string, jsonOutput bool) error {
	// Overview mode
	if manifestID == "" && tierID == "" && category == "" {
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

	// Pick manifest
	var chosen *Manifest
	if manifestID != "" {
		for i, m := range manifests {
			if m.ID == manifestID {
				chosen = &manifests[i]
				break
			}
		}
		if chosen == nil {
			return fmt.Errorf("unknown manifest: %s (available: %s)",
				manifestID, manifestIDs(manifests))
		}
	} else {
		chosen = &manifests[0]
	}

	groups := ListByCategory(chosen.Files, *chosen, tierID, category)

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"id":      chosen.ID,
			"name":    chosen.Name,
			"version": chosen.Version,
			"groups":  groups,
		})
	}

	tierLabel := tierID
	if tierLabel == "" {
		tierLabel = "all tiers"
	}
	fmt.Println(titleStyle.Render(fmt.Sprintf("\n%s v%s — %s", chosen.Name, chosen.Version, tierLabel)))
	fmt.Println(formatGroups(groups))
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
			name := filepath.Base(f.Dest)
			for _, suf := range []string{".instructions.md", ".prompt.md", ".agent.md"} {
				name = strings.TrimSuffix(name, suf)
			}
			if name == "SKILL.md" {
				parts := strings.Split(f.Dest, "/")
				if len(parts) >= 2 {
					name = parts[len(parts)-2]
				}
			}
			fmt.Fprintf(&b, "  %s\n", name)
			if f.Description != "" {
				fmt.Fprintf(&b, "    %s\n", f.Description)
			}
			fmt.Fprintf(&b, "    tier: %s  →  %s\n", f.Tier, f.Dest)
		}
	}
	return b.String()
}

func manifestIDs(ms []Manifest) string {
	var ids []string
	for _, m := range ms {
		ids = append(ids, m.ID)
	}
	return strings.Join(ids, ", ")
}
