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
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// scriptedProber returns a different version snapshot per call. Once it runs
// out of scripted snapshots it keeps replaying the last one. Calls are
// counted so tests can assert how many ticks executed.
type scriptedProber struct {
	mu        sync.Mutex
	snapshots []map[string]string
	errs      []map[string]error
	calls     atomic.Int32
}

func newScriptedProber(snapshots ...map[string]string) *scriptedProber {
	return &scriptedProber{snapshots: snapshots}
}

func (p *scriptedProber) probe() (map[string]string, map[string]error) {
	idx := int(p.calls.Add(1)) - 1
	p.mu.Lock()
	defer p.mu.Unlock()
	if idx >= len(p.snapshots) {
		idx = len(p.snapshots) - 1
	}
	var errs map[string]error
	if idx < len(p.errs) {
		errs = p.errs[idx]
	}
	return p.snapshots[idx], errs
}

// waitFor polls cond until it returns true or the deadline elapses. Used in
// place of fixed sleeps so tests stay fast and resilient on slow CI.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatalf("waitFor timed out: %s", msg)
}

func TestWatcher_DisabledWhenIntervalNonPositive(t *testing.T) {
	assert.Nil(t, NewWatcher(WatcherOptions{Interval: 0}))
	assert.Nil(t, NewWatcher(WatcherOptions{Interval: -1 * time.Second}))
}

func TestWatcher_BaselineDoesNotFireCallback(t *testing.T) {
	// Given a prober that always returns the same versions
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "1.0.0"},
	)
	var fired atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, _ RefreshTrigger) error {
			fired.Add(1)
			return nil
		},
	})
	require.NotNil(t, w)

	// When the watcher runs through several ticks
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()
	waitFor(t, func() bool { return prober.calls.Load() >= 3 }, "expected at least 3 probes")

	// Then the callback never fires because every snapshot matches the baseline
	assert.Equal(t, int32(0), fired.Load())
}

func TestWatcher_FiresExactlyOncePerChange(t *testing.T) {
	// Given a prober scripted with one upgrade in the middle
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
	)
	var fired atomic.Int32
	var seenPrev, seenCurr atomic.Value
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, tr RefreshTrigger) error {
			fired.Add(1)
			seenPrev.Store(tr.PrevVersions)
			seenCurr.Store(tr.Versions)
			return nil
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()
	waitFor(t, func() bool { return prober.calls.Load() >= 5 }, "expected at least 5 probes")

	// Then exactly one transition fires the callback with old/new versions
	assert.Equal(t, int32(1), fired.Load())
	prev, _ := seenPrev.Load().(map[string]string)
	curr, _ := seenCurr.Load().(map[string]string)
	assert.Equal(t, "1.0.0", prev["/lib/a.so"])
	assert.Equal(t, "2.0.0", curr["/lib/a.so"])
}

func TestWatcher_LibraryAppearingTriggersChange(t *testing.T) {
	// Given baseline with no libraries, then one library appears
	prober := newScriptedProber(
		map[string]string{},
		map[string]string{"/lib/a.so": "1.0.0"},
	)
	var fired atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, _ RefreshTrigger) error {
			fired.Add(1)
			return nil
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()
	waitFor(t, func() bool { return fired.Load() >= 1 }, "expected callback to fire when library appears")
}

func TestWatcher_ProbeErrorsAreNotChange(t *testing.T) {
	// Given identical versions but with a parse error appearing on the second tick
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "1.0.0"},
	)
	prober.errs = []map[string]error{
		nil,
		{"/lib/b.so": errors.New("parse failed")},
		{"/lib/b.so": errors.New("parse failed")},
	}
	var fired atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, _ RefreshTrigger) error {
			fired.Add(1)
			return nil
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()
	waitFor(t, func() bool { return prober.calls.Load() >= 3 }, "expected at least 3 probes")

	// Then errors alone do not constitute a change
	assert.Equal(t, int32(0), fired.Load())
}

func TestWatcher_CallbackErrorRecordedNotFatal(t *testing.T) {
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
		map[string]string{"/lib/a.so": "3.0.0"},
	)
	cbErr := errors.New("regen failed")
	var fired atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, _ RefreshTrigger) error {
			fired.Add(1)
			return cbErr
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()
	waitFor(t, func() bool { return fired.Load() >= 2 }, "expected callback to fire on each change")

	st := w.Status()
	assert.ErrorIs(t, st.LastErr, cbErr)
	assert.NotEmpty(t, st.Versions)
}

func TestWatcher_StatusHookCalledOnBaselineAndEachTick(t *testing.T) {
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
	)
	var hookCalls atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, _ RefreshTrigger) error { return nil },
		StatusHook: func(s WatcherStatus) {
			if !s.LastRun.IsZero() {
				hookCalls.Add(1)
			}
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()
	waitFor(t, func() bool { return hookCalls.Load() >= 3 }, "hook should fire on baseline + each tick")
}

