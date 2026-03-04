package storage

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	excludeStartMark = "# >>> Peregrine Activate (managed — do not edit)"
	excludeEndMark   = "# <<< Peregrine Activate"
)

// SyncGitExclude updates the managed block in .git/info/exclude.
func SyncGitExclude(projectDir string, paths []string) error {
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		return nil
	}
	content := string(data)
	block := strings.Join(append([]string{excludeStartMark}, append(paths, excludeEndMark)...), "\n")

	startIdx := strings.Index(content, excludeStartMark)
	endIdx := strings.Index(content, excludeEndMark)
	if startIdx >= 0 && endIdx >= 0 && endIdx >= startIdx {
		content = content[:startIdx] + block + content[endIdx+len(excludeEndMark):]
	} else {
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + block + "\n"
	}

	return os.WriteFile(excludePath, []byte(content), 0644)
}

// RemoveGitExcludeBlock removes the managed block from .git/info/exclude.
func RemoveGitExcludeBlock(projectDir string) error {
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		return nil
	}
	content := string(data)
	startIdx := strings.Index(content, excludeStartMark)
	endIdx := strings.Index(content, excludeEndMark)
	if startIdx < 0 || endIdx < 0 || endIdx < startIdx {
		return nil
	}
	content = content[:startIdx] + content[endIdx+len(excludeEndMark):]
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	return os.WriteFile(excludePath, []byte(content), 0644)
}
