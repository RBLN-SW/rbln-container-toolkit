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

package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/oci"
)

func TestCreateSymlinks_ValidLinks(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)

	// When
	cmd := exec.Command(binaryPath, "create-symlinks",
		"--container-spec", stateFile,
		"--link", "librbln.so.1::librbln.so",
		"--link", "/usr/lib64/librbln.so.1.0.0::/usr/lib64/librbln.so.1",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "create-symlinks should succeed with valid links. Output: %s", output)
	link1 := filepath.Join(containerRoot, "librbln.so")
	link2 := filepath.Join(containerRoot, "usr/lib64/librbln.so.1")
	target1, err1 := os.Readlink(link1)
	target2, err2 := os.Readlink(link2)
	assert.NoError(t, err1, "first symlink should exist")
	assert.NoError(t, err2, "second symlink should exist")
	assert.Equal(t, "librbln.so.1", target1)
	assert.Equal(t, "/usr/lib64/librbln.so.1.0.0", target2)
}

func TestCreateSymlinks_InvalidLinkFormat(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)

	// When
	cmd := exec.Command(binaryPath, "create-symlinks",
		"--container-spec", stateFile,
		"--link", "invalid-format-no-separator",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail with invalid link format")
	assert.Contains(t, string(output), "invalid symlink specification", "error should indicate invalid format")
	assert.Contains(t, string(output), "expected target::link format", "error should mention expected format")
}

func TestCreateSymlinks_EmptyLinks(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)

	// When
	cmd := exec.Command(binaryPath, "create-symlinks",
		"--container-spec", stateFile,
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "should succeed with no links (graceful no-op). Output: %s", output)
}

