package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ── Update command ──────────────────────────────────────────────

// UpdateFiles re-installs only currently-tracked files, respecting skipped versions.
func UpdateFiles(m Manifest, sidecar *repoSidecar, cfg Config, projectDir string, useRemote bool, repo, branch string) (updated []string, skipped []string, err error) {
	if sidecar == nil {
		return nil, nil, fmt.Errorf("no sidecar found; nothing to update")
	}

	installedSet := make(map[string]bool)
	for _, f := range sidecar.Files {
		installedSet[f] = true
	}

	for _, f := range m.Files {
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

	// Update sidecar version
	sidecar.Version = m.Version
	if err := writeRepoSidecar(projectDir, *sidecar); err != nil {
		return updated, skipped, err
	}

	return updated, skipped, nil
}

func runUpdateCommand(manifests []Manifest, cfg Config, projectDir string, useRemote bool, repo, branch string, jsonOutput bool) error {
	chosen := findManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	sidecar, _ := readRepoSidecar(projectDir)
	if sidecar == nil {
		return fmt.Errorf("no installed files found; run 'repo add' first")
	}

	updated, skipped, err := UpdateFiles(*chosen, sidecar, cfg, projectDir, useRemote, repo, branch)
	if err != nil {
		return err
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{
			"updated": updated,
			"skipped": skipped,
		})
	}

	for _, f := range updated {
		fmt.Printf("  ✓  %s\n", f)
	}
	for _, f := range skipped {
		fmt.Printf("  ⊘  %s (skipped)\n", f)
	}
	fmt.Printf("\nUpdated %d files, skipped %d.\n", len(updated), len(skipped))
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

func runInstallFileCommand(manifests []Manifest, cfg Config, projectDir, file string, useRemote bool, repo, branch string, jsonOutput bool) error {
	chosen := findManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	target := findManifestFile(chosen.Files, file)
	if target == nil {
		return fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	if err := InstallSingleFile(*target, *chosen, projectDir, useRemote, repo, branch); err != nil {
		return err
	}

	// Clear skipped version on reinstall
	if _, ok := cfg.SkippedVersions[target.Dest]; ok {
		delete(cfg.SkippedVersions, target.Dest)
		_ = WriteProjectConfig(projectDir, &Config{SkippedVersions: cfg.SkippedVersions})
	}

	if jsonOutput {
		return printJSON(map[string]interface{}{"ok": true, "file": target.Dest})
	}
	fmt.Printf("  ✓  %s\n", target.Dest)
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

func runDiffCommand(manifests []Manifest, cfg Config, projectDir, file string) error {
	chosen := findManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	target := findManifestFile(chosen.Files, file)
	if target == nil {
		return fmt.Errorf("file %q not found in manifest %s", file, chosen.ID)
	}

	diff, err := DiffFile(*target, *chosen, projectDir)
	if err != nil {
		return err
	}

	if diff == "" {
		fmt.Println("Files are identical.")
	} else {
		fmt.Print(diff)
	}
	return nil
}

// ── File overrides in SelectFiles ───────────────────────────────

// SelectFilesWithOverrides filters manifest files by tier and applies file overrides.
// "pinned" files are always included regardless of tier.
// "excluded" files are always omitted.
func SelectFilesWithOverrides(files []ManifestFile, m Manifest, tierID string, overrides map[string]string) []ManifestFile {
	allowed := GetAllowedFileTiers(m, tierID)
	var result []ManifestFile
	for _, f := range files {
		ov := overrides[f.Dest]
		if ov == "excluded" {
			continue
		}
		if ov == "pinned" || allowed[f.Tier] {
			result = append(result, f)
		}
	}
	return result
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
