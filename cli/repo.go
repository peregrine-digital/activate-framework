package main

import (
	"fmt"
	"os"
	"path/filepath"
)

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

		if err := writeManifestFile(f, chosen.BasePath, destPath, useRemote, repo, branch); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, err)
			continue
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

	_ = WriteProjectConfig(projectDir, &Config{Manifest: chosen.ID, Tier: cfg.Tier})

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
