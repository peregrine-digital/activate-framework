package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	excludeMarkerStart = "# >>> Peregrine Activate config (managed)"
	excludeMarkerEnd   = "# <<< Peregrine Activate config"
)

// EnsureGitExclude adds .activate.json to .git/info/exclude if not present.
// Safe to call repeatedly (idempotent). Silently does nothing outside a git repo.
func EnsureGitExclude(projectDir string) error {
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")

	content, err := os.ReadFile(excludePath)
	if err != nil {
		// Try creating .git/info/
		infoDir := filepath.Join(projectDir, ".git", "info")
		if err := os.MkdirAll(infoDir, 0755); err != nil {
			return nil // not a git repo — silently skip
		}
		content = []byte{}
	}

	// Already present?
	if strings.Contains(string(content), excludeMarkerStart) {
		return nil
	}

	block := fmt.Sprintf("\n%s\n%s\n%s\n", excludeMarkerStart, projectConfigFile, excludeMarkerEnd)

	text := string(content)
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	text += block

	return os.WriteFile(excludePath, []byte(text), 0644)
}
