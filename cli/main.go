// Package main is the zero-dependency CLI for Activate Framework.
// It discovers manifests, presents an interactive TUI for manifest/tier/target
// selection, and installs files to the chosen location.
//
// Build: go build -o activate ./cli
// Usage: activate [install|list] [flags]
package main

import (
	"fmt"
	"os"
	"strings"
)

const version = "0.1.0"

type cliArgs struct {
	command      string // "menu" (default), "install", "list", "state", "config", "repo", "update", "diff"
	configAction string // "get" or "set" for config command
	repoAction   string // "add" or "remove" for repo command
	manifest     string
	tier         string
	target       string
	category     string
	scope        string // "project", "global", "resolved"
	projectDir   string
	file         string // --file flag for per-file operations
	remote       bool
	repo         string
	branch       string
	list         bool // legacy --list flag
	json         bool
	help         bool
	version      bool
}

func parseArgs(args []string) cliArgs {
	a := cliArgs{
		repo:   DefaultRepo,
		branch: DefaultBranch,
	}

	i := 0
	// Check for subcommand
	if i < len(args) && !strings.HasPrefix(args[i], "-") {
		switch args[i] {
		case "menu", "install", "list", "state", "config", "repo", "update", "diff", "sync", "version", "help":
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

	// Normalize: --list flag maps to list command
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
	sync        Detect version mismatch and re-inject if needed
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
`, version, DefaultRepo, DefaultBranch)
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

	// ── Discover manifests ──────────────────────────────────────
	var manifests []Manifest
	var err error

	if args.remote {
		fmt.Printf("Fetching manifests from %s@%s...\n\n", args.repo, args.branch)
		manifests, err = DiscoverRemoteManifests(args.repo, args.branch)
	} else {
		var bundleDir string
		bundleDir, err = ResolveBundleDir(resolveExeDir())
		if err != nil {
			// Try from cwd as fallback
			cwd, _ := os.Getwd()
			bundleDir, err = ResolveBundleDir(cwd)
		}
		if err == nil {
			manifests, err = DiscoverManifests(bundleDir)
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
	cwd, _ := os.Getwd()
	projectDir := cwd
	if strings.TrimSpace(args.projectDir) != "" {
		projectDir = args.projectDir
	}
	overrides := &Config{}
	if args.manifest != "" {
		overrides.Manifest = args.manifest
	}
	if args.tier != "" {
		overrides.Tier = args.tier
	}
	cfg := ResolveConfig(projectDir, overrides)

	// ── Dispatch command ────────────────────────────────────────
	switch args.command {
	case "menu":
		state := DetectInstallState(projectDir)
		if err := RunInteractiveMenu(manifests, cfg, state, projectDir, args.remote, args.repo, args.branch); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "list":
		if err := RunList(manifests, args.manifest, args.tier, args.category, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "state":
		if err := runStateCommand(manifests, projectDir, cfg, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "update":
		if err := runUpdateCommand(manifests, cfg, projectDir, args.remote, args.repo, args.branch, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "sync":
		if err := runSyncCommand(manifests, cfg, projectDir, args.remote, args.repo, args.branch, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "diff":
		if args.file == "" {
			fmt.Fprintln(os.Stderr, "Error: diff requires --file <path>")
			os.Exit(1)
		}
		if err := runDiffCommand(manifests, cfg, projectDir, args.file); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "config":
		if err := runConfigCommand(projectDir, args, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "repo":
		if err := runRepoCommand(manifests, cfg, projectDir, args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "install":
		// Per-file install
		if args.file != "" {
			if err := runInstallFileCommand(manifests, cfg, projectDir, args.file, args.remote, args.repo, args.branch, args.json); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			return
		}
		// If --target is set with --tier, run non-interactive
		if args.target != "" && args.tier != "" {
			if err := runNonInteractive(manifests, cfg, args); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s\n", err)
				os.Exit(1)
			}
			return
		}
		if err := RunInteractiveInstall(manifests, cfg, args.remote, args.repo, args.branch); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}
	}
}

func runNonInteractive(manifests []Manifest, cfg Config, args cliArgs) error {
	return installWithResolvedConfig(manifests, cfg, args.target, args.remote, args.repo, args.branch)
}

func installWithResolvedConfig(manifests []Manifest, cfg Config, target string, useRemote bool, repo, branch string) error {
	// Find manifest
	var chosen *Manifest
	for i, m := range manifests {
		if m.ID == cfg.Manifest {
			chosen = &manifests[i]
			break
		}
	}
	if chosen == nil {
		return fmt.Errorf("unknown manifest: %s", cfg.Manifest)
	}

	if strings.HasPrefix(target, "~/") {
		home, _ := os.UserHomeDir()
		target = home + target[1:]
	}

	files := SelectFiles(chosen.Files, *chosen, cfg.Tier)
	fmt.Printf("\nInstalling %d files to %s:\n\n", len(files), target)

	if useRemote {
		return InstallFilesFromRemote(files, chosen.BasePath, target, chosen.Version, chosen.ID, repo, branch)
	}
	if err := InstallFiles(files, chosen.BasePath, target, chosen.Version, chosen.ID); err != nil {
		return err
	}

	cwd, _ := os.Getwd()
	_ = WriteProjectConfig(cwd, &Config{Manifest: chosen.ID, Tier: cfg.Tier})
	_ = EnsureGitExclude(cwd)

	fmt.Printf("\nDone. %s v%s (%s) installed.\n", chosen.Name, chosen.Version, cfg.Tier)
	return nil
}

func runStateCommand(manifests []Manifest, projectDir string, cfg Config, jsonOutput bool) error {
	state := DetectInstallState(projectDir)
	sidecar, _ := readRepoSidecar(projectDir)

	chosen := findManifestByID(manifests, cfg.Manifest)

	if jsonOutput {
		out := map[string]interface{}{
			"projectDir": projectDir,
			"state":      state,
			"config":     cfg,
		}
		if chosen != nil {
			out["files"] = ComputeFileStatuses(*chosen, sidecar, cfg, projectDir)
		}
		return printJSON(out)
	}

	fmt.Printf("Project: %s\n", projectDir)
	fmt.Printf("Global config:  %t\n", state.HasGlobalConfig)
	fmt.Printf("Project config: %t\n", state.HasProjectConfig)
	fmt.Printf("Install marker: %t\n", state.HasInstallMarker)
	if state.HasInstallMarker {
		fmt.Printf("Installed: %s v%s\n", state.InstalledManifest, state.InstalledVersion)
	}
	fmt.Printf("Effective config: manifest=%s tier=%s\n", cfg.Manifest, cfg.Tier)

	if chosen == nil {
		return nil
	}

	statuses := ComputeFileStatuses(*chosen, sidecar, cfg, projectDir)
	groups := make(map[string][]FileStatus)
	for _, s := range statuses {
		groups[s.Category] = append(groups[s.Category], s)
	}

	fmt.Println()
	for _, cat := range categoryOrder {
		files, ok := groups[cat]
		if !ok {
			continue
		}
		label := categoryLabels[cat]
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

func runConfigCommand(projectDir string, args cliArgs, jsonOutput bool) error {
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
		switch scope {
		case "global":
			cfg, _ := ReadGlobalConfig()
			if cfg == nil {
				cfg = &Config{}
			}
			if jsonOutput {
				return printJSON(cfg)
			}
			fmt.Printf("global config: manifest=%s tier=%s\n", cfg.Manifest, cfg.Tier)
			return nil

		case "project":
			cfg, _ := ReadProjectConfig(projectDir)
			if cfg == nil {
				cfg = &Config{}
			}
			if jsonOutput {
				return printJSON(cfg)
			}
			fmt.Printf("project config: manifest=%s tier=%s\n", cfg.Manifest, cfg.Tier)
			return nil

		case "resolved":
			cfg := ResolveConfig(projectDir, nil)
			if jsonOutput {
				return printJSON(cfg)
			}
			fmt.Printf("resolved config: manifest=%s tier=%s\n", cfg.Manifest, cfg.Tier)
			return nil
		}

	case "set":
		updates := &Config{}
		if args.manifest != "" {
			updates.Manifest = args.manifest
		}
		if args.tier != "" {
			updates.Tier = args.tier
		}
		if updates.Manifest == "" && updates.Tier == "" {
			return fmt.Errorf("config set requires --manifest and/or --tier")
		}

		switch scope {
		case "global":
			if err := WriteGlobalConfig(updates); err != nil {
				return err
			}
		case "project":
			if err := WriteProjectConfig(projectDir, updates); err != nil {
				return err
			}
			_ = EnsureGitExclude(projectDir)
		default:
			return fmt.Errorf("invalid --scope for config set: %s (use project|global)", scope)
		}

		if jsonOutput {
			return printJSON(map[string]interface{}{
				"ok":    true,
				"action": "set",
				"scope": scope,
			})
		}
		fmt.Printf("saved %s config\n", scope)
		return nil
	}

	return fmt.Errorf("invalid config action: %s (use get|set)", action)
}

func runRepoCommand(manifests []Manifest, cfg Config, projectDir string, args cliArgs) error {
	action := args.repoAction
	if action == "" {
		action = "add"
	}

	switch action {
	case "add":
		return RepoAdd(manifests, cfg, projectDir, args.remote, args.repo, args.branch)
	case "remove":
		return RepoRemove(projectDir)
	default:
		return fmt.Errorf("invalid repo action: %s (use add|remove)", action)
	}
}
