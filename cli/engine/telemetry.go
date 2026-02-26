package engine

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/storage"
)

const (
	telemetryLogFile    = "copilot-telemetry.jsonl"
	copilotUserEndpoint = "https://api.github.com/copilot_internal/user"
	telemetryUserAgent  = "Peregrine-Activate-Telemetry"
)

// IsTelemetryEnabled checks if telemetry is opt-in enabled.
func IsTelemetryEnabled(cfg model.Config) bool {
	return cfg.TelemetryEnabled != nil && *cfg.TelemetryEnabled
}

// ResolveGitHubToken returns a GitHub token from env or gh CLI.
func ResolveGitHubToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

// FetchCopilotUserData fetches Copilot quota data from the GitHub API.
func FetchCopilotUserData(token string) (map[string]interface{}, error) {
	if token == "" {
		return nil, fmt.Errorf("no GitHub token available (set GITHUB_TOKEN or run 'gh auth login')")
	}

	req, err := http.NewRequest("GET", copilotUserEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", telemetryUserAgent)

	resp, err := storage.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// ExtractPremiumQuota pulls the premium_interactions quota from user data.
func ExtractPremiumQuota(data map[string]interface{}) (entitlement, remaining *int) {
	snapshots, ok := data["quota_snapshots"].(map[string]interface{})
	if !ok {
		return nil, nil
	}
	for _, v := range snapshots {
		snap, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if snap["quota_id"] != "premium_interactions" {
			continue
		}
		if unlimited, ok := snap["unlimited"].(bool); ok && unlimited {
			return nil, nil
		}
		if ent, ok := snap["entitlement"].(float64); ok {
			e := int(ent)
			entitlement = &e
		}
		if rem, ok := snap["remaining"].(float64); ok {
			r := int(rem)
			remaining = &r
		}
		return entitlement, remaining
	}
	return nil, nil
}

// BuildTelemetryEntry creates a log entry from API data.
func BuildTelemetryEntry(data map[string]interface{}) model.TelemetryEntry {
	now := time.Now().UTC()
	entry := model.TelemetryEntry{
		Date:      now.Format("2006-01-02"),
		Timestamp: now.Format(time.RFC3339Nano),
		Source:    "github_copilot_internal",
		Version:   1,
	}

	entitlement, remaining := ExtractPremiumQuota(data)
	entry.PremiumEntitlement = entitlement
	entry.PremiumRemaining = remaining

	if entitlement != nil && remaining != nil {
		used := *entitlement - *remaining
		if used < 0 {
			used = 0
		}
		entry.PremiumUsed = &used
	}

	if resetDate, ok := data["quota_reset_date_utc"].(string); ok {
		entry.QuotaResetDateUTC = resetDate
	}

	return entry
}

func telemetryLogDir() string {
	return storage.StoreBase()
}

func TelemetryLogPath() string {
	return filepath.Join(telemetryLogDir(), telemetryLogFile)
}

// AppendTelemetryEntry appends an entry to the JSONL log file.
func AppendTelemetryEntry(entry model.TelemetryEntry) error {
	dir := telemetryLogDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, telemetryLogFile)

	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(append(line, '\n'))
	return err
}

// ReadTelemetryLog reads all entries from the JSONL log.
func ReadTelemetryLog() ([]model.TelemetryEntry, error) {
	path := filepath.Join(telemetryLogDir(), telemetryLogFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil
	}

	var entries []model.TelemetryEntry
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e model.TelemetryEntry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// ArchiveLogIfNeeded archives the active log when the quota reset date changes.
func ArchiveLogIfNeeded(currentResetDate, previousResetDate string) (string, error) {
	if currentResetDate == previousResetDate || previousResetDate == "" {
		return "", nil
	}

	dir := telemetryLogDir()
	activePath := filepath.Join(dir, telemetryLogFile)
	if _, err := os.Stat(activePath); err != nil {
		return "", nil
	}

	dateStamp := previousResetDate
	if t, err := time.Parse(time.RFC3339, previousResetDate); err == nil {
		dateStamp = t.Format("2006-01-02")
	}

	archiveName := fmt.Sprintf("copilot-telemetry-%s.jsonl", dateStamp)
	archivePath := filepath.Join(dir, archiveName)

	if err := os.Rename(activePath, archivePath); err != nil {
		data, readErr := os.ReadFile(activePath)
		if readErr != nil {
			return "", readErr
		}
		if writeErr := os.WriteFile(archivePath, data, 0644); writeErr != nil {
			return "", writeErr
		}
		os.Remove(activePath)
	}

	return archivePath, nil
}

// RunTelemetry performs a single telemetry log run.
func RunTelemetry(token string) (*model.TelemetryEntry, error) {
	if token == "" {
		token = ResolveGitHubToken()
	}

	data, err := FetchCopilotUserData(token)
	if err != nil {
		return nil, err
	}

	entry := BuildTelemetryEntry(data)
	if err := AppendTelemetryEntry(entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// ComputeFileStatuses builds a status list for every file in the manifest.
func ComputeFileStatuses(m model.Manifest, sidecar *model.RepoSidecar, cfg model.Config, projectDir string) []model.FileStatus {
	allowedTiers := model.GetAllowedFileTiers(m, cfg.Tier)

	installedSet := make(map[string]bool)
	if sidecar != nil {
		for _, f := range sidecar.Files {
			installedSet[f] = true
		}
	}

	result := make([]model.FileStatus, 0, len(m.Files))
	for _, f := range m.Files {
		destRel := ".github/" + f.Dest

		cat := f.Category
		if cat == "" {
			cat = model.InferCategory(f.Src)
		}

		fs := model.FileStatus{
			Dest:        f.Dest,
			DisplayName: model.FileDisplayName(f.Dest),
			Category:    cat,
			Tier:        f.Tier,
			InTier:      allowedTiers[f.Tier],
		}
		if f.Description != "" {
			fs.Description = f.Description
		}

		if ov, ok := cfg.FileOverrides[f.Dest]; ok {
			fs.Override = ov
		}

		fs.Installed = installedSet[destRel]

		if m.BasePath != "" {
			bv, _ := storage.ReadFileVersion(m.BasePath + "/" + f.Src)
			fs.BundledVersion = bv
		}

		if fs.Installed {
			iv, _ := storage.ReadFileVersion(projectDir + "/" + destRel)
			fs.InstalledVersion = iv
		}

		if fs.Installed && fs.BundledVersion != "" && fs.InstalledVersion != "" && fs.BundledVersion != fs.InstalledVersion {
			fs.UpdateAvailable = true
		}

		if sv, ok := cfg.SkippedVersions[f.Dest]; ok && sv == fs.BundledVersion {
			fs.Skipped = true
			fs.UpdateAvailable = false
		}

		result = append(result, fs)
	}
	return result
}

// DetectInstallState checks config and install status for state-aware boot flow.
func DetectInstallState(projectDir string) model.InstallState {
	state := model.InstallState{}

	if _, err := os.Stat(storage.GlobalConfigPath()); err == nil {
		state.HasGlobalConfig = true
	}
	if _, err := os.Stat(storage.ProjectConfigPath(projectDir)); err == nil {
		state.HasProjectConfig = true
	}

	if sc, _ := storage.ReadRepoSidecar(projectDir); sc != nil {
		state.HasInstallMarker = true
		state.InstalledManifest = sc.Manifest
		state.InstalledVersion = sc.Version
	}

	return state
}
