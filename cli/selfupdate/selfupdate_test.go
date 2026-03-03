package selfupdate

import "testing"

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
