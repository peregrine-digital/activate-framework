package engine

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// withTestServers overrides storage.RawBase and storage.APIBase for the duration
// of a test, pointing them at the supplied httptest servers.
func withTestServers(t *testing.T, rawSrv, apiSrv *httptest.Server) {
	t.Helper()
	origRaw, origAPI := storage.RawBase, storage.APIBase
	origResolver := storage.TokenResolver
	storage.RawBase = rawSrv.URL
	storage.APIBase = apiSrv.URL
	storage.TokenResolver = func() string { return "" }
	storage.ResetTokenCache()
	t.Cleanup(func() {
		storage.RawBase = origRaw
		storage.APIBase = origAPI
		storage.TokenResolver = origResolver
		storage.ResetTokenCache()
	})
}

// ── DiscoverRemoteManifests ─────────────────────────────────────

func TestDiscoverRemoteManifestsViaIndex(t *testing.T) {
	manifest := manifestJSON{
		Name:        "Test Plugin",
		Description: "A test plugin",
		BasePath:    "plugins/test",
		Files: []model.ManifestFile{
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
		Name:  "Activate Framework",
		Files: []model.ManifestFile{{Src: "b.md", Dest: ".github/b.md", Tier: "standard"}},
	}
	manifestPayload, _ := json.Marshal(manifest)

	raw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "manifests/index.json"):
			w.WriteHeader(http.StatusNotFound)
		case strings.HasSuffix(r.URL.Path, "manifests/adhoc.json"):
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
	if results[0].Name != "Activate Framework" {
		t.Fatalf("got name %q, want %q", results[0].Name, "Activate Framework")
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
