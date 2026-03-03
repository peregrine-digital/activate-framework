// Package main is the zero-dependency CLI for Activate Framework.
// It discovers manifests, presents an interactive TUI for manifest/tier/target
// selection, and installs files to the chosen location.
//
// Build: go build -o activate ./cli
// Usage: activate [install|list] [flags]
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/peregrine-digital/activate-framework/cli/commands"
	"github.com/peregrine-digital/activate-framework/cli/engine"
	"github.com/peregrine-digital/activate-framework/cli/model"
	"github.com/peregrine-digital/activate-framework/cli/selfupdate"
	"github.com/peregrine-digital/activate-framework/cli/storage"
	"github.com/peregrine-digital/activate-framework/cli/transport"
	"github.com/peregrine-digital/activate-framework/cli/tui"
)

const version = "0.1.6-rc.1"

type cliArgs struct {
	command      string
	configAction string
	repoAction   string
	manifest     string
	tier         string
	target       string
	category     string
	scope        string
	projectDir   string
	file         string
	remote       bool
	stdio        bool
	repo         string
	branch       string
	list         bool
	json         bool
	help         bool
	version      bool
}

func parseArgs(args []string) cliArgs {
	a := cliArgs{
		repo:   storage.DefaultRepo,
		branch: storage.DefaultBranch,
	}

	i := 0
	if i < len(args) && !strings.HasPrefix(args[i], "-") {
		switch args[i] {
		case "menu", "install", "list", "state", "config", "repo", "update", "diff", "sync", "serve", "self-update", "version", "help":
			a.command = args[i]
			i++
		}
	}

	if a.command == "config" && i < len(args) && !strings.HasPrefix(args[i], "-") {
		switch args[i] {
		case "get", "set":
			a.configAction = args[i]
			i++
		}
	}
	if a.command == "repo" && i < len(args) && !strings.HasPrefix(args[i], "-") {
		switch args[i] {
		case "add", "remove":
			a.repoAction = args[i]
			i++
		}
	}

	for ; i < len(args); i++ {
		switch args[i] {
		case "--manifest":
			if i+1 < len(args) {
				i++
				a.manifest = args[i]
			}
		case "--tier":
			if i+1 < len(args) {
				i++
				a.tier = args[i]
			}
		case "--target":
			if i+1 < len(args) {
				i++
				a.target = args[i]
			}
		case "--category":
			if i+1 < len(args) {
				i++
				a.category = args[i]
			}
		case "--scope":
			if i+1 < len(args) {
				i++
				a.scope = args[i]
			}
		case "--project-dir":
			if i+1 < len(args) {
				i++
				a.projectDir = args[i]
			}
		case "--repo":
			if i+1 < len(args) {
				i++
				a.repo = args[i]
			}
		case "--branch":
			if i+1 < len(args) {
				i++
				a.branch = args[i]
			}
		case "--remote":
			a.remote = true
		case "--stdio":
			a.stdio = true
		case "--file":
			if i+1 < len(args) {
				i++
				a.file = args[i]
			}
		case "--list":
			a.list = true
		case "--json":
			a.json = true
		case "--help", "-h":
			a.help = true
		case "--version", "-v":
			a.version = true
		}
	}

	if a.list && a.command == "" {
		a.command = "list"
	}
	if a.command == "" {
		a.command = "menu"
	}

	return a
}

func printUsage() {
	fmt.Printf(`activate v%s — Activate Framework CLI installer

Usage:
  activate [command] [flags]

Commands:
	menu        State-aware interactive menu (default)
	install     Interactive installer (or --file for single file)
	update      Re-install currently installed files
	self-update Update the activate binary to the latest release
	sync        Detect version mismatch and re-inject if needed
	serve       Start JSON-RPC daemon (--stdio for stdio transport)
	diff        Show diff between bundled and installed file
  list        List available manifests and files
	state       Print install/config state (human or JSON)
	config      Read/write config (get/set)
	repo        Add/remove managed files in current repository
  version     Print version
  help        Show this help

Flags:
  --manifest <id>     Select manifest by id
  --tier <tier>       Select tier (e.g. minimal, standard, advanced)
  --target <dir>      Target directory (default: ~/.copilot)
  --file <path>       Target a single file (install, diff)
  --category <cat>    Filter by category (list command)
	--scope <scope>     Config scope: project|global|resolved
	--project-dir <dir> Resolve config/state against this project dir
  --remote            Fetch files from GitHub instead of local bundle
  --repo <owner/repo> GitHub repository (default: %s)
  --branch <name>     Branch or tag (default: %s)
  --json              Machine-readable JSON output (list command)
  -h, --help          Show this help message
  -v, --version       Print version
`, version, storage.DefaultRepo, storage.DefaultBranch)
}

