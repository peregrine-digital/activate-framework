package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// ── Brand colors ────────────────────────────────────────────────
const (
	colorGold   = "#E8C228" // Peregrine falcon gold
	colorDim    = "#666666" // Subtitle gray
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
	subtleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))

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

// falconArt is a simplified peregrine falcon rendered in Unicode block
// characters, colored in Peregrine gold (#E8C228).
const falconArt = `    ██
   ████
  ██████
 ███  ██▓
 ██   █▓▓▒
  ██ ██▓▒░
   ████▒░
    ██▒░
   ██░
  ██`

func renderBanner() string {
	falcon := goldStyle.Render(falconArt)

	wordmark := brightStyle.Render("P E R E G R I N E")
	subtitle := dimStyle.Render("D I G I T A L   S E R V I C E S")
	tagline := dimStyle.Render("─── Activate Framework Installer ───")

	text := lipgloss.JoinVertical(lipgloss.Left,
		"",
		wordmark,
		subtitle,
		"",
		tagline,
	)

	logo := lipgloss.JoinHorizontal(lipgloss.Center, falcon, "   ", text)
	return bannerBox.Render(logo)
}

// ── Interactive install ─────────────────────────────────────────

// RunInteractiveInstall runs the full-page TUI installer wizard.
func RunInteractiveInstall(manifests []Manifest, cfg Config, useRemote bool, repo, branch string) error {
	// Print branded banner
	fmt.Println()
	fmt.Println(renderBanner())
	fmt.Println()

	// ── Form state ──────────────────────────────────────────────
	var manifestID string
	var tierID string
	var targetDir string
	var confirm bool

	home, _ := os.UserHomeDir()
	defaultTarget := filepath.Join(home, ".copilot")

	// ── Resolve defaults from config ────────────────────────────

	// Manifest default
	manifestID = cfg.Manifest
	found := false
	for _, m := range manifests {
		if m.ID == manifestID {
			found = true
			break
		}
	}
	if !found {
		manifestID = manifests[0].ID
	}

	// ── Build manifest options ──────────────────────────────────
	var manifestOptions []huh.Option[string]
	for _, m := range manifests {
		label := m.Name
		desc := fmt.Sprintf("v%s · %d files", m.Version, len(m.Files))
		if m.Description != "" {
			desc += " — " + m.Description
		}
		manifestOptions = append(manifestOptions, huh.NewOption(
			fmt.Sprintf("%s  %s", label, dimStyle.Render(desc)),
			m.ID,
		))
	}

	// ── Build tier options (dynamic based on manifest) ──────────
	// We'll build these after manifest selection if needed. For the
	// form wizard, we use a callback-style by splitting into groups.
	// Group 1: manifest selection → Group 2: tier + target → Group 3: confirm

	manifestSelect := huh.NewSelect[string]().
		Title("Select manifest").
		Description("Choose which set of files to install").
		Options(manifestOptions...).
		Value(&manifestID)

	// ── Group 1: Manifest selection ─────────────────────────────
	group1 := huh.NewGroup(manifestSelect).
		Title("  1 · Manifest").
		Description("   Which collection of files?")

	// We'll skip group 1 entirely if there's only one manifest
	if len(manifests) == 1 {
		group1 = group1.WithHide(true)
		manifestID = manifests[0].ID
	}

	// ── Run manifest selection first so we can build tier options ─
	if len(manifests) > 1 {
		err := huh.NewForm(group1).
			WithTheme(huh.ThemeCharm()).
			Run()
		if err != nil {
			return err
		}
	}

	// Resolve chosen manifest
	var chosen Manifest
	for _, m := range manifests {
		if m.ID == manifestID {
			chosen = m
			break
		}
	}

	// ── Build tier options based on chosen manifest ─────────────
	availableTiers := DiscoverAvailableTiers(chosen)

	// Resolve tier default
	tierID = cfg.Tier
	tierFound := false
	for _, t := range availableTiers {
		if t.ID == tierID {
			tierFound = true
			break
		}
	}
	if !tierFound && len(availableTiers) > 0 {
		tierID = availableTiers[0].ID
	}

	var tierOptions []huh.Option[string]
	for _, t := range availableTiers {
		files := SelectFiles(chosen.Files, chosen, t.ID)
		desc := fmt.Sprintf("%d files", len(files))
		tierOptions = append(tierOptions, huh.NewOption(
			fmt.Sprintf("%-12s %s", t.Label, dimStyle.Render(desc)),
			t.ID,
		))
	}

	tierSelect := huh.NewSelect[string]().
		Title("Select tier").
		Description("Higher tiers include everything from lower tiers").
		Options(tierOptions...).
		Value(&tierID)

	targetInput := huh.NewInput().
		Title("Target directory").
		Description("Where to install files (leave empty for default)").
		Placeholder(defaultTarget).
		Value(&targetDir)

	// Dynamic confirm description showing summary
	confirmField := huh.NewConfirm().
		Title("Ready to install?").
		Affirmative("  Install  ").
		Negative("  Cancel  ").
		Value(&confirm)

	// ── Group 2: Tier + Target ──────────────────────────────────
	group2Fields := []huh.Field{targetInput}
	if len(availableTiers) > 1 {
		group2Fields = []huh.Field{tierSelect, targetInput}
	} else if len(availableTiers) == 1 {
		tierID = availableTiers[0].ID
	}

	group2 := huh.NewGroup(group2Fields...).
		Title(fmt.Sprintf("  2 · Configure — %s v%s", chosen.Name, chosen.Version)).
		Description("   Choose your tier and destination")

	// ── Group 3: Confirm ────────────────────────────────────────
	group3 := huh.NewGroup(confirmField).
		Title("  3 · Confirm").
		Description("   Review and install")

	// ── Run the wizard ──────────────────────────────────────────
	err := huh.NewForm(group2, group3).
		WithTheme(huh.ThemeCharm()).
		Run()
	if err != nil {
		return err
	}

	if !confirm {
		fmt.Println(dimStyle.Render("\n  Cancelled.\n"))
		return nil
	}

	// ── Resolve target ──────────────────────────────────────────
	if strings.TrimSpace(targetDir) == "" {
		targetDir = defaultTarget
	}
	if strings.HasPrefix(targetDir, "~/") {
		targetDir = filepath.Join(home, targetDir[2:])
	}
	targetDir, _ = filepath.Abs(targetDir)

	// ── Show summary ────────────────────────────────────────────
	files := SelectFiles(chosen.Files, chosen, tierID)

	summary := fmt.Sprintf(
		"%s  %s v%s\n%s  %s\n%s  %d files\n%s  %s",
		dimStyle.Render("Manifest:"),
		brightStyle.Render(chosen.Name),
		chosen.Version,
		dimStyle.Render("Tier:    "),
		brightStyle.Render(tierID),
		dimStyle.Render("Files:   "),
		len(files),
		dimStyle.Render("Target:  "),
		brightStyle.Render(targetDir),
	)
	fmt.Println(summaryBox.Render(summary))
	fmt.Println()

	// ── Install ─────────────────────────────────────────────────
	if useRemote {
		if err := InstallFilesFromRemote(files, chosen.BasePath, targetDir, chosen.Version, chosen.ID, repo, branch); err != nil {
			return err
		}
	} else {
		if err := InstallFiles(files, chosen.BasePath, targetDir, chosen.Version, chosen.ID); err != nil {
			return err
		}
	}

	// Persist config
	cwd, _ := os.Getwd()
	_ = WriteProjectConfig(cwd, &Config{Manifest: chosen.ID, Tier: tierID})
	_ = EnsureGitExclude(cwd)

	// ── Success banner ──────────────────────────────────────────
	result := fmt.Sprintf(
		"%s  %s v%s (%s) installed\n%s  %s",
		successStyle.Render("✓"),
		chosen.Name,
		chosen.Version,
		tierID,
		dimStyle.Render("→"),
		targetDir,
	)
	fmt.Println(resultBox.Render(result))
	return nil
}

// ── List command ────────────────────────────────────────────────

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

// formatGroups renders category groups for terminal display.
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
