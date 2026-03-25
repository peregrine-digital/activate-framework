package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
	"github.com/peregrine-digital/activate-framework/cli/transport"
)

// readRawMessage reads one Content-Length framed message and returns raw JSON bytes.
func readRawMessage(t *testing.T, r *bufio.Reader) json.RawMessage {
	t.Helper()
	contentLen := -1
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatalf("readRawMessage header: %v", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			n, err := strconv.Atoi(val)
			if err != nil {
				t.Fatalf("invalid Content-Length: %s", val)
			}
			contentLen = n
		}
	}
	if contentLen < 0 {
		t.Fatal("missing Content-Length header")
	}
	body := make([]byte, contentLen)
	if _, err := io.ReadFull(r, body); err != nil {
		t.Fatalf("readRawMessage body: %v", err)
	}
	return body
}

// writeRequest writes a Content-Length framed JSON-RPC request to w.
func writeRequest(t *testing.T, w io.Writer, method string, id int, params interface{}) {
	t.Helper()
	req := transport.Request{
		JSONRPC: "2.0",
		ID:      json.RawMessage(fmt.Sprintf("%d", id)),
		Method:  method,
	}
	if params != nil {
		p, err := json.Marshal(params)
		if err != nil {
			t.Fatalf("marshal params: %v", err)
		}
		req.Params = p
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		t.Fatalf("write header: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write body: %v", err)
	}
}

// sendRequest writes a JSON-RPC request and reads the response.
func sendRequest(t *testing.T, w io.Writer, reader *bufio.Reader, method string, id int, params interface{}) *transport.Response {
	t.Helper()
	writeRequest(t, w, method, id, params)

	body := readRawMessage(t, reader)
	var resp transport.Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return &resp
}

// readNotification reads one Content-Length framed message and parses it as a Notification.
func readNotification(t *testing.T, clientReader *bufio.Reader) *transport.Notification {
	t.Helper()
	body := readRawMessage(t, clientReader)
	var notif transport.Notification
	if err := json.Unmarshal(body, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	return &notif
}

// resultMap re-marshals resp.Result into a map for assertions.
func resultMap(t *testing.T, resp *transport.Response) map[string]interface{} {
	t.Helper()
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	data, err := json.Marshal(resp.Result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal result map: %v", err)
	}
	return m
}

// testHarness sets up temp dirs, a manifest, a service, pipes, and a daemon.
// Returns the client transport, client reader, a cleanup func, and the project dir.
type harness struct {
	clientWriter io.Writer
	clientReader *bufio.Reader
	projectDir   string
	manifest     model.Manifest
	remoteFiles  *mutableFiles
	cleanup      func()
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	homeDir := t.TempDir()
	old := storage.ActivateBaseDir
	storage.ActivateBaseDir = homeDir
	t.Cleanup(func() { storage.ActivateBaseDir = old })

	projectDir := t.TempDir()

	// Create .git/info/exclude so git exclude operations work
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	basePath := "plugins/test"

	// Serve source files from httptest server
	mf := &mutableFiles{files: map[string]string{
		basePath + "/instructions/setup.instructions.md": "---\nversion: '1.0.0'\n---\n# Setup Instructions\nHello world\n",
		basePath + "/prompts/build.prompt.md":            "---\nversion: '1.0.0'\n---\n# Build Prompt\nBuild it\n",
	}}
	_, repo, branch := serveRemoteMutableFiles(t, mf)

	// Write project config so refreshConfig resolves to our test manifest and test repo
	if err := storage.WriteProjectConfig(projectDir, &model.Config{
		Manifest: "test-manifest", Tier: "standard",
		Repo: repo, Branch: branch,
	}); err != nil {
		t.Fatal(err)
	}

	manifest := model.Manifest{
		ID:       "test-manifest",
		Name:     "Test Manifest",
		BasePath: basePath,
		Files: []model.ManifestFile{
			{Src: "instructions/setup.instructions.md", Dest: "instructions/setup.instructions.md", Tier: "core"},
			{Src: "prompts/build.prompt.md", Dest: "prompts/build.prompt.md", Tier: "core"},
		},
	}

	cfg := model.Config{Manifest: "test-manifest", Tier: "standard", Repo: repo, Branch: branch}
	svc := NewService(projectDir, []model.Manifest{manifest}, cfg)

	// Set up pipes: daemon reads from serverRead, writes to serverWrite
	// client reads from clientRead, writes to clientWrite
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	serverTransport := transport.NewTransport(serverRead, serverWrite)
	clientReader := bufio.NewReader(clientRead)

	daemon := NewDaemon(svc, serverTransport, "")

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Serve()
	}()

	cleanup := func() {
		clientWrite.Close()
		clientRead.Close()
		<-errCh
	}

	return &harness{
		clientWriter: clientWrite,
		clientReader: clientReader,
		projectDir:   projectDir,
		manifest:     manifest,
		remoteFiles:  mf,
		cleanup:      cleanup,
	}
}