func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func resolveExeDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func main() {
	args := parseArgs(os.Args[1:])

	if args.help || args.command == "help" {
		printUsage()
		os.Exit(0)
	}

	if args.version || args.command == "version" {
		fmt.Printf("activate v%s\n", version)
		os.Exit(0)
	}

	// ── Self-update (no manifests needed) ──────────────────────
	if args.command == "self-update" {
		fmt.Printf("Checking for updates (current: v%s)...\n", version)
		result, err := selfupdate.Run(version)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
		fmt.Println(result.Message)
		os.Exit(0)
	}

	// ── Passive update hint (non-blocking, cached) ─────────────
	if args.command != "serve" {
		if cached := selfupdate.CheckCached(version, ""); cached != nil && cached.UpdateAvail {
			fmt.Fprintf(os.Stderr, "Update available: v%s → v%s (run 'activate self-update')\n\n", cached.CurrentVersion, cached.LatestVersion)
		}
	}

	// ── Discover manifests ──────────────────────────────────────
	var manifests []model.Manifest
	var err error

	if args.remote {
		fmt.Printf("Fetching manifests from %s@%s...\n\n", args.repo, args.branch)
		manifests, err = engine.DiscoverRemoteManifests(args.repo, args.branch)
	} else {
		var bundleDir string
		bundleDir, err = engine.ResolveBundleDir(resolveExeDir())
		if err != nil {
			cwd, _ := os.Getwd()
			bundleDir, err = engine.ResolveBundleDir(cwd)
		}
		if err == nil {
			manifests, err = engine.DiscoverManifests(bundleDir)
		}
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s\n", err)
		os.Exit(1)
	}
	if len(manifests) == 0 {
		fmt.Fprintln(os.Stderr, "No manifests found.")
		os.Exit(1)
	}

	// ── Resolve config ──────────────────────────────────────────
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot determine working directory: %s\n", err)
		os.Exit(1)
	}
	projectDir := cwd
	if strings.TrimSpace(args.projectDir) != "" {
		projectDir = args.projectDir
	}
	overrides := &model.Config{}
	if args.manifest != "" {
		overrides.Manifest = args.manifest
	}
	if args.tier != "" {
		overrides.Tier = args.tier
	}
	cfg := storage.ResolveConfig(projectDir, overrides)

	// ── Create service ─────────────────────────────────────────
	svc := commands.NewService(projectDir, manifests, cfg, args.remote, args.repo, args.branch)

	// ── Dispatch command ────────────────────────────────────────
	switch args.command {
	case "menu":
		if err := tui.RunInteractiveMenu(svc); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "list":
		if err := tui.RunList(svc, args.manifest, args.tier, args.category, args.json, printJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "state":
		if err := runStateCommand(svc, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "update":
		if err := commands.RunUpdateCommand(svc, args.json, printJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "sync":
		if err := commands.RunSyncCommand(svc, args.json, printJSON); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "serve":
		if !args.stdio {
			fmt.Fprintln(os.Stderr, "Error: serve requires --stdio")
			os.Exit(1)
		}
		t := transport.NewTransport(os.Stdin, os.Stdout)
		daemon := commands.NewDaemon(svc, t, version)
		if err := daemon.Serve(); err != nil {
			fmt.Fprintf(os.Stderr, "daemon error: %s\n", err)
			os.Exit(1)
		}

	case "diff":
		if args.file == "" {
			fmt.Fprintln(os.Stderr, "Error: diff requires --file <path>")
			os.Exit(1)
		}
		if err := commands.RunDiffCommand(svc, args.file); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "config":
		if err := runConfigCommand(svc, args, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "repo":
		if err := runRepoCommand(svc, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "install":
		if args.file != "" {
			if err := commands.RunInstallFileCommand(svc, args.file, args.json, printJSON); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			return
		}
		if args.target != "" && args.tier != "" {
			if err := runNonInteractive(manifests, cfg, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			return
		}
		if err := tui.RunInteractiveInstall(svc); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	}
}

func runNonInteractive(manifests []model.Manifest, cfg model.Config, args cliArgs) error {
	target := args.target
	if strings.HasPrefix(target, "~/") {
		home, _ := os.UserHomeDir()
		target = home + target[1:]
	}

	chosen := model.FindManifestByID(manifests, cfg.Manifest)
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	files := model.SelectFiles(chosen.Files, *chosen, cfg.Tier)
	fmt.Printf("\nInstalling %d files to %s:\n\n", len(files), target)

	if args.remote {
		return engine.InstallFilesFromRemote(files, chosen.BasePath, target, chosen.Version, chosen.ID, args.repo, args.branch)
	}
	if err := engine.InstallFiles(files, chosen.BasePath, target, chosen.Version, chosen.ID); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err == nil {
		_ = storage.WriteProjectConfig(cwd, &model.Config{Manifest: chosen.ID, Tier: cfg.Tier})
	}

	fmt.Printf("\nDone. %s v%s (%s) installed.\n", chosen.Name, chosen.Version, cfg.Tier)
	return nil
}

func runStateCommand(svc *commands.ActivateService, jsonOutput bool) error {
	result := svc.GetState()

	if jsonOutput {
		return printJSON(result)
	}

	fmt.Printf("Project: %s\n", result.ProjectDir)
	fmt.Printf("Global config:  %t\n", result.State.HasGlobalConfig)
	fmt.Printf("Project config: %t\n", result.State.HasProjectConfig)
	fmt.Printf("Install marker: %t\n", result.State.HasInstallMarker)
	if result.State.HasInstallMarker {
		fmt.Printf("Installed: %s v%s\n", result.State.InstalledManifest, result.State.InstalledVersion)
	}
	fmt.Printf("Effective config: manifest=%s tier=%s\n", result.Config.Manifest, result.Config.Tier)

	if len(result.Files) == 0 {
		return nil
	}

	groups := make(map[string][]model.FileStatus)
	for _, s := range result.Files {
		groups[s.Category] = append(groups[s.Category], s)
	}

	fmt.Println()
	for _, cat := range model.CategoryOrder {
		files, ok := groups[cat]
		if !ok {
			continue
		}
		label := model.CategoryLabels[cat]
		if label == "" {
			label = cat
		}
		fmt.Printf("── %s ──\n", label)
		for _, f := range files {
			icon := "○"
			if f.Installed {
				icon = "✓"
			}
			suffix := ""
			if f.UpdateAvailable {
				suffix = fmt.Sprintf(" (update: %s → %s)", f.InstalledVersion, f.BundledVersion)
			}
			if f.Skipped {
				suffix += " [skipped]"
			}
			if f.Override != "" {
				suffix += fmt.Sprintf(" [%s]", f.Override)
			}
			fmt.Printf("  %s  %s%s\n", icon, f.Dest, suffix)
		}
	}
	return nil
}

func runConfigCommand(svc *commands.ActivateService, args cliArgs, jsonOutput bool) error {
	action := args.configAction
	if action == "" {
		action = "get"
	}

	scope := args.scope
	if scope == "" {
		if action == "set" {
			scope = "project"
		} else {
			scope = "resolved"
		}
	}

	switch action {
	case "get":
		cfg, err := svc.GetConfig(scope)
		if err != nil {
			return err
		}
		if jsonOutput {
			return printJSON(cfg)
		}
		fmt.Printf("%s config: manifest=%s tier=%s\n", scope, cfg.Manifest, cfg.Tier)
		return nil

	case "set":
		updates := &model.Config{}
		if args.manifest != "" {
			updates.Manifest = args.manifest
		}
		if args.tier != "" {
			updates.Tier = args.tier
		}
		if updates.Manifest == "" && updates.Tier == "" {
			return fmt.Errorf("config set requires --manifest and/or --tier")
		}

		result, err := svc.SetConfig(scope, updates)
		if err != nil {
			return err
		}

		if jsonOutput {
			return printJSON(result)
		}
		fmt.Printf("saved %s config\n", result.Scope)
		return nil
	}

	return fmt.Errorf("invalid config action: %s (use get|set)", action)
}

func runRepoCommand(svc *commands.ActivateService, args cliArgs) error {
	action := args.repoAction
	if action == "" {
		action = "add"
	}

	switch action {
	case "add":
		_, err := svc.RepoAdd()
		return err
	case "remove":
		return svc.RepoRemove()
	default:
		return fmt.Errorf("invalid repo action: %s (use add|remove)", action)
	}
}
