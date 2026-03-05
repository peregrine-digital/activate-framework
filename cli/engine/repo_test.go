package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

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

	basePath := "plugins/test"
	srcRel := "instructions/test.instructions.md"
	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/" + srcRel: "hello",
	})

	manifest := model.Manifest{
		ID:       "m1",
		Name:     "Manifest One",
		Version:  "1.0.0",
		BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: srcRel, Dest: srcRel, Tier: "core"},
		},
	}

	cfg := model.Config{Manifest: "m1", Tier: "minimal", Repo: repo, Branch: branch}
	if err := RepoAdd([]model.Manifest{manifest}, cfg, projectDir, nil); err != nil {
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

	if _, err := os.Stat(storage.SidecarPath(projectDir)); err != nil {
		t.Fatalf("expected repo sidecar written, err=%v", err)
	}

	projectCfgData, err := os.ReadFile(storage.ProjectConfigPath(projectDir))
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
	sc := model.RepoSidecar{
		Manifest: "m1",
		Version:  "1.0.0",
		Tier:     "minimal",
		Files:    []string{".github/instructions/a.md", ".github/agents/b.md"},
	}
	if err := storage.WriteRepoSidecar(projectDir, sc); err != nil {
		t.Fatal(err)
	}

	// Verify exclude block was written
	data, _ := os.ReadFile(excludePath)
	startMark := "# >>> Peregrine Activate (managed — do not edit)"
	if !strings.Contains(string(data), startMark) {
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
	if _, err := os.Stat(storage.SidecarPath(projectDir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar deleted, err=%v", err)
	}
	// Assert: git exclude block removed
	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), startMark) {
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
	sc := model.RepoSidecar{
		Manifest:   "m1",
		Version:    "1.0.0",
		Tier:       "minimal",
		Files:      []string{},
		McpServers: []string{"managed-server"},
	}
	scData, _ := json.Marshal(sc)
	scPath := storage.SidecarPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(scPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scPath, scData, 0644); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := storage.DeleteRepoSidecar(projectDir); err != nil {
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

	basePath := "plugins/test"
	mcpServerJSON := `{"test-mcp-server": {"command": "npx", "args": ["server"]}}`

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/instructions/setup.instructions.md": "instructions content",
		basePath + "/mcp-servers/my-server.json":         mcpServerJSON,
	})

	manifest := model.Manifest{
		ID:       "m1",
		Name:     "Test Manifest",
		Version:  "1.0.0",
		BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/setup.instructions.md", Dest: "instructions/setup.instructions.md", Tier: "core"},
			{Src: "mcp-servers/my-server.json", Dest: "mcp-servers/my-server.json", Tier: "core"},
		},
	}

	cfg := model.Config{Manifest: "m1", Tier: "minimal", Repo: repo, Branch: branch}
	if err := RepoAdd([]model.Manifest{manifest}, cfg, projectDir, nil); err != nil {
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
	scData, err := os.ReadFile(storage.SidecarPath(projectDir))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(scData), "test-mcp-server") {
		t.Fatalf("expected sidecar to track test-mcp-server, got:\n%s", string(scData))
	}
}

func TestRepoAddDeltaSkipsCurrentVersion(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	basePath := "plugins/test"
	srcRel := "instructions/test.instructions.md"
	localContent := "---\nversion: '1.0.0'\n---\nlocal copy"

	// Serve a DIFFERENT body so we can tell if it was re-fetched
	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/" + srcRel: "---\nversion: '1.0.0'\n---\nremote copy",
	})

	// Pre-install the file on disk at version 1.0.0
	destPath := filepath.Join(projectDir, ".github", srcRel)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte(localContent), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := model.Manifest{
		ID: "m1", Name: "Test", Version: "1.0.0", BasePath: basePath,
		Files: []model.ManifestFile{{Src: srcRel, Dest: srcRel, Tier: "core"}},
	}

	// Remote versions say file is at 1.0.0 — same as on disk
	remoteVersions := map[string]string{basePath + "/" + srcRel: "1.0.0"}

	cfg := model.Config{Manifest: "m1", Tier: "minimal", Repo: repo, Branch: branch}
	if err := RepoAdd([]model.Manifest{manifest}, cfg, projectDir, remoteVersions); err != nil {
		t.Fatal(err)
	}

	// File should still have LOCAL content (not re-fetched)
	data, _ := os.ReadFile(destPath)
	if string(data) != localContent {
		t.Fatalf("expected delta skip to preserve local copy, got: %q", string(data))
	}

	// File should still be tracked in sidecar
	sc, _ := storage.ReadRepoSidecar(projectDir)
	if sc == nil || len(sc.Files) != 1 {
		t.Fatalf("expected 1 file in sidecar, got %v", sc)
	}
}

func TestRepoAddDeltaRedownloadsStaleVersion(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	basePath := "plugins/test"
	srcRel := "instructions/test.instructions.md"
	newContent := "---\nversion: '2.0.0'\n---\nupdated"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/" + srcRel: newContent,
	})

	// Pre-install old version on disk
	destPath := filepath.Join(projectDir, ".github", srcRel)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("---\nversion: '1.0.0'\n---\nold"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := model.Manifest{
		ID: "m1", Name: "Test", Version: "2.0.0", BasePath: basePath,
		Files: []model.ManifestFile{{Src: srcRel, Dest: srcRel, Tier: "core"}},
	}

	remoteVersions := map[string]string{basePath + "/" + srcRel: "2.0.0"}

	cfg := model.Config{Manifest: "m1", Tier: "minimal", Repo: repo, Branch: branch}
	if err := RepoAdd([]model.Manifest{manifest}, cfg, projectDir, remoteVersions); err != nil {
		t.Fatal(err)
	}

	// File should be updated to new content
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "updated") {
		t.Fatalf("expected updated content, got: %s", string(data))
	}
}

func TestRepoAddDeltaDownloadsNewFile(t *testing.T) {
	projectDir := t.TempDir()
	setupTestStore(t)

	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	basePath := "plugins/test"
	srcRel := "instructions/new.instructions.md"
	content := "---\nversion: '1.0.0'\n---\nbrand new"

	_, repo, branch := serveRemoteFiles(t, map[string]string{
		basePath + "/" + srcRel: content,
	})

	manifest := model.Manifest{
		ID: "m1", Name: "Test", Version: "1.0.0", BasePath: basePath,
		Files: []model.ManifestFile{{Src: srcRel, Dest: srcRel, Tier: "core"}},
	}

	// Remote versions provided but file doesn't exist on disk
	remoteVersions := map[string]string{basePath + "/" + srcRel: "1.0.0"}

	cfg := model.Config{Manifest: "m1", Tier: "minimal", Repo: repo, Branch: branch}
	if err := RepoAdd([]model.Manifest{manifest}, cfg, projectDir, remoteVersions); err != nil {
		t.Fatal(err)
	}

	destPath := filepath.Join(projectDir, ".github", srcRel)
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "brand new") {
		t.Fatalf("expected new content, got: %s", string(data))
	}
}
