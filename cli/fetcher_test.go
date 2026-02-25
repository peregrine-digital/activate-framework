package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// withTestServers overrides rawBase and apiBase for the duration of a test,
// pointing them at the supplied httptest servers. It restores the originals
// via t.Cleanup and unsets GITHUB_TOKEN so the default (raw) path is used.
func withTestServers(t *testing.T, rawSrv, apiSrv *httptest.Server) {
	t.Helper()
	origRaw, origAPI := rawBase, apiBase
	origToken := os.Getenv("GITHUB_TOKEN")
	rawBase = rawSrv.URL
	apiBase = apiSrv.URL
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() {
		rawBase = origRaw
		apiBase = origAPI
		if origToken != "" {
			os.Setenv("GITHUB_TOKEN", origToken)
		}
	})
}

// ── FetchFile ───────────────────────────────────────────────────

func TestFetchFileSuccess(t *testing.T) {
	const body = "# Hello from raw\n"
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	data, err := FetchFile("README.md", "owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != body {
		t.Fatalf("got %q, want %q", data, body)
	}
}

func TestFetchFile404(t *testing.T) {
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	_, err := FetchFile("missing.md", "owner/repo", "main")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("error should mention 404, got: %v", err)
	}
}

func TestFetchFileNon200(t *testing.T) {
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	_, err := FetchFile("file.md", "owner/repo", "main")
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("error should mention 500, got: %v", err)
	}
}

// ── FetchFile with API (GITHUB_TOKEN) ───────────────────────────

func TestFetchFileWithToken(t *testing.T) {
	const body = "api content"
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-tok" {
			t.Errorf("bad auth header: %s", auth)
		}
		if accept := r.Header.Get("Accept"); !strings.Contains(accept, "raw") {
			t.Errorf("bad accept header: %s", accept)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(body))
	}))
	defer api.Close()
	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()

	withTestServers(t, raw, api)
	t.Setenv("GITHUB_TOKEN", "test-tok")

	data, err := FetchFile("file.txt", "owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != body {
		t.Fatalf("got %q, want %q", data, body)
	}
}

// ── FetchJSON ───────────────────────────────────────────────────

func TestFetchJSONSuccess(t *testing.T) {
	payload := map[string]string{"hello": "world"}
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	var got map[string]string
	if err := FetchJSON("data.json", "owner/repo", "main", &got); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["hello"] != "world" {
		t.Fatalf("got %v, want hello=world", got)
	}
}

func TestFetchJSONInvalid(t *testing.T) {
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json at all"))
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	var got map[string]string
	err := FetchJSON("bad.json", "owner/repo", "main", &got)
	if err == nil {
		t.Fatal("expected JSON unmarshal error")
	}
}

// ── DiscoverRemoteManifests ─────────────────────────────────────

func TestDiscoverRemoteManifestsViaIndex(t *testing.T) {
	manifest := manifestJSON{
		Name:        "Test Plugin",
		Description: "A test plugin",
		Version:     "1.0.0",
		BasePath:    "plugins/test",
		Files: []ManifestFile{
			{Src: "a.md", Dest: ".github/a.md", Tier: "minimal"},
		},
	}
	indexPayload, _ := json.Marshal(map[string][]string{
		"manifests": {"test-plugin"},
	})
	manifestPayload, _ := json.Marshal(manifest)

	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "manifests/index.json"):
			w.Write(indexPayload)
		case strings.HasSuffix(r.URL.Path, "manifests/test-plugin.json"):
			w.Write(manifestPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	results, err := DiscoverRemoteManifests("owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(results))
	}
	if results[0].Name != "Test Plugin" {
		t.Fatalf("got name %q, want %q", results[0].Name, "Test Plugin")
	}
	if results[0].ID != "test-plugin" {
		t.Fatalf("got id %q, want %q", results[0].ID, "test-plugin")
	}
}

func TestDiscoverRemoteManifestsFallback(t *testing.T) {
	manifest := manifestJSON{
		Name:    "Activate Framework",
		Version: "2.0.0",
		Files:   []ManifestFile{{Src: "b.md", Dest: ".github/b.md", Tier: "standard"}},
	}
	manifestPayload, _ := json.Marshal(manifest)

	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "manifests/index.json"):
			w.WriteHeader(http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "manifests/activate-framework.json"):
			w.Write(manifestPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	results, err := DiscoverRemoteManifests("owner/repo", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(results))
	}
	if results[0].Version != "2.0.0" {
		t.Fatalf("got version %q, want %q", results[0].Version, "2.0.0")
	}
}

func TestDiscoverRemoteManifestsNoneFound(t *testing.T) {
	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer raw.Close()
	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()
	withTestServers(t, raw, api)

	_, err := DiscoverRemoteManifests("owner/repo", "main")
	if err == nil {
		t.Fatal("expected error when no manifests found")
	}
	if !strings.Contains(err.Error(), "no manifests found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Network errors ──────────────────────────────────────────────

func TestFetchFileNetworkError(t *testing.T) {
	// Start and immediately close to get an unreachable URL.
	raw := httptest.NewServer(http.NotFoundHandler())
	url := raw.URL
	raw.Close()

	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()

	origRaw := rawBase
	rawBase = url
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() { rawBase = origRaw })

	_, err := FetchFile("file.md", "owner/repo", "main")
	if err == nil {
		t.Fatal("expected network error")
	}
}

func TestFetchFileAPINetworkError(t *testing.T) {
	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()

	api := httptest.NewServer(http.NotFoundHandler())
	url := api.URL
	api.Close()

	origRaw, origAPI := rawBase, apiBase
	rawBase = raw.URL
	apiBase = url
	t.Setenv("GITHUB_TOKEN", "tok")
	t.Cleanup(func() { rawBase = origRaw; apiBase = origAPI })

	_, err := FetchFile("file.md", "owner/repo", "main")
	if err == nil {
		t.Fatal("expected network error")
	}
}
