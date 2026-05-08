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
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// CleanupFunc is the function type for cleanup operations.
type CleanupFunc func() error

// Daemon manages the daemon lifecycle.
type Daemon struct {
	config *Config
	state  *StateMachine

	cleanup       CleanupFunc
	onStateChange func(State)

	pidFile   *os.File
	pidLocked bool

	healthServer *HealthServer

	watcher *Watcher

	// Channels for signal coordination
	waitingForSignal chan struct{}
	signalReceived   chan struct{}
}

// NewDaemon creates a new Daemon with the given configuration and cleanup function.
func NewDaemon(config *Config, cleanup CleanupFunc) *Daemon {
	return &Daemon{
		config:           config,
		state:            NewStateMachine(),
		cleanup:          cleanup,
		healthServer:     NewHealthServer(config.HealthPort),
		waitingForSignal: make(chan struct{}, 1),
		signalReceived:   make(chan struct{}, 1),
	}
}

// SetWatcher attaches a CDI version watcher that runs alongside the daemon.
// Pass nil to disable. Must be called before Run; calling after Run has no
// effect because the watcher is launched only during the StateRunning
// transition.
func (d *Daemon) SetWatcher(w *Watcher) {
	d.watcher = w
}

// PublishWatcherStatus mirrors a watcher status snapshot into the health
// server's `cdi-refresh` check. Callers wire this as the watcher's
// StatusHook so /ready can report when the daemon last observed the host's
// UMD libraries and whether the most recent regeneration succeeded.
func (d *Daemon) PublishWatcherStatus(s WatcherStatus) {
	if d.healthServer == nil {
		return
	}
	d.healthServer.AddCheck("cdi-refresh", watcherCheck(s))
}

// watcherCheck collapses a WatcherStatus into the {Status, Message} shape
// the health server already exposes. Status is "ok" by default and "error"
// only when the most recent refresh callback returned an error; the message
// always includes last_run and library counts so operators can confirm the
// watcher is alive even when nothing has changed.
//
// LastErr semantics: the watcher records the result of the most recent
// regeneration *attempt*, not the freshness of the on-disk spec. On a
// callback failure the watcher rolls its baseline back to the previous
// snapshot, so the next tick re-detects the same version change and
// re-runs the callback automatically — transient errors (disk full,
// momentary EIO) self-heal without operator intervention. While retries
// are in flight the previously-written CDI spec is still valid for new
// container starts. A permanent failure keeps Status "error" sticky and
// retries every refresh interval until resolved.
func watcherCheck(s WatcherStatus) CheckResult {
	lastRun := "never"
	if !s.LastRun.IsZero() {
		lastRun = s.LastRun.UTC().Format(time.RFC3339)
	}
	status := "ok"
	msg := fmt.Sprintf("last_run=%s libraries=%d", lastRun, len(s.Versions))
	if s.LastErr != nil {
		status = "error"
		msg = fmt.Sprintf("%s error=%v", msg, s.LastErr)
	}
	return CheckResult{Status: status, Message: msg}
}

