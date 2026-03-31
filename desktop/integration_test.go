package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestDaemonSubprocess_AddRemoveAddLifecycle builds the real CLI binary,
// spawns it as `activate serve --stdio`, and exercises the full
// add→state→remove→state→add→state lifecycle over real pipes.
//
// This catches:
// - stdout corruption from rogue fmt.Printf calls in engine code
// - stale state after mutations
// - readLoop death from malformed frames
func TestDaemonSubprocess_AddRemoveAddLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess integration test in short mode")
	}

	// ── Build the CLI binary from source ──
	binPath := filepath.Join(t.TempDir(), "activate")
	buildCmd := exec.Command("go", "build", "-o", binPath, "./cli")
	buildCmd.Dir = filepath.Join("..")
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build CLI: %v\n%s", err, buildOut)
	}

	// ── Set up fake remote server ──
	basePath := "plugins/test"
	remoteFiles := map[string]string{
		basePath + "/instructions/setup.md": "---\nversion: '1.0.0'\n---\n# Setup\nHello\n",
		basePath + "/prompts/build.md":      "---\nversion: '1.0.0'\n---\n# Build\nWorld\n",
		"manifests/test-manifest.json": mustJSON(t, map[string]interface{}{
			"id": "test-manifest", "name": "Test Manifest", "basePath": basePath,
			"files": []map[string]interface{}{
				{"src": "instructions/setup.md", "dest": "instructions/setup.md", "tier": "core"},
				{"src": "prompts/build.md", "dest": "prompts/build.md", "tier": "core"},
			},
		}),
	}
	repo := "test/repo"
	branch := "main"
	mux := http.NewServeMux()
	raw := httptest.NewServer(mux)
	defer raw.Close()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		prefix := "/" + repo + "/" + branch + "/"
		if strings.HasPrefix(r.URL.Path, prefix) {
			key := strings.TrimPrefix(r.URL.Path, prefix)
			if content, ok := remoteFiles[key]; ok {
				w.Write([]byte(content))
				return
			}
		}
		// Also serve manifest index for discovery
		if r.URL.Path == "/repos/"+repo+"/contents/manifests" {
			items := []map[string]string{
				{"name": "test-manifest.json", "type": "file", "download_url": raw.URL + "/" + repo + "/" + branch + "/manifests/test-manifest.json"},
			}
			json.NewEncoder(w).Encode(items)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})

	// ── Set up test directories ──
	projectDir := t.TempDir()
	activateDir := t.TempDir()

	// Create .git/info/exclude
	os.MkdirAll(filepath.Join(projectDir, ".git", "info"), 0755)
	os.WriteFile(filepath.Join(projectDir, ".git", "info", "exclude"), []byte(""), 0644)

	// Pre-create repo store with config + manifest cache
	absProject, _ := filepath.Abs(projectDir)
	hash := sha256Hex(absProject)
	repoStoreDir := filepath.Join(activateDir, "repos", hash)
	os.MkdirAll(repoStoreDir, 0755)

	os.WriteFile(filepath.Join(repoStoreDir, "repo.json"),
		[]byte(mustJSON(t, map[string]string{"path": absProject})), 0644)

	os.WriteFile(filepath.Join(repoStoreDir, "config.json"),
		[]byte(mustJSON(t, map[string]string{
			"manifest": "test-manifest", "tier": "standard",
			"repo": repo, "branch": branch,
		})), 0644)

	manifestObj := map[string]interface{}{
		"id": "test-manifest", "name": "Test Manifest", "basePath": basePath,
		"files": []map[string]interface{}{
			{"src": "instructions/setup.md", "dest": "instructions/setup.md", "tier": "core"},
			{"src": "prompts/build.md", "dest": "prompts/build.md", "tier": "core"},
		},
	}
	os.WriteFile(filepath.Join(repoStoreDir, "manifest-cache.json"),
		[]byte(mustJSON(t, []interface{}{manifestObj})), 0644)

	// ── Start daemon subprocess ──
	cmd := exec.Command(binPath, "serve", "--stdio")
	cmd.Dir = projectDir
	cmd.Env = []string{
		"HOME=" + activateDir, // daemon uses ~/.activate, but ACTIVATE_BASE overrides
		"PATH=" + os.Getenv("PATH"),
		"ACTIVATE_BASE=" + activateDir,
		"ACTIVATE_RAW_BASE=" + raw.URL,
		"ACTIVATE_API_BASE=" + raw.URL,
		"GITHUB_TOKEN=", // no auth needed
	}
	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start daemon: %v", err)
	}
	defer func() {
		stdinPipe.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	reader := bufio.NewReaderSize(stdoutPipe, 64*1024)

	// ── Protocol helpers ──

	reqID := 0
	send := func(method string, params interface{}) {
		t.Helper()
		reqID++
		req := map[string]interface{}{"jsonrpc": "2.0", "id": reqID, "method": method}
		if params != nil {
			req["params"] = params
		}
		body, _ := json.Marshal(req)
		fmt.Fprintf(stdinPipe, "Content-Length: %d\r\n\r\n", len(body))
		stdinPipe.Write(body)
	}

	readMsg := func(timeout time.Duration) (json.RawMessage, error) {
		type result struct {
			data json.RawMessage
			err  error
		}
		ch := make(chan result, 1)
		go func() {
			contentLen := -1
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					ch <- result{err: fmt.Errorf("read header: %w", err)}
					return
				}
				line = strings.TrimRight(line, "\r\n")
				if line == "" {
					break
				}
				if strings.HasPrefix(line, "Content-Length:") {
					val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
					n, _ := strconv.Atoi(val)
					contentLen = n
				}
			}
			if contentLen < 0 {
				ch <- result{err: fmt.Errorf("missing Content-Length header")}
				return
			}
			body := make([]byte, contentLen)
			_, err := io.ReadFull(reader, body)
			ch <- result{data: body, err: err}
		}()
		select {
		case r := <-ch:
			return r.data, r.err
		case <-time.After(timeout):
			return nil, fmt.Errorf("read timeout after %s", timeout)
		}
	}

	// callAndRead sends an RPC and reads the response, skipping any interleaved notifications.
	callAndRead := func(method string, params interface{}) map[string]interface{} {
		t.Helper()
		send(method, params)
		for {
			msg, err := readMsg(15 * time.Second)
			if err != nil {
				t.Fatalf("[%s] read: %v", method, err)
			}
			var parsed map[string]interface{}
			json.Unmarshal(msg, &parsed)
			if parsed["id"] != nil {
				if parsed["error"] != nil {
					t.Fatalf("[%s] RPC error: %v", method, parsed["error"])
				}
				return parsed
			}
			t.Logf("[%s] skipped notification: %v", method, parsed["method"])
		}
	}

	drainNotification := func(label string) {
		t.Helper()
		msg, err := readMsg(5 * time.Second)
		if err != nil {
			t.Fatalf("[%s] drain notification: %v", label, err)
		}
		var parsed map[string]interface{}
		json.Unmarshal(msg, &parsed)
		t.Logf("[%s] notification: method=%v", label, parsed["method"])
	}

	getInstallMarker := func(resp map[string]interface{}) bool {
		result, _ := resp["result"].(map[string]interface{})
		state, _ := result["state"].(map[string]interface{})
		marker, _ := state["hasInstallMarker"].(bool)
		return marker
	}

	// ── Test lifecycle ──

	// 1. Initialize
	t.Log("=== Initialize ===")
	callAndRead("activate/initialize", map[string]string{"projectDir": projectDir})

	// 2. Initial state — not installed
	t.Log("=== GetState (initial) ===")
	state0 := callAndRead("activate/state", nil)
	if getInstallMarker(state0) {
		t.Fatal("FAIL: expected hasInstallMarker=false initially")
	}
	t.Log("✓ initial state: not installed")

	// 3. RepoAdd — Install files
	t.Log("=== RepoAdd ===")
	addResp := callAndRead("activate/repoAdd", nil)
	t.Logf("repoAdd result: %v", addResp["result"])

	// Must receive stateChanged notification after repoAdd.
	// If stdout is corrupted, this will timeout — proving the bug.
	drainNotification("after repoAdd")

	// 4. GetState — MUST be installed
	t.Log("=== GetState (after add) ===")
	state1 := callAndRead("activate/state", nil)
	if !getInstallMarker(state1) {
		t.Fatal("FAIL: expected hasInstallMarker=true after repoAdd")
	}
	t.Log("✓ after add: installed")

	// 5. RepoRemove — Uninstall
	t.Log("=== RepoRemove ===")
	callAndRead("activate/repoRemove", nil)
	drainNotification("after repoRemove")

	// 6. GetState — not installed
	t.Log("=== GetState (after remove) ===")
	state2 := callAndRead("activate/state", nil)
	if getInstallMarker(state2) {
		t.Fatal("FAIL: expected hasInstallMarker=false after repoRemove")
	}
	t.Log("✓ after remove: not installed")

	// 7. RepoAdd again
	t.Log("=== RepoAdd (second) ===")
	callAndRead("activate/repoAdd", nil)
	drainNotification("after second repoAdd")

	// 8. GetState — MUST be installed again
	t.Log("=== GetState (after second add) ===")
	state3 := callAndRead("activate/state", nil)
	if !getInstallMarker(state3) {
		t.Fatal("FAIL: expected hasInstallMarker=true after second repoAdd")
	}
	t.Log("✓ after second add: installed")

	t.Log("✓ Full lifecycle passed over real subprocess")
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func mustJSON(t *testing.T, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return string(data)
}
