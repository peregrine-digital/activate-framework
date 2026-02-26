package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/model"
)

func TestInstallFiles_CopiesFiles(t *testing.T) {
	src := t.TempDir()
	dest := t.TempDir()

	// Create source file
	srcDir := filepath.Join(src, "instructions")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "a.md"), []byte("hello"), 0644)

	files := []model.ManifestFile{
		{Src: "instructions/a.md", Dest: ".github/instructions/a.md"},
	}
	if err := InstallFiles(files, src, dest, "1.0.0", "test"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dest, ".github", "instructions", "a.md"))
	if err != nil {
		t.Fatalf("file not copied: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestInstallFiles_MissingSrc(t *testing.T) {
	dest := t.TempDir()
	files := []model.ManifestFile{
		{Src: "no/such/file.md", Dest: "out.md"},
	}
	err := InstallFiles(files, "/nonexistent", dest, "1.0.0", "test")
	if err == nil {
		t.Fatal("expected error for missing source")
	}
}

func TestResolveBundleDir_DirectMatch(t *testing.T) {
	dir := t.TempDir()
	mDir := filepath.Join(dir, "manifests")
	os.MkdirAll(mDir, 0755)
	os.WriteFile(filepath.Join(mDir, "test.json"), []byte("{}"), 0644)

	result, err := ResolveBundleDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != dir {
		t.Fatalf("expected %q, got %q", dir, result)
	}
}

func TestResolveBundleDir_WalksUp(t *testing.T) {
	root := t.TempDir()
	mDir := filepath.Join(root, "manifests")
	os.MkdirAll(mDir, 0755)
	os.WriteFile(filepath.Join(mDir, "test.json"), []byte("{}"), 0644)

	child := filepath.Join(root, "sub", "deep")
	os.MkdirAll(child, 0755)

	result, err := ResolveBundleDir(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != root {
		t.Fatalf("expected %q, got %q", root, result)
	}
}

func TestResolveBundleDir_NotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := ResolveBundleDir(dir)
	if err == nil {
		t.Fatal("expected error when no manifests found")
	}
}

func TestResolveBundleDir_LegacyManifestJSON(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{}"), 0644)

	result, err := ResolveBundleDir(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != dir {
		t.Fatalf("expected %q, got %q", dir, result)
	}
}
