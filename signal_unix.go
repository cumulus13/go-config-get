//go:build !windows

package configget

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// WatchSignal starts a SIGHUP-based config reloader. Each SIGHUP causes
// the config file to be reloaded and all registered OnChange callbacks
// to be invoked with a consistent Snapshot.
//
// On non-Unix platforms this is a no-op (see signal_windows.go).
func (c *ConfigGet) WatchSignal(wopts WatchOptions) *SIGHUPWatcher {
	sw := &SIGHUPWatcher{
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)

	go func() {
		defer close(sw.doneCh)
		defer signal.Stop(sigCh)
		for {
			select {
			case <-sw.stopCh:
				slog.Debug("config-get signal watcher: stopped")
				return
			case <-sigCh:
				slog.Debug("config-get signal watcher: SIGHUP received, reloading")
				if err := c.Reload(); err != nil {
					msg := "config-get signal watcher: reload failed: " + err.Error()
					if wopts.OnError != nil {
						wopts.OnError(err)
					} else {
						slog.Warn(msg)
					}
					continue
				}
				path, _ := c.Path()
				data, err := c.Data()
				if err != nil {
					continue
				}
				snap := newSnapshot(map[string]interface{}(data))
				sw.notify(ChangeEvent{Path: path, Snapshot: snap})
			}
		}
	}()

	return sw
}