// Run starts the daemon and blocks until shutdown.
func (d *Daemon) Run(ctx context.Context) error {
	// Notify state change for initial state (already starting)
	if d.onStateChange != nil {
		d.onStateChange(StateStarting)
	}

	if d.pidLocked {
		defer func() { _ = d.releasePIDLock() }()
	} else {
		if err := d.acquirePIDLock(); err != nil {
			d.setState(StateFailed)
			return fmt.Errorf("acquire PID lock: %w", err)
		}
		defer func() { _ = d.releasePIDLock() }()
	}

	// Start health server
	healthCtx, healthCancel := context.WithCancel(ctx)
	defer healthCancel()

	go func() {
		log.Printf("INFO: Starting health server on port %d", d.config.HealthPort)
		if err := d.healthServer.Start(healthCtx); err != nil {
			log.Printf("WARNING: Health server error: %v", err)
		}
	}()

	// Mark startup complete
	d.healthServer.SetStarted(true)

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP, syscall.SIGQUIT)
	defer signal.Stop(sigCh)

	// Signal handler goroutine
	go func() {
		select {
		case sig := <-sigCh:
			log.Printf("INFO: Received signal: %v", sig)
			select {
			case <-d.waitingForSignal:
				// Main loop is ready, signal it
				close(d.signalReceived)
			default:
				// Early signal before ready - shutdown immediately
				d.healthServer.SetReady(false)
				d.setState(StateShuttingDown)
				d.doCleanup()
				d.setState(StateStopped)
				os.Exit(0)
			}
		case <-ctx.Done():
			// Context canceled
			select {
			case <-d.waitingForSignal:
				close(d.signalReceived)
			default:
			}
		}
	}()

	// Mark as running and ready
	d.setState(StateRunning)
	d.healthServer.SetReady(true)
	d.healthServer.AddCheck("daemon", CheckResult{Status: "ready", Message: "Daemon is running"})
	log.Println("INFO: Setup complete. Waiting for signal...")

	// Start CDI version watcher if configured. Tied to a derived context so
	// shutdown propagates a cancel and we can wait for the goroutine to exit.
	watcherCtx, watcherCancel := context.WithCancel(ctx)
	defer watcherCancel()
	watcherDone := make(chan struct{})
	if d.watcher != nil {
		go func() {
			defer close(watcherDone)
			err := d.watcher.Run(watcherCtx)
			// Both ctx.Canceled (graceful shutdown) and DeadlineExceeded
			// (parent context with a timeout) are normal exit paths and
			// should not log a WARNING. Anything else is genuinely
			// unexpected.
			if err != nil &&
				!errors.Is(err, context.Canceled) &&
				!errors.Is(err, context.DeadlineExceeded) {
				log.Printf("WARNING: cdi-watcher exited unexpectedly: %v", err)
			}
		}()
	} else {
		close(watcherDone)
	}

	// Wait for signal or context cancellation
	close(d.waitingForSignal) // Signal that we're ready
	select {
	case <-d.signalReceived:
		log.Println("INFO: Starting shutdown...")
	case <-ctx.Done():
		log.Println("INFO: Context canceled, starting shutdown...")
	}

	// Perform graceful shutdown
	d.healthServer.SetReady(false)
	d.setState(StateShuttingDown)

	// Stop watcher and wait for its goroutine to return so cleanup runs
	// without a concurrent CDI regeneration in flight. We bound the wait
	// by ShutdownTimeout: the VersionProber API has no context, so a
	// probe stuck in a slow filesystem call (e.g. unresponsive NFS) would
	// otherwise hold the daemon open past its configured timeout. After
	// the timeout we proceed with cleanup and let the goroutine leak —
	// the process is about to exit anyway.
	watcherCancel()
	select {
	case <-watcherDone:
	case <-time.After(d.config.ShutdownTimeout):
		log.Printf("WARNING: cdi-watcher did not stop within %s; proceeding with cleanup (probe may be blocked on slow I/O)", d.config.ShutdownTimeout)
	}

	// Stop health server
	healthCancel()

	d.doCleanup()
	d.setState(StateStopped)

	log.Println("INFO: Shutdown complete.")
	return nil
}

// doCleanup performs the cleanup operations with timeout.
func (d *Daemon) doCleanup() {
	if d.config.NoCleanupOnExit {
		log.Println("INFO: Skipping cleanup (--no-cleanup-on-exit)")
		return
	}

	if d.cleanup == nil {
		return
	}

	log.Printf("INFO: Starting cleanup (timeout: %v)...", d.config.ShutdownTimeout)

	// Create context with timeout for cleanup
	ctx, cancel := context.WithTimeout(context.Background(), d.config.ShutdownTimeout)
	defer cancel()

	// Run cleanup in goroutine
	cleanupDone := make(chan error, 1)
	go func() {
		cleanupDone <- d.cleanup()
	}()

	// Wait for cleanup or timeout
	select {
	case err := <-cleanupDone:
		if err != nil {
			log.Printf("WARNING: Cleanup failed: %v", err)
		} else {
			log.Println("INFO: Cleanup complete.")
		}
	case <-ctx.Done():
		log.Printf("WARNING: Cleanup timeout exceeded (%v), forcing exit", d.config.ShutdownTimeout)
	}
}

