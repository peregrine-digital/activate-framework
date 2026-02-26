package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── Update command ──────────────────────────────────────────────

// UpdateFiles re-installs only currently-tracked files, respecting skipped versions.
// Also refreshes MCP server entries from manifest.
func UpdateFiles(m Manifest, sidecar *repoSidecar, cfg Config, projectDir string, useRemote bool, repo, branch string) (updated []string, skipped []string, err error) {
	if sidecar == nil {
		return nil, nil, fmt.Errorf("no sidecar found; nothing to update")
	}

	installedSet := make(map[string]bool)
	for _, f := range sidecar.Files {
		installedSet[f] = true
	}

	// Collect MCP files for batch injection
	var mcpFiles []ManifestFile

	for _, f := range m.Files {
		cat := f.Category
		if cat == "" {
			cat = InferCategory(f.Src)
		}
		if cat == "mcp-servers" {
			mcpFiles = append(mcpFiles, f)
			continue
		}

		destRel := ".github/" + f.Dest
		if !installedSet[destRel] {
			continue
		}

		// Check for skipped version
		if sv, ok := cfg.SkippedVersions[f.Dest]; ok {
			bv := ""
			if m.BasePath != "" {
				bv, _ = ReadFileVersion(filepath.Join(m.BasePath, f.Src))
			}
			if sv == bv {
				skipped = append(skipped, f.Dest)
				continue
			}
		}

		destPath := filepath.Join(projectDir, destRel)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return updated, skipped, err
		}

		if useRemote {
			srcPath := f.Src
			if m.BasePath != "" {
				srcPath = m.BasePath + "/" + f.Src
			}
			data, fetchErr := FetchFile(srcPath, repo, branch)
			if fetchErr != nil {
				fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, fetchErr)
				continue
			}
			if writeErr := os.WriteFile(destPath, data, 0644); writeErr != nil {
				return updated, skipped, writeErr
			}
		} else {
			srcPath := filepath.Join(m.BasePath, f.Src)
			data, readErr := os.ReadFile(srcPath)
			if readErr != nil {
				return updated, skipped, readErr
			}
			if writeErr := os.WriteFile(destPath, data, 0644); writeErr != nil {
				return updated, skipped, writeErr
			}
		}

		updated = append(updated, f.Dest)
	}

	// Re-inject MCP servers
	if len(mcpFiles) > 0 || len(sidecar.McpServers) > 0 {
		names, mcpErr := InjectMcpFromManifest(mcpFiles, m.BasePath, projectDir, sidecar.McpServers)
		if mcpErr != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", mcpErr)
		} else {
			sidecar.McpServers = names
		}
	}

	// Update sidecar version
	sidecar.Version = m.Version
	if err := writeRepoSidecar(projectDir, *sidecar); err != nil {
		return updated, skipped, err
	}

	return updated, skipped, nil
}

func runUpdateCommand(svc *ActivateService, jsonOutput bool) error {
	result, err := svc.Update()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}

	for _, f := range result.Updated {
		fmt.Printf("  ✓  %s\n", f)
	}
	for _, f := range result.Skipped {
		fmt.Printf("  ⊘  %s (skipped)\n", f)
	}
	fmt.Printf("\nUpdated %d files, skipped %d.\n", len(result.Updated), len(result.Skipped))
	return nil
}

// ── Per-file install ────────────────────────────────────────────

// InstallSingleFile installs one manifest file and updates the sidecar.
func InstallSingleFile(f ManifestFile, m Manifest, projectDir string, useRemote bool, repo, branch string) error {
	destRel := ".github/" + f.Dest
	destPath := filepath.Join(projectDir, destRel)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	if useRemote {
		srcPath := f.Src
		if m.BasePath != "" {
			srcPath = m.BasePath + "/" + f.Src
		}
		data, err := FetchFile(srcPath, repo, branch)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", f.Src, err)
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	} else {
		srcPath := filepath.Join(m.BasePath, f.Src)
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("read %s: %w", srcPath, err)
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}

	// Update sidecar
	sidecar, _ := readRepoSidecar(projectDir)
	if sidecar == nil {
		sidecar = &repoSidecar{Manifest: m.ID, Version: m.Version, Tier: ""}
	}
	if !containsString(sidecar.Files, destRel) {
		sidecar.Files = append(sidecar.Files, destRel)
	}
	return writeRepoSidecar(projectDir, *sidecar)
}

