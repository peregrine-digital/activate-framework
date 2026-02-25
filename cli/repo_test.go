package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncRepoGitExcludeIfPresentSkipsWhenMissing(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	if err := syncRepoGitExcludeIfPresent(projectDir, []string{".github/test.md"}); err != nil {
		t.Fatal(err)
	}
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if _, err := os.Stat(excludePath); !os.IsNotExist(err) {
		t.Fatalf("expected exclude file to remain missing, err=%v", err)
	}
}

func TestWriteAndDeleteRepoSidecarLifecycle(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte("base-entry\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(projectDir, ".github", "old.md")
	keepFile := filepath.Join(projectDir, ".github", "keep.md")
	if err := os.MkdirAll(filepath.Dir(oldFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	prev := repoSidecar{Manifest: "m1", Version: "1", Tier: "minimal", Files: []string{".github/old.md", ".github/keep.md"}}
	prevData, _ := json.Marshal(prev)
	scPath := sidecarPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(scPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scPath, prevData, 0644); err != nil {
		t.Fatal(err)
	}

	next := repoSidecar{Manifest: "m1", Version: "1", Tier: "minimal", Files: []string{".github/keep.md"}}
	if err := writeRepoSidecar(projectDir, next); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be removed, err=%v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("expected keep file to remain, err=%v", err)
	}

	excludeData, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	excludeText := string(excludeData)
	if !strings.Contains(excludeText, repoExcludeStartMark) || !strings.Contains(excludeText, ".github/keep.md") {
		t.Fatalf("expected managed exclude block with keep path, got:\n%s", excludeText)
	}

	if err := deleteRepoSidecar(projectDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sidecarPath(projectDir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar deleted, err=%v", err)
	}
	if _, err := os.Stat(keepFile); !os.IsNotExist(err) {
		t.Fatalf("expected keep file deleted by sidecar cleanup, err=%v", err)
	}

	excludeData, err = os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	excludeText = string(excludeData)
	if strings.Contains(excludeText, repoExcludeStartMark) || strings.Contains(excludeText, repoExcludeEndMark) {
		t.Fatalf("expected managed exclude block removed, got:\n%s", excludeText)
	}
	if !strings.Contains(excludeText, "base-entry") {
		t.Fatalf("expected baseline exclude content preserved, got:\n%s", excludeText)
	}
}

func TestRepoAddLocalCopiesManagedFiles(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte("root\n"), 0644); err != nil {
		t.Fatal(err)
	}

	bundleDir := t.TempDir()
	srcRel := filepath.Join("instructions", "test.instructions.md")
	srcPath := filepath.Join(bundleDir, srcRel)
	if err := os.MkdirAll(filepath.Dir(srcPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		ID:       "m1",
		Name:     "Manifest One",
		Version:  "1.0.0",
		BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: srcRel, Dest: srcRel, Tier: "core"},
		},
	}

	cfg := Config{Manifest: "m1", Tier: "minimal"}
	if err := RepoAdd([]Manifest{manifest}, cfg, projectDir, false, "", ""); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(projectDir, ".github", srcRel)
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected copied content: %q", string(data))
	}

	if _, err := os.Stat(sidecarPath(projectDir)); err != nil {
		t.Fatalf("expected repo sidecar written, err=%v", err)
	}

	projectCfgData, err := os.ReadFile(projectConfigPath(projectDir))
	if err != nil {
		t.Fatalf("expected project config written, err=%v", err)
	}
	if !strings.Contains(string(projectCfgData), "\"manifest\": \"m1\"") || !strings.Contains(string(projectCfgData), "\"tier\": \"minimal\"") {
		t.Fatalf("unexpected project config content:\n%s", string(projectCfgData))
	}
}

func TestRepoRemoveCleanup(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	// Set up .git/info/exclude so the exclude block can be written
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte("baseline\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create tracked files
	fileA := filepath.Join(projectDir, ".github", "instructions", "a.md")
	fileB := filepath.Join(projectDir, ".github", "agents", "b.md")
	for _, f := range []string{fileA, fileB} {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Write sidecar with those files and an exclude block
	sc := repoSidecar{
		Manifest: "m1",
		Version:  "1.0.0",
		Tier:     "minimal",
		Files:    []string{".github/instructions/a.md", ".github/agents/b.md"},
	}
	if err := writeRepoSidecar(projectDir, sc); err != nil {
		t.Fatal(err)
	}

	// Verify exclude block was written
	data, _ := os.ReadFile(excludePath)
	if !strings.Contains(string(data), repoExcludeStartMark) {
		t.Fatal("expected exclude block to exist before removal")
	}

	// Act
	if err := RepoRemove(projectDir); err != nil {
		t.Fatal(err)
	}

	// Assert: tracked files deleted
	for _, f := range []string{fileA, fileB} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Fatalf("expected tracked file deleted: %s, err=%v", f, err)
		}
	}
	// Assert: sidecar deleted
	if _, err := os.Stat(sidecarPath(projectDir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar deleted, err=%v", err)
	}
	// Assert: git exclude block removed
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), repoExcludeStartMark) {
		t.Fatalf("expected exclude block removed, got:\n%s", string(data))
	}
	if !strings.Contains(string(data), "baseline") {
		t.Fatalf("expected baseline content preserved, got:\n%s", string(data))
	}
}

func TestRepoRemoveNoSidecar(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	// No sidecar file exists — should not error
	if err := RepoRemove(projectDir); err != nil {
		t.Fatalf("expected no error when sidecar is missing, got: %v", err)
	}
}

