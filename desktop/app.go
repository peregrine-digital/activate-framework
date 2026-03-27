package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/menu"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App manages the daemon lifecycle and exposes RPC methods to the Wails frontend.
type App struct {
	ctx                context.Context
	daemon             *daemonClient
	projectDir         string
	workspaceMenuItems []*menu.MenuItem
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// findBinary locates the activate CLI binary.
func findBinary() string {
	// Check standard install location first
	home, _ := os.UserHomeDir()
	standard := filepath.Join(home, ".activate", "bin", "activate")
	if _, err := os.Stat(standard); err == nil {
		return standard
	}
	// Fall back to PATH
	if p, err := exec.LookPath("activate"); err == nil {
		return p
	}
	return ""
}

// SetWorkspaceMenuVisible shows or hides workspace-only menu items.
func (a *App) SetWorkspaceMenuVisible(visible bool) {
	for _, item := range a.workspaceMenuItems {
		if visible {
			item.Show()
		} else {
			item.Hide()
		}
	}
	wailsRuntime.MenuUpdateApplicationMenu(a.ctx)
}

// InitWorkspace spawns a daemon for the given project directory.
func (a *App) InitWorkspace(dir string) error {
	// Stop any existing daemon
	if a.daemon != nil {
		a.daemon.stop()
		a.daemon = nil
	}

	bin := findBinary()
	if bin == "" {
		return fmt.Errorf("activate CLI not found — install it first")
	}

	env := os.Environ()
	dc, err := startDaemon(bin, dir, env)
	if err != nil {
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	dc.onNotification = func(method string) {
		if method == "activate/stateChanged" {
			wailsRuntime.EventsEmit(a.ctx, "stateChanged")
		}
	}

	// Initialize the daemon with the project directory
	var initResult json.RawMessage
	err = dc.callInto(&initResult, "activate/initialize", map[string]string{
		"projectDir": dir,
	})
	if err != nil {
		dc.stop()
		return fmt.Errorf("daemon initialize failed: %w", err)
	}

	a.daemon = dc
	a.projectDir = dir
	return nil
}

// CloseWorkspace stops the daemon for the current workspace.
func (a *App) CloseWorkspace() {
	if a.daemon != nil {
		a.daemon.stop()
		a.daemon = nil
	}
	a.projectDir = ""
}

// SelectWorkspace opens a native directory picker and spawns a daemon.
func (a *App) SelectWorkspace() (map[string]interface{}, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Workspace",
	})
	if err != nil {
		return nil, err
	}
	if dir == "" {
		return nil, nil
	}
	if err := a.InitWorkspace(dir); err != nil {
		return nil, err
	}
	return map[string]interface{}{"projectDir": dir}, nil
}

func (a *App) requireDaemon() error {
	if a.daemon == nil {
		return fmt.Errorf("no workspace open")
	}
	return nil
}

// Version returns the desktop app version (set at build time).
func (a *App) Version() string {
	return version
}

// CLIFound returns true if the activate CLI binary is available.
func (a *App) CLIFound() bool {
	return findBinary() != ""
}

// ── RPC Forwarding Methods ─────────────────────────────────────

func (a *App) GetState() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	result, err := a.daemon.call("activate/state", nil)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) GetConfig(scope string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/configGet", map[string]string{"scope": scope})
}

func (a *App) SetConfig(params json.RawMessage) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	result, err := a.daemon.call("activate/configSet", params)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *App) InstallFile(dest string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/fileInstall", map[string]string{"file": dest})
}

func (a *App) UninstallFile(dest string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/fileUninstall", map[string]string{"file": dest})
}

func (a *App) DiffFile(dest string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/fileDiff", map[string]string{"file": dest})
}

func (a *App) SkipUpdate(dest string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/fileSkip", map[string]string{"file": dest})
}

func (a *App) SetOverride(dest, override string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/fileOverride", map[string]interface{}{
		"file": dest, "override": override,
	})
}

func (a *App) UpdateAll() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/update", nil)
}

func (a *App) AddToWorkspace() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/repoAdd", nil)
}

func (a *App) RemoveFromWorkspace() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/repoRemove", nil)
}

func (a *App) ListManifests() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/manifestList", nil)
}

func (a *App) ListPresets() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/presetList", nil)
}

func (a *App) ListBranches() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/branchList", nil)
}

func (a *App) RunTelemetry() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/telemetryRun", map[string]string{"token": ""})
}

func (a *App) ReadTelemetryLog() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/telemetryLog", nil)
}

func (a *App) CheckForUpdates() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/checkUpdate", map[string]interface{}{
		"force":          true,
		"desktopVersion": version,
	})
}

func (a *App) SyncManifests() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/sync", nil)
}

// OpenFile opens a file in the OS default application.
func (a *App) OpenFile(file string) error {
	if a.projectDir == "" {
		return nil
	}
	// file.dest is relative to the install dir (.github/)
	fullPath := filepath.Join(a.projectDir, ".github", file)
	if _, err := os.Stat(fullPath); err != nil {
		return err
	}
	return open(fullPath)
}

// UpdateCLI tells the daemon to self-update its CLI binary.
// The daemon process will die during binary replacement.
// Returns the update result or error.
func (a *App) UpdateCLI() (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	// The daemon will die during self-update (binary replacement),
	// so we expect a timeout or connection error. That's OK.
	result, err := a.daemon.call("activate/selfUpdate", map[string]interface{}{
		"token": os.Getenv("GITHUB_TOKEN"),
	})
	// The daemon will die during self-update (binary replacement) — expected.
	_ = err
	// Stop old daemon reference
	a.daemon.stop()
	a.daemon = nil
	return result, nil
}

// RestartDaemon re-spawns the daemon after a CLI update.
func (a *App) RestartDaemon() error {
	if a.projectDir == "" {
		return fmt.Errorf("no workspace open")
	}
	return a.InitWorkspace(a.projectDir)
}

// InstallCLI runs the install script to install the CLI binary.
func (a *App) InstallCLI() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	installDir := filepath.Join(home, ".activate", "bin")

	// Download and run the install script
	cmd := exec.Command("sh", "-c",
		`curl -fsSL https://raw.githubusercontent.com/peregrine-digital/activate-framework/main/install-cli.sh | INSTALL_DIR="`+installDir+`" sh`)
	cmd.Env = append(os.Environ(), "INSTALL_DIR="+installDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("install failed: %w\n%s", err, string(out))
	}
	return nil
}
