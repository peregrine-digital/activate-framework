package selfupdate

import (
	"encoding/json"
	"fmt"
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

	for _, asset := range release.Assets {
		if strings.HasSuffix(asset.Name, ".vsix") {
			version := strings.TrimPrefix(release.TagName, "v")
			info := VsixInfo{
				Version:     version,
				DownloadURL: asset.BrowserDownloadURL,
				AssetName:   asset.Name,
			}
			if currentExtVersion == "" || version != currentExtVersion {
				info.Available = true
			}
			return info
		}
	}

	return VsixInfo{}
}