func TestDeleteRepoSidecarCleansMcpServers(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()

	// Set up .git/info/exclude so cleanup doesn't fail
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Write a .vscode/mcp.json with managed + unmanaged servers
	mcpPath := filepath.Join(projectDir, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(mcpPath), 0755); err != nil {
		t.Fatal(err)
	}
	mcpData := `{
  "servers": {
    "managed-server": {"command": "run-managed"},
    "user-server": {"command": "run-user"}
  }
}
`
	if err := os.WriteFile(mcpPath, []byte(mcpData), 0644); err != nil {
		t.Fatal(err)
	}

	// Write sidecar referencing managed-server
	sc := repoSidecar{
		Manifest:   "m1",
		Version:    "1.0.0",
		Tier:       "minimal",
		Files:      []string{},
		McpServers: []string{"managed-server"},
	}
	scData, _ := json.Marshal(sc)
	scPath := sidecarPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(scPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scPath, scData, 0644); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := deleteRepoSidecar(projectDir); err != nil {
		t.Fatal(err)
	}

	// Assert: managed server removed, user server preserved
	result, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(result), "managed-server") {
		t.Fatalf("expected managed-server removed from mcp.json, got:\n%s", string(result))
	}
	if !strings.Contains(string(result), "user-server") {
		t.Fatalf("expected user-server preserved in mcp.json, got:\n%s", string(result))
	}
}

func TestRepoAddFiltersMcpFiles(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	// Set up .git/info/exclude
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Create bundle directory with a regular file and an MCP server file
	bundleDir := t.TempDir()

	regularSrc := filepath.Join(bundleDir, "instructions", "setup.instructions.md")
	if err := os.MkdirAll(filepath.Dir(regularSrc), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(regularSrc, []byte("instructions content"), 0644); err != nil {
		t.Fatal(err)
	}

	mcpSrc := filepath.Join(bundleDir, "mcp-servers", "my-server.json")
	if err := os.MkdirAll(filepath.Dir(mcpSrc), 0755); err != nil {
		t.Fatal(err)
	}
	mcpServerJSON := `{"test-mcp-server": {"command": "npx", "args": ["server"]}}`
	if err := os.WriteFile(mcpSrc, []byte(mcpServerJSON), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		ID:       "m1",
		Name:     "Test Manifest",
		Version:  "1.0.0",
		BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: "instructions/setup.instructions.md", Dest: "instructions/setup.instructions.md", Tier: "core"},
			{Src: "mcp-servers/my-server.json", Dest: "mcp-servers/my-server.json", Tier: "core"},
		},
	}

	cfg := Config{Manifest: "m1", Tier: "minimal"}
	if err := RepoAdd([]Manifest{manifest}, cfg, projectDir, false, "", ""); err != nil {
		t.Fatal(err)
	}

	// Assert: regular file IS copied to .github/
	regularDest := filepath.Join(projectDir, ".github", "instructions", "setup.instructions.md")
	if _, err := os.Stat(regularDest); err != nil {
		t.Fatalf("expected regular file copied to .github/, err=%v", err)
	}

	// Assert: MCP server file is NOT copied to .github/
	mcpDest := filepath.Join(projectDir, ".github", "mcp-servers", "my-server.json")
	if _, err := os.Stat(mcpDest); !os.IsNotExist(err) {
		t.Fatalf("expected MCP file NOT copied to .github/, err=%v", err)
	}

	// Assert: MCP server entry IS in .vscode/mcp.json
	mcpCfgPath := filepath.Join(projectDir, ".vscode", "mcp.json")
	mcpCfgData, err := os.ReadFile(mcpCfgPath)
	if err != nil {
		t.Fatalf("expected .vscode/mcp.json to exist, err=%v", err)
	}
	if !strings.Contains(string(mcpCfgData), "test-mcp-server") {
		t.Fatalf("expected test-mcp-server in mcp.json, got:\n%s", string(mcpCfgData))
	}

	// Assert: sidecar tracks McpServers
	scData, err := os.ReadFile(sidecarPath(projectDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(scData), "test-mcp-server") {
		t.Fatalf("expected sidecar to track test-mcp-server, got:\n%s", string(scData))
	}
}

func TestWriteRepoSidecarDeletesStaleFiles(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()

	// Set up .git/info/exclude
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Create files a, b, c on disk
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		p := filepath.Join(projectDir, ".github", name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content-"+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Write initial sidecar with [a, b, c]
	first := repoSidecar{
		Manifest: "m1", Version: "1", Tier: "minimal",
		Files: []string{".github/a.md", ".github/b.md", ".github/c.md"},
	}
	if err := writeRepoSidecar(projectDir, first); err != nil {
		t.Fatal(err)
	}

	// All three should exist
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if _, err := os.Stat(filepath.Join(projectDir, ".github", name)); err != nil {
			t.Fatalf("expected %s to exist after first write, err=%v", name, err)
		}
	}

	// Write again with only [a, b]
	second := repoSidecar{
		Manifest: "m1", Version: "1", Tier: "minimal",
		Files: []string{".github/a.md", ".github/b.md"},
	}
	if err := writeRepoSidecar(projectDir, second); err != nil {
		t.Fatal(err)
	}

	// a and b should still exist
	for _, name := range []string{"a.md", "b.md"} {
		if _, err := os.Stat(filepath.Join(projectDir, ".github", name)); err != nil {
			t.Fatalf("expected %s to still exist, err=%v", name, err)
		}
	}
	// c should be deleted
	if _, err := os.Stat(filepath.Join(projectDir, ".github", "c.md")); !os.IsNotExist(err) {
		t.Fatalf("expected c.md to be deleted, err=%v", err)
	}
}
