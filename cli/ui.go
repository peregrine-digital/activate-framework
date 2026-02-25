package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	subtleStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// RunInteractiveInstall runs the full TUI installer flow.
func RunInteractiveInstall(manifests []Manifest, cfg Config, useRemote bool, repo, branch string) error {
	// ── Manifest selection ──────────────────────────────────────
	var chosen Manifest
	if len(manifests) == 1 {
		chosen = manifests[0]
	} else {
		var manifestID string

		// Check if config already has a manifest
		defaultManifestID := cfg.Manifest
		found := false
		for _, m := range manifests {
			if m.ID == defaultManifestID {
				found = true
				break
			}
		}
		if !found {
			defaultManifestID = manifests[0].ID
		}

		var options []huh.Option[string]
		for _, m := range manifests {
			desc := fmt.Sprintf("v%s — %d files", m.Version, len(m.Files))
			if m.Description != "" {
				desc += " · " + m.Description
			}
			options = append(options, huh.NewOption(fmt.Sprintf("%s  %s", m.Name, subtleStyle.Render(desc)), m.ID))
		}

		err := huh.NewSelect[string]().
			Title("Which manifest?").
			Options(options...).
			Value(&manifestID).
			Run()
		if err != nil {
			return err
		}

		_ = defaultManifestID
		for _, m := range manifests {
			if m.ID == manifestID {
				chosen = m
				break
			}
		}
	}

	fmt.Println(titleStyle.Render(fmt.Sprintf("\n%s v%s Installer\n", chosen.Name, chosen.Version)))

	// ── Tier selection ──────────────────────────────────────────
	availableTiers := DiscoverAvailableTiers(chosen)
	var tierID string

	// Default from config
	defaultTierID := cfg.Tier
	tierFound := false
	for _, t := range availableTiers {
		if t.ID == defaultTierID {
			tierFound = true
			break
		}
	}
	if !tierFound && len(availableTiers) > 0 {
		defaultTierID = availableTiers[0].ID
	}

	if len(availableTiers) == 1 {
		tierID = availableTiers[0].ID
	} else {
		var tierOptions []huh.Option[string]
		for _, t := range availableTiers {
			files := SelectFiles(chosen.Files, chosen, t.ID)
			desc := fmt.Sprintf("%d files", len(files))
			tierOptions = append(tierOptions, huh.NewOption(
				fmt.Sprintf("%-12s %s", t.Label, subtleStyle.Render(desc)),
				t.ID,
			))
		}

		err := huh.NewSelect[string]().
			Title("Which tier?").
			Description("Higher tiers include everything from lower tiers").
			Options(tierOptions...).
			Value(&tierID).
			Run()
		if err != nil {
			return err
		}
	}

	_ = defaultTierID

	// ── Target directory ────────────────────────────────────────
	home, _ := os.UserHomeDir()
	defaultTarget := filepath.Join(home, ".copilot")

	var targetDir string
	err := huh.NewInput().
		Title("Target directory").
		Description("Where to install files").
		Placeholder(defaultTarget).
		Value(&targetDir).
		Run()
	if err != nil {
		return err
	}
	if strings.TrimSpace(targetDir) == "" {
		targetDir = defaultTarget
	}
	// Expand ~
	if strings.HasPrefix(targetDir, "~/") {
		targetDir = filepath.Join(home, targetDir[2:])
	}
	targetDir, _ = filepath.Abs(targetDir)

	// ── Confirmation ────────────────────────────────────────────
	files := SelectFiles(chosen.Files, chosen, tierID)

	fmt.Printf("\n  Manifest:  %s v%s\n", chosen.Name, chosen.Version)
	fmt.Printf("  Tier:      %s\n", tierID)
	fmt.Printf("  Files:     %d\n", len(files))
	fmt.Printf("  Target:    %s\n\n", targetDir)

	var confirm bool
	err = huh.NewConfirm().
		Title("Install?").
		Affirmative("Yes").
		Negative("No").
		Value(&confirm).
		Run()
	if err != nil {
		return err
	}
	if !confirm {
		fmt.Println(subtleStyle.Render("Cancelled."))
		return nil
	}

	// ── Install ─────────────────────────────────────────────────
	fmt.Println()
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

	fmt.Println(successStyle.Render(
		fmt.Sprintf("\n✓ Done. %s v%s (%s) installed to %s", chosen.Name, chosen.Version, tierID, targetDir),
	))
	return nil
}

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
		fmt.Println(titleStyle.Render("\nAvailable manifests:\n"))
		fmt.Println(FormatManifestList(manifests))
		fmt.Println(subtleStyle.Render("Use --manifest <id> to see files for a specific manifest.\n"))
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

// formatGroups renders category groups for terminal display.
func formatGroups(groups []CategoryGroup) string {
	var b strings.Builder
	for _, g := range groups {
		fmt.Fprintf(&b, "\n%s (%d)\n", g.Label, len(g.Files))
		b.WriteString(strings.Repeat("─", 40) + "\n")
		for _, f := range g.Files {
			name := filepath.Base(f.Dest)
			// Strip known suffixes
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
