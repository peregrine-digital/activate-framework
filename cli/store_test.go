package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoStorePath_Deterministic(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	defer func() { activateBaseDir = old }()

	dir := "/Users/test/my-project"
	path1 := repoStorePath(dir)
	path2 := repoStorePath(dir)
	if path1 != path2 {
		t.Errorf("repoStorePath not deterministic: %q != %q", path1, path2)
	}
}

func TestRepoStorePath_DifferentPaths(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	defer func() { activateBaseDir = old }()

	p1 := repoStorePath("/Users/test/project-a")
	p2 := repoStorePath("/Users/test/project-b")
	if p1 == p2 {
		t.Error("different project dirs should produce different store paths")
	}
}

func TestEnsureRepoMeta_CreatesMetadata(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	defer func() { activateBaseDir = old }()

	projectDir := t.TempDir()
	if err := ensureRepoMeta(projectDir); err != nil {
		t.Fatalf("ensureRepoMeta: %v", err)
	}

	metaPath := filepath.Join(repoStorePath(projectDir), "repo.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read repo.json: %v", err)
	}

	var meta repoMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal repo.json: %v", err)
	}

	abs, _ := filepath.Abs(projectDir)
	if meta.Path != abs {
		t.Errorf("repo.json path = %q, want %q", meta.Path, abs)
	}
}

func TestEnsureRepoMeta_Idempotent(t *testing.T) {
	old := activateBaseDir
	activateBaseDir = t.TempDir()
	defer func() { activateBaseDir = old }()

	projectDir := t.TempDir()
	if err := ensureRepoMeta(projectDir); err != nil {
		t.Fatal(err)
	}

	metaPath := filepath.Join(repoStorePath(projectDir), "repo.json")
	info1, _ := os.Stat(metaPath)

	if err := ensureRepoMeta(projectDir); err != nil {
		t.Fatal(err)
	}

	info2, _ := os.Stat(metaPath)
	if info1.ModTime() != info2.ModTime() {
		t.Error("ensureRepoMeta should not rewrite existing repo.json")
	}
}
