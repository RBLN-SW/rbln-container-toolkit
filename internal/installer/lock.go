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

// Package installer provides the installation orchestration logic.
package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Lock represents a file lock for preventing concurrent installer execution.
type Lock struct {
	path string
	file *os.File
}

// NewLock creates a new Lock instance.
func NewLock(pidFile string) *Lock {
	return &Lock{path: pidFile}
}

// Acquire attempts to acquire an exclusive lock on the PID file.
// Returns an error if another instance is already running.
func (l *Lock) Acquire() error {
	// Ensure parent directory exists
	dir := filepath.Dir(l.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open or create the PID file
	f, err := os.OpenFile(l.path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open PID file %s: %w", l.path, err)
	}
	l.file = f

	// Try to acquire exclusive non-blocking lock
	err = unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err != nil {
		f.Close()
		l.file = nil
		return &ErrAlreadyRunning{PidFile: l.path}
	}

	// Write our PID to the file
	if err := f.Truncate(0); err != nil {
		_ = l.Release()
		return fmt.Errorf("failed to truncate PID file: %w", err)
	}
	if _, err := fmt.Fprintf(f, "%d\n", os.Getpid()); err != nil {
		_ = l.Release()
		return fmt.Errorf("failed to write PID: %w", err)
	}

	return nil
}

// Release releases the lock and removes the PID file.
func (l *Lock) Release() error {
	if l.file == nil {
		return nil
	}

	// Unlock
	_ = unix.Flock(int(l.file.Fd()), unix.LOCK_UN)

	// Close file
	if err := l.file.Close(); err != nil {
		return fmt.Errorf("failed to close PID file: %w", err)
	}
	l.file = nil

	// Remove PID file
	if err := os.Remove(l.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove PID file: %w", err)
	}

	return nil
}

// ErrAlreadyRunning indicates another installer instance is running.
type ErrAlreadyRunning struct {
	PidFile string
}

func (e *ErrAlreadyRunning) Error() string {
	return fmt.Sprintf("another instance is already running (PID file: %s)", e.PidFile)
}

// IsAlreadyRunning checks if the error indicates another instance is running.
func IsAlreadyRunning(err error) bool {
	_, ok := err.(*ErrAlreadyRunning)
	return ok
}
