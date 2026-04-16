//go:build windows

package storage

import (
	"os"

	"golang.org/x/sys/windows"
)

// acquireLock opens (or creates) the lock file and acquires an exclusive
// lock using LockFileEx. The returned function releases the lock and closes
// the file. The OS automatically releases the lock if the process exits.
func acquireLock(path string) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	ol := &windows.Overlapped{}
	err = windows.LockFileEx(
		windows.Handle(f.Fd()),
		windows.LOCKFILE_EXCLUSIVE_LOCK,
		0,
		1, 0,
		ol,
	)
	if err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, ol)
		f.Close()
	}, nil
}
