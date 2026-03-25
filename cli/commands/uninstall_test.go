package commands

import (
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

	// Create the directory structure
	binDir := filepath.Join(tmp, "bin")
	reposDir := filepath.Join(tmp, "repos", "abc123")
	os.MkdirAll(binDir, 0755)
	os.MkdirAll(reposDir, 0755)
	os.WriteFile(filepath.Join(binDir, "activate"), []byte("binary"), 0755)
	os.WriteFile(filepath.Join(tmp, "config.json"), []byte(`{}`), 0644)
	os.WriteFile(filepath.Join(reposDir, "installed.json"), []byte(`{}`), 0644)

	// Create a fake shell profile with the marker
	zshenv := filepath.Join(fakeHome, ".zshenv")
	os.WriteFile(zshenv, []byte("export FOO=bar\n\n# Added by Activate CLI installer\nexport PATH=\"/fake/.activate/bin:$PATH\"\n"), 0644)

	// Run uninstall with force (skip prompt)
	if err := RunUninstall(true); err != nil {
		t.Fatalf("RunUninstall failed: %v", err)
	}

	// Verify directory is gone
	if _, err := os.Stat(tmp); !os.IsNotExist(err) {
		t.Errorf("expected %s to be removed, but it still exists", tmp)
	}

	// Verify shell profile was cleaned
	data, _ := os.ReadFile(zshenv)
	if contains(string(data), "Activate CLI installer") {
		t.Errorf("expected marker removed from zshenv, got:\n%s", string(data))
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
