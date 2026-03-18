package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/menu"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

var dlog *log.Logger

func init() {
	f, err := os.OpenFile("/tmp/activate-desktop.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		dlog = log.New(os.Stderr, "[desktop] ", log.LstdFlags)
	} else {
		dlog = log.New(f, "[desktop] ", log.LstdFlags)
	}
}

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
	dlog.Printf("InitWorkspace called: dir=%s", dir)
	// Stop any existing daemon
	if a.daemon != nil {
		dlog.Println("Stopping existing daemon")
		a.daemon.stop()
		a.daemon = nil
	}

	bin := findBinary()
	if bin == "" {
		dlog.Println("ERROR: activate CLI not found")
		return fmt.Errorf("activate CLI not found — install it first")
	}

	dlog.Printf("Starting daemon: %s serve --stdio (dir=%s)", bin, dir)

	env := os.Environ()
	dc, err := startDaemon(bin, dir, env)
	if err != nil {
		dlog.Printf("ERROR: start daemon: %v", err)
		return fmt.Errorf("failed to start daemon: %w", err)
	}

	dc.onNotification = func(method string) {
		dlog.Printf("Daemon notification: %s", method)
		if method == "activate/stateChanged" {
			wailsRuntime.EventsEmit(a.ctx, "stateChanged")
		}
	}

	// Initialize the daemon with the project directory
	dlog.Println("Sending initialize RPC…")
	var initResult json.RawMessage
	err = dc.callInto(&initResult, "activate/initialize", map[string]string{
		"projectDir": dir,
	})
	if err != nil {
		dlog.Printf("ERROR: initialize: %v", err)
		dc.stop()
		return fmt.Errorf("daemon initialize failed: %w", err)
	}
	dlog.Printf("Initialize complete (len=%d)", len(initResult))

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

// DebugLog allows the frontend to write debug messages to the log file.
func (a *App) DebugLog(msg string) {
	dlog.Printf("[frontend] %s", msg)
}

// ── RPC Forwarding Methods ─────────────────────────────────────

func (a *App) GetState() (json.RawMessage, error) {
	dlog.Println("GetState called")
	if err := a.requireDaemon(); err != nil {
		dlog.Printf("ERROR: GetState: %v", err)
		return nil, err
	}
	result, err := a.daemon.call("activate/state", nil)
	if err != nil {
		dlog.Printf("ERROR: GetState RPC: %v", err)
		return nil, err
	}
	dlog.Printf("GetState done (len=%d)", len(result))
	return result, nil
}

func (a *App) GetConfig(scope string) (json.RawMessage, error) {
	if err := a.requireDaemon(); err != nil {
		return nil, err
	}
	return a.daemon.call("activate/configGet", map[string]string{"scope": scope})
}

func (a *App) SetConfig(params json.RawMessage) (json.RawMessage, error) {
	dlog.Printf("SetConfig called: %s", string(params))
	if err := a.requireDaemon(); err != nil {
		dlog.Printf("ERROR: SetConfig: %v", err)
		return nil, err
	}
	result, err := a.daemon.call("activate/configSet", params)
	if err != nil {
		dlog.Printf("ERROR: SetConfig RPC: %v", err)
		return nil, err
	}
	dlog.Printf("SetConfig done: %s", string(result))
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
	return a.daemon.call("activate/checkUpdate", map[string]interface{}{"force": true})
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
	fullPath := filepath.Join(a.projectDir, file)
	if _, err := os.Stat(fullPath); err != nil {
		return err
	}
	return open(fullPath)
}
