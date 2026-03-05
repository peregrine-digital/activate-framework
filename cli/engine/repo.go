package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// RepoAdd installs manifest files into a project and creates the sidecar.
// If contentCache is non-nil, files are written from cached bytes (no HTTP).
// Files already on disk at the matching cached version are skipped (delta).
func RepoAdd(manifests []model.Manifest, cfg model.Config, projectDir string, contentCache map[string][]byte) error {
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

	// Separate files into delta-skipped (already current) and needing write.
	type writeJob struct {
		file     model.ManifestFile
		destRel  string
		destPath string
		srcPath  string
	}
	var toWrite []writeJob

	for _, f := range regularFiles {
		destRel := filepath.ToSlash(filepath.Join(".github", f.Dest))
		destPath := filepath.Join(projectDir, destRel)

		srcPath := f.Src
		if chosen.BasePath != "" {
			srcPath = chosen.BasePath + "/" + f.Src
		}

		// Delta: skip if file on disk matches cached remote version.
		if contentCache != nil {
			if cached, ok := contentCache[srcPath]; ok {
				rv := model.ParseFrontmatterVersion(cached)
				if rv != "" {
					if iv, err := storage.ReadFileVersion(destPath); err == nil && iv == rv {
						installed = append(installed, destRel)
						continue
					}
				}
			}
		}

		toWrite = append(toWrite, writeJob{file: f, destRel: destRel, destPath: destPath, srcPath: srcPath})
	}

	// Write files — from cache if available, otherwise fetch from GitHub.
	// Uncached files are fetched concurrently.
	type writeResult struct {
		destRel string
		err     error
	}

	// Split into cached (instant) and uncached (need HTTP).
	var uncachedJobs []writeJob
	var cachedResults []writeResult
	for _, j := range toWrite {
		if contentCache != nil {
			if data, ok := contentCache[j.srcPath]; ok {
				if err := os.MkdirAll(filepath.Dir(j.destPath), 0755); err != nil {
					cachedResults = append(cachedResults, writeResult{destRel: j.destRel, err: err})
					continue
				}
				err := os.WriteFile(j.destPath, data, 0644)
				cachedResults = append(cachedResults, writeResult{destRel: j.destRel, err: err})
				continue
			}
		}
		uncachedJobs = append(uncachedJobs, j)
	}

	// Process cached writes.
	for _, r := range cachedResults {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", r.destRel, r.err)
			continue
		}
		fmt.Printf("  ✓  %s\n", r.destRel)
		installed = append(installed, r.destRel)
	}

	// Fetch uncached files concurrently (bounded to 8 workers).
	if len(uncachedJobs) > 0 {
		results := make([]writeResult, len(uncachedJobs))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 8)
		for i, job := range uncachedJobs {
			wg.Add(1)
			go func(idx int, j writeJob) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				err := storage.WriteManifestFile(j.file, chosen.BasePath, j.destPath, repo, branch)
				results[idx] = writeResult{destRel: j.destRel, err: err}
			}(i, job)
		}
		wg.Wait()

		for _, r := range results {
			if r.err != nil {
				fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", r.destRel, r.err)
				continue
			}
			fmt.Printf("  ✓  %s\n", r.destRel)
			installed = append(installed, r.destRel)
		}
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
