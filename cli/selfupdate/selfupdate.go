// Package selfupdate provides self-update functionality for the activate CLI
// binary using GitHub releases.
package selfupdate

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/creativeprojects/go-selfupdate"
)

const (
	// GitHubOwner is the repository owner for release lookups.
	GitHubOwner = "peregrine-digital"
	// GitHubRepo is the repository name for release lookups.
	GitHubRepo = "activate-framework"
)

// isPrerelease returns true if the version string contains a pre-release suffix.
func isPrerelease(version string) bool {
	return strings.ContainsAny(version, "-+")
}

// Result describes what happened during a self-update attempt.
type Result struct {
	Updated        bool   `json:"updated"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Message        string `json:"message"`
}

// CheckUpdate checks whether a newer release is available without applying it.
func CheckUpdate(currentVersion string) (*Result, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("creating update source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:      source,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Prerelease: isPrerelease(currentVersion),
		OldSavePath: "",
	})
	if err != nil {
		return nil, fmt.Errorf("creating updater: %w", err)
	}

	latest, found, err := updater.DetectLatest(context.Background(), selfupdate.NewRepositorySlug(GitHubOwner, GitHubRepo))
	if err != nil {
		return nil, fmt.Errorf("checking for updates: %w", err)
	}

	if !found {
		return &Result{
			CurrentVersion: currentVersion,
			Message:        "no releases found",
		}, nil
	}

	if latest.LessOrEqual(currentVersion) {
		return &Result{
			CurrentVersion: currentVersion,
			LatestVersion:  latest.Version(),
			Message:        fmt.Sprintf("already up to date (v%s)", currentVersion),
		}, nil
	}

	return &Result{
		CurrentVersion: currentVersion,
		LatestVersion:  latest.Version(),
		Message:        fmt.Sprintf("update available: v%s → v%s", currentVersion, latest.Version()),
	}, nil
}

// Run checks for the latest release and applies the update to the running binary.
func Run(currentVersion string) (*Result, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("creating update source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{
		Source:     source,
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		Prerelease: isPrerelease(currentVersion),
	})
	if err != nil {
		return nil, fmt.Errorf("creating updater: %w", err)
	}

	latest, err := updater.UpdateSelf(context.Background(), currentVersion, selfupdate.NewRepositorySlug(GitHubOwner, GitHubRepo))
	if err != nil {
		return nil, fmt.Errorf("applying update: %w", err)
	}

	if latest.Version() == currentVersion {
		return &Result{
			CurrentVersion: currentVersion,
			LatestVersion:  latest.Version(),
			Message:        fmt.Sprintf("already up to date (v%s)", currentVersion),
		}, nil
	}

	return &Result{
		Updated:        true,
		CurrentVersion: currentVersion,
		LatestVersion:  latest.Version(),
		Message:        fmt.Sprintf("updated v%s → v%s", currentVersion, latest.Version()),
	}, nil
}
