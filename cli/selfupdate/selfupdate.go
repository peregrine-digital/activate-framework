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

// apiBase is the GitHub API base URL. Override in tests.
var apiBase = "https://api.github.com"

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

// updaterConfig builds the go-selfupdate Config for a given version and token.
// Extracted for testability — callers can verify that the Prerelease flag
// and token are set correctly without hitting the network.
func updaterConfig(currentVersion, token string) (selfupdate.Config, error) {
	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{
		APIToken: token,
	})
	if err != nil {
		return selfupdate.Config{}, fmt.Errorf("creating update source: %w", err)
	}
	return selfupdate.Config{
		Source:     source,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Prerelease: isPrerelease(currentVersion),
	}, nil
}

// CheckUpdate checks whether a newer release is available without applying it.
func CheckUpdate(currentVersion, token string) (*Result, error) {
	cfg, err := updaterConfig(currentVersion, token)
	if err != nil {
		return nil, err
	}

	updater, err := selfupdate.NewUpdater(cfg)
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
func Run(currentVersion, token string) (*Result, error) {
	cfg, err := updaterConfig(currentVersion, token)
	if err != nil {
		return nil, err
	}

	updater, err := selfupdate.NewUpdater(cfg)
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