func TestDaemonInitialize(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{
		ProjectDir: h.projectDir,
	})

	m := resultMap(t, resp)
	if _, ok := m["version"]; !ok {
		t.Error("expected version in result")
	}
	caps, ok := m["capabilities"]
	if !ok {
		t.Fatal("expected capabilities in result")
	}
	capsList, ok := caps.([]interface{})
	if !ok {
		t.Fatalf("capabilities type = %T, want []interface{}", caps)
	}
	if len(capsList) == 0 {
		t.Error("expected non-empty capabilities")
	}
}

func TestDaemonStateGet(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	// Initialize first
	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 2, nil)
	m := resultMap(t, resp)

	if _, ok := m["projectDir"]; !ok {
		t.Error("expected projectDir in state result")
	}
	if _, ok := m["state"]; !ok {
		t.Error("expected state in state result")
	}
	if _, ok := m["config"]; !ok {
		t.Error("expected config in state result")
	}
}

func TestDaemonConfigGetResolved(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigGet, 2, nil)
	m := resultMap(t, resp)

	if m["manifest"] != "test-manifest" {
		t.Errorf("manifest = %v, want test-manifest", m["manifest"])
	}
	if m["tier"] != "standard" {
		t.Errorf("tier = %v, want standard", m["tier"])
	}
}

func TestDaemonConfigSet(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Set tier to minimal
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigSet, 2, transport.ConfigSetParams{
		Tier: "minimal",
	})
	m := resultMap(t, resp)
	if m["ok"] != true {
		t.Errorf("expected ok=true, got %v", m["ok"])
	}
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	// Verify via configGet
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigGet, 3, transport.ConfigGetParams{Scope: "project"})
	m2 := resultMap(t, resp2)
	if m2["tier"] != "minimal" {
		t.Errorf("tier after set = %v, want minimal", m2["tier"])
	}
}

func TestDaemonManifestList(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodManifestList, 2, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var manifests []map[string]interface{}
	if err := json.Unmarshal(data, &manifests); err != nil {
		t.Fatalf("unmarshal manifest list: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
	if manifests[0]["id"] != "test-manifest" {
		t.Errorf("manifest id = %v, want test-manifest", manifests[0]["id"])
	}
}

func TestDaemonManifestFiles(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodManifestFiles, 2, transport.ManifestFilesParams{
		Manifest: "test-manifest",
	})
	m := resultMap(t, resp)

	if m["manifest"] != "test-manifest" {
		t.Errorf("manifest = %v, want test-manifest", m["manifest"])
	}

	cats, ok := m["categories"].([]interface{})
	if !ok {
		t.Fatalf("categories type = %T, want []interface{}", m["categories"])
	}
	if len(cats) == 0 {
		t.Error("expected at least one category")
	}

	totalFiles, ok := m["totalFiles"].(float64)
	if !ok || totalFiles < 1 {
		t.Errorf("totalFiles = %v, want >= 1", m["totalFiles"])
	}
}

func TestDaemonRepoAddAndRemove(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Repo add
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 2, nil)
	m := resultMap(t, resp)
	if m["manifest"] != "test-manifest" {
		t.Errorf("manifest = %v, want test-manifest", m["manifest"])
	}
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	// Verify files installed
	destPath := filepath.Join(h.projectDir, ".github", "instructions", "setup.instructions.md")
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected installed file, err=%v", err)
	}

	// Repo remove
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoRemove, 3, nil)
	m2 := resultMap(t, resp2)
	if m2["ok"] != true {
		t.Errorf("expected ok=true, got %v", m2["ok"])
	}
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	// Verify files removed
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("expected file removed after repo remove, err=%v", err)
	}
}

