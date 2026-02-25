package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
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
	req := Request{
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
func sendRequest(t *testing.T, w io.Writer, reader *bufio.Reader, method string, id int, params interface{}) *Response {
	t.Helper()
	writeRequest(t, w, method, id, params)

	body := readRawMessage(t, reader)
	var resp Response
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	return &resp
}

// readNotification reads one Content-Length framed message and parses it as a Notification.
func readNotification(t *testing.T, clientReader *bufio.Reader) *Notification {
	t.Helper()
	body := readRawMessage(t, clientReader)
	var notif Notification
	if err := json.Unmarshal(body, &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	return &notif
}

// resultMap re-marshals resp.Result into a map for assertions.
func resultMap(t *testing.T, resp *Response) map[string]interface{} {
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
	bundleDir    string
	manifest     Manifest
	cleanup      func()
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	projectDir := t.TempDir()
	bundleDir := t.TempDir()

	// Create .git/info/exclude so git exclude operations work
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Write project config so refreshConfig resolves to our test manifest
	if err := WriteProjectConfig(projectDir, &Config{Manifest: "test-manifest", Tier: "standard"}); err != nil {
		t.Fatal(err)
	}

	// Create source files with frontmatter versions
	srcFiles := map[string]string{
		"instructions/setup.instructions.md": "---\nversion: '1.0.0'\n---\n# Setup Instructions\nHello world\n",
		"prompts/build.prompt.md":            "---\nversion: '1.0.0'\n---\n# Build Prompt\nBuild it\n",
	}
	for rel, content := range srcFiles {
		p := filepath.Join(bundleDir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	manifest := Manifest{
		ID:       "test-manifest",
		Name:     "Test Manifest",
		Version:  "1.0.0",
		BasePath: bundleDir,
		Files: []ManifestFile{
			{Src: "instructions/setup.instructions.md", Dest: "instructions/setup.instructions.md", Tier: "core"},
			{Src: "prompts/build.prompt.md", Dest: "prompts/build.prompt.md", Tier: "core"},
		},
	}

	cfg := Config{Manifest: "test-manifest", Tier: "standard"}
	svc := NewService(projectDir, []Manifest{manifest}, cfg, false, "", "")

	// Set up pipes: daemon reads from serverRead, writes to serverWrite
	// client reads from clientRead, writes to clientWrite
	clientRead, serverWrite := io.Pipe()
	serverRead, clientWrite := io.Pipe()

	serverTransport := NewTransport(serverRead, serverWrite)
	clientReader := bufio.NewReader(clientRead)

	daemon := NewDaemon(svc, serverTransport)

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
		bundleDir:    bundleDir,
		manifest:     manifest,
		cleanup:      cleanup,
	}
}

func TestDaemonInitialize(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{
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
	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodStateGet, 2, nil)
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

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodConfigGet, 2, nil)
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

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// Set tier to minimal
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodConfigSet, 2, ConfigSetParams{
		Tier: "minimal",
	})
	m := resultMap(t, resp)
	if m["ok"] != true {
		t.Errorf("expected ok=true, got %v", m["ok"])
	}
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	// Verify via configGet
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, MethodConfigGet, 3, ConfigGetParams{Scope: "project"})
	m2 := resultMap(t, resp2)
	if m2["tier"] != "minimal" {
		t.Errorf("tier after set = %v, want minimal", m2["tier"])
	}
}

func TestDaemonManifestList(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodManifestList, 2, nil)
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
	if manifests[0]["ID"] != "test-manifest" {
		t.Errorf("manifest id = %v, want test-manifest", manifests[0]["ID"])
	}
}

func TestDaemonManifestFiles(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodManifestFiles, 2, ManifestFilesParams{
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

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// Repo add
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodRepoAdd, 2, nil)
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
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, MethodRepoRemove, 3, nil)
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

func TestDaemonFileInstallAndUninstall(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// First repo add to create sidecar
	sendRequest(t, h.clientWriter, h.clientReader, MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Uninstall a specific file
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodFileUninstall, 3, FileParams{
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
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, MethodFileInstall, 4, FileParams{
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

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// Repo add first
	sendRequest(t, h.clientWriter, h.clientReader, MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Sync should say up to date since versions match
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodSync, 3, nil)
	m := resultMap(t, resp)
	// Consume state-changed notification
	readNotification(t, h.clientReader)

	if m["reason"] != "up to date" {
		t.Errorf("sync reason = %v, want 'up to date'", m["reason"])
	}
}

func TestDaemonFileDiff(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// Repo add
	sendRequest(t, h.clientWriter, h.clientReader, MethodRepoAdd, 2, nil)
	readNotification(t, h.clientReader)

	// Modify the installed file
	destPath := filepath.Join(h.projectDir, ".github", "instructions", "setup.instructions.md")
	if err := os.WriteFile(destPath, []byte("---\nversion: '1.0.0'\n---\n# Modified\nChanged content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodFileDiff, 3, FileParams{
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

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// Set override to pinned
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodFileOverride, 2, FileOverrideParams{
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
	resp2 := sendRequest(t, h.clientWriter, h.clientReader, MethodConfigGet, 3, ConfigGetParams{Scope: "project"})
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
	if resp.Error.Code != ErrCodeMethodNotFound {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeMethodNotFound)
	}
}

func TestDaemonInvalidParams(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// fileInstall with no params (nil Params → unmarshal error)
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodFileInstall, 2, nil)

	if resp.Error == nil {
		t.Fatal("expected error for missing params on fileInstall")
	}
	if resp.Error.Code != ErrCodeInvalidParams {
		t.Errorf("error code = %d, want %d", resp.Error.Code, ErrCodeInvalidParams)
	}
}

func TestDaemonStateChangedNotification(t *testing.T) {
	h := newHarness(t)
	defer h.cleanup()

	sendRequest(t, h.clientWriter, h.clientReader, MethodInitialize, 1, InitializeParams{ProjectDir: h.projectDir})

	// configSet is mutating: expect response + notification
	resp := sendRequest(t, h.clientWriter, h.clientReader, MethodConfigSet, 2, ConfigSetParams{
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
