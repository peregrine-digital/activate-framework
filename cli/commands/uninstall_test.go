package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/peregrine-digital/activate-framework/cli/storage"
)

func TestRunUninstall_RemovesActivateDir(t *testing.T) {
	// Set up a fake ~/.activate in a temp dir
	tmp := t.TempDir()
	storage.ActivateBaseDir = tmp
	t.Cleanup(func() { storage.ActivateBaseDir = "" })

	// Create fake HOME so shell cleanup doesn't touch real profiles
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	// Create a fake workspace with injected files
	fakeProject := t.TempDir()
	githubDir := filepath.Join(fakeProject, ".github", "instructions")
	os.MkdirAll(githubDir, 0755)
	os.WriteFile(filepath.Join(githubDir, "test.md"), []byte("injected"), 0644)

	// Create .git/info/exclude with managed block
	gitInfoDir := filepath.Join(fakeProject, ".git", "info")
	os.MkdirAll(gitInfoDir, 0755)
	excludeContent := "# existing\n\n# >>> Peregrine Activate (managed — do not edit)\n.github/instructions/test.md\n# <<< Peregrine Activate\n"
	os.WriteFile(filepath.Join(gitInfoDir, "exclude"), []byte(excludeContent), 0644)

	// Create the sidecar and repo metadata in ~/.activate/repos/<hash>/
	repoStore := storage.RepoStorePath(fakeProject)
	os.MkdirAll(repoStore, 0755)
	absProject, _ := filepath.Abs(fakeProject)
	metaJSON, _ := json.Marshal(struct{ Path string }{Path: absProject})
	os.WriteFile(filepath.Join(repoStore, "repo.json"), metaJSON, 0644)
	sidecarJSON := []byte(`{"files":[".github/instructions/test.md"]}`)
	os.WriteFile(filepath.Join(repoStore, "installed.json"), sidecarJSON, 0644)

	// Create binary and config
	binDir := filepath.Join(tmp, "bin")
	os.MkdirAll(binDir, 0755)
	os.WriteFile(filepath.Join(binDir, "activate"), []byte("binary"), 0755)
	os.WriteFile(filepath.Join(tmp, "config.json"), []byte(`{}`), 0644)

	// Create a fake shell profile with the marker
	zshenv := filepath.Join(fakeHome, ".zshenv")
	os.WriteFile(zshenv, []byte("export FOO=bar\n\n# Added by Activate CLI installer\nexport PATH=\"/fake/.activate/bin:$PATH\"\n"), 0644)

	// Run uninstall
	if err := RunUninstall(true); err != nil {
		t.Fatalf("RunUninstall failed: %v", err)
	}

	// Verify ~/.activate is gone
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed", tmp)
	}

	// Verify injected file was removed from workspace
	injectedFile := filepath.Join(githubDir, "test.md")
	if _, err := os.Stat(injectedFile); !os.IsNotExist(err) {
		t.Errorf("expected injected file %s to be removed", injectedFile)
	}

	// Verify .git/info/exclude was cleaned
	excludeData, _ := os.ReadFile(filepath.Join(gitInfoDir, "exclude"))
	if contains(string(excludeData), "Peregrine Activate") {
		t.Errorf("expected managed block removed from .git/info/exclude, got:\n%s", string(excludeData))
	}

	// Verify shell profile was cleaned
	data, _ := os.ReadFile(zshenv)
	if contains(string(data), "Activate CLI installer") {
		t.Errorf("expected marker removed from zshenv")
	}
}

func TestRemoveMarkerBlock(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, ".zshenv")

	content := `# existing stuff
export FOO=bar

# Added by Activate CLI installer
export PATH="/Users/test/.activate/bin:$PATH"

# other stuff
export BAZ=qux
`
	os.WriteFile(profile, []byte(content), 0644)

	changed := removeMarkerBlock(profile)
	if !changed {
		t.Fatal("expected removeMarkerBlock to return true")
	}

	result, _ := os.ReadFile(profile)
	got := string(result)

	if contains(got, "Activate CLI installer") {
		t.Errorf("marker line should be removed, got:\n%s", got)
	}
	if contains(got, ".activate/bin") {
		t.Errorf("PATH export should be removed, got:\n%s", got)
	}
	if !contains(got, "export FOO=bar") {
		t.Errorf("existing content should be preserved, got:\n%s", got)
	}
	if !contains(got, "export BAZ=qux") {
		t.Errorf("trailing content should be preserved, got:\n%s", got)
	}
}

func TestRemoveMarkerBlock_NoMarker(t *testing.T) {
	tmp := t.TempDir()
	profile := filepath.Join(tmp, ".bashrc")

	content := "export FOO=bar\n"
	os.WriteFile(profile, []byte(content), 0644)

	changed := removeMarkerBlock(profile)
	if changed {
		t.Error("expected no change when marker is absent")
	}
}

func TestRemoveMarkerBlock_MissingFile(t *testing.T) {
	changed := removeMarkerBlock("/nonexistent/file")
	if changed {
		t.Error("expected false for missing file")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