// TestDaemonRepoAddRemoveAddRoundTrip tests the full lifecycle over RPC:
// add → verify state → remove → verify state → add again → verify state.
// This catches stale daemon state after remove operations.
func TestDaemonRepoAddRemoveAddRoundTrip(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	destPath := filepath.Join(h.projectDir, ".github", "instructions", "setup.instructions.md")

	// 1. Add
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 2, nil)
	if resp.Error != nil {
		t.Fatalf("first repoAdd error: %v", resp.Error)
	}
	readNotification(t, h.clientReader)

	if _, err := os.Stat(destPath); err != nil {
		t.Fatal("file should exist after first add")
	}

	// Verify state shows installed
	state1Resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 3, nil)
	state1 := resultMap(t, state1Resp)
	stateObj1, _ := state1["state"].(map[string]interface{})
	if stateObj1["hasInstallMarker"] != true {
		t.Fatalf("expected hasInstallMarker=true after add, got %v", stateObj1["hasInstallMarker"])
	}

	// 2. Remove
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoRemove, 4, nil)
	if resp2.Error != nil {
		t.Fatalf("repoRemove error: %v", resp2.Error)
	}
	readNotification(t, h.clientReader)

	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatal("file should not exist after remove")
	}

	// Verify state shows not installed
	state2Resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 5, nil)
	state2 := resultMap(t, state2Resp)
	stateObj2, _ := state2["state"].(map[string]interface{})
	if stateObj2["hasInstallMarker"] != false {
		t.Fatalf("expected hasInstallMarker=false after remove, got %v", stateObj2["hasInstallMarker"])
	}

	// 3. Add again — this is the critical test
	resp3 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 6, nil)
	if resp3.Error != nil {
		t.Fatalf("second repoAdd error: %v", resp3.Error)
	}
	readNotification(t, h.clientReader)

	if _, err := os.Stat(destPath); err != nil {
		t.Fatal("file should exist after second add")
	}

	// Verify state shows installed again
	state3Resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 7, nil)
	state3 := resultMap(t, state3Resp)
	stateObj3, _ := state3["state"].(map[string]interface{})
	if stateObj3["hasInstallMarker"] != true {
		t.Fatalf("expected hasInstallMarker=true after second add, got %v", stateObj3["hasInstallMarker"])
	}
	// Verify files are populated
	files3, _ := state3["files"].([]interface{})
	if len(files3) == 0 {
		t.Fatal("expected files in state after second add")
	}
}

// TestDaemonSetConfigUpdatesState verifies that changing config via RPC
// is reflected in subsequent state queries.
func TestDaemonSetConfigUpdatesState(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Get initial state
	state1Resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 2, nil)
	state1 := resultMap(t, state1Resp)
	config1, _ := state1["config"].(map[string]interface{})
	if config1["tier"] != "standard" {
		t.Fatalf("initial tier = %v, want standard", config1["tier"])
	}

	// Change tier
	setResp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigSet, 3, transport.ConfigSetParams{
		Scope:   "project",
		Updates: &model.Config{Tier: "core"},
	})
	if setResp.Error != nil {
		t.Fatalf("setConfig error: %v", setResp.Error)
	}
	readNotification(t, h.clientReader)

	// Verify new state reflects the change
	state2Resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 4, nil)
	state2 := resultMap(t, state2Resp)
	config2, _ := state2["config"].(map[string]interface{})
	if config2["tier"] != "core" {
		t.Fatalf("tier after set = %v, want core", config2["tier"])
	}
}

