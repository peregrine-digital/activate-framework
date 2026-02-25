package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

const (
	DefaultRepo   = "peregrine-digital/activate-framework"
	DefaultBranch = "main"
)

// Package-level base URLs; overridden in tests to point at httptest servers.
var (
	rawBase = "https://raw.githubusercontent.com"
	apiBase = "https://api.github.com"
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
	url := fmt.Sprintf("%s/repos/%s/contents/%s?ref=%s", apiBase, repo, filePath, branch)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github.raw+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", filePath, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetch %s: %d %s", filePath, resp.StatusCode, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func fetchRaw(filePath, repo, branch string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s/%s", rawBase, repo, branch, filePath)
	resp, err := http.Get(url)
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
	return io.ReadAll(resp.Body)
}

// FetchJSON fetches and parses a remote JSON file.
func FetchJSON(filePath, repo, branch string, v interface{}) error {
	data, err := FetchFile(filePath, repo, branch)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// DiscoverRemoteManifests fetches manifest metadata from GitHub.
func DiscoverRemoteManifests(repo, branch string) ([]Manifest, error) {
	// Try index.json first
	var index struct {
		Manifests []string `json:"manifests"`
	}
	if err := FetchJSON("manifests/index.json", repo, branch, &index); err == nil && len(index.Manifests) > 0 {
		var results []Manifest
		for _, id := range index.Manifests {
			m, err := LoadRemoteManifest(id, repo, branch)
			if err != nil {
				continue
			}
			results = append(results, m)
		}
		if len(results) > 0 {
			return results, nil
		}
	}

	// Fallback: try known manifest IDs
	known := []string{"activate-framework", "ironarch"}
	var results []Manifest
	for _, id := range known {
		m, err := LoadRemoteManifest(id, repo, branch)
		if err != nil {
			continue
		}
		results = append(results, m)
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("no manifests found in %s@%s", repo, branch)
	}
	return results, nil
}

// LoadRemoteManifest fetches a single manifest by ID from GitHub.
func LoadRemoteManifest(id, repo, branch string) (Manifest, error) {
	var raw manifestJSON
	if err := FetchJSON(fmt.Sprintf("manifests/%s.json", id), repo, branch, &raw); err != nil {
		return Manifest{}, err
	}
	name := raw.Name
	if name == "" {
		name = id
	}
	version := raw.Version
	if version == "" {
		version = "unknown"
	}
	return Manifest{
		ID:          id,
		Name:        name,
		Description: raw.Description,
		Version:     version,
		BasePath:    raw.BasePath,
		Tiers:       raw.Tiers,
		Files:       raw.Files,
	}, nil
}

// InstallFilesFromRemote downloads and writes files from GitHub.
func InstallFilesFromRemote(files []ManifestFile, basePath, targetDir, version, manifestID, repo, branch string) error {
	for _, f := range files {
		srcPath := f.Src
		if basePath != "" {
			srcPath = basePath + "/" + f.Src
		}
		destPath := filepath.Join(targetDir, f.Dest)

		data, err := FetchFile(srcPath, repo, branch)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗  %s: %s\n", f.Dest, err)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
		fmt.Printf("  ✓  %s\n", f.Dest)
	}

	// Write version marker
	versionFile := filepath.Join(targetDir, ".github", ".activate-version")
	if err := os.MkdirAll(filepath.Dir(versionFile), 0755); err != nil {
		return err
	}
	vData, _ := json.MarshalIndent(map[string]string{
		"manifest": manifestID,
		"version":  version,
		"remote":   repo + "@" + branch,
	}, "", "  ")
	return os.WriteFile(versionFile, vData, 0644)
}
