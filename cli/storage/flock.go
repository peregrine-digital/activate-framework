package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// ProjectLockPath returns the advisory lock file path for a project directory.
func ProjectLockPath(projectDir string) string {
	return filepath.Join(RepoStorePath(projectDir), "lock")
}

// GlobalLockPath returns the advisory lock file path for global state.
func GlobalLockPath() string {
	return filepath.Join(StoreBase(), "lock")
}

// WithProjectLock acquires an exclusive file lock scoped to the project
// directory, executes fn, then releases the lock. This prevents multiple
// daemon processes from writing to the same project files simultaneously.
func WithProjectLock(projectDir string, fn func() error) error {
	if projectDir == "" {
		return fn()
	}
	return withLock(ProjectLockPath(projectDir), fn)
}

// WithGlobalLock acquires an exclusive file lock on global state (~/.activate),
// executes fn, then releases the lock.
func WithGlobalLock(fn func() error) error {
	return withLock(GlobalLockPath(), fn)
}

func withLock(lockPath string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return fmt.Errorf("create lock dir: %w", err)
	}

	unlock, err := acquireLock(lockPath)
	if err != nil {
		return fmt.Errorf("acquire lock %s: %w", lockPath, err)
	}
	defer unlock()

	return fn()
}
