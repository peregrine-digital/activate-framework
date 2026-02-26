package main

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	repoExcludeStartMark = "# >>> Peregrine Activate (managed — do not edit)"
	repoExcludeEndMark   = "# <<< Peregrine Activate"
)

func syncRepoGitExcludeIfPresent(projectDir string, paths []string) error {
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		return nil // Only manage if exclude file already exists
	}
	content := string(data)
	block := strings.Join(append([]string{repoExcludeStartMark}, append(paths, repoExcludeEndMark)...), "\n")

	startIdx := strings.Index(content, repoExcludeStartMark)
	endIdx := strings.Index(content, repoExcludeEndMark)
	if startIdx >= 0 && endIdx >= 0 && endIdx >= startIdx {
		content = content[:startIdx] + block + content[endIdx+len(repoExcludeEndMark):]
	} else {
		if len(content) > 0 && !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += "\n" + block + "\n"
	}

	return os.WriteFile(excludePath, []byte(content), 0644)
}

func removeRepoGitExcludeBlockIfPresent(projectDir string) error {
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	data, err := os.ReadFile(excludePath)
	if err != nil {
		return nil
	}
	content := string(data)
	startIdx := strings.Index(content, repoExcludeStartMark)
	endIdx := strings.Index(content, repoExcludeEndMark)
	if startIdx < 0 || endIdx < 0 || endIdx < startIdx {
		return nil
	}
	content = content[:startIdx] + content[endIdx+len(repoExcludeEndMark):]
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	return os.WriteFile(excludePath, []byte(content), 0644)
}
