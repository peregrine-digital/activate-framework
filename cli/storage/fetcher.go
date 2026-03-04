package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	DefaultRepo   = "peregrine-digital/activate-framework"
	DefaultBranch = "main"

	httpTimeout = 30 // seconds
)

// Package-level base URLs; overridden in tests to point at httptest servers.
var (
	RawBase = "https://raw.githubusercontent.com"
	APIBase = "https://api.github.com"

	// HTTPClient is the shared client for all outbound requests.
	HTTPClient = &http.Client{Timeout: httpTimeout * time.Second}
)

// resolvedToken caches the GitHub token for the process lifetime.
var (
	tokenOnce sync.Once
	tokenVal  string
)

// TokenResolver is the function used to resolve GitHub tokens.
// Override in tests to control auth behavior (e.g., suppress gh CLI lookup).
var TokenResolver = defaultTokenResolver

func defaultTokenResolver() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// ResolveToken returns a GitHub token from GITHUB_TOKEN env or `gh auth token`.
// Result is cached — resolved once per process.
func ResolveToken() string {
	tokenOnce.Do(func() {
		tokenVal = TokenResolver()
	})
	return tokenVal
}

// ResetTokenCache clears the cached token so it will be re-resolved on next call.
// Exported for test use only.
func ResetTokenCache() {
	tokenOnce = sync.Once{}
	tokenVal = ""
}

// GitHubGet performs an HTTP GET to a GitHub URL with automatic auth.
// Attaches Authorization header when a token is available. Callers can set
// additional headers on the returned request by using GitHubDo instead.
func GitHubGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	return GitHubDo(req)
}

// GitHubDo executes an HTTP request with automatic GitHub auth.
// If a token is available, it sets the Authorization header (unless already set).
// All GitHub HTTP calls should go through this function.
func GitHubDo(req *http.Request) (*http.Response, error) {
	if req.Header.Get("Authorization") == "" {
		if token := ResolveToken(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}
	return HTTPClient.Do(req)
}

// FetchFile downloads a file from GitHub. Uses the Contents API when
// authenticated, raw.githubusercontent.com otherwise.
func FetchFile(filePath, repo, branch string) ([]byte, error) {
	if ResolveToken() != "" {
		return fetchWithAPI(filePath, repo, branch)
	}
	return fetchRaw(filePath, repo, branch)
}

func fetchWithAPI(filePath, repo, branch string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", APIBase, repo, filePath, branch)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", filePath, err)
	}
	req.Header.Set("Accept", "application/vnd.github.raw+json")

	resp, err := GitHubDo(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", filePath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch %s: %d %s", filePath, resp.StatusCode, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

func fetchRaw(filePath, repo, branch string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", RawBase, repo, branch, filePath)
	resp, err := GitHubGet(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", filePath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("fetch %s: 404 Not Found (repo may be private — set GITHUB_TOKEN or run 'gh auth login')", filePath)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch %s: %d %s", filePath, resp.StatusCode, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 10<<20))
}

// FetchJSON fetches and parses a remote JSON file.
func FetchJSON(filePath, repo, branch string, v interface{}) error {
	data, err := FetchFile(filePath, repo, branch)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
