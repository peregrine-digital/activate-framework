package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

func TestReadMcpConfigMissing(t *testing.T) {
	cfg, err := ReadMcpConfig(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Servers == nil || len(cfg.Servers) != 0 {
		t.Fatalf("expected empty servers map, got %v", cfg.Servers)
	}
}

func TestReadWriteMcpConfig(t *testing.T) {
	dir := t.TempDir()

	cfg := &McpConfig{
		Servers: map[string]json.RawMessage{
			"fetch": json.RawMessage(`{"type":"stdio","command":"npx"}`),
		},
	}
	if err := WriteMcpConfig(dir, cfg); err != nil {
		t.Fatal(err)
	}

	read, err := ReadMcpConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(read.Servers))
	}
	// JSON roundtrip may reformat, so unmarshal and compare
	var fetchCfg map[string]interface{}
	if err := json.Unmarshal(read.Servers["fetch"], &fetchCfg); err != nil {
		t.Fatal(err)
	}
	if fetchCfg["type"] != "stdio" || fetchCfg["command"] != "npx" {
		t.Fatalf("unexpected server config: %v", fetchCfg)
	}
}

func TestLoadMcpServerConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fetch.json")
	os.WriteFile(path, []byte(`{"fetch":{"type":"stdio","command":"npx","args":["-y","@anthropic-ai/mcp-server-fetch"]}}`), 0644)

	servers, err := LoadMcpServerConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if _, ok := servers["fetch"]; !ok {
		t.Fatal("expected 'fetch' server")
	}
}

func TestLoadMcpServerConfigInvalid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	os.WriteFile(path, []byte(`not json`), 0644)

	_, err := LoadMcpServerConfig(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestMergeMcpServers(t *testing.T) {
	dir := t.TempDir()

	// Pre-existing user server
	os.MkdirAll(filepath.Join(dir, ".vscode"), 0755)
	os.WriteFile(filepath.Join(dir, mcpConfigRel), []byte(`{"servers":{"user-server":{"type":"stdio","command":"my-tool"}}}`), 0644)

	managed := map[string]json.RawMessage{
		"fetch":  json.RawMessage(`{"type":"stdio","command":"npx"}`),
		"github": json.RawMessage(`{"type":"stdio","command":"npx"}`),
	}

	injected, err := MergeMcpServers(dir, managed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(injected) != 2 {
		t.Fatalf("expected 2 injected, got %d", len(injected))
	}

	// Verify user server preserved
	cfg, _ := ReadMcpConfig(dir)
	if _, ok := cfg.Servers["user-server"]; !ok {
		t.Fatal("expected user-server preserved")
	}
	if _, ok := cfg.Servers["fetch"]; !ok {
		t.Fatal("expected fetch server added")
	}
	if _, ok := cfg.Servers["github"]; !ok {
		t.Fatal("expected github server added")
	}
}

func TestMergeMcpServersRemovesStale(t *testing.T) {
	dir := t.TempDir()

	// Previous state: fetch + github managed
	os.MkdirAll(filepath.Join(dir, ".vscode"), 0755)
	os.WriteFile(filepath.Join(dir, mcpConfigRel), []byte(`{
		"servers": {
			"fetch":  {"type":"stdio","command":"npx"},
			"github": {"type":"stdio","command":"npx"},
			"user":   {"type":"stdio","command":"custom"}
		}
	}`), 0644)

	// New state: only fetch (github removed from manifest)
	managed := map[string]json.RawMessage{
		"fetch": json.RawMessage(`{"type":"stdio","command":"npx-v2"}`),
	}
	previousNames := []string{"fetch", "github"}

	_, err := MergeMcpServers(dir, managed, previousNames)
	if err != nil {
		t.Fatal(err)
	}

	cfg, _ := ReadMcpConfig(dir)
	if _, ok := cfg.Servers["github"]; ok {
		t.Fatal("expected github server removed (stale)")
	}
	if _, ok := cfg.Servers["fetch"]; !ok {
		t.Fatal("expected fetch server still present")
	}
	if _, ok := cfg.Servers["user"]; !ok {
		t.Fatal("expected user server preserved")
	}
}

func TestRemoveMcpServers(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".vscode"), 0755)
	os.WriteFile(filepath.Join(dir, mcpConfigRel), []byte(`{"servers":{"fetch":{"type":"stdio"},"github":{"type":"stdio"},"user":{"type":"stdio"}}}`), 0644)

	if err := RemoveMcpServers(dir, []string{"fetch", "github"}); err != nil {
		t.Fatal(err)
	}

	cfg, _ := ReadMcpConfig(dir)
	if len(cfg.Servers) != 1 {
		t.Fatalf("expected 1 server remaining, got %d", len(cfg.Servers))
	}
	if _, ok := cfg.Servers["user"]; !ok {
		t.Fatal("expected user server preserved")
	}
}

func TestInjectMcpFromManifest(t *testing.T) {
	projectDir := t.TempDir()
	basePath := "plugins/test"

	mcpContent := `{"fetch":{"type":"stdio","command":"npx","args":["-y","@anthropic-ai/mcp-server-fetch"]}}`

	// Set up httptest server
	repo := "test/repo"
	branch := "main"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + repo + "/" + branch + "/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			key := strings.TrimPrefix(r.URL.Path, prefix)
			if key == basePath+"/mcp-servers/fetch.json" {
				w.Write([]byte(mcpContent))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer raw.Close()
	origRaw := RawBase
	origResolver := TokenResolver
	RawBase = raw.URL
	TokenResolver = func() string { return "" }
	ResetTokenCache()
	defer func() {
		RawBase = origRaw
		TokenResolver = origResolver
		ResetTokenCache()
	}()

	files := []model.ManifestFile{
		{Src: "mcp-servers/fetch.json", Dest: "mcp-servers/fetch.json", Tier: "core", Category: "mcp-servers"},
		{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
	}

	injected, err := InjectMcpFromManifest(files, basePath, projectDir, nil, repo, branch)
	if err != nil {
		t.Fatal(err)
	}
	if len(injected) != 1 || injected[0] != "fetch" {
		t.Fatalf("expected [fetch], got %v", injected)
	}

	cfg, _ := ReadMcpConfig(projectDir)
	if _, ok := cfg.Servers["fetch"]; !ok {
		t.Fatal("expected fetch server in mcp.json")
	}
}

func TestInjectMcpFromManifestNoMcpFiles(t *testing.T) {
	files := []model.ManifestFile{
		{Src: "instructions/general.md", Dest: "instructions/general.md", Tier: "core"},
	}

	injected, err := InjectMcpFromManifest(files, t.TempDir(), t.TempDir(), nil, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if injected != nil {
		t.Fatalf("expected nil, got %v", injected)
	}
}

func TestSidecarMcpServersField(t *testing.T) {
	projectDir := t.TempDir()
	excludeDir := filepath.Join(projectDir, ".git", "info")
	os.MkdirAll(excludeDir, 0755)
	os.WriteFile(filepath.Join(excludeDir, "exclude"), []byte(""), 0644)

	sc := model.RepoSidecar{
		Manifest:   "m1",
		Version:    "1.0.0",
		Tier:       "standard",
		Files:      []string{".github/a.md"},
		McpServers: []string{"fetch", "github"},
	}
	if err := WriteRepoSidecar(projectDir, sc); err != nil {
		t.Fatal(err)
	}

	read, _ := ReadRepoSidecar(projectDir)
	if read == nil {
		t.Fatal("expected sidecar")
	}
	if len(read.McpServers) != 2 {
		t.Fatalf("expected 2 mcp servers in sidecar, got %d", len(read.McpServers))
	}
}
