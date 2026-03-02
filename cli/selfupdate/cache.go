package selfupdate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	// CacheFileName is the file name for the update check cache.
	CacheFileName = "update-check.json"
	// CheckInterval is how long between automatic update checks.
	CheckInterval = 24 * time.Hour
)

// CacheEntry stores the result of the most recent update check.
type CacheEntry struct {
	CheckedAt      time.Time `json:"checkedAt"`
	LatestVersion  string    `json:"latestVersion"`
	CurrentVersion string    `json:"currentVersion"`
	UpdateAvail    bool      `json:"updateAvailable"`
	Extension      VsixInfo  `json:"extension,omitempty"`
}

func cachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".activate", CacheFileName), nil
}

// ReadCache reads the cached update check result from disk.
func ReadCache() (*CacheEntry, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// WriteCache persists an update check result to disk.
func WriteCache(entry *CacheEntry) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// CheckCached returns a cached result if fresh (< CheckInterval), otherwise
// performs a live check against GitHub and caches the result.
// currentExtVersion is the VS Code extension version (pass "" from CLI).
// Errors during the live check are silently swallowed and nil is returned,
// so callers can safely use this for non-critical notifications.
func CheckCached(currentVersion, currentExtVersion string) *CacheEntry {
	if cached, err := ReadCache(); err == nil {
		if time.Since(cached.CheckedAt) < CheckInterval && cached.CurrentVersion == currentVersion {
			return cached
		}
	}

	result, err := CheckUpdate(currentVersion)
	if err != nil {
		return nil
	}

	vsix := CheckVsix(currentExtVersion)

	entry := &CacheEntry{
		CheckedAt:      time.Now(),
		LatestVersion:  result.LatestVersion,
		CurrentVersion: currentVersion,
		UpdateAvail:    result.LatestVersion != "" && result.LatestVersion != currentVersion && !isUpToDate(result),
		Extension:      vsix,
	}
	_ = WriteCache(entry)
	return entry
}

func isUpToDate(r *Result) bool {
	return r.LatestVersion == "" || r.LatestVersion == r.CurrentVersion
}