func TestWatcherCheck_OkAndError(t *testing.T) {
	t0 := time.Date(2026, 5, 8, 11, 30, 0, 0, time.UTC)

	ok := watcherCheck(WatcherStatus{
		LastRun:  t0,
		Versions: map[string]string{"/lib/a.so": "1.0.0", "/lib/b.so": "1.0.0"},
	})
	assert.Equal(t, "ok", ok.Status)
	assert.Contains(t, ok.Message, "last_run=2026-05-08T11:30:00Z")
	assert.Contains(t, ok.Message, "libraries=2")
	assert.NotContains(t, ok.Message, "error=")

	bad := watcherCheck(WatcherStatus{
		LastRun:  t0,
		Versions: map[string]string{"/lib/a.so": "1.0.0"},
		LastErr:  errors.New("regen failed"),
	})
	assert.Equal(t, "error", bad.Status)
	assert.Contains(t, bad.Message, "error=regen failed")
}

// Zero-valued LastRun must surface as "never" rather than the misleading
// epoch placeholder "0001-01-01T00:00:00Z" that monitoring would otherwise
// see if /ready is queried before the watcher has run a baseline.
func TestWatcherCheck_ZeroLastRunRendersAsNever(t *testing.T) {
	got := watcherCheck(WatcherStatus{})
	assert.Equal(t, "ok", got.Status)
	assert.Contains(t, got.Message, "last_run=never")
	assert.Contains(t, got.Message, "libraries=0")
}

func TestDaemon_PublishWatcherStatus_AddsCheck(t *testing.T) {
	cfg := NewDaemonConfig()
	cfg.PidFile = filepath.Join(t.TempDir(), "test.pid")
	d := NewDaemon(cfg, nil)

	d.PublishWatcherStatus(WatcherStatus{
		LastRun:  time.Date(2026, 5, 8, 11, 30, 0, 0, time.UTC),
		Versions: map[string]string{"/lib/a.so": "1.0.0"},
	})

	d.healthServer.mu.RLock()
	got, ok := d.healthServer.checks["cdi-refresh"]
	d.healthServer.mu.RUnlock()
	require.True(t, ok)
	assert.Equal(t, "ok", got.Status)
	assert.Contains(t, got.Message, "libraries=1")
}

func TestWatcher_SkipsCallbackWhenContextCanceledDuringProbe(t *testing.T) {
	// Given a prober that only releases after we cancel the context. If the
	// watcher honors ctx, the callback must NOT fire even though the probe
	// returned a changed snapshot.
	gate := make(chan struct{})
	released := make(chan struct{})
	calls := atomic.Int32{}
	prober := func() (map[string]string, map[string]error) {
		n := calls.Add(1)
		if n == 1 {
			// baseline
			return map[string]string{"/lib/a.so": "1.0.0"}, nil
		}
		<-gate
		close(released)
		return map[string]string{"/lib/a.so": "2.0.0"}, nil
	}
	var fired atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober,
		OnChange: func(_ context.Context, _ RefreshTrigger) error {
			fired.Add(1)
			return nil
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Wait for the second probe to be pending, cancel ctx, then release the probe.
	waitFor(t, func() bool { return calls.Load() >= 2 }, "second probe should start")
	cancel()
	close(gate)
	<-released

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after cancel")
	}

	assert.Equal(t, int32(0), fired.Load(), "callback must not fire when ctx is canceled before/after probe")
}

// TestWatcher_CallbackCannotCorruptWatcherBaseline verifies the symmetric
// invariant on the callback side: a misbehaving RefreshCallback that
// writes to tr.Versions or tr.PrevVersions must not be able to mutate the
// watcher's retained `w.last`. Without the defensive clone, the next tick
// would pick up the corrupted map as `prev` and either miss a real change
// or report a phantom one.
func TestWatcher_CallbackCannotCorruptWatcherBaseline(t *testing.T) {
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
		map[string]string{"/lib/a.so": "2.0.0"},
	)
	cbDone := make(chan struct{}, 1)
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, tr RefreshTrigger) error {
			// Hostile callback: stomp on the trigger map. If the watcher
			// passed its own `w.last` by reference, this would corrupt
			// the baseline used for the next diff.
			tr.Versions["/lib/a.so"] = "CORRUPT"
			tr.PrevVersions["/lib/a.so"] = "CORRUPT"
			cbDone <- struct{}{}
			return nil
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()

	// Wait for the version-change callback to land (and corrupt its trigger).
	select {
	case <-cbDone:
	case <-time.After(2 * time.Second):
		t.Fatal("callback never fired")
	}

	// Watcher's view must still reflect the actual probe result.
	st := w.Status()
	assert.Equal(t, "2.0.0", st.Versions["/lib/a.so"], "callback mutation must not leak into Status()")
}