func TestDaemonFileInstallAndUninstall(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// First repo add to create sidecar
	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Uninstall a specific file
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileUninstall, 3, transport.FileParams{
		File: "instructions/setup.instructions.md",
	})
	m := resultMap(t, resp)
	if m["ok"] != true {
		t.Errorf("expected ok=true for uninstall, got %v", m["ok"])
	}
	readNotification(t, h.clientReader)

	destPath := filepath.Join(h.projectDir, ".github", "instructions", "setup.instructions.md")
	if _, err := os.Stat(destPath); !os.IsNotExist(err) {
		t.Fatalf("expected file uninstalled, err=%v", err)
	}

	// Re-install the file
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileInstall, 4, transport.FileParams{
		File: "instructions/setup.instructions.md",
	})
	m2 := resultMap(t, resp2)
	if m2["ok"] != true {
		t.Errorf("expected ok=true for install, got %v", m2["ok"])
	}
	readNotification(t, h.clientReader)

	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected file re-installed, err=%v", err)
	}
}

func TestDaemonSync(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Repo add first
	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Sync should find nothing to update since versions match
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodSync, 3, nil)
	m := resultMap(t, resp)
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	if m["action"] != "updated" {
		t.Errorf("sync action = %v, want 'updated'", m["action"])
	}
}

func TestDaemonFileDiff(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Repo add
	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Modify the installed file
	destPath := filepath.Join(h.projectDir, ".github", "instructions", "setup.instructions.md")
	if err := os.WriteFile(destPath, []byte("---\nversion: '1.0.0'\n---\n# Modified\nChanged content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileDiff, 3, transport.FileParams{
		File: "instructions/setup.instructions.md",
	})
	m := resultMap(t, resp)

	if m["identical"] == true {
		t.Error("expected files to differ after modification")
	}
	diff, _ := m["diff"].(string)
	if diff == "" {
		t.Error("expected non-empty diff")
	}
}

func TestDaemonFileOverride(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Set override to pinned
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileOverride, 2, transport.FileOverrideParams{
		File:     "instructions/setup.instructions.md",
		Override: "pinned",
	})
	m := resultMap(t, resp)
	if m["ok"] != true {
		t.Errorf("expected ok=true, got %v", m["ok"])
	}
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	// Verify via configGet (project scope)
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigGet, 3, transport.ConfigGetParams{Scope: "project"})
	m2 := resultMap(t, resp2)

	overrides, ok := m2["fileOverrides"].(map[string]interface{})
	if !ok {
		t.Fatalf("fileOverrides type = %T, want map", m2["fileOverrides"])
	}
	if overrides["instructions/setup.instructions.md"] != "pinned" {
		t.Errorf("override = %v, want pinned", overrides["instructions/setup.instructions.md"])
	}
}

func TestDaemonMethodNotFound(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	resp := sendRequest(t, h.clientWriter, h.clientReader, "activate/nonexistent", 1, nil)

	if resp.Error == nil {
		t.Fatal("expected error for unknown method")
	}
	if resp.Error.Code != transport.ErrCodeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, transport.ErrCodeMethodNotFound)
	}
}

func TestDaemonInvalidParams(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// fileInstall with no params (nil Params → unmarshal error)
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileInstall, 2, nil)

	if resp.Error == nil {
		t.Fatal("expected error for missing params on fileInstall")
	}
	if resp.Error.Code != transport.ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, transport.ErrCodeInvalidParams)
	}
}

