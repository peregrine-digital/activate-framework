package selfupdate

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// VsixInfo describes an available extension update from a GitHub release.
type VsixInfo struct {
	Available   bool   `json:"available"`
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
	SHA256      string `json:"sha256,omitempty"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// assetAPIURL returns the GitHub API URL for downloading a release asset.
// For private repos, this endpoint (with Accept: application/octet-stream)
// is the only way to download assets.
func assetAPIURL(assetID int) string {
	return fmt.Sprintf("%s/repos/%s/%s/releases/assets/%d",
		apiBase, GitHubOwner, GitHubRepo, assetID)
}

// CheckVsix queries the latest GitHub release for a .vsix asset.
// Uses /releases?per_page=1 instead of /releases/latest to support
// repos that only have pre-releases. Token is required for private repos.
// Returns VsixInfo with Available=false if none found or on error.
func CheckVsix(currentExtVersion, token string) VsixInfo {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=1", apiBase, GitHubOwner, GitHubRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VsixInfo{}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		req.Header.Set("Authorization", "Bearer "+envToken)
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return VsixInfo{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return VsixInfo{}
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil || len(releases) == 0 {
		return VsixInfo{}
	}
	release := releases[0]

	var vsixAsset *githubAsset
	var checksumsAsset *githubAsset

	for i, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".vsix") {
			vsixAsset = &release.Assets[i]
		}
		if asset.Name == "checksums.txt" || asset.Name == "SHA256SUMS" || asset.Name == "sha256sums.txt" {
			checksumsAsset = &release.Assets[i]
		}
	}

	if vsixAsset == nil {
		return VsixInfo{}
	}

	version := strings.TrimPrefix(release.TagName, "v")
	info := VsixInfo{
		Version:     version,
		DownloadURL: assetAPIURL(vsixAsset.ID),
		AssetName:   vsixAsset.Name,
	}
	if currentExtVersion == "" || version != currentExtVersion {
		info.Available = true
	}

	// Fetch checksum for the VSIX if a checksums file is available.
	if checksumsAsset != nil {
		info.SHA256 = fetchChecksum(assetAPIURL(checksumsAsset.ID), vsixAsset.Name, token)
	}

	return info
}

// fetchChecksum downloads a checksums file and extracts the hash for the given filename.
// Expected format: "<hash>  <filename>" or "<hash> <filename>" (one per line).
func fetchChecksum(url, filename, token string) string {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Accept", "application/octet-stream")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	} else if envToken := os.Getenv("GITHUB_TOKEN"); envToken != "" {
		req.Header.Set("Authorization", "Bearer "+envToken)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return ""
	}

	for _, line := range strings.Split(string(body), "\n") {
		// Format: "sha256hash  filename" or "sha256hash filename"
		parts := strings.Fields(strings.TrimSpace(line))
		if len(parts) == 2 && parts[1] == filename {
			return parts[0]
		}
	}
	return ""
}