// TestWatcher_DefensivelyCopiesProberSnapshots ensures the watcher does
// not retain references to maps owned by the prober. A misbehaving prober
// that mutates its return map between calls must not be able to corrupt
// the watcher's previous snapshot or the values surfaced through Status().
func TestWatcher_DefensivelyCopiesProberSnapshots(t *testing.T) {
	// Single map reused across all calls; mutated after baseline.
	shared := map[string]string{"/lib/a.so": "1.0.0"}
	probeReturned := make(chan struct{}, 4)
	prober := func() (map[string]string, map[string]error) {
		probeReturned <- struct{}{}
		return shared, nil
	}

	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober,
		OnChange: func(_ context.Context, _ RefreshTrigger) error { return nil },
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()

	// Drain baseline + first tick to guarantee w.last is set.
	<-probeReturned
	<-probeReturned

	// Mutate the shared map. If the watcher kept a direct reference, the
	// next tick's diff would compare {"/lib/a.so": "9.9.9"} against the
	// "1.0.0" current snapshot — but they're the SAME map, so versionsDiffer
	// would return false and we'd silently miss the transition. With a
	// defensive copy, the next probe call still returns "9.9.9" in the
	// caller-owned map, but our retained snapshot stayed "1.0.0", so the
	// diff fires correctly.
	shared["/lib/a.so"] = "9.9.9"

	st := w.Status()
	assert.Equal(t, "1.0.0", st.Versions["/lib/a.so"], "Status() must reflect the snapshot at probe time, not later mutations")
}

// TestWatcher_RunReturnsImmediatelyIfCtxAlreadyCanceled verifies the
// short-circuit added so an already-canceled context cannot trigger a
// baseline probe. Without it, Run would always perform one synchronous
// probe before observing the cancel.
// TestWatcher_RetriesAfterCallbackFailure verifies that a transient
// callback failure does not strand the watcher: the baseline is rolled
// back to the pre-change snapshot so the next tick re-detects the same
// version transition and re-runs the callback. Without this, a single
// CDI regeneration error would stall self-healing until the next driver
// upgrade or daemon restart.
func TestWatcher_RetriesAfterCallbackFailure(t *testing.T) {
	prober := newScriptedProber(
		map[string]string{"/lib/a.so": "1.0.0"}, // baseline
		map[string]string{"/lib/a.so": "2.0.0"}, // first observation of upgrade — callback fails
		map[string]string{"/lib/a.so": "2.0.0"}, // same upgrade — should be re-detected and retried
		map[string]string{"/lib/a.so": "2.0.0"}, // settle on success
	)
	var attempts atomic.Int32
	w := NewWatcher(WatcherOptions{
		Interval: 5 * time.Millisecond,
		Probe:    prober.probe,
		OnChange: func(_ context.Context, _ RefreshTrigger) error {
			n := attempts.Add(1)
			if n == 1 {
				return errors.New("transient regen failure")
			}
			return nil
		},
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go func() { _ = w.Run(ctx) }()

	// Both the failed first attempt and the successful retry must fire.
	waitFor(t, func() bool { return attempts.Load() >= 2 }, "callback should retry after a failure")

	// After successful retry, Status reports the new version and no error.
	waitFor(t, func() bool {
		st := w.Status()
		return st.Versions["/lib/a.so"] == "2.0.0" && st.LastErr == nil
	}, "Status should reflect successful retry")
}

func TestWatcher_RunReturnsImmediatelyIfCtxAlreadyCanceled(t *testing.T) {
	var probed atomic.Int32
	prober := func() (map[string]string, map[string]error) {
		probed.Add(1)
		return nil, nil
	}
	w := NewWatcher(WatcherOptions{
		Interval: 50 * time.Millisecond,
		Probe:    prober,
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // canceled before Run starts

	err := w.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Equal(t, int32(0), probed.Load(), "baseline must not probe when ctx is already canceled")
}

func TestWatcher_RunReturnsOnContextCancel(t *testing.T) {
	prober := newScriptedProber(map[string]string{"/lib/a.so": "1.0.0"})
	w := NewWatcher(WatcherOptions{
		Interval: 50 * time.Millisecond,
		Probe:    prober.probe,
	})
	require.NotNil(t, w)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	cancel()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after context cancel")
	}
}

func TestVersionsDiffer(t *testing.T) {
	tests := []struct {
		name string
		a, b map[string]string
		want bool
	}{
		{"both nil", nil, nil, false},
		{"both empty", map[string]string{}, map[string]string{}, false},
		{"same single", map[string]string{"x": "1"}, map[string]string{"x": "1"}, false},
		{"same multi", map[string]string{"x": "1", "y": "2"}, map[string]string{"y": "2", "x": "1"}, false},
		{"different value", map[string]string{"x": "1"}, map[string]string{"x": "2"}, true},
		{"key removed", map[string]string{"x": "1", "y": "2"}, map[string]string{"x": "1"}, true},
		{"key added", map[string]string{"x": "1"}, map[string]string{"x": "1", "y": "2"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, versionsDiffer(tc.a, tc.b))
		})
	}
}
