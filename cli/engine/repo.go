package engine

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// RepoAdd installs manifest files into a project and creates the sidecar.
// If remoteVersions is non-nil, files already on disk at the matching remote
// version are skipped (delta install) to avoid unnecessary HTTP fetches.
func RepoAdd(manifests []model.Manifest, cfg model.Config, projectDir string, remoteVersions map[string]string) error {
	chosen := model.FindManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	repo := cfg.Repo
	branch := cfg.Branch
	if repo == "" {
		repo = storage.DefaultRepo
	}
	if branch == "" {
		branch = storage.DefaultBranch
	}

	files := model.SelectFiles(chosen.Files, *chosen, cfg.Tier)
	installed := make([]string, 0, len(files)+1)

	var regularFiles []model.ManifestFile
	var mcpFiles []model.ManifestFile
	for _, f := range files {
		cat := f.Category
		if cat == "" {
			cat = model.InferCategory(f.Src)
		}
		if cat == "mcp-servers" {
			mcpFiles = append(mcpFiles, f)
		} else {
			regularFiles = append(regularFiles, f)
		}
	}

	prevSidecar, _ := storage.ReadRepoSidecar(projectDir)
	var previousMcpNames []string
	if prevSidecar != nil {
		previousMcpNames = prevSidecar.McpServers
	}

	for _, f := range regularFiles {
		destRel := filepath.ToSlash(filepath.Join(".github", f.Dest))
		destPath := filepath.Join(projectDir, destRel)

		// Delta install: skip download if file on disk matches remote version.
		if remoteVersions != nil {
			srcPath := f.Src
			if chosen.BasePath != "" {
				srcPath = chosen.BasePath + "/" + f.Src
			}
			if rv := remoteVersions[srcPath]; rv != "" {
				if iv, err := storage.ReadFileVersion(destPath); err == nil && iv == rv {
					installed = append(installed, destRel)
					continue
				}
			}
		}

		if err := storage.WriteManifestFile(f, chosen.BasePath, destPath, repo, branch); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, err)
			continue
		}

		fmt.Printf("  ✓  %s\n", destRel)
		installed = append(installed, destRel)
	}

	var mcpServerNames []string
	if len(mcpFiles) > 0 || len(previousMcpNames) > 0 {
		names, err := storage.InjectMcpFromManifest(mcpFiles, chosen.BasePath, projectDir, previousMcpNames, repo, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  MCP config: %s\n", err)
		} else {
			mcpServerNames = names
			for _, name := range names {
				fmt.Printf("  ✓  MCP server: %s\n", name)
			}
		}
	}

	if err := storage.WriteRepoSidecar(projectDir, model.RepoSidecar{
		Manifest:   chosen.ID,
		Version:    chosen.Version,
		Tier:       cfg.Tier,
		Files:      installed,
		McpServers: mcpServerNames,
		Source:     repo + "@" + branch,
	}); err != nil {
		return err
	}

	// Cache the manifest for offline fallback
	_ = storage.WriteManifestCache(projectDir, manifests)

	_ = storage.WriteProjectConfig(projectDir, &model.Config{Manifest: chosen.ID, Tier: cfg.Tier})

	fmt.Printf("\nAdded %d managed files to repository.\n", len(installed))
	return nil
}

// RepoRemove removes all managed files and the sidecar.
func RepoRemove(projectDir string) error {
	sc, _ := storage.ReadRepoSidecar(projectDir)
	if sc == nil {
		fmt.Println("No managed repo sidecar found; nothing to remove.")
		return nil
	}
	count := len(sc.Files)
	if err := storage.DeleteRepoSidecar(projectDir); err != nil {
		return err
	}
	fmt.Printf("Removed %d managed files from repository.\n", count)
	return nil
}
