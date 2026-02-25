package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	repoSidecarRel       = ".github/.activate-installed.json"
	repoExcludeStartMark = "# >>> Peregrine Activate (managed — do not edit)"
	repoExcludeEndMark   = "# <<< Peregrine Activate"
)

type repoSidecar struct {
	Manifest   string   `json:"manifest"`
	Version    string   `json:"version"`
	Tier       string   `json:"tier"`
	Files      []string `json:"files"`
	McpServers []string `json:"mcpServers,omitempty"`
	Source     string   `json:"source,omitempty"`
}

func sidecarPath(projectDir string) string {
	return filepath.Join(projectDir, repoSidecarRel)
}

func readRepoSidecar(projectDir string) (*repoSidecar, error) {
	data, err := os.ReadFile(sidecarPath(projectDir))
	if err != nil {
		return nil, nil
	}
	var sc repoSidecar
	if err := json.Unmarshal(data, &sc); err != nil {
		return nil, nil
	}
	return &sc, nil
}

func writeRepoSidecar(projectDir string, next repoSidecar) error {
	prev, _ := readRepoSidecar(projectDir)
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
		_ = os.Remove(filepath.Join(projectDir, oldPath))
	}

	path := sidecarPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return err
	}

	excludePaths := []string{repoSidecarRel}
	excludePaths = append(excludePaths, next.Files...)
	return syncRepoGitExcludeIfPresent(projectDir, excludePaths)
}

func deleteRepoSidecar(projectDir string) error {
	sc, _ := readRepoSidecar(projectDir)
	if sc != nil {
		for _, rel := range sc.Files {
			_ = os.Remove(filepath.Join(projectDir, rel))
		}
		// Clean up managed MCP servers from .vscode/mcp.json
		if len(sc.McpServers) > 0 {
			_ = RemoveMcpServers(projectDir, sc.McpServers)
		}
	}
	_ = os.Remove(sidecarPath(projectDir))
	return removeRepoGitExcludeBlockIfPresent(projectDir)
}

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

func findManifestByID(manifests []Manifest, manifestID string) *Manifest {
	for i := range manifests {
		if manifests[i].ID == manifestID {
			return &manifests[i]
		}
	}
	return nil
}

func RepoAdd(manifests []Manifest, cfg Config, projectDir string, useRemote bool, repo, branch string) error {
	chosen := findManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	files := SelectFiles(chosen.Files, *chosen, cfg.Tier)
	installed := make([]string, 0, len(files)+1)

	// Separate MCP server files from regular files
	var regularFiles []ManifestFile
	var mcpFiles []ManifestFile
	for _, f := range files {
		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Src)
		}
		if cat == "mcp-servers" {
			mcpFiles = append(mcpFiles, f)
		} else {
			regularFiles = append(regularFiles, f)
		}
	}

	// Read previous sidecar for MCP cleanup
	prevSidecar, _ := readRepoSidecar(projectDir)
	var previousMcpNames []string
	if prevSidecar != nil {
		previousMcpNames = prevSidecar.McpServers
	}

	for _, f := range regularFiles {
		destRel := filepath.ToSlash(filepath.Join(".github", f.Dest))
		destPath := filepath.Join(projectDir, destRel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		if useRemote {
			srcPath := f.Src
			if chosen.BasePath != "" {
				srcPath = chosen.BasePath + "/" + f.Src
			}
			data, err := FetchFile(srcPath, repo, branch)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, err)
				continue
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return err
			}
		} else {
			srcPath := filepath.Join(chosen.BasePath, f.Src)
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destPath, data, 0644); err != nil {
				return err
			}
		}

		fmt.Printf("  ✓  %s\n", destRel)
		installed = append(installed, destRel)
	}

	// Handle MCP server files
	var mcpServerNames []string
	if len(mcpFiles) > 0 || len(previousMcpNames) > 0 {
		names, err := InjectMcpFromManifest(mcpFiles, chosen.BasePath, projectDir, previousMcpNames)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", err)
		} else {
			mcpServerNames = names
			for _, name := range names {
				fmt.Printf("  ✓  MCP server: %s\n", name)
			}
		}
	}

	marker := map[string]string{"manifest": chosen.ID, "version": chosen.Version}
	if useRemote {
		marker["remote"] = repo + "@" + branch
	}
	markerPath := filepath.Join(projectDir, ".github", ".activate-version")
	if err := os.MkdirAll(filepath.Dir(markerPath), 0755); err != nil {
		return err
	}
	markerData, _ := json.MarshalIndent(marker, "", "  ")
	if err := os.WriteFile(markerPath, append(markerData, '\n'), 0644); err != nil {
		return err
	}
	installed = append(installed, filepath.ToSlash(filepath.Join(".github", ".activate-version")))

	source := "bundled"
	if useRemote {
		source = "remote"
	}
	if err := writeRepoSidecar(projectDir, repoSidecar{
		Manifest:   chosen.ID,
		Version:    chosen.Version,
		Tier:       cfg.Tier,
		Files:      installed,
		McpServers: mcpServerNames,
		Source:     source,
	}); err != nil {
		return err
	}

	if err := WriteProjectConfig(projectDir, &Config{Manifest: chosen.ID, Tier: cfg.Tier}); err == nil {
		_ = EnsureGitExclude(projectDir)
	}

	fmt.Printf("\nAdded %d managed files to repository.\n", len(installed))
	return nil
}

func RepoRemove(projectDir string) error {
	sc, _ := readRepoSidecar(projectDir)
	if sc == nil {
		fmt.Println("No managed repo sidecar found; nothing to remove.")
		return nil
	}
	count := len(sc.Files)
	if err := deleteRepoSidecar(projectDir); err != nil {
		return err
	}
	fmt.Printf("Removed %d managed files from repository.\n", count)
	return nil
}
