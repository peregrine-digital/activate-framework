package selfupdate

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// withTestAPI overrides apiBase for the duration of a test.
func withTestAPI(t *testing.T, srv *httptest.Server) {
	t.Helper()
	orig := apiBase
	origToken := os.Getenv("GITHUB_TOKEN")
	apiBase = srv.URL
	os.Unsetenv("GITHUB_TOKEN")
	t.Cleanup(func() {
		apiBase = orig
		if origToken != "" {
			os.Setenv("GITHUB_TOKEN", origToken)
		}
	})
}

// fakeRelease builds a GitHub release JSON array response.
func fakeRelease(tag string, assets []githubAsset) []byte {
	releases := []githubRelease{{TagName: tag, Assets: assets}}
	data, _ := json.Marshal(releases)
	return data
}

func TestCheckVsixFindsUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/releases") {
			w.WriteHeader(404)
			return
		}
		w.Write(fakeRelease("v0.2.0-rc.1", []githubAsset{
			{ID: 42, Name: "activate-0.2.0-rc.1.vsix", BrowserDownloadURL: "https://example.com/old"},
		}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if !info.Available {
		t.Fatal("expected Available=true")
	}
	if info.Version != "0.2.0-rc.1" {
		t.Fatalf("expected version 0.2.0-rc.1, got %s", info.Version)
	}
	if info.AssetName != "activate-0.2.0-rc.1.vsix" {
		t.Fatalf("expected asset name, got %s", info.AssetName)
	}
	// Download URL should use API asset endpoint, not browser URL
	if strings.Contains(info.DownloadURL, "example.com") {
		t.Fatal("download URL should be API asset URL, not browser URL")
	}
	if !strings.Contains(info.DownloadURL, "/releases/assets/42") {
		t.Fatalf("expected API asset URL with ID 42, got %s", info.DownloadURL)
	}
}

func TestCheckVsixAlreadyUpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeRelease("v0.1.0", []githubAsset{
			{ID: 1, Name: "ext.vsix"},
		}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if info.Available {
		t.Fatal("should not report available when versions match")
	}
}

func TestCheckVsixNoVsixAsset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(fakeRelease("v0.2.0", []githubAsset{
			{ID: 1, Name: "activate_0.2.0_darwin-arm64.tar.gz"},
		}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if info.Available {
		t.Fatal("should not report available when no .vsix asset exists")
	}
}

func TestCheckVsixEmptyReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[]"))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if info.Available {
		t.Fatal("should not report available for empty releases")
	}
}

func TestCheckVsixSendsAuthToken(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write(fakeRelease("v0.1.0", []githubAsset{{ID: 1, Name: "ext.vsix"}}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	CheckVsix("0.1.0", "test-token-123")
	if receivedAuth != "Bearer test-token-123" {
		t.Fatalf("expected auth header 'Bearer test-token-123', got %q", receivedAuth)
	}
}

func TestCheckVsixFallsBackToEnvToken(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write(fakeRelease("v0.1.0", []githubAsset{{ID: 1, Name: "ext.vsix"}}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)
	os.Setenv("GITHUB_TOKEN", "env-token-456")

	CheckVsix("0.1.0", "")
	if receivedAuth != "Bearer env-token-456" {
		t.Fatalf("expected env token fallback, got %q", receivedAuth)
	}
}

func TestCheckVsixParamTokenTakesPrecedence(t *testing.T) {
	var receivedAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write(fakeRelease("v0.1.0", []githubAsset{{ID: 1, Name: "ext.vsix"}}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)
	os.Setenv("GITHUB_TOKEN", "env-token")

	CheckVsix("0.1.0", "param-token")
	if receivedAuth != "Bearer param-token" {
		t.Fatalf("param token should take precedence over env, got %q", receivedAuth)
	}
}

func TestCheckVsixWithChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/assets/99") {
			// Checksums file download
			if r.Header.Get("Accept") != "application/octet-stream" {
				t.Errorf("checksum fetch should use Accept: application/octet-stream, got %q", r.Header.Get("Accept"))
			}
			w.Write([]byte("abc123def456  activate-0.2.0.vsix\nfedcba987654  other-file.tar.gz\n"))
			return
		}
		w.Write(fakeRelease("v0.2.0", []githubAsset{
			{ID: 42, Name: "activate-0.2.0.vsix"},
			{ID: 99, Name: "checksums.txt"},
		}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if info.SHA256 != "abc123def456" {
		t.Fatalf("expected checksum abc123def456, got %q", info.SHA256)
	}
}

func TestCheckVsixServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if info.Available {
		t.Fatal("should not report available on server error")
	}
}

func TestCheckVsixUsesReleasesEndpointNotLatest(t *testing.T) {
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		// /releases/latest would return 404 for pre-release-only repos
		if strings.HasSuffix(r.URL.Path, "/latest") {
			w.WriteHeader(404)
			return
		}
		w.Write(fakeRelease("v0.2.0-rc.1", []githubAsset{
			{ID: 1, Name: "ext.vsix"},
		}))
	}))
	defer srv.Close()
	withTestAPI(t, srv)

	info := CheckVsix("0.1.0", "")
	if strings.HasSuffix(requestPath, "/latest") {
		t.Fatal("should use /releases?per_page=1, not /releases/latest")
	}
	if !info.Available {
		t.Fatal("should find pre-release update")
	}
}

func TestFetchChecksumSendsAuth(t *testing.T) {
	var receivedAuth, receivedAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedAccept = r.Header.Get("Accept")
		w.Write([]byte("deadbeef  myfile.vsix\n"))
	}))
	defer srv.Close()

	hash := fetchChecksum(srv.URL+"/asset", "myfile.vsix", "my-token")
	if hash != "deadbeef" {
		t.Fatalf("expected deadbeef, got %q", hash)
	}
	if receivedAuth != "Bearer my-token" {
		t.Fatalf("expected auth header, got %q", receivedAuth)
	}
	if receivedAccept != "application/octet-stream" {
		t.Fatalf("expected octet-stream accept, got %q", receivedAccept)
	}
}

func TestAssetAPIURL(t *testing.T) {
	url := assetAPIURL(12345)
	if !strings.Contains(url, "/releases/assets/12345") {
		t.Fatalf("expected asset URL with ID, got %s", url)
	}
	if !strings.Contains(url, GitHubOwner) || !strings.Contains(url, GitHubRepo) {
		t.Fatalf("expected owner/repo in URL, got %s", url)
	}
}

func TestIsPrerelease(t *testing.T) {
	cases := []struct {
		version string
		want    bool
	}{
		{"0.1.0", false},
		{"1.0.0", false},
		{"0.1.0-rc.1", true},
		{"0.1.0-beta", true},
		{"0.1.0+build", true},
		{"", false},
	}
	for _, tc := range cases {
		got := isPrerelease(tc.version)
		if got != tc.want {
			t.Errorf("isPrerelease(%q) = %v, want %v", tc.version, got, tc.want)
		}
	}
}
