package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/peregrine-digital/activate-framework/cli/commands"
	"github.com/peregrine-digital/activate-framework/cli/engine"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/tui/screens"
	"github.com/peregrine-digital/activate-framework/cli/tui/style"
)

// ── Main menu ───────────────────────────────────────────────────

type menuValues struct {
	choice string
}

type mainMenuModel struct {
	form       *huh.Form
	width      int
	height     int
	manifests  []model.Manifest
	cfg        model.Config
	state      model.InstallState
	projectDir string

	vals   *menuValues
	action string

	mode      string // "menu" | "text"
	textTitle string
	textBody  string
}

func (m mainMenuModel) Init() tea.Cmd {
	return m.form.Init()
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
				m.form = buildMainMenuForm(m.state, &m.vals.choice)
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
			switch m.vals.choice {
			case "list":
				m.mode = "text"
				m.textTitle = "Frameworks"
				m.textBody = strings.TrimSpace(model.FormatManifestList(m.manifests))
				m.form = buildMainMenuForm(m.state, &m.vals.choice)
				return m, nil
			case "state":
				m.mode = "text"
				m.textTitle = "Current State"
				m.textBody = m.stateBody()
				m.form = buildMainMenuForm(m.state, &m.vals.choice)
				return m, nil
			default:
				m.action = m.vals.choice
				return m, tea.Quit
			}
		}
	}

	return m, cmd
}

func (m mainMenuModel) View() string {
	var sections []string
	sections = append(sections, style.RenderBanner())
	sections = append(sections, "")
	sections = append(sections, style.DimStyle.Render("  Main Menu"))
	sections = append(sections, style.DimStyle.Render("  "+m.stateText()))
	sections = append(sections, "")

	if m.mode == "text" {
		body := strings.TrimSpace(m.textBody)
		if body == "" {
			body = "(no details)"
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(style.ColorGold)).
			Padding(1, 2).
			Render(m.textTitle + "\n\n" + body)
		sections = append(sections, box)
		sections = append(sections, "")
		sections = append(sections, style.DimStyle.Render("  enter/esc to return · ctrl+c quit"))
	} else {
		sections = append(sections, m.form.View())
		sections = append(sections, "")
		sections = append(sections, style.DimStyle.Render("  ↑/↓ navigate · enter select · ctrl+c quit"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	return style.CenterContent(content, m.height)
}

func (m mainMenuModel) stateText() string {
	text := ""
	if m.state.HasProjectConfig {
		text = "saved config detected"
	} else {
		text = "no project config detected"
	}
	if m.state.HasInstallMarker {
		text += fmt.Sprintf(" · installed %s", m.state.InstalledManifest)
	}
	return text
}

func (m mainMenuModel) stateBody() string {
	return fmt.Sprintf(
		"Project: %s\nGlobal config:  %t\nProject config: %t\nInstall marker: %t\nInstalled: %s\n\nEffective config:\n  manifest: %s\n  tier: %s",
		m.projectDir,
		m.state.HasGlobalConfig,
		m.state.HasProjectConfig,
		m.state.HasInstallMarker,
		m.state.InstalledManifest,
		m.cfg.Manifest,
		m.cfg.Tier,
	)
}

func buildMainMenuForm(state model.InstallState, choice *string) *huh.Form {
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
		items = append(items, menuItem{label: "Update all files", value: "update-all"})
	}
	items = append(items,
		menuItem{label: "Manage files", value: "manage-files"},
		menuItem{label: "Settings", value: "settings"},
		menuItem{label: "Telemetry", value: "telemetry"},
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
		stateText += fmt.Sprintf(" · installed %s", state.InstalledManifest)
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

// RunInteractiveMenu starts the state-aware main menu loop.
func RunInteractiveMenu(svc commands.ActivateAPI) error {
	for {
		svc.RefreshConfig()
		cfg := svc.CurrentConfig()
		state := engine.DetectInstallState(svc.CurrentProjectDir())

		vals := &menuValues{}
		menuModel := mainMenuModel{
			manifests:  svc.CurrentManifests(),
			cfg:        cfg,
			state:      state,
			projectDir: svc.CurrentProjectDir(),
			mode:       "menu",
			vals:       vals,
		}
		menuModel.form = buildMainMenuForm(menuModel.state, &vals.choice)

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
				if err := InstallWithResolvedConfig(svc.CurrentManifests(), cfg, resolveTargetPath(target)); err != nil {
					_ = runFullscreenText("Action Failed", "Quick install", err.Error())
				}
			}

		case "repo-add":
			addResult, err := svc.RepoAdd()
			if err != nil {
				_ = runFullscreenText("Action Failed", "Repo add", err.Error())
			} else {
				msg := fmt.Sprintf("✓ Added managed files to repository\n\n  Manifest: %s\n  Tier:     %s", addResult.Manifest, addResult.Tier)
				_ = runFullscreenText("Repo Add Complete", "", msg)
			}

		case "repo-remove":
			if err := svc.RepoRemove(); err != nil {
				_ = runFullscreenText("Action Failed", "Repo remove", err.Error())
			} else {
				_ = runFullscreenText("Repo Remove Complete", "", "✓ Removed all managed files from repository")
			}

		case "manage-files":
			if err := screens.RunFileBrowser(svc); err != nil {
				_ = runFullscreenText("Action Failed", "File browser", err.Error())
			}

		case "update-all":
			updateResult, err := svc.Update()
			if err != nil {
				_ = runFullscreenText("Action Failed", "Update all", err.Error())
			} else {
				_ = runFullscreenText("Update Complete", "", FormatUpdateResult(updateResult))
			}

		case "settings":
			if _, err := screens.RunSettings(svc); err != nil {
				_ = runFullscreenText("Action Failed", "Settings", err.Error())
			}

		case "telemetry":
			if err := screens.RunTelemetryScreen(svc); err != nil {
				_ = runFullscreenText("Action Failed", "Telemetry", err.Error())
			}

		default:
			return nil
		}
	}
}