func TestCreateSymlinks_InvalidOCIState(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	stateFile := filepath.Join(t.TempDir(), "invalid-state.json")
	err := os.WriteFile(stateFile, []byte("{ invalid json"), 0644)
	require.NoError(t, err)

	// When
	cmd := exec.Command(binaryPath, "create-symlinks",
		"--container-spec", stateFile,
		"--link", "target::link",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail with invalid JSON")
	assert.Contains(t, string(output), "failed to load container state", "error message should indicate state loading failure")
}

func TestCreateSymlinks_MissingBundle(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	stateFile := filepath.Join(t.TempDir(), "state.json")
	state := testOCIState{
		OCIVersion: "1.0.0",
		ID:         "test-container",
		Status:     "creating",
		Bundle:     "",
	}
	stateData, err := json.Marshal(state)
	require.NoError(t, err)
	err = os.WriteFile(stateFile, stateData, 0644)
	require.NoError(t, err)

	// When
	cmd := exec.Command(binaryPath, "create-symlinks",
		"--container-spec", stateFile,
		"--link", "target::link",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail when bundle is empty")
	assert.Contains(t, string(output), "failed to determine container root", "error should indicate container root issue")
}

func TestExecuteCreateSymlinks_Success(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := createSymlinksOptions{
		links:         []string{"librbln.so.1::librbln.so", "/usr/lib64/librbln.so.1.0.0::/usr/lib64/librbln.so.1"},
		containerSpec: "",
	}

	var capturedCalls []struct {
		containerRoot string
		target        string
		link          string
	}

	mockLoader := func(_ string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockCreator := func(containerRoot, target, link string) error {
		capturedCalls = append(capturedCalls, struct {
			containerRoot string
			target        string
			link          string
		}{containerRoot, target, link})
		return nil
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.NoError(t, err)
	assert.Len(t, capturedCalls, 2)
	assert.Equal(t, "/container/root", capturedCalls[0].containerRoot)
	assert.Equal(t, "librbln.so.1", capturedCalls[0].target)
	assert.Equal(t, "librbln.so", capturedCalls[0].link)
	assert.Equal(t, "/container/root", capturedCalls[1].containerRoot)
	assert.Equal(t, "/usr/lib64/librbln.so.1.0.0", capturedCalls[1].target)
	assert.Equal(t, "/usr/lib64/librbln.so.1", capturedCalls[1].link)
}

func TestExecuteCreateSymlinks_StateLoadError(t *testing.T) {
	// Given
	opts := createSymlinksOptions{
		links:         []string{"target::link"},
		containerSpec: "/nonexistent/state.json",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return nil, errors.New("file not found")
	}
	mockCreator := func(containerRoot, target, link string) error {
		return nil
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load container state")
}

func TestExecuteCreateSymlinks_ContainerRootError(t *testing.T) {
	// Given
	opts := createSymlinksOptions{
		links:         []string{"target::link"},
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: ""}, nil
	}
	mockCreator := func(containerRoot, target, link string) error {
		return nil
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine container root")
}

func TestExecuteCreateSymlinks_InvalidLinkFormat(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := createSymlinksOptions{
		links:         []string{"invalid-no-separator"},
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockCreator := func(containerRoot, target, link string) error {
		return nil
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid symlink specification")
	assert.Contains(t, err.Error(), "expected target::link format")
}

func TestExecuteCreateSymlinks_DuplicateLinks(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := createSymlinksOptions{
		links:         []string{"target::link", "target::link", "target::link"},
		containerSpec: "",
	}

	callCount := 0
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockCreator := func(containerRoot, target, link string) error {
		callCount++
		return nil
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount, "duplicate links should be deduplicated")
}

func TestExecuteCreateSymlinks_EmptyLinks(t *testing.T) {
	// Given
	opts := createSymlinksOptions{
		links:         []string{},
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: "/bundle"}, nil
	}
	mockCreator := func(containerRoot, target, link string) error {
		return nil
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.NoError(t, err, "empty links should return nil without processing")
}

func TestExecuteCreateSymlinks_CreatorError(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := createSymlinksOptions{
		links:         []string{"target::link"},
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockCreator := func(containerRoot, target, link string) error {
		return errors.New("permission denied")
	}

	// When
	err := executeCreateSymlinks(opts, mockLoader, mockCreator)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create symlink")
	assert.Contains(t, err.Error(), "permission denied")
}

func TestCreateSymlinkInContainer_NewSymlink(t *testing.T) {
	// Given
	containerRoot := t.TempDir()
	target := "librbln.so.1"
	link := "librbln.so"

	// When
	err := createSymlinkInContainer(containerRoot, target, link)

	// Then
	assert.NoError(t, err)
	linkPath := filepath.Join(containerRoot, link)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget)
}

func TestCreateSymlinkInContainer_ExistingCorrectSymlink(t *testing.T) {
	// Given
	containerRoot := t.TempDir()
	target := "librbln.so.1"
	link := "librbln.so"
	linkPath := filepath.Join(containerRoot, link)
	err := os.Symlink(target, linkPath)
	require.NoError(t, err)

	// When
	err = createSymlinkInContainer(containerRoot, target, link)

	// Then
	assert.NoError(t, err, "should be idempotent when symlink already correct")
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget)
}

func TestCreateSymlinkInContainer_ReplacesIncorrectSymlink(t *testing.T) {
	// Given
	containerRoot := t.TempDir()
	oldTarget := "librbln.so.0"
	newTarget := "librbln.so.1"
	link := "librbln.so"
	linkPath := filepath.Join(containerRoot, link)
	err := os.Symlink(oldTarget, linkPath)
	require.NoError(t, err)

	// When
	err = createSymlinkInContainer(containerRoot, newTarget, link)

	// Then
	assert.NoError(t, err)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, newTarget, actualTarget, "should replace old symlink with new target")
}

func TestCreateSymlinkInContainer_CreatesParentDirectory(t *testing.T) {
	// Given
	containerRoot := t.TempDir()
	target := "librbln.so.1"
	link := "usr/lib64/librbln.so"

	// When
	err := createSymlinkInContainer(containerRoot, target, link)

	// Then
	assert.NoError(t, err)
	linkPath := filepath.Join(containerRoot, link)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget)
	parentDir := filepath.Dir(linkPath)
	info, err := os.Stat(parentDir)
	assert.NoError(t, err)
	assert.True(t, info.IsDir(), "parent directory should be created")
}

func TestCreateSymlinkInContainer_ReplacesRegularFile(t *testing.T) {
	// Given
	containerRoot := t.TempDir()
	target := "librbln.so.1"
	link := "librbln.so"
	linkPath := filepath.Join(containerRoot, link)
	err := os.WriteFile(linkPath, []byte("regular file"), 0644)
	require.NoError(t, err)

	// When
	err = createSymlinkInContainer(containerRoot, target, link)

	// Then
	assert.NoError(t, err)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget, "should replace regular file with symlink")
}

func TestForceCreateSymlink_ReplacesExisting(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	target := "new-target"
	linkPath := filepath.Join(tempDir, "link")
	err := os.Symlink("old-target", linkPath)
	require.NoError(t, err)

	// When
	err = forceCreateSymlink(target, linkPath)

	// Then
	assert.NoError(t, err)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget)
}

func TestForceCreateSymlink_CreatesNew(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	target := "target"
	linkPath := filepath.Join(tempDir, "link")

	// When
	err := forceCreateSymlink(target, linkPath)

	// Then
	assert.NoError(t, err)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget)
}

func TestForceCreateSymlink_RemovesRegularFile(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	target := "target"
	linkPath := filepath.Join(tempDir, "file")
	err := os.WriteFile(linkPath, []byte("content"), 0644)
	require.NoError(t, err)

	// When
	err = forceCreateSymlink(target, linkPath)

	// Then
	assert.NoError(t, err)
	actualTarget, err := os.Readlink(linkPath)
	assert.NoError(t, err)
	assert.Equal(t, target, actualTarget)
}
