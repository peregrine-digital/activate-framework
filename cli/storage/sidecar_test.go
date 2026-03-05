package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

func TestSyncRepoGitExcludeIfPresentSkipsWhenMissing(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	if err := SyncGitExclude(projectDir, []string{".github/test.md"}); err != nil {
		t.Fatal(err)
	}
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if _, err := os.Stat(excludePath); !os.IsNotExist(err) {
		t.Fatalf("expected exclude file to remain missing, err=%v", err)
	}
}

func TestWriteAndDeleteRepoSidecarLifecycle(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte("base-entry\n"), 0644); err != nil {
		t.Fatal(err)
	}

	oldFile := filepath.Join(projectDir, ".github", "old.md")
	keepFile := filepath.Join(projectDir, ".github", "keep.md")
	if err := os.MkdirAll(filepath.Dir(oldFile), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldFile, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keepFile, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	prev := model.RepoSidecar{Manifest: "m1", Tier: "minimal", Files: []string{".github/old.md", ".github/keep.md"}}
	prevData, _ := json.Marshal(prev)
	scPath := SidecarPath(projectDir)
	if err := os.MkdirAll(filepath.Dir(scPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scPath, prevData, 0644); err != nil {
		t.Fatal(err)
	}

	next := model.RepoSidecar{Manifest: "m1", Tier: "minimal", Files: []string{".github/keep.md"}}
	if err := WriteRepoSidecar(projectDir, next); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected old file to be removed, err=%v", err)
	}
	if _, err := os.Stat(keepFile); err != nil {
		t.Fatalf("expected keep file to remain, err=%v", err)
	}

	excludeData, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	excludeText := string(excludeData)
	startMark := "# >>> Peregrine Activate (managed — do not edit)"
	if !strings.Contains(excludeText, startMark) || !strings.Contains(excludeText, ".github/keep.md") {
		t.Fatalf("expected managed exclude block with keep path, got:\n%s", excludeText)
	}

	if err := DeleteRepoSidecar(projectDir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SidecarPath(projectDir)); !os.IsNotExist(err) {
		t.Fatalf("expected sidecar deleted, err=%v", err)
	}
	if _, err := os.Stat(keepFile); !os.IsNotExist(err) {
		t.Fatalf("expected keep file deleted by sidecar cleanup, err=%v", err)
	}

	excludeData, err = os.ReadFile(excludePath)
	if err != nil {
		t.Fatal(err)
	}
	excludeText = string(excludeData)
	endMark := "# <<< Peregrine Activate"
	if strings.Contains(excludeText, startMark) || strings.Contains(excludeText, endMark) {
		t.Fatalf("expected managed exclude block removed, got:\n%s", excludeText)
	}
	if !strings.Contains(excludeText, "base-entry") {
		t.Fatalf("expected baseline exclude content preserved, got:\n%s", excludeText)
	}
}

func TestWriteRepoSidecarDeletesStaleFiles(t *testing.T) {
	setupTestStore(t)
	projectDir := t.TempDir()

	// Set up .git/info/exclude
	excludePath := filepath.Join(projectDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(excludePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(excludePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	// Create files a, b, c on disk
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		p := filepath.Join(projectDir, ".github", name)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("content-"+name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Write initial sidecar with [a, b, c]
	first := model.RepoSidecar{
		Manifest: "m1", Tier: "minimal",
		Files: []string{".github/a.md", ".github/b.md", ".github/c.md"},
	}
	if err := WriteRepoSidecar(projectDir, first); err != nil {
		t.Fatal(err)
	}

	// All three should exist
	for _, name := range []string{"a.md", "b.md", "c.md"} {
		if _, err := os.Stat(filepath.Join(projectDir, ".github", name)); err != nil {
			t.Fatalf("expected %s to exist after first write, err=%v", name, err)
		}
	}

	// Write again with only [a, b]
	second := model.RepoSidecar{
		Manifest: "m1", Tier: "minimal",
		Files: []string{".github/a.md", ".github/b.md"},
	}
	if err := WriteRepoSidecar(projectDir, second); err != nil {
		t.Fatal(err)
	}

	// a and b should still exist
	for _, name := range []string{"a.md", "b.md"} {
		if _, err := os.Stat(filepath.Join(projectDir, ".github", name)); err != nil {
			t.Fatalf("expected %s to still exist, err=%v", name, err)
		}
	}
	// c should be deleted
	if _, err := os.Stat(filepath.Join(projectDir, ".github", "c.md")); !os.IsNotExist(err) {
		t.Fatalf("expected c.md to be deleted, err=%v", err)
	}
}
