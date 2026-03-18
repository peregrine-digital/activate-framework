package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/peregrine-digital/activate-framework/cli/commands"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/wailsapp/wails/v2/pkg/menu"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App wraps the CLI's ActivateService for desktop use.
type App struct {
	ctx               context.Context
	svc               *commands.ActivateService
	workspaceMenuItems []*menu.MenuItem
}

func NewApp() *App {
	return &App{
		svc: commands.NewService("", nil, model.Config{}),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
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

// SelectWorkspace opens a native directory picker and initializes the service.
func (a *App) SelectWorkspace() (commands.StateResult, error) {
	dir, err := wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Workspace",
	})
	if err != nil {
		return commands.StateResult{}, err
	}
	if dir == "" {
		return a.svc.GetState(), nil
	}
	a.svc.Initialize(dir)
	return a.svc.GetState(), nil
}

func (a *App) InitWorkspace(dir string) {
	a.svc.Initialize(dir)
}

func (a *App) GetState() commands.StateResult {
	return a.svc.GetState()
}

func (a *App) GetConfig(scope string) (*model.Config, error) {
	return a.svc.GetConfig(scope)
}

func (a *App) SetConfig(scope string, updates *model.Config) (*commands.SetConfigResult, error) {
	return a.svc.SetConfig(scope, updates)
}

func (a *App) RefreshConfig() {
	a.svc.RefreshConfig()
}

func (a *App) ListManifests() []model.Manifest {
	return a.svc.ListManifests()
}

func (a *App) ListFiles(manifestID, tierID, category string) (*commands.ListFilesResult, error) {
	return a.svc.ListFiles(manifestID, tierID, category)
}

func (a *App) InstallFile(file string) (*commands.FileResult, error) {
	return a.svc.InstallFile(file)
}

func (a *App) UninstallFile(file string) (*commands.FileResult, error) {
	return a.svc.UninstallFile(file)
}

func (a *App) DiffFile(file string) (*commands.DiffResult, error) {
	return a.svc.DiffFile(file)
}

func (a *App) SkipUpdate(file string) (*commands.FileResult, error) {
	return a.svc.SkipUpdate(file)
}

func (a *App) SetOverride(file, override string) (*commands.FileResult, error) {
	return a.svc.SetOverride(file, override)
}

func (a *App) Sync() (*commands.SyncResult, error) {
	return a.svc.Sync()
}

func (a *App) Update() (*commands.UpdateResult, error) {
	return a.svc.Update()
}

func (a *App) RepoAdd() (*commands.RepoAddResult, error) {
	return a.svc.RepoAdd()
}

func (a *App) RepoRemove() error {
	return a.svc.RepoRemove()
}

func (a *App) ListBranches(repo string) ([]string, error) {
	return a.svc.ListBranches(repo)
}

func (a *App) RunTelemetry(token string) (*commands.TelemetryRunResult, error) {
	return a.svc.RunTelemetry(token)
}

func (a *App) ReadTelemetryLog() ([]model.TelemetryEntry, error) {
	return a.svc.ReadTelemetryLog()
}

// OpenFile opens a file in the OS default application.
func (a *App) OpenFile(file string) error {
	projectDir := a.svc.CurrentProjectDir()
	if projectDir == "" {
		return nil
	}
	fullPath := filepath.Join(projectDir, file)
	if _, err := os.Stat(fullPath); err != nil {
		return err
	}
	return open(fullPath)
}
