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
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// CheckVsix queries the latest GitHub release for a .vsix asset.
// Returns VsixInfo with Available=false if none found or on error.
func CheckVsix(currentExtVersion string) VsixInfo {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", GitHubOwner, GitHubRepo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return VsixInfo{}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return VsixInfo{}
	}

	var vsixAsset *githubAsset
	var checksumsURL string

	for i, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".vsix") {
			vsixAsset = &release.Assets[i]
		}
		if asset.Name == "checksums.txt" || asset.Name == "SHA256SUMS" || asset.Name == "sha256sums.txt" {
			checksumsURL = asset.BrowserDownloadURL
		}
	}

	if vsixAsset == nil {
		return VsixInfo{}
	}

	version := strings.TrimPrefix(release.TagName, "v")
	info := VsixInfo{
		Version:     version,
		DownloadURL: vsixAsset.BrowserDownloadURL,
		AssetName:   vsixAsset.Name,
	}
	if currentExtVersion == "" || version != currentExtVersion {
		info.Available = true
	}

	// Fetch checksum for the VSIX if a checksums file is available.
	if checksumsURL != "" {
		info.SHA256 = fetchChecksum(checksumsURL, vsixAsset.Name)
	}

	return info
}

// fetchChecksum downloads a checksums file and extracts the hash for the given filename.
// Expected format: "<hash>  <filename>" or "<hash> <filename>" (one per line).
func fetchChecksum(url, filename string) string {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
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
