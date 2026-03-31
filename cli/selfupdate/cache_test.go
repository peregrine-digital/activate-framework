package selfupdate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAndReadCache(t *testing.T) {
	// Use a temp dir to avoid touching the real ~/.activate
	tmp := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", tmp)
	defer os.Setenv("HOME", origHome)

	entry := &CacheEntry{
		CheckedAt:      time.Now().Truncate(time.Second),
		LatestVersion:  "1.2.3",
		CurrentVersion: "1.0.0",
		UpdateAvail:    true,
	}

	if err := WriteCache(entry); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}

	// Verify file exists
	p := filepath.Join(tmp, ".activate", CacheFileName)
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("cache file not created: %v", err)
	}

	got, err := ReadCache()
	if err != nil {
		t.Fatalf("ReadCache: %v", err)
	}
	if got.LatestVersion != "1.2.3" || got.CurrentVersion != "1.0.0" || !got.UpdateAvail {
		t.Fatalf("unexpected cache entry: %+v", got)
	}
}

func TestReadCacheMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	_, err := ReadCache()
	if err == nil {
		t.Fatal("expected error for missing cache file")
	}
}

func TestCheckCachedReturnsCachedWhenFresh(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	entry := &CacheEntry{
		CheckedAt:      time.Now(),
		LatestVersion:  "2.0.0",
		CurrentVersion: "1.0.0",
		UpdateAvail:    true,
	}
	if err := WriteCache(entry); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}

	got := CheckCached("1.0.0", "", "", "")
	if got == nil {
		t.Fatal("expected cached result")
	}
	if got.LatestVersion != "2.0.0" {
		t.Fatalf("expected cached latestVersion 2.0.0, got %q", got.LatestVersion)
	}
}

func TestCheckCachedStaleTriggersRefresh(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write a stale cache entry
	entry := &CacheEntry{
		CheckedAt:      time.Now().Add(-48 * time.Hour),
		LatestVersion:  "2.0.0",
		CurrentVersion: "1.0.0",
		UpdateAvail:    true,
	}
	if err := WriteCache(entry); err != nil {
		t.Fatalf("WriteCache: %v", err)
	}

	// CheckCached will try a live check which may fail (no network in tests).
	// It should return nil on network error, not the stale entry.
	got := CheckCached("1.0.0", "", "", "")
	// Either nil (network error) or fresh entry — stale entry should NOT be returned as-is.
	if got != nil && got.CheckedAt.Equal(entry.CheckedAt) {
		t.Fatal("stale cache entry should not be returned without refresh")
	}
}