func TestDaemonStateChangedNotification(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// configSet is mutating: expect response + notification
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigSet, 2, transport.ConfigSetParams{
		Tier: "advanced",
	})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	notif := readNotification(t, h.clientReader)
	if notif.Method != "activate/stateChanged" {
		t.Errorf("notification method = %q, want activate/stateChanged", notif.Method)
	}
}

func TestDaemonFileSkip(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Skip update for an existing file
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileSkip, 2, transport.FileParams{
		File: "instructions/setup.instructions.md",
	})
	m := resultMap(t, resp)
	if m["ok"] != true {
		t.Errorf("expected ok=true, got %v", m["ok"])
	}
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	// Verify skipped version persisted via configGet
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodConfigGet, 3, transport.ConfigGetParams{Scope: "project"})
	m2 := resultMap(t, resp2)
	skipped, ok := m2["skippedVersions"].(map[string]interface{})
	if !ok {
		t.Fatalf("skippedVersions type = %T, want map", m2["skippedVersions"])
	}
	if skipped["instructions/setup.instructions.md"] != "1.0.0" {
		t.Errorf("skipped version = %v, want 1.0.0", skipped["instructions/setup.instructions.md"])
	}
}

func TestDaemonFileSkipNotFound(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodFileSkip, 2, transport.FileParams{
		File: "nonexistent/file.md",
	})
	if resp.Error == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestDaemonUpdate(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Install files first via repoAdd
	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Update — files are already current, so updated should be empty
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodUpdate, 3, nil)
	m := resultMap(t, resp)
	readNotification(t, h.clientReader)

	updated, _ := m["updated"].([]interface{})
	// Update re-copies all tracked files
	if len(updated) != 2 {
		t.Errorf("expected 2 updated files, got %d", len(updated))
	}
}

func TestDaemonTelemetryLog(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Read telemetry log (empty since no telemetry has been run)
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodTelemetryLog, 2, nil)
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	// Result should be null or empty array
	data, _ := json.Marshal(resp.Result)
	var entries []interface{}
	if err := json.Unmarshal(data, &entries); err != nil {
		// null is valid (no log file)
		if string(data) != "null" {
			t.Fatalf("unexpected result: %s", string(data))
		}
	}
}

// newEmptyHarness creates a daemon with NO manifests, simulating what happens
// when `serve --stdio` starts before manifest discovery (the real main.go path).
// This catches regressions where the daemon crashes or blocks without manifests.
func newEmptyHarness(t *testing.T) *harness {
	t.Helper()

	homeDir := t.TempDir()
	old := storage.ActivateBaseDir
	storage.ActivateBaseDir = homeDir
	t.Cleanup(func() { storage.ActivateBaseDir = old })

	projectDir := t.TempDir()

	// No config, no manifests, no bundle — bare minimum
	svc := NewService("", nil, model.Config{})

	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	serverTransport := transport.NewTransport(serverRead, serverWrite)
	clientReader := bufio.NewReader(clientRead)

	daemon := NewDaemon(svc, serverTransport, "1.0.0-test")

	errCh := make(chan error, 1)
	go func() {
		errCh <- daemon.Serve()
	}()

	cleanup := func() {
		clientWrite.Close()
		clientRead.Close()
		<-errCh
	}

	return &harness{
		clientWriter: clientWrite,
		clientReader: clientReader,
		projectDir:   projectDir,
		cleanup:      cleanup,
	}
}

