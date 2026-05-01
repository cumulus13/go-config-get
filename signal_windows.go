//go:build windows

package configget

import "log/slog"

// WatchSignal on Windows is a no-op stub — SIGHUP does not exist on Windows.
// Use Watch (mtime polling) instead for cross-platform hot-reload.
func (c *ConfigGet) WatchSignal(wopts WatchOptions) *SIGHUPWatcher {
	slog.Warn("config-get: WatchSignal is not supported on Windows; use Watch() instead")
	sw := &SIGHUPWatcher{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
	// doneCh is never closed naturally; Stop() will close stopCh which the
	// no-op goroutine below watches so Stop() doesn't deadlock.
	go func() {
		<-sw.stopCh
		close(sw.doneCh)
	}()
	return sw
}
