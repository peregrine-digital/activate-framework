package commands

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/peregrine-digital/activate-framework/cli/storage"
	"github.com/peregrine-digital/activate-framework/cli/transport"
)

// configWatcher watches config and sidecar files for external changes
// and sends stateChanged notifications to the client.
type configWatcher struct {
	watcher   *fsnotify.Watcher
	transport *transport.Transport
	mu        sync.Mutex
	done      chan struct{}
}

func newConfigWatcher(t *transport.Transport) (*configWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &configWatcher{
		watcher:   w,
		transport: t,
		done:      make(chan struct{}),
	}, nil
}

// watchPaths sets up watches on directories containing config and sidecar files.
// We watch directories instead of individual files because:
// 1. fsnotify.Add() fails if the file doesn't exist yet
// 2. Directory watches catch creates, writes, AND removes of files within
// This enables cross-process sync (e.g., extension and desktop app).
func (cw *configWatcher) watchPaths(projectDir string) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Watch ~/.activate/ directory (contains config.json)
	globalDir := storage.StoreBase()
	_ = cw.watcher.Add(globalDir)

	if projectDir != "" {
		// Watch ~/.activate/repos/<hash>/ directory (contains config.json + installed.json)
		repoDir := storage.RepoStorePath(projectDir)
		_ = cw.watcher.Add(repoDir)
	}
}

// run processes file events with debouncing. It blocks until close() is called.
func (cw *configWatcher) run() {
	// Debounce: coalesce rapid changes into a single notification
	const debounce = 150 * time.Millisecond
	var timer *time.Timer

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}
			// Reset debounce timer on each event
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(debounce, func() {
				_ = cw.transport.WriteNotification(transport.StateChangedNotification())
			})

		case _, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			// Silently ignore watcher errors — non-fatal

		case <-cw.done:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

func (cw *configWatcher) close() {
	close(cw.done)
	cw.watcher.Close()
}
