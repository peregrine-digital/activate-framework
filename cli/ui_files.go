package main

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── File browser ────────────────────────────────────────────────
//
// Two-mode Bubble Tea model:
//   - "browse"  → categorized file list (huh.Select)
//   - "actions" → context-aware action menu for the selected file
//   - "text"    → read-only display (diff output, action result)
//
// Called from RunInteractiveMenu as a sub-screen.

// fileBrowserValues holds heap-allocated form bindings that survive
// Bubble Tea's value-receiver copies.
type fileBrowserValues struct {
	selectedFile string
	action       string
}

type fileBrowserModel struct {
	svc    ActivateAPI
	files  []FileStatus
	vals   *fileBrowserValues
	form   *huh.Form
	mode   string // "browse", "actions", "text"
	width  int
	height int

	// text-mode fields
	textTitle string
	textBody  string

	// result: set when the browser should close
	done bool
}

// RunFileBrowser launches the file browser as a fullscreen Bubble Tea program.
func RunFileBrowser(svc ActivateAPI) error {
	m := newFileBrowserModel(svc)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newFileBrowserModel(svc ActivateAPI) fileBrowserModel {
	state := svc.GetState()
	vals := &fileBrowserValues{}
	form := buildFileBrowseForm(state.Files, &vals.selectedFile)

	return fileBrowserModel{
		svc:  svc,
		files: state.Files,
		vals:  vals,
		form:  form,
		mode:  "browse",
	}
}

func (m fileBrowserModel) Init() tea.Cmd {
	return m.form.Init()
}

func (m fileBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			switch m.mode {
			case "text":
				return m.switchToBrowse()
			case "actions":
				return m.switchToBrowse()
			case "browse":
				m.done = true
				return m, tea.Quit
			}
		case "enter", "q":
			if m.mode == "text" {
				return m.switchToBrowse()
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

	if f.State != huh.StateCompleted {
		return m, cmd
	}

	// Form completed — dispatch based on current mode
	switch m.mode {
	case "browse":
		if m.vals.selectedFile == "_back" {
			m.done = true
			return m, tea.Quit
		}
		return m.switchToActions()
	case "actions":
		return m.handleAction()
	}

	return m, cmd
}

func (m fileBrowserModel) View() string {
	var sections []string
	sections = append(sections, renderBanner())
	sections = append(sections, "")

	m.svc.RefreshConfig()
	cfg := m.svc.CurrentConfig()
	subtitle := fmt.Sprintf("  manifest=%s · tier=%s · %d files",
		cfg.Manifest, cfg.Tier, len(m.files))

	switch m.mode {
	case "browse":
		sections = append(sections, dimStyle.Render("  Manage Files"))
		sections = append(sections, dimStyle.Render(subtitle))
		sections = append(sections, "")
		sections = append(sections, m.form.View())
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  ↑/↓ navigate · enter select · esc back · ctrl+c quit"))
	case "actions":
		fs := m.findSelectedFile()
		if fs != nil {
			sections = append(sections, dimStyle.Render("  "+fs.DisplayName))
			sections = append(sections, dimStyle.Render("  "+fileStatusLine(*fs)))
		}
		sections = append(sections, "")
		sections = append(sections, m.form.View())
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  ↑/↓ navigate · enter select · esc back · ctrl+c quit"))
	case "text":
		sections = append(sections, dimStyle.Render("  "+m.textTitle))
		sections = append(sections, "")
		body := strings.TrimSpace(m.textBody)
		if body == "" {
			body = "(no output)"
		}
		box := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(colorGold)).
			Padding(1, 2).
			Render(body)
		sections = append(sections, box)
		sections = append(sections, "")
		sections = append(sections, dimStyle.Render("  enter/esc to return · ctrl+c quit"))
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

// ── Mode transitions ────────────────────────────────────────────

func (m fileBrowserModel) switchToBrowse() (tea.Model, tea.Cmd) {
	// Refresh file state from service
	state := m.svc.GetState()
	m.files = state.Files
	m.vals.selectedFile = ""
	m.vals.action = ""
	m.mode = "browse"
	m.form = buildFileBrowseForm(m.files, &m.vals.selectedFile)
	return m, m.form.Init()
}

func (m fileBrowserModel) switchToActions() (tea.Model, tea.Cmd) {
	fs := m.findSelectedFile()
	if fs == nil {
		return m.switchToBrowse()
	}
	m.mode = "actions"
	m.vals.action = ""
	m.form = buildFileActionsForm(*fs, &m.vals.action)
	return m, m.form.Init()
}

func (m fileBrowserModel) handleAction() (tea.Model, tea.Cmd) {
	action := m.vals.action
	dest := m.vals.selectedFile

	switch action {
	case "back":
		return m.switchToBrowse()

	case "install", "update":
		result, err := m.svc.InstallFile(dest)
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		verb := "Installed"
		if action == "update" {
			verb = "Updated"
		}
		m.mode = "text"
		m.textTitle = verb
		m.textBody = fmt.Sprintf("%s %s", verb, result.File)
		return m, nil

	case "uninstall":
		result, err := m.svc.UninstallFile(dest)
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Uninstalled"
		m.textBody = fmt.Sprintf("Removed %s", result.File)
		return m, nil

	case "diff":
		result, err := m.svc.DiffFile(dest)
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		if result.Identical {
			m.mode = "text"
			m.textTitle = "Diff: " + dest
			m.textBody = "Files are identical — no differences."
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Diff: " + dest
		m.textBody = result.Diff
		return m, nil

	case "skip":
		_, err := m.svc.SkipUpdate(dest)
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Skipped"
		m.textBody = fmt.Sprintf("Will skip updates for %s at this version.", dest)
		return m, nil

	case "pin":
		_, err := m.svc.SetOverride(dest, "pinned")
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Pinned"
		m.textBody = fmt.Sprintf("%s is now pinned (always installed regardless of tier).", dest)
		return m, nil

	case "exclude":
		_, err := m.svc.SetOverride(dest, "excluded")
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Excluded"
		m.textBody = fmt.Sprintf("%s is now excluded (will not be installed).", dest)
		return m, nil

	case "clear-override":
		_, err := m.svc.SetOverride(dest, "")
		if err != nil {
			m.mode = "text"
			m.textTitle = "Error"
			m.textBody = err.Error()
			return m, nil
		}
		m.mode = "text"
		m.textTitle = "Override Cleared"
		m.textBody = fmt.Sprintf("Removed override for %s. Normal tier rules apply.", dest)
		return m, nil
	}

	return m.switchToBrowse()
}

// ── Helpers ─────────────────────────────────────────────────────

func (m fileBrowserModel) findSelectedFile() *FileStatus {
	for i := range m.files {
		if m.files[i].Dest == m.vals.selectedFile {
			return &m.files[i]
		}
	}
	return nil
}

// ── Form builders ───────────────────────────────────────────────

func buildFileBrowseForm(files []FileStatus, selected *string) *huh.Form {
	grouped := groupFilesByCategory(files)
	options := make([]huh.Option[string], 0, len(files)+1)

	for _, group := range grouped {
		for _, f := range group.files {
			label := formatFileOption(f)
			options = append(options, huh.NewOption(label, f.Dest))
		}
	}

	options = append(options, huh.NewOption(dimStyle.Render("← Back to menu"), "_back"))

	*selected = ""
	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select a file").
				Options(options...).
				Value(selected).
				Height(20),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

func buildFileActionsForm(fs FileStatus, action *string) *huh.Form {
	options := fileActionsForStatus(fs)
	*action = ""

	return huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title(fs.DisplayName).
				Description(fileStatusLine(fs)).
				Options(options...).
				Value(action),
		),
	).WithTheme(huh.ThemeCharm()).WithShowHelp(false)
}

// fileActionsForStatus returns context-aware action options.
func fileActionsForStatus(fs FileStatus) []huh.Option[string] {
	var opts []huh.Option[string]

	if fs.Override == "excluded" {
		opts = append(opts, huh.NewOption("Clear exclusion", "clear-override"))
		opts = append(opts, huh.NewOption("← Back", "back"))
		return opts
	}

	if !fs.Installed {
		opts = append(opts, huh.NewOption("Install", "install"))
		if fs.Override == "" {
			opts = append(opts, huh.NewOption("Exclude from installs", "exclude"))
		}
		opts = append(opts, huh.NewOption("← Back", "back"))
		return opts
	}

	// Installed
	if fs.UpdateAvailable {
		opts = append(opts,
			huh.NewOption("Update to "+fs.BundledVersion, "update"),
			huh.NewOption("Show diff", "diff"),
			huh.NewOption("Skip this version", "skip"),
		)
	} else {
		opts = append(opts, huh.NewOption("Show diff", "diff"))
	}

	opts = append(opts, huh.NewOption("Uninstall", "uninstall"))

	// Override options
	switch fs.Override {
	case "pinned":
		opts = append(opts, huh.NewOption("Remove pin", "clear-override"))
	case "":
		opts = append(opts,
			huh.NewOption("Pin (always install)", "pin"),
			huh.NewOption("Exclude from installs", "exclude"),
		)
	}

	opts = append(opts, huh.NewOption("← Back", "back"))
	return opts
}

// ── Formatting ──────────────────────────────────────────────────

type fileGroup struct {
	category string
	files    []FileStatus
}

func groupFilesByCategory(files []FileStatus) []fileGroup {
	order := make([]string, 0)
	byCategory := make(map[string][]FileStatus)

	for _, f := range files {
		cat := f.Category
		if cat == "" {
			cat = "Other"
		}
		if _, exists := byCategory[cat]; !exists {
			order = append(order, cat)
		}
		byCategory[cat] = append(byCategory[cat], f)
	}

	groups := make([]fileGroup, 0, len(order))
	for _, cat := range order {
		groups = append(groups, fileGroup{category: cat, files: byCategory[cat]})
	}
	return groups
}

func formatFileOption(fs FileStatus) string {
	icon := fileStatusIcon(fs)
	version := fileVersionLabel(fs)
	name := fs.DisplayName
	if name == "" {
		name = fs.Dest
	}

	// Pad for alignment
	parts := []string{icon, " ", name}
	if version != "" {
		parts = append(parts, "  ", dimStyle.Render(version))
	}
	if fs.Category != "" {
		parts = append(parts, "  ", dimStyle.Render("["+fs.Category+"]"))
	}
	return strings.Join(parts, "")
}

func fileStatusIcon(fs FileStatus) string {
	if fs.Override == "excluded" {
		return "🚫"
	}
	if fs.Override == "pinned" {
		return "📌"
	}
	if !fs.Installed {
		return "○"
	}
	if fs.UpdateAvailable {
		if fs.Skipped {
			return "⏭"
		}
		return "⬆"
	}
	return "✓"
}

func fileVersionLabel(fs FileStatus) string {
	if fs.Override == "excluded" {
		return "excluded"
	}
	if !fs.Installed {
		return fs.BundledVersion
	}
	if fs.UpdateAvailable {
		return fs.InstalledVersion + " → " + fs.BundledVersion
	}
	return fs.InstalledVersion
}

func fileStatusLine(fs FileStatus) string {
	parts := []string{}
	if fs.Installed {
		parts = append(parts, "installed")
		if fs.UpdateAvailable {
			parts = append(parts, fmt.Sprintf("update available: %s → %s",
				fs.InstalledVersion, fs.BundledVersion))
		} else {
			parts = append(parts, "v"+fs.InstalledVersion)
		}
	} else {
		parts = append(parts, "not installed", "v"+fs.BundledVersion)
	}

	if fs.Override != "" {
		parts = append(parts, "override: "+fs.Override)
	}
	if fs.Skipped {
		parts = append(parts, "version skipped")
	}
	return strings.Join(parts, " · ")
}
