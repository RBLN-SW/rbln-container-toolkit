//go:build linux || darwin

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

package installer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLock_InitializesPath(t *testing.T) {
	// Given
	pidFile := "/run/rbln/test.pid"

	// When
	lock := NewLock(pidFile)

	// Then
	assert.Equal(t, pidFile, lock.path)
	assert.Nil(t, lock.file)
}

func TestLock_Acquire(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock := NewLock(pidFile)

	// When
	err := lock.Acquire()
	defer lock.Release()

	// Then
	require.NoError(t, err)
	_, statErr := os.Stat(pidFile)
	assert.NoError(t, statErr, "PID file should exist after Acquire")
	assert.NotNil(t, lock.file)
}

func TestLock_Release(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock := NewLock(pidFile)
	require.NoError(t, lock.Acquire())

	// When
	err := lock.Release()

	// Then
	assert.NoError(t, err)
	assert.Nil(t, lock.file)
	_, statErr := os.Stat(pidFile)
	assert.True(t, os.IsNotExist(statErr), "PID file should be removed after Release")
}

func TestLock_ReleaseIdempotent(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock := NewLock(pidFile)
	require.NoError(t, lock.Acquire())
	lock.Release()

	// When
	err := lock.Release()

	// Then
	assert.NoError(t, err)
}

func TestLock_ConcurrentAcquisitionFails(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock1 := NewLock(pidFile)
	require.NoError(t, lock1.Acquire())
	defer lock1.Release()

	lock2 := NewLock(pidFile)

	// When
	err := lock2.Acquire()

	// Then
	assert.Error(t, err)
	assert.True(t, IsAlreadyRunning(err))
}

func TestLock_AcquireAfterRelease(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock1 := NewLock(pidFile)
	require.NoError(t, lock1.Acquire())
	lock1.Release()

	lock2 := NewLock(pidFile)

	// When
	err := lock2.Acquire()
	defer lock2.Release()

	// Then
	assert.NoError(t, err)
}

func TestLock_CreatesDirectory(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "dir")
	pidFile := filepath.Join(nestedDir, "test.pid")
	lock := NewLock(pidFile)

	// When
	err := lock.Acquire()
	defer lock.Release()

	// Then
	require.NoError(t, err)
	_, statErr := os.Stat(nestedDir)
	assert.NoError(t, statErr, "Parent directory should be created")
}

func TestLock_WritesPID(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock := NewLock(pidFile)

	// When
	err := lock.Acquire()
	defer lock.Release()

	// Then
	require.NoError(t, err)
	content, readErr := os.ReadFile(pidFile)
	require.NoError(t, readErr)
	assert.NotEmpty(t, strings.TrimSpace(string(content)), "PID file should contain PID")
}

func TestErrAlreadyRunning_Error(t *testing.T) {
	// Given
	err := &ErrAlreadyRunning{PidFile: "/run/test.pid"}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "/run/test.pid")
}

func TestIsAlreadyRunning_WithErrAlreadyRunning(t *testing.T) {
	// Given
	err := &ErrAlreadyRunning{PidFile: "/test.pid"}

	// When
	result := IsAlreadyRunning(err)

	// Then
	assert.True(t, result)
}

func TestIsAlreadyRunning_WithOtherError(t *testing.T) {
	// Given
	err := os.ErrNotExist

	// When
	result := IsAlreadyRunning(err)

	// Then
	assert.False(t, result)
}

func TestIsAlreadyRunning_WithNilError(t *testing.T) {
	// Given
	var err error

	// When
	result := IsAlreadyRunning(err)

	// Then
	assert.False(t, result)
}
