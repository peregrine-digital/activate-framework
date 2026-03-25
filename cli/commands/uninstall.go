package commands

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/storage"
)

const shellMarker = "# Added by Activate CLI installer"

// RunUninstall removes the activate binary, cache, config, and shell PATH entries.
func RunUninstall(force bool) error {
	base := storage.StoreBase()

	if !force {
		fmt.Printf("This will remove:\n")
		fmt.Printf("  • %s (binary, config, cache)\n", base)
		fmt.Printf("  • PATH entries from shell profiles\n")
		fmt.Printf("\nContinue? [y/N] ")

		reader := bufio.NewReader(os.Stdin)
		answer, _ := reader.ReadString('\n')
		answer = strings.TrimSpace(strings.ToLower(answer))
		if answer != "y" && answer != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	// Remove ~/.activate directory (binary, config, cache — everything)
	if err := os.RemoveAll(base); err != nil {
		return fmt.Errorf("failed to remove %s: %w", base, err)
	}
	fmt.Printf("  ✓ Removed %s\n", base)

	// Clean shell profile PATH entries
	home, _ := os.UserHomeDir()
	cleaned := removeShellEntries(home)
	for _, profile := range cleaned {
		fmt.Printf("  ✓ Cleaned PATH from %s\n", profile)
	}

	fmt.Println("\nActivate CLI uninstalled. Restart your terminal to apply PATH changes.")
	return nil
}

// removeShellEntries removes the activate PATH marker + export line from shell profiles.
func removeShellEntries(home string) []string {
	if home == "" {
		return nil
	}

	profiles := []string{
		filepath.Join(home, ".zshenv"),
		filepath.Join(home, ".bashrc"),
		filepath.Join(home, ".bash_profile"),
		filepath.Join(home, ".profile"),
	}

	var cleaned []string
	for _, profile := range profiles {
		if removeMarkerBlock(profile) {
			cleaned = append(cleaned, profile)
		}
	}
	return cleaned
}

// removeMarkerBlock removes the marker line and the line following it from a file.
func removeMarkerBlock(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	var out []string
	changed := false

	for i := 0; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == shellMarker {
			changed = true
			// Skip the marker and the next line (the export PATH line)
			if i+1 < len(lines) {
				i++
			}
			// Also skip a preceding blank line if present
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
				out = out[:len(out)-1]
			}
			continue
		}
		out = append(out, lines[i])
	}

	if !changed {
		return false
	}

	// Trim trailing blank lines
	for len(out) > 0 && strings.TrimSpace(out[len(out)-1]) == "" {
		out = out[:len(out)-1]
	}
	if len(out) > 0 {
		// Ensure file ends with newline
		result := strings.Join(out, "\n") + "\n"
		_ = os.WriteFile(path, []byte(result), 0644)
	} else {
		// File is now empty — leave it alone (don't delete shell profile)
		_ = os.WriteFile(path, []byte(""), 0644)
	}

	return true
}
