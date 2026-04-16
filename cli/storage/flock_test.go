package storage

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithProjectLock_MutualExclusion(t *testing.T) {
	old := ActivateBaseDir
	ActivateBaseDir = t.TempDir()
	defer func() { ActivateBaseDir = old }()

	projectDir := t.TempDir()
	// Ensure the repo store directory exists for the lock file.
	if err := os.MkdirAll(RepoStorePath(projectDir), 0755); err != nil {
		t.Fatal(err)
	}

	var running atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithProjectLock(projectDir, func() error {
				cur := running.Add(1)
				// Track the highest concurrency observed
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				running.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("WithProjectLock: %v", err)
			}
		}()
	}

	wg.Wait()

	if mc := maxConcurrent.Load(); mc > 1 {
		t.Errorf("max concurrent = %d, want 1 (lock did not provide mutual exclusion)", mc)
	}
}

func TestWithProjectLock_EmptyDir_SkipsLock(t *testing.T) {
	called := false
	err := WithProjectLock("", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("fn was not called when projectDir is empty")
	}
}

func TestWithGlobalLock_MutualExclusion(t *testing.T) {
	old := ActivateBaseDir
	ActivateBaseDir = t.TempDir()
	defer func() { ActivateBaseDir = old }()

	var running atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := WithGlobalLock(func() error {
				cur := running.Add(1)
				for {
					old := maxConcurrent.Load()
					if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				running.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("WithGlobalLock: %v", err)
			}
		}()
	}

	wg.Wait()

	if mc := maxConcurrent.Load(); mc > 1 {
		t.Errorf("max concurrent = %d, want 1 (lock did not provide mutual exclusion)", mc)
	}
}

func TestWithProjectLock_CreatesLockFile(t *testing.T) {
	old := ActivateBaseDir
	ActivateBaseDir = t.TempDir()
	defer func() { ActivateBaseDir = old }()

	projectDir := t.TempDir()

	err := WithProjectLock(projectDir, func() error { return nil })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lockPath := filepath.Join(RepoStorePath(projectDir), "lock")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file was not created")
	}
}

func TestWithProjectLock_PropagatesError(t *testing.T) {
	old := ActivateBaseDir
	ActivateBaseDir = t.TempDir()
	defer func() { ActivateBaseDir = old }()

	projectDir := t.TempDir()
	sentinel := os.ErrPermission

	err := WithProjectLock(projectDir, func() error {
		return sentinel
	})
	if err != sentinel {
		t.Errorf("got %v, want %v", err, sentinel)
	}
}
