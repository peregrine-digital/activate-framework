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
	command  string // "install" (default) or "list"
	manifest string
	tier     string
	target   string
	category string
	remote   bool
	repo     string
	branch   string
	list     bool // legacy --list flag
	json     bool
	help     bool
	version  bool
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
		case "install", "list", "version", "help":
			a.command = args[i]
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
		a.command = "install"
	}

	return a
}

func printUsage() {
	fmt.Printf(`activate v%s — Activate Framework CLI installer

Usage:
  activate [command] [flags]

Commands:
  install     Interactive installer (default)
  list        List available manifests and files
  version     Print version
  help        Show this help

Flags:
  --manifest <id>     Select manifest by id
  --tier <tier>       Select tier (e.g. minimal, standard, advanced)
  --target <dir>      Target directory (default: ~/.copilot)
  --category <cat>    Filter by category (list command)
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
	overrides := &Config{}
	if args.manifest != "" {
		overrides.Manifest = args.manifest
	}
	if args.tier != "" {
		overrides.Tier = args.tier
	}
	cfg := ResolveConfig(cwd, overrides)

	// ── Dispatch command ────────────────────────────────────────
	switch args.command {
	case "list":
		if err := RunList(manifests, args.manifest, args.tier, args.category, args.json); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %s\n", err)
			os.Exit(1)
		}

	case "install":
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

	target := args.target
	if strings.HasPrefix(target, "~/") {
		home, _ := os.UserHomeDir()
		target = home + target[1:]
	}

	files := SelectFiles(chosen.Files, *chosen, cfg.Tier)
	fmt.Printf("\nInstalling %d files to %s:\n\n", len(files), target)

	if args.remote {
		return InstallFilesFromRemote(files, chosen.BasePath, target, chosen.Version, chosen.ID, args.repo, args.branch)
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