// TestDaemonServeWithoutManifests verifies that the daemon starts and responds
// to initialize even when no manifests are available. This mirrors main.go where
// `serve` is dispatched BEFORE manifest discovery — manifests are loaded lazily
// during initialize when a projectDir is provided.
func TestDaemonServeWithoutManifests(t *testing.T) {
	h := newEmptyHarness(t)
	defer h.cleanup()

	// Initialize with no projectDir — daemon should still respond
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, nil)
	m := resultMap(t, resp)

	// Version should come through
	if m["version"] != "1.0.0-test" {
		t.Errorf("version = %v, want 1.0.0-test", m["version"])
	}
	// Capabilities should be present
	caps, ok := m["capabilities"].([]interface{})
	if !ok || len(caps) == 0 {
		t.Error("expected non-empty capabilities even without manifests")
	}
}

// TestDaemonNoManifestsStateGet verifies that stateGet works without manifests,
// returning an empty/default state rather than crashing.
func TestDaemonNoManifestsStateGet(t *testing.T) {
	h := newEmptyHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, nil)

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 2, nil)
	// Should succeed (not crash) even without manifests
	if resp.Error != nil {
		t.Fatalf("stateGet should not error without manifests: %v", resp.Error)
	}
}

// TestDaemonNoManifestsManifestList verifies that listing manifests returns
// an empty list (not an error or crash) when no manifests are available.
func TestDaemonNoManifestsManifestList(t *testing.T) {
	h := newEmptyHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, nil)

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodManifestList, 2, nil)
	if resp.Error != nil {
		t.Fatalf("manifestList should not error without manifests: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var manifests []interface{}
	if err := json.Unmarshal(data, &manifests); err != nil {
		// null is acceptable
		if string(data) != "null" {
			t.Fatalf("unexpected result: %s", string(data))
		}
		return
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0 manifests, got %d", len(manifests))
	}
}

// TestDaemonServeBeforeDiscoveryOrder verifies the contract that main.go relies on:
// the daemon can accept requests, then initialize with a projectDir later.
func TestDaemonServeBeforeDiscoveryOrder(t *testing.T) {
	h := newEmptyHarness(t)
	defer h.cleanup()

	// 1. Initialize with no projectDir (daemon starts before manifests are known)
	resp1 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, nil)
	if resp1.Error != nil {
		t.Fatalf("empty initialize failed: %v", resp1.Error)
	}

	// 2. stateGet should work (empty state)
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 2, nil)
	if resp2.Error != nil {
		t.Fatalf("stateGet after empty init failed: %v", resp2.Error)
	}

	// 3. Re-initialize with a projectDir (simulates extension sending initialize)
	resp3 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 3,
		transport.InitializeParams{ProjectDir: h.projectDir})
	if resp3.Error != nil {
		t.Fatalf("re-initialize with projectDir failed: %v", resp3.Error)
	}

	// 4. stateGet should now reflect the projectDir
	resp4 := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodStateGet, 4, nil)
	m := resultMap(t, resp4)
	if m["projectDir"] != h.projectDir {
		t.Errorf("projectDir = %v, want %v", m["projectDir"], h.projectDir)
	}
}

func TestDaemonTelemetryRunDisabled(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	// Telemetry is disabled by default — should return error
	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodTelemetryRun, 2, transport.TelemetryRunParams{})
	if resp.Error == nil {
		t.Fatal("expected error when telemetry is disabled")
	}
}

func TestDaemonBranchList(t *testing.T) {
	// Set up a mock API server that returns branches
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/repos/") && strings.Contains(r.URL.Path, "/branches") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode([]map[string]string{
				{"name": "main"},
				{"name": "develop"},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer api.Close()
	origAPI := storage.APIBase
	storage.APIBase = api.URL
	t.Cleanup(func() { storage.APIBase = origAPI })

	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, transport.MethodInitialize, 1, transport.InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, transport.MethodBranchList, 2, transport.BranchListParams{})
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var branches []string
	if err := json.Unmarshal(data, &branches); err != nil {
		t.Fatalf("unmarshal branches: %v", err)
	}
	if len(branches) != 2 || branches[0] != "main" || branches[1] != "develop" {
		t.Fatalf("unexpected branches: %v", branches)
	}
}
