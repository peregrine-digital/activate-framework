package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// withTestServers overrides RawBase and APIBase for the duration of a test,
// pointing them at the supplied httptest servers. It restores the originals
// via t.Cleanup, suppresses real token resolution so the raw path is used.
func withTestServers(t *testing.T, rawSrv, apiSrv *httptest.Server) {
	t.Helper()
	origRaw, origAPI := RawBase, APIBase
	origResolver := TokenResolver
	RawBase = rawSrv.URL
	APIBase = apiSrv.URL
	TokenResolver = func() string { return "" }
	ResetTokenCache()
	t.Cleanup(func() {
		RawBase = origRaw
		APIBase = origAPI
		TokenResolver = origResolver
		ResetTokenCache()
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
	TokenResolver = func() string { return "test-tok" }
	ResetTokenCache()

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

// ── Network errors ──────────────────────────────────────────────

func TestFetchFileNetworkError(t *testing.T) {
	// Start and immediately close to get an unreachable URL.
	raw := httptest.NewServer(http.NotFoundHandler())
	url := raw.URL
	raw.Close()

	api := httptest.NewServer(http.NotFoundHandler())
	defer api.Close()

	origRaw := RawBase
	origResolver := TokenResolver
	RawBase = url
	TokenResolver = func() string { return "" }
	ResetTokenCache()
	t.Cleanup(func() {
		RawBase = origRaw
		TokenResolver = origResolver
		ResetTokenCache()
	})

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

	origRaw, origAPI := RawBase, APIBase
	origResolver := TokenResolver
	RawBase = raw.URL
	APIBase = url
	TokenResolver = func() string { return "tok" }
	ResetTokenCache()
	t.Cleanup(func() {
		RawBase = origRaw
		APIBase = origAPI
		TokenResolver = origResolver
		ResetTokenCache()
	})

	_, err := FetchFile("file.md", "owner/repo", "main")
	if err == nil {
		t.Fatal("expected network error")
	}
}

// ── FetchBranches ───────────────────────────────────────────────

func TestFetchBranchesSuccess(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/repos/owner/repo/branches") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]map[string]interface{}{
			{"name": "main"},
			{"name": "develop"},
			{"name": "feat/branch-picker"},
		})
	}))
	defer api.Close()
	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()
	withTestServers(t, raw, api)

	branches, err := FetchBranches("owner/repo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(branches) != 3 {
		t.Fatalf("expected 3 branches, got %d", len(branches))
	}
	if branches[0] != "main" || branches[1] != "develop" || branches[2] != "feat/branch-picker" {
		t.Fatalf("unexpected branches: %v", branches)
	}
}

func TestFetchBranches404(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer api.Close()
	raw := httptest.NewServer(http.NotFoundHandler())
	defer raw.Close()
	withTestServers(t, raw, api)

	_, err := FetchBranches("owner/nonexistent")
	if err == nil {
		t.Fatal("expected error for 404")
	}
}
