//go:build !windows

package storage

import (
	"os"
	"syscall"
)

// acquireLock opens (or creates) the lock file and acquires an exclusive
// advisory lock using flock(2). The returned function releases the lock
// and closes the file. The OS automatically releases the lock if the
// process exits while holding it.
func acquireLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
