package main

import (
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/storage"
)

func TestParseArgsDefaultsToMenu(t *testing.T) {
	args := parseArgs([]string{})
	if args.command != "menu" {
		t.Fatalf("expected default command menu, got %q", args.command)
	}
	if args.repo != storage.DefaultRepo {
		t.Fatalf("expected default repo %q, got %q", storage.DefaultRepo, args.repo)
	}
	if args.branch != storage.DefaultBranch {
		t.Fatalf("expected default branch %q, got %q", storage.DefaultBranch, args.branch)
	}
}

func TestParseArgsConfigSet(t *testing.T) {
	args := parseArgs([]string{
		"config", "set",
		"--scope", "project",
		"--manifest", "ironarch",
		"--tier", "minimal",
		"--project-dir", "/tmp/demo",
	})

	if args.command != "config" {
		t.Fatalf("expected command config, got %q", args.command)
	}
	if args.configAction != "set" {
		t.Fatalf("expected config action set, got %q", args.configAction)
	}
	if args.scope != "project" || args.manifest != "ironarch" || args.tier != "minimal" {
		t.Fatalf("unexpected parsed config args: %+v", args)
	}
	if args.projectDir != "/tmp/demo" {
		t.Fatalf("expected project dir /tmp/demo, got %q", args.projectDir)
	}
}

func TestParseArgsRepoRemove(t *testing.T) {
	args := parseArgs([]string{"repo", "remove", "--remote", "--repo", "x/y", "--branch", "dev"})
	if args.command != "repo" {
		t.Fatalf("expected command repo, got %q", args.command)
	}
	if args.repoAction != "remove" {
		t.Fatalf("expected repo action remove, got %q", args.repoAction)
	}
	if !args.remote || args.repo != "x/y" || args.branch != "dev" {
		t.Fatalf("unexpected parsed repo args: %+v", args)
	}
}

func TestParseArgsLegacyListFlag(t *testing.T) {
	args := parseArgs([]string{"--list", "--json"})
	if args.command != "list" {
		t.Fatalf("expected command list from --list flag, got %q", args.command)
	}
	if !args.json {
		t.Fatalf("expected json flag to be true")
	}
}
