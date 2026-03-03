package selfupdate

import (
	"runtime"
	"testing"
)

func TestCheckUpdateInvalidVersion(t *testing.T) {
	// A nonsensical version should still produce a result (not panic).
	// This exercises the wiring without requiring network access.
	result, err := CheckUpdate("0.0.0-test", "")
	if err != nil {
		// Network errors are expected in CI/offline environments; skip gracefully.
		t.Skipf("skipping: network error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.CurrentVersion != "0.0.0-test" {
		t.Fatalf("expected currentVersion 0.0.0-test, got %q", result.CurrentVersion)
	}
}

func TestResultFields(t *testing.T) {
	r := Result{
		Updated:        true,
		CurrentVersion: "0.1.0",
		LatestVersion:  "0.2.0",
		Message:        "updated",
	}
	if !r.Updated {
		t.Fatal("expected Updated to be true")
	}
	if r.CurrentVersion != "0.1.0" || r.LatestVersion != "0.2.0" {
		t.Fatalf("unexpected version fields: %+v", r)
	}
}

// ── updaterConfig tests ─────────────────────────────────────────

func TestUpdaterConfigPrereleaseEnabled(t *testing.T) {
	// Pre-release versions MUST set Prerelease: true so go-selfupdate
	// includes pre-release GitHub releases in results.
	cases := []string{"0.1.0-rc.1", "0.2.0-beta", "1.0.0-alpha.2", "0.1.0+build123"}
	for _, v := range cases {
		cfg, err := updaterConfig(v, "")
		if err != nil {
			t.Fatalf("updaterConfig(%q): %v", v, err)
		}
		if !cfg.Prerelease {
			t.Errorf("updaterConfig(%q).Prerelease = false, want true", v)
		}
	}
}

func TestUpdaterConfigPrereleaseDisabled(t *testing.T) {
	// Stable versions MUST NOT set Prerelease: true so users on stable
	// only see stable releases.
	cases := []string{"0.1.0", "1.0.0", "2.3.4"}
	for _, v := range cases {
		cfg, err := updaterConfig(v, "")
		if err != nil {
			t.Fatalf("updaterConfig(%q): %v", v, err)
		}
		if cfg.Prerelease {
			t.Errorf("updaterConfig(%q).Prerelease = true, want false", v)
		}
	}
}

func TestUpdaterConfigSetsOSArch(t *testing.T) {
	cfg, err := updaterConfig("1.0.0", "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OS != runtime.GOOS {
		t.Errorf("OS = %q, want %q", cfg.OS, runtime.GOOS)
	}
	if cfg.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", cfg.Arch, runtime.GOARCH)
	}
}

func TestUpdaterConfigAcceptsToken(t *testing.T) {
	// Verify config builds without error when a token is provided.
	// The token is passed to the Source, which we can't inspect directly,
	// but we can verify no error occurs.
	cfg, err := updaterConfig("0.1.0-rc.1", "ghp_testtoken123")
	if err != nil {
		t.Fatalf("updaterConfig with token: %v", err)
	}
	if cfg.Source == nil {
		t.Fatal("expected non-nil Source when token is provided")
	}
}
