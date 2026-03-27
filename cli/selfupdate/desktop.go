package selfupdate

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/storage"
)

// DesktopInfo describes an available desktop app update from a GitHub release.
type DesktopInfo struct {
	Available   bool   `json:"available"`
	Version     string `json:"version"`
	DownloadURL string `json:"downloadUrl"`
	AssetName   string `json:"assetName"`
	SHA256      string `json:"sha256,omitempty"`
}

// desktopAssetName returns the expected archive name for the current platform.
func desktopAssetName(version string) string {
	os := runtime.GOOS
	arch := runtime.GOARCH
	ext := "tar.gz"
	if os == "darwin" || os == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("activate-desktop_%s_%s-%s.%s", version, os, arch, ext)
}

// CheckDesktop queries the latest GitHub release for a desktop app asset
// matching the current OS/ARCH. Token is required for private repos.
// Returns DesktopInfo with Available=false if none found or on error.
func CheckDesktop(currentDesktopVersion, token string) DesktopInfo {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=1", storage.APIBase, GitHubOwner, GitHubRepo)

	req, err := newGitHubRequest("GET", url, token)
	if err != nil {
		return DesktopInfo{}
	}

	resp, err := storage.GitHubDo(req)
	if err != nil {
		return DesktopInfo{}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return DesktopInfo{}
	}

	releases, err := decodeReleases(resp.Body)
	if err != nil || len(releases) == 0 {
		return DesktopInfo{}
	}
	release := releases[0]
	releaseVersion := strings.TrimPrefix(release.TagName, "v")

	expectedName := desktopAssetName(releaseVersion)

	var desktopAsset *githubAsset
	var checksumsAsset *githubAsset

	for i, asset := range release.Assets {
		if asset.Name == expectedName {
			desktopAsset = &release.Assets[i]
		}
		if isChecksumFile(asset.Name) {
			checksumsAsset = &release.Assets[i]
		}
	}

	if desktopAsset == nil {
		return DesktopInfo{}
	}

	info := DesktopInfo{
		Version:     releaseVersion,
		DownloadURL: assetAPIURL(desktopAsset.ID),
		AssetName:   desktopAsset.Name,
	}
	if currentDesktopVersion == "" || releaseVersion != currentDesktopVersion {
		info.Available = true
	}

	if checksumsAsset != nil {
		info.SHA256 = fetchChecksum(assetAPIURL(checksumsAsset.ID), desktopAsset.Name, token)
	}

	return info
}

// isChecksumFile returns true for common checksum file names.
func isChecksumFile(name string) bool {
	return name == "checksums.txt" || name == "desktop-checksums.txt" ||
		name == "SHA256SUMS" || name == "sha256sums.txt"
}
