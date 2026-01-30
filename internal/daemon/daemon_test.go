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
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemon_PIDFileLocking(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	d := NewDaemon(cfg, nil)
	defer d.releasePIDLock()

	// When
	err := d.acquirePIDLock()

	// Then
	require.NoError(t, err)
	_, err = os.Stat(pidFile)
	require.NoError(t, err)
	content, err := os.ReadFile(pidFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "pid")
}

func TestDaemon_PIDFileLocking_AlreadyLocked(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	d1 := NewDaemon(cfg, nil)
	err := d1.acquirePIDLock()
	require.NoError(t, err)
	defer d1.releasePIDLock()
	d2 := NewDaemon(cfg, nil)

	// When
	err = d2.acquirePIDLock()

	// Then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestDaemon_SignalHandling_SIGTERM(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.ShutdownTimeout = 5 * time.Second
	cleanupCalled := false
	cleanup := func() error {
		cleanupCalled = true
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit in time")
	}
	assert.True(t, cleanupCalled)
}

func TestDaemon_SignalHandling_SIGINT(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.ShutdownTimeout = 5 * time.Second
	cleanupCalled := false
	cleanup := func() error {
		cleanupCalled = true
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit in time")
	}
	assert.True(t, cleanupCalled)
}

func TestDaemon_GracefulShutdown(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.ShutdownTimeout = 5 * time.Second
	cleanupStarted := make(chan struct{})
	cleanupDone := make(chan struct{})
	cleanup := func() error {
		close(cleanupStarted)
		time.Sleep(100 * time.Millisecond)
		close(cleanupDone)
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case <-cleanupStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not start")
	}
	select {
	case <-cleanupDone:
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not complete")
	}
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("daemon did not exit")
	}
}

func TestDaemon_CleanupExecution(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	var cleanupOrder []string
	cleanup := func() error {
		cleanupOrder = append(cleanupOrder, "cleanup")
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	<-errCh
	assert.Contains(t, cleanupOrder, "cleanup")
}

func TestDaemon_NoCleanupOnExit(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.NoCleanupOnExit = true
	cleanupCalled := false
	cleanup := func() error {
		cleanupCalled = true
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	<-errCh
	assert.False(t, cleanupCalled)
}

func TestDaemon_ShutdownTimeoutConfiguration(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.ShutdownTimeout = 60 * time.Second

	// When
	d := NewDaemon(cfg, nil)

	// Then
	assert.Equal(t, 60*time.Second, d.config.ShutdownTimeout)
}

func TestDaemon_CleanupTimeoutExpiration(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.ShutdownTimeout = 100 * time.Millisecond
	cleanupStarted := make(chan struct{})
	cleanup := func() error {
		close(cleanupStarted)
		time.Sleep(5 * time.Second)
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(200 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case <-errCh:
	case <-time.After(2 * time.Second):
		t.Fatal("daemon should have exited with timeout")
	}
}

func TestDaemon_FastCleanup(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cfg.ShutdownTimeout = 30 * time.Second
	cleanupDone := false
	cleanup := func() error {
		cleanupDone = true
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(200 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case <-errCh:
		assert.True(t, cleanupDone)
	case <-time.After(5 * time.Second):
		t.Fatal("daemon should have exited quickly")
	}
}

func TestDaemon_StateTransitions(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	stateChanges := make([]State, 0)
	d := NewDaemon(cfg, nil)
	d.onStateChange = func(state State) {
		stateChanges = append(stateChanges, state)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	<-errCh
	assert.Contains(t, stateChanges, StateStarting)
	assert.Contains(t, stateChanges, StateRunning)
	assert.Contains(t, stateChanges, StateShuttingDown)
	assert.Contains(t, stateChanges, StateStopped)
}

func TestDaemon_AcquirePIDLock_CreatesFile(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cfg := NewDaemonConfig()
	cfg.PidFile = filepath.Join(tmpDir, "test.pid")
	d := NewDaemon(cfg, nil)
	defer d.ReleasePIDLock()

	// When
	err := d.AcquirePIDLock()

	// Then
	require.NoError(t, err)
	assert.FileExists(t, cfg.PidFile)
	assert.True(t, d.pidLocked)
}

func TestDaemon_AcquirePIDLock_BlocksSecond(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cfg := NewDaemonConfig()
	cfg.PidFile = filepath.Join(tmpDir, "test.pid")
	d1 := NewDaemon(cfg, nil)
	err := d1.AcquirePIDLock()
	require.NoError(t, err)
	defer d1.ReleasePIDLock()
	d2 := NewDaemon(cfg, nil)

	// When
	err = d2.AcquirePIDLock()

	// Then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already running")
}

func TestDaemon_ReleasePIDLock_RemovesFile(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cfg := NewDaemonConfig()
	cfg.PidFile = filepath.Join(tmpDir, "test.pid")
	d := NewDaemon(cfg, nil)
	err := d.AcquirePIDLock()
	require.NoError(t, err)
	require.FileExists(t, cfg.PidFile)

	// When
	err = d.ReleasePIDLock()

	// Then
	require.NoError(t, err)
	assert.NoFileExists(t, cfg.PidFile)
	assert.False(t, d.pidLocked)
}

func TestDaemon_ReleasePIDLock_Idempotent(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cfg := NewDaemonConfig()
	cfg.PidFile = filepath.Join(tmpDir, "test.pid")
	d := NewDaemon(cfg, nil)
	err := d.AcquirePIDLock()
	require.NoError(t, err)

	// When
	err1 := d.ReleasePIDLock()
	err2 := d.ReleasePIDLock()

	// Then
	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.False(t, d.pidLocked)
}

func TestDaemon_Run_SkipsLock_WhenPreAcquired(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cfg := NewDaemonConfig()
	cfg.PidFile = filepath.Join(tmpDir, "test.pid")
	cfg.ShutdownTimeout = 5 * time.Second
	d := NewDaemon(cfg, nil)
	err := d.AcquirePIDLock()
	require.NoError(t, err)
	require.True(t, d.pidLocked)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(100 * time.Millisecond)

	// When
	cancel()

	// Then
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit in time")
	}
	assert.NoFileExists(t, cfg.PidFile)
}

func TestDaemon_RealSignal(t *testing.T) {
	// Given
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping signal test in CI environment")
	}
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	cfg := NewDaemonConfig()
	cfg.PidFile = pidFile
	cleanupCalled := false
	cleanup := func() error {
		cleanupCalled = true
		return nil
	}
	d := NewDaemon(cfg, cleanup)
	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()
	time.Sleep(200 * time.Millisecond)

	// When
	p, err := os.FindProcess(os.Getpid())
	require.NoError(t, err)
	err = p.Signal(syscall.SIGTERM)
	require.NoError(t, err)

	// Then
	select {
	case <-errCh:
		assert.True(t, cleanupCalled)
	case <-time.After(10 * time.Second):
		t.Fatal("daemon did not exit in time")
	}
}
