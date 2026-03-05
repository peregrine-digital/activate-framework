package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

// SidecarPath returns the path to the installed.json sidecar file.
func SidecarPath(projectDir string) string {
	return filepath.Join(RepoStorePath(projectDir), "installed.json")
}

// ReadRepoSidecar reads the sidecar file for a project directory.
func ReadRepoSidecar(projectDir string) (*model.RepoSidecar, error) {
	data, err := os.ReadFile(SidecarPath(projectDir))
	if err != nil {
		return nil, nil
	}
	var sc model.RepoSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, nil
	}
	return &sc, nil
}

// WriteRepoSidecar writes the sidecar file, deleting files that were
// removed from the tracked set.
func WriteRepoSidecar(projectDir string, next model.RepoSidecar) error {
	prev, _ := ReadRepoSidecar(projectDir)
	prevSet := make(map[string]struct{})
	nextSet := make(map[string]struct{})

	var prevFiles []string
	if prev != nil {
		prevFiles = prev.Files
	}

	for _, path := range prevFiles {
		prevSet[path] = struct{}{}
	}
	for _, path := range next.Files {
		nextSet[path] = struct{}{}
	}

	for oldPath := range prevSet {
		if _, exists := nextSet[oldPath]; exists {
			continue
		}
		full := filepath.Join(projectDir, oldPath)
		_ = os.Remove(full)
		pruneEmptyDirs(filepath.Dir(full), filepath.Join(projectDir, ".github"))
	}

	if err := EnsureRepoMeta(projectDir); err != nil {
		return err
	}
	path := SidecarPath(projectDir)
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return err
	}

	return SyncGitExclude(projectDir, next.Files)
}

// DeleteRepoSidecar removes the sidecar and all tracked files.
func DeleteRepoSidecar(projectDir string) error {
	sc, _ := ReadRepoSidecar(projectDir)
	if sc != nil {
		for _, rel := range sc.Files {
			full := filepath.Join(projectDir, rel)
			_ = os.Remove(full)
			pruneEmptyDirs(filepath.Dir(full), filepath.Join(projectDir, ".github"))
		}
		if len(sc.McpServers) > 0 {
			_ = RemoveMcpServers(projectDir, sc.McpServers)
		}
	}
	_ = os.Remove(SidecarPath(projectDir))
	return RemoveGitExcludeBlock(projectDir)
}

// pruneEmptyDirs removes empty directories from dir upward, stopping at
// (and never removing) boundary. This cleans up e.g. .github/agents/
// after all agent files are uninstalled.
func pruneEmptyDirs(dir, boundary string) {
	boundary = filepath.Clean(boundary)
	for {
		dir = filepath.Clean(dir)
		if dir == boundary || !strings.HasPrefix(dir, boundary+string(filepath.Separator)) {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		_ = os.Remove(dir)
		dir = filepath.Dir(dir)
	}
}
