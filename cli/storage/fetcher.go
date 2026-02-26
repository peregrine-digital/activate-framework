package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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

// FetchFile downloads a file from GitHub. Uses the API with auth token if
// GITHUB_TOKEN is set, otherwise raw.githubusercontent.com for public repos.
func FetchFile(filePath, repo, branch string) ([]byte, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token != "" {
		return fetchWithAPI(filePath, repo, branch, token)
	}
	return fetchRaw(filePath, repo, branch)
}

func fetchWithAPI(filePath, repo, branch, token string) ([]byte, error) {
	url := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", APIBase, repo, filePath, branch)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", filePath, err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.raw+json")

	resp, err := HTTPClient.Do(req)
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
	resp, err := HTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", filePath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("fetch %s: 404 Not Found (repo may be private — set GITHUB_TOKEN)", filePath)
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
