/*
Copyright 2026 Rebellions Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package daemon

import (
	"context"
	"log"
	"maps"
	"sort"
	"strings"
	"sync"
	"time"
)

// VersionProber returns a snapshot of UMD library path -> embedded RBLN
// version, alongside per-path parse errors. The prober is responsible for
// both discovering which libraries currently exist on the host and reading
// their versions, so the watcher does not need to know how libraries are
// laid out.
type VersionProber func() (versions map[string]string, errs map[string]error)

// RefreshTrigger is the payload passed to a RefreshCallback when the watcher
// detects a version change between two consecutive ticks.
type RefreshTrigger struct {
	// Versions is the snapshot observed at this tick.
	Versions map[string]string
	// PrevVersions is the snapshot from the previous tick (the baseline at
	// startup, otherwise the previous tick's result).
	PrevVersions map[string]string
	// Errors are the per-path parse failures observed at this tick. Treated
	// as informational; they do not by themselves trigger a refresh.
	Errors map[string]error
}

// RefreshCallback is invoked when the watcher observes a version change.
// Return errors are logged and surfaced via Status, but do not stop the loop.
type RefreshCallback func(ctx context.Context, t RefreshTrigger) error

// WatcherStatus is a thread-safe snapshot of the watcher's most recent tick.
type WatcherStatus struct {
	LastRun  time.Time
	Versions map[string]string
	LastErr  error
}

// WatcherOptions configures NewWatcher.
type WatcherOptions struct {
	// Interval is the polling period. A non-positive value disables the watcher.
	Interval time.Duration
	// Probe is the snapshot function invoked each tick. Required.
	Probe VersionProber
	// OnChange is invoked once per detected version change. May be nil for
	// observation-only operation (useful in tests).
	OnChange RefreshCallback
	// StatusHook, if non-nil, is invoked after every probe (baseline plus
	// each tick) with the watcher's current status. Used by the daemon to
	// surface the watcher's heartbeat through the health server without
	// coupling the watcher to that subsystem.
	StatusHook func(WatcherStatus)
}

// Watcher periodically takes a UMD library version snapshot and invokes a
// callback when the snapshot changes between ticks. Driver upgrades on the
// host change the embedded `rbln version:` string in librbln-*.so files;
// that string is what the watcher keys on.
type Watcher struct {
	interval   time.Duration
	probe      VersionProber
	onChange   RefreshCallback
	statusHook func(WatcherStatus)

	mu      sync.RWMutex
	last    map[string]string
	lastRun time.Time
	lastErr error
}

// NewWatcher creates a watcher. Returns nil if Interval is non-positive (so
// callers can do `if w := NewWatcher(...); w != nil { go w.Run(ctx) }`) or
// if Probe is nil.
func NewWatcher(opts WatcherOptions) *Watcher {
	if opts.Interval <= 0 || opts.Probe == nil {
		return nil
	}
	return &Watcher{
		interval:   opts.Interval,
		probe:      opts.Probe,
		onChange:   opts.OnChange,
		statusHook: opts.StatusHook,
	}
}

// Run blocks until ctx is canceled. The first probe establishes a baseline
// and never fires the callback; subsequent ticks compare against the
// baseline.
//
// The probe itself runs synchronously and the VersionProber API does not
// take a context, so a hung probe (e.g. unresponsive NFS mount) cannot be
// canceled mid-call. We bound the exposure two ways: the watcher checks
// ctx before starting any probe so a context already canceled is a no-op,
// and the daemon's shutdown path waits on Run's exit with a hard timeout
// (see Daemon.Run) so a single stuck probe cannot block process exit.
func (w *Watcher) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	w.baseline()

	t := time.NewTicker(w.interval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if ctx.Err() != nil {
				return ctx.Err()
			}
			w.tick(ctx)
		}
	}
}

// Status returns a copy of the most recent observation.
func (w *Watcher) Status() WatcherStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cp := make(map[string]string, len(w.last))
	for k, v := range w.last {
		cp[k] = v
	}
	return WatcherStatus{
		LastRun:  w.lastRun,
		Versions: cp,
		LastErr:  w.lastErr,
	}
}

func (w *Watcher) baseline() {
	versions, errs := w.probe()
	for path, err := range errs {
		log.Printf("WARNING: cdi-watcher: baseline probe failed for %s: %v", path, err)
	}
	// Defensive clone: a misbehaving prober could legally reuse or mutate
	// the returned map across calls, which would corrupt our retained
	// `last` state. Owning a private copy isolates us from that contract.
	snapshot := maps.Clone(versions)
	w.mu.Lock()
	w.last = snapshot
	w.lastRun = time.Now()
	w.mu.Unlock()
	log.Printf("INFO: cdi-watcher: baseline established: %s", formatVersions(snapshot))
	w.publishStatus()
}

func (w *Watcher) tick(ctx context.Context) {
	versions, errs := w.probe()
	for path, err := range errs {
		log.Printf("WARNING: cdi-watcher: probe failed for %s: %v", path, err)
	}

	// Defensive clone of the prober's snapshot; see baseline() for the
	// rationale. We also retain `prev` after swapping it out so the
	// callback receives a snapshot that cannot be mutated under it.
	snapshot := maps.Clone(versions)
	w.mu.Lock()
	prev := w.last
	w.last = snapshot
	w.lastRun = time.Now()
	w.mu.Unlock()

	// Probing can outlive a shutdown signal. Bail out before the callback so
	// CDI regeneration does not race with the daemon's cleanup goroutine,
	// which itself removes the spec file.
	if ctx.Err() != nil {
		return
	}

	if !versionsDiffer(prev, snapshot) {
		w.publishStatus()
		return
	}

	log.Printf("INFO: cdi-watcher: version change detected: %s -> %s", formatVersions(prev), formatVersions(snapshot))

	if w.onChange == nil {
		w.publishStatus()
		return
	}

	// Clone before handing maps to the callback. `snapshot` is also stored
	// as `w.last`, and `prev` is the previous tick's `w.last` — passing
	// them by reference would let a misbehaving callback that mutates
	// tr.Versions or tr.PrevVersions corrupt the watcher's baseline and
	// produce false-negative or false-positive diffs on the next tick.
	// This mirrors the defensive clone already applied for the prober's
	// return value so the ownership invariant is symmetric on both sides.
	err := w.onChange(ctx, RefreshTrigger{
		Versions:     maps.Clone(snapshot),
		PrevVersions: maps.Clone(prev),
		Errors:       errs,
	})

	w.mu.Lock()
	w.lastErr = err
	if err != nil {
		// Roll the baseline back to the pre-change snapshot so the next
		// tick re-detects the same version transition and re-attempts
		// the callback. This makes transient regeneration failures
		// (disk full, momentary EIO) self-healing without operator
		// intervention; permanent failures keep retrying every interval
		// while LastErr stays sticky in /ready.
		w.last = prev
	}
	w.mu.Unlock()

	if err != nil {
		log.Printf("ERROR: cdi-watcher: refresh callback failed (will retry on next tick): %v", err)
	}
	w.publishStatus()
}

func (w *Watcher) publishStatus() {
	if w.statusHook == nil {
		return
	}
	w.statusHook(w.Status())
}

// versionsDiffer reports whether two snapshots represent a CDI-relevant change.
// Equal length AND identical key/value pairs means no change.
func versionsDiffer(a, b map[string]string) bool {
	if len(a) != len(b) {
		return true
	}
	for k, v := range a {
		if b[k] != v {
			return true
		}
	}
	return false
}

func formatVersions(m map[string]string) string {
	if len(m) == 0 {
		return "<none>"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, ", ")
}