// setState transitions the daemon to a new state.
func (d *Daemon) setState(state State) {
	if err := d.state.TransitionTo(state); err != nil {
		log.Printf("WARNING: State transition failed: %v", err)
		return
	}

	if d.onStateChange != nil {
		d.onStateChange(state)
	}
}

// acquirePIDLock creates and locks the PID file.
func (d *Daemon) acquirePIDLock() error {
	// Handle --force: terminate existing instance first
	if d.config.Force {
		if err := d.terminateExisting(); err != nil {
			return fmt.Errorf("force terminate existing: %w", err)
		}
	}

	// Ensure directory exists
	dir := filepathDir(d.config.PidFile)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create PID directory: %w", err)
		}
	}

	// Open/create PID file
	f, err := os.OpenFile(d.config.PidFile, os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("open PID file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		existingPID := d.readExistingPID(f)
		f.Close()
		if existingPID > 0 {
			return fmt.Errorf("another instance is already running (PID %d)\n  To stop the existing instance: sudo kill %d\n  To force takeover: sudo rbln-ctk-daemon --force", existingPID, existingPID)
		}
		return fmt.Errorf("another instance is already running (PID file locked)\n  To force takeover: sudo rbln-ctk-daemon --force")
	}

	// Write PID info
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate PID file: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return fmt.Errorf("seek PID file: %w", err)
	}
	info := struct {
		PID int `json:"pid"`
	}{
		PID: os.Getpid(),
	}
	if err := json.NewEncoder(f).Encode(info); err != nil {
		return fmt.Errorf("encode PID info: %w", err)
	}

	d.pidFile = f
	d.pidLocked = true

	return nil
}

func (d *Daemon) terminateExisting() error {
	f, err := os.Open(d.config.PidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open PID file: %w", err)
	}
	defer f.Close()

	pid := d.readExistingPID(f)
	if pid <= 0 {
		return nil
	}

	if err := syscall.Kill(pid, 0); err != nil {
		log.Printf("INFO: No existing process found (PID %d)", pid)
		return nil
	}

	log.Printf("INFO: Existing daemon found (PID %d), sending SIGTERM...", pid)
	if err := syscall.Kill(pid, syscall.SIGTERM); err != nil {
		return fmt.Errorf("send SIGTERM to PID %d: %w", pid, err)
	}

	log.Println("INFO: Waiting for graceful shutdown (timeout: 30s)...")
	const forceTimeout = 30
	for i := 0; i < forceTimeout; i++ {
		time.Sleep(1 * time.Second)
		if err := syscall.Kill(pid, 0); err != nil {
			log.Printf("INFO: Previous instance terminated after %ds", i+1)
			return nil
		}
	}

	return fmt.Errorf("process %d did not terminate within 30s, please kill it manually", pid)
}

func (d *Daemon) readExistingPID(f *os.File) int {
	_, _ = f.Seek(0, 0)
	var info struct {
		PID int `json:"pid"`
	}
	if err := json.NewDecoder(f).Decode(&info); err != nil {
		return 0
	}
	return info.PID
}

// AcquirePIDLock acquires the PID file lock. If already held, Run() skips lock acquisition.
func (d *Daemon) AcquirePIDLock() error {
	return d.acquirePIDLock()
}

// ReleasePIDLock releases the PID file lock and removes the file.
func (d *Daemon) ReleasePIDLock() error {
	return d.releasePIDLock()
}

// releasePIDLock releases the PID file lock and removes the file.
func (d *Daemon) releasePIDLock() error {
	if !d.pidLocked || d.pidFile == nil {
		return nil
	}

	// Release lock
	_ = unix.Flock(int(d.pidFile.Fd()), unix.LOCK_UN)

	// Close file
	d.pidFile.Close()

	// Remove PID file
	os.Remove(d.config.PidFile)

	d.pidLocked = false
	d.pidFile = nil

	return nil
}

// Helper to get directory from path
func filepathDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}
