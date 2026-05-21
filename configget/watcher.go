package configget

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// WatchOptions — tuning knobs for the file watcher
// ---------------------------------------------------------------------------

// WatchOptions controls the behaviour of the background file watcher.
type WatchOptions struct {
	// Interval is how often the watcher polls the config file's mtime.
	// Default: 2 seconds. Minimum enforced: 100 ms.
	Interval time.Duration

	// OnError is called when the config file cannot be re-parsed after a
	// detected change. If nil, errors are logged via slog.Warn and the
	// previous (last-good) snapshot is kept.
	OnError func(err error)
}

func (o WatchOptions) interval() time.Duration {
	if o.Interval <= 0 {
		return 2 * time.Second
	}
	if o.Interval < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return o.Interval
}

// ---------------------------------------------------------------------------
// ChangeEvent — delivered to subscribers on every config change
// ---------------------------------------------------------------------------

// ChangeEvent is passed to OnChange callbacks when the config file is modified.
type ChangeEvent struct {
	// Path is the absolute path of the config file that changed.
	Path string

	// Snapshot is a consistent, immutable copy of the entire config at the
	// moment the change was detected. All keys come from the same file
	// version — no torn reads.
	Snapshot Snapshot
}

// ---------------------------------------------------------------------------
// Snapshot — immutable point-in-time view of parsed config data
// ---------------------------------------------------------------------------

// Snapshot is an immutable, goroutine-safe point-in-time copy of the config.
// Obtain one from ChangeEvent.Snapshot or ConfigGet.Snapshot().
type Snapshot struct {
	data map[string]interface{}
}

func newSnapshot(d map[string]interface{}) Snapshot {
	out := make(map[string]interface{}, len(d))
	for k, v := range d {
		out[k] = v
	}
	return Snapshot{data: out}
}

// Get returns the raw value for key, or nil if absent.
func (s Snapshot) Get(key string) interface{} { return s.data[key] }

// Has reports whether key exists in this snapshot.
func (s Snapshot) Has(key string) bool { _, ok := s.data[key]; return ok }

// Keys returns all keys present in this snapshot.
func (s Snapshot) Keys() []string {
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// AsMap returns a shallow copy of the snapshot as a plain map.
func (s Snapshot) AsMap() map[string]interface{} {
	out := make(map[string]interface{}, len(s.data))
	for k, v := range s.data {
		out[k] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// Watcher — background mtime-polling file watcher
// ---------------------------------------------------------------------------

// Watcher watches a config file for changes using mtime polling and notifies
// all registered OnChange subscribers. It uses no external dependencies and
// works identically on Linux, macOS, and Windows.
//
// Create via ConfigGet.Watch(); stop via Watcher.Stop().
//
//	cfg := configget.New("myapp.toml", "myapp", configget.Options{})
//	w, err := cfg.Watch(configget.WatchOptions{Interval: 5 * time.Second})
//	if err != nil { log.Fatal(err) }
//	w.OnChange(func(ev configget.ChangeEvent) {
//	    log.Println("config changed, new DB_HOST =", ev.Snapshot.Get("DB_HOST"))
//	})
//	defer w.Stop()
type Watcher struct {
	cfg      *ConfigGet
	wopts    WatchOptions
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	handlers []func(ChangeEvent)
}

// Watch starts a background mtime-polling watcher for cfg's config file.
// The watcher goroutine runs until Stop() is called.
// Returns an error only if the config file path cannot be resolved.
func (c *ConfigGet) Watch(wopts WatchOptions) (*Watcher, error) {
	// Ensure path is resolved before starting the goroutine.
	if err := c.ensureLoaded(); err != nil {
		return nil, fmt.Errorf("config-get Watch: %w", err)
	}

	path, _ := c.Path()
	var initialMtime time.Time
	if fi, err := os.Stat(path); err == nil {
		initialMtime = fi.ModTime()
	}

	w := &Watcher{
		cfg:    c,
		wopts:  wopts,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	go w.run(initialMtime)
	slog.Debug("config-get watcher: started", "path", path, "interval", wopts.interval())
	return w, nil
}

// OnChange registers a callback that is invoked (in its own goroutine) every
// time the config file changes on disk. Multiple callbacks can be registered.
func (w *Watcher) OnChange(fn func(ChangeEvent)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, fn)
}

// Stop shuts the watcher down and waits for the background goroutine to exit.
func (w *Watcher) Stop() {
	close(w.stopCh)
	<-w.doneCh
}

func (w *Watcher) notify(ev ChangeEvent) {
	w.mu.RLock()
	handlers := make([]func(ChangeEvent), len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.RUnlock()
	for _, h := range handlers {
		h := h
		go h(ev)
	}
}

func (w *Watcher) run(initialMtime time.Time) {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.wopts.interval())
	defer ticker.Stop()

	lastMtime := initialMtime

	for {
		select {
		case <-w.stopCh:
			slog.Debug("config-get watcher: stopped")
			return

		case <-ticker.C:
			path, err := w.cfg.Path()
			if err != nil || path == "" {
				continue
			}

			fi, err := os.Stat(path)
			if err != nil {
				// File temporarily missing (e.g. during atomic write); skip tick.
				slog.Debug("config-get watcher: stat failed", "path", path, "err", err)
				continue
			}

			mtime := fi.ModTime()
			if !mtime.After(lastMtime) {
				continue // unchanged
			}

			slog.Debug("config-get watcher: change detected",
				"path", path, "old_mtime", lastMtime, "new_mtime", mtime)
			lastMtime = mtime

			// Reload: full write-lock inside Reload() — snapshot-safe.
			if err := w.cfg.Reload(); err != nil {
				reloadErr := fmt.Errorf("config-get watcher: reload failed: %w", err)
				if w.wopts.OnError != nil {
					w.wopts.OnError(reloadErr)
				} else {
					slog.Warn(reloadErr.Error())
				}
				// Keep lastMtime updated so we retry next tick.
				continue
			}

			// Build a consistent snapshot from the freshly-loaded data.
			data, err := w.cfg.Data()
			if err != nil {
				continue
			}
			snap := newSnapshot(map[string]interface{}(data))
			w.notify(ChangeEvent{Path: path, Snapshot: snap})
		}
	}
}

// ---------------------------------------------------------------------------
// SIGHUPWatcher — declared here; implemented in signal_unix.go / signal_windows.go
// ---------------------------------------------------------------------------

// SIGHUPWatcher listens for SIGHUP and reloads the config on each signal.
// On Windows, WatchSignal() is a documented no-op; use Watch() instead.
type SIGHUPWatcher struct {
	stopCh   chan struct{}
	doneCh   chan struct{}
	mu       sync.RWMutex
	handlers []func(ChangeEvent)
}

// OnChange registers a callback invoked on each SIGHUP-triggered reload.
func (s *SIGHUPWatcher) OnChange(fn func(ChangeEvent)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, fn)
}

// Stop shuts the SIGHUP watcher down and waits for the goroutine to exit.
func (s *SIGHUPWatcher) Stop() {
	close(s.stopCh)
	<-s.doneCh
}

func (s *SIGHUPWatcher) notify(ev ChangeEvent) {
	s.mu.RLock()
	handlers := make([]func(ChangeEvent), len(s.handlers))
	copy(handlers, s.handlers)
	s.mu.RUnlock()
	for _, h := range handlers {
		h := h
		go h(ev)
	}
}
