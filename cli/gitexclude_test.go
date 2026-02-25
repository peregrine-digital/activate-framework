package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readExclude(t *testing.T, projectDir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(projectDir, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("reading exclude file: %v", err)
	}
	return string(b)
}

func setupGitDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "info"), 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestEnsureGitExclude_EmptyFile(t *testing.T) {
	dir := setupGitDir(t)
	// Create an empty exclude file
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readExclude(t, dir)
	if !strings.Contains(got, excludeMarkerStart) {
		t.Error("missing start marker")
	}
	if !strings.Contains(got, excludeMarkerEnd) {
		t.Error("missing end marker")
	}
	if !strings.Contains(got, ".activate.json") {
		t.Error("missing .activate.json entry")
	}
}

func TestEnsureGitExclude_Idempotent(t *testing.T) {
	dir := setupGitDir(t)
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("first call: %v", err)
	}
	first := readExclude(t, dir)

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("second call: %v", err)
	}
	second := readExclude(t, dir)

	if first != second {
		t.Errorf("content changed on second call:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
	if strings.Count(second, excludeMarkerStart) != 1 {
		t.Error("start marker duplicated")
	}
}

func TestEnsureGitExclude_AppendsWithTrailingNewline(t *testing.T) {
	dir := setupGitDir(t)
	existing := "*.log\n.DS_Store\n"
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readExclude(t, dir)
	if !strings.HasPrefix(got, existing) {
		t.Error("existing content was not preserved")
	}
	if !strings.Contains(got, excludeMarkerStart) {
		t.Error("missing start marker")
	}
}

func TestEnsureGitExclude_AppendsWithoutTrailingNewline(t *testing.T) {
	dir := setupGitDir(t)
	existing := "*.log\n.DS_Store" // no trailing newline
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readExclude(t, dir)
	// The original content should be followed by a newline before the block
	if !strings.HasPrefix(got, existing+"\n") {
		t.Error("expected a newline to be inserted after existing content without trailing newline")
	}
	if !strings.Contains(got, excludeMarkerStart) {
		t.Error("missing start marker")
	}
}

func TestEnsureGitExclude_CreatesInfoDir(t *testing.T) {
	dir := t.TempDir()
	// Create only .git/ — no info/ subdirectory
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readExclude(t, dir)
	if !strings.Contains(got, excludeMarkerStart) {
		t.Error("missing start marker after creating info dir")
	}
}

func TestEnsureGitExclude_SkipsNonGitRepo(t *testing.T) {
	dir := t.TempDir()
	// No .git directory at all; make it read-only so MkdirAll fails
	if err := os.Chmod(dir, 0555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	err := EnsureGitExclude(dir)
	if err != nil {
		t.Fatalf("expected nil error for non-git repo, got: %v", err)
	}

	// Ensure no .git directory was created
	if _, statErr := os.Stat(filepath.Join(dir, ".git")); statErr == nil {
		t.Error(".git directory should not have been created")
	}
}

func TestEnsureGitExclude_BlockContent(t *testing.T) {
	dir := setupGitDir(t)
	if err := os.WriteFile(filepath.Join(dir, ".git", "info", "exclude"), []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := EnsureGitExclude(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := readExclude(t, dir)

	// Verify the block structure: start marker, then .activate.json, then end marker
	startIdx := strings.Index(got, excludeMarkerStart)
	entryIdx := strings.Index(got, ".activate.json")
	endIdx := strings.Index(got, excludeMarkerEnd)

	if startIdx == -1 || entryIdx == -1 || endIdx == -1 {
		t.Fatalf("block missing components:\n%s", got)
	}
	if !(startIdx < entryIdx && entryIdx < endIdx) {
		t.Errorf("block markers not in expected order (start=%d, entry=%d, end=%d)", startIdx, entryIdx, endIdx)
	}

	// Verify each element is on its own line
	lines := strings.Split(got, "\n")
	var foundStart, foundEntry, foundEnd bool
	for _, line := range lines {
		switch strings.TrimSpace(line) {
		case excludeMarkerStart:
			foundStart = true
		case ".activate.json":
			foundEntry = true
		case excludeMarkerEnd:
			foundEnd = true
		}
	}
	if !foundStart || !foundEntry || !foundEnd {
		t.Errorf("block elements not on separate lines:\n%s", got)
	}
}