// UninstallSingleFile removes one file and updates the sidecar.
func UninstallSingleFile(dest string, projectDir string) error {
	destRel := dest
	if !strings.HasPrefix(destRel, ".github/") {
		destRel = ".github/" + destRel
	}

	sidecar, _ := readRepoSidecar(projectDir)
	if sidecar == nil {
		return fmt.Errorf("no sidecar found; nothing to uninstall")
	}

	newFiles := make([]string, 0, len(sidecar.Files))
	for _, f := range sidecar.Files {
		if f != destRel {
			newFiles = append(newFiles, f)
		}
	}
	sidecar.Files = newFiles
	// writeRepoSidecar diffs old vs new and deletes removed files
	return writeRepoSidecar(projectDir, *sidecar)
}

func runInstallFileCommand(svc *ActivateService, file string, jsonOutput bool) error {
	result, err := svc.InstallFile(file)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}
	fmt.Printf("  ✓  %s\n", result.File)
	return nil
}

// ── File diff ───────────────────────────────────────────────────

// DiffFile produces a unified diff between bundled and installed versions.
func DiffFile(f ManifestFile, m Manifest, projectDir string) (string, error) {
	srcPath := filepath.Join(m.BasePath, f.Src)
	bundled, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read bundled %s: %w", f.Src, err)
	}

	destRel := ".github/" + f.Dest
	installedPath := filepath.Join(projectDir, destRel)
	installed, err := os.ReadFile(installedPath)
	if err != nil {
		return "", fmt.Errorf("read installed %s: %w", destRel, err)
	}

	return unifiedDiff(string(bundled), string(installed), "bundled/"+f.Src, "installed/"+destRel), nil
}

func runDiffCommand(svc *ActivateService, file string) error {
	result, err := svc.DiffFile(file)
	if err != nil {
		return err
	}

	if result.Identical {
		fmt.Println("Files are identical.")
	} else {
		fmt.Print(result.Diff)
	}
	return nil
}

// ── Sync command (auto-setup equivalent) ────────────────────────

// SyncNeeded checks if the installed manifest version differs from the available version.
func SyncNeeded(m Manifest, sidecar *repoSidecar) bool {
	if sidecar == nil {
		return false
	}
	return sidecar.Version != m.Version
}

func runSyncCommand(svc *ActivateService, jsonOutput bool) error {
	result, err := svc.Sync()
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(result)
	}

	switch result.Action {
	case "none":
		if result.Reason == "not installed" {
			fmt.Println("Not installed. Run 'repo add' first.")
		} else {
			fmt.Printf("Already up to date (v%s).\n", result.AvailableVersion)
		}
	case "updated":
		fmt.Printf("Updated from v%s to v%s.\n", result.PreviousVersion, result.AvailableVersion)
		for _, f := range result.Updated {
			fmt.Printf("  ✓  %s\n", f)
		}
		for _, f := range result.Skipped {
			fmt.Printf("  ⊘  %s (skipped)\n", f)
		}
	}
	return nil
}

// ── Helpers ─────────────────────────────────────────────────────

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func findManifestFile(files []ManifestFile, name string) *ManifestFile {
	for i, f := range files {
		if f.Dest == name || f.Src == name {
			return &files[i]
		}
	}
	return nil
}

// unifiedDiff produces a simple line-by-line unified diff.
func unifiedDiff(a, b, labelA, labelB string) string {
	linesA := strings.Split(a, "\n")
	linesB := strings.Split(b, "\n")

	if strings.Join(linesA, "\n") == strings.Join(linesB, "\n") {
		return ""
	}

	var out strings.Builder
	out.WriteString(fmt.Sprintf("--- %s\n", labelA))
	out.WriteString(fmt.Sprintf("+++ %s\n", labelB))

	n, m := len(linesA), len(linesB)

	// LCS-based diff
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if linesA[i] == linesB[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}

	i, j := 0, 0
	for i < n || j < m {
		if i < n && j < m && linesA[i] == linesB[j] {
			out.WriteString(fmt.Sprintf(" %s\n", linesA[i]))
			i++
			j++
		} else if j < m && (i >= n || dp[i][j+1] >= dp[i+1][j]) {
			out.WriteString(fmt.Sprintf("+%s\n", linesB[j]))
			j++
		} else if i < n {
			out.WriteString(fmt.Sprintf("-%s\n", linesA[i]))
			i++
		}
	}

	return out.String()
}
