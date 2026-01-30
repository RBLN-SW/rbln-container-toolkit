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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/oci"
)

type testOCIState struct {
	OCIVersion  string            `json:"ociVersion"`
	ID          string            `json:"id"`
	Status      string            `json:"status"`
	Pid         int               `json:"pid,omitempty"`
	Bundle      string            `json:"bundle"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type testOCISpec struct {
	Root *testOCIRoot `json:"root,omitempty"`
}

type testOCIRoot struct {
	Path     string `json:"path"`
	Readonly bool   `json:"readonly,omitempty"`
}

func createTestBundle(t *testing.T, rootPath string) string {
	t.Helper()
	bundleDir := t.TempDir()

	spec := testOCISpec{
		Root: &testOCIRoot{
			Path: rootPath,
		},
	}

	specData, err := json.Marshal(spec)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(bundleDir, "config.json"), specData, 0644)
	require.NoError(t, err)

	return bundleDir
}

func createTestOCIState(t *testing.T, bundleDir string) string {
	t.Helper()
	stateFile := filepath.Join(t.TempDir(), "state.json")

	state := testOCIState{
		OCIVersion: "1.0.0",
		ID:         "test-container-123",
		Status:     "creating",
		Bundle:     bundleDir,
	}

	stateData, err := json.Marshal(state)
	require.NoError(t, err)

	err = os.WriteFile(stateFile, stateData, 0644)
	require.NoError(t, err)

	return stateFile
}

func buildHookBinary(t *testing.T) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "rbln-cdi-hook")

	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("Could not build test binary: %v\nOutput: %s", err, output)
	}

	return binaryPath
}

func TestUpdateLdcache_ValidOCIState(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)
	ldsoconfDir := filepath.Join(containerRoot, "etc", "ld.so.conf.d")
	err := os.MkdirAll(ldsoconfDir, 0755)
	require.NoError(t, err)

	if _, statErr := os.Stat("/sbin/ldconfig"); os.IsNotExist(statErr) {
		t.Skipf("ldconfig not available on this system")
	}

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
		"--folder", "/opt/rbln/lib",
		"--ldconfig-path", "/sbin/ldconfig",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "update-ldcache should succeed with valid state. Output: %s", output)
}

func TestUpdateLdcache_InvalidOCIStateJSON(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	stateFile := filepath.Join(t.TempDir(), "invalid-state.json")
	err := os.WriteFile(stateFile, []byte("{ invalid json"), 0644)
	require.NoError(t, err)

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail with invalid JSON")
	assert.Contains(t, string(output), "failed to load container state", "error message should indicate state loading failure")
}

func TestUpdateLdcache_MissingBundle(t *testing.T) {
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
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail when bundle is empty")
	assert.Contains(t, string(output), "failed to determine container root", "error should indicate container root issue")
}

func TestUpdateLdcache_NonexistentBundle(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	stateFile := filepath.Join(t.TempDir(), "state.json")
	state := testOCIState{
		OCIVersion: "1.0.0",
		ID:         "test-container",
		Status:     "creating",
		Bundle:     "/nonexistent/bundle/path",
	}
	stateData, err := json.Marshal(state)
	require.NoError(t, err)
	err = os.WriteFile(stateFile, stateData, 0644)
	require.NoError(t, err)

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail when bundle doesn't exist")
	assert.Contains(t, string(output), "failed to determine container root", "error should indicate container root issue")
}

func TestUpdateLdcache_SystemRootProtection(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	bundleDir := t.TempDir()
	spec := testOCISpec{
		Root: &testOCIRoot{
			Path: "/",
		},
	}
	specData, err := json.Marshal(spec)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(bundleDir, "config.json"), specData, 0644)
	require.NoError(t, err)

	stateFile := filepath.Join(t.TempDir(), "state.json")
	state := testOCIState{
		OCIVersion: "1.0.0",
		ID:         "test-container",
		Status:     "creating",
		Bundle:     bundleDir,
	}
	stateData, err := json.Marshal(state)
	require.NoError(t, err)
	err = os.WriteFile(stateFile, stateData, 0644)
	require.NoError(t, err)

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail when root is /")
	outputStr := string(output)
	assert.True(t,
		strings.Contains(outputStr, "system root") ||
			strings.Contains(outputStr, "container root"),
		"error should indicate system root protection. Got: %s", outputStr)
}

func TestUpdateLdcache_LdconfigNotFound(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)
	ldsoconfDir := filepath.Join(containerRoot, "etc", "ld.so.conf.d")
	err := os.MkdirAll(ldsoconfDir, 0755)
	require.NoError(t, err)

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
		"--ldconfig-path", "/nonexistent/ldconfig",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail when ldconfig binary not found")
	outputStr := string(output)
	assert.True(t,
		strings.Contains(outputStr, "ldconfig") ||
			strings.Contains(outputStr, "not found"),
		"error should mention ldconfig issue. Got: %s", outputStr)
}

func TestUpdateLdcache_MissingContainerSpecFile(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", "/nonexistent/state.json",
		"--folder", "/usr/lib64",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.Error(t, err, "should fail when state file doesn't exist")
	assert.Contains(t, string(output), "failed to load container state", "error should indicate state loading failure")
}

func TestUpdateLdcache_EmptyFolders(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)
	ldsoconfDir := filepath.Join(containerRoot, "etc", "ld.so.conf.d")
	err := os.MkdirAll(ldsoconfDir, 0755)
	require.NoError(t, err)

	if _, statErr := os.Stat("/sbin/ldconfig"); os.IsNotExist(statErr) {
		t.Skipf("ldconfig not available on this system")
	}

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--ldconfig-path", "/sbin/ldconfig",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "should succeed with no folders (graceful skip). Output: %s", output)
}

func TestUpdateLdcache_RelativeRootPath(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	bundleDir := t.TempDir()
	containerRoot := filepath.Join(bundleDir, "rootfs")
	err := os.MkdirAll(containerRoot, 0755)
	require.NoError(t, err)

	spec := testOCISpec{
		Root: &testOCIRoot{
			Path: "rootfs",
		},
	}
	specData, err := json.Marshal(spec)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(bundleDir, "config.json"), specData, 0644)
	require.NoError(t, err)

	stateFile := filepath.Join(t.TempDir(), "state.json")
	state := testOCIState{
		OCIVersion: "1.0.0",
		ID:         "test-container",
		Status:     "creating",
		Bundle:     bundleDir,
	}
	stateData, err := json.Marshal(state)
	require.NoError(t, err)
	err = os.WriteFile(stateFile, stateData, 0644)
	require.NoError(t, err)

	ldsoconfDir := filepath.Join(containerRoot, "etc", "ld.so.conf.d")
	err = os.MkdirAll(ldsoconfDir, 0755)
	require.NoError(t, err)

	if _, statErr := os.Stat("/sbin/ldconfig"); os.IsNotExist(statErr) {
		t.Skipf("ldconfig not available on this system")
	}

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
		"--ldconfig-path", "/sbin/ldconfig",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "should resolve relative rootfs path correctly. Output: %s", output)
}

func TestUpdateLdcache_MultipleFolders(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)
	containerRoot := t.TempDir()
	bundleDir := createTestBundle(t, containerRoot)
	stateFile := createTestOCIState(t, bundleDir)
	ldsoconfDir := filepath.Join(containerRoot, "etc", "ld.so.conf.d")
	err := os.MkdirAll(ldsoconfDir, 0755)
	require.NoError(t, err)

	if _, statErr := os.Stat("/sbin/ldconfig"); os.IsNotExist(statErr) {
		t.Skipf("ldconfig not available on this system")
	}

	// When
	cmd := exec.Command(binaryPath, "update-ldcache",
		"--container-spec", stateFile,
		"--folder", "/usr/lib64",
		"--folder", "/opt/rbln/lib",
		"--folder", "/usr/local/lib",
		"--ldconfig-path", "/sbin/ldconfig",
	)
	output, err := cmd.CombinedOutput()

	// Then
	assert.NoError(t, err, "should handle multiple folders. Output: %s", output)
}

func TestUpdateLdcache_HelpOutput(t *testing.T) {
	// Given
	binaryPath := buildHookBinary(t)

	// When
	cmd := exec.Command(binaryPath, "update-ldcache", "--help")
	output, err := cmd.CombinedOutput()

	// Then
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
		assert.Fail(t, "--help should not fail", err)
	}

	outputStr := string(output)
	assert.Contains(t, outputStr, "--folder", "help should mention --folder flag")
	assert.Contains(t, outputStr, "--ldconfig-path", "help should mention --ldconfig-path flag")
	assert.Contains(t, outputStr, "--container-spec", "help should mention --container-spec flag")
	assert.Contains(t, outputStr, "OCI", "help should mention OCI")
}

func TestExecuteUpdateLdcache_StateLoadError(t *testing.T) {
	// Given
	opts := updateLdcacheOptions{
		folders:       []string{"/usr/lib64"},
		ldconfigPath:  "/sbin/ldconfig",
		containerSpec: "/nonexistent/state.json",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return nil, errors.New("file not found")
	}
	mockRunnerFactory := func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
		return nil, nil
	}
	mockRunner := func(cmd *exec.Cmd) error {
		return nil
	}

	// When
	err := executeUpdateLdcache(opts, mockLoader, mockRunnerFactory, mockRunner)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load container state")
}

func TestExecuteUpdateLdcache_ContainerRootError(t *testing.T) {
	// Given
	opts := updateLdcacheOptions{
		folders:       []string{"/usr/lib64"},
		ldconfigPath:  "/sbin/ldconfig",
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: ""}, nil
	}
	mockRunnerFactory := func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
		return nil, nil
	}
	mockRunner := func(cmd *exec.Cmd) error {
		return nil
	}

	// When
	err := executeUpdateLdcache(opts, mockLoader, mockRunnerFactory, mockRunner)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to determine container root")
}

func TestExecuteUpdateLdcache_RunnerCreationError(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := updateLdcacheOptions{
		folders:       []string{"/usr/lib64"},
		ldconfigPath:  "/sbin/ldconfig",
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockRunnerFactory := func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
		return nil, errors.New("failed to create runner")
	}
	mockRunner := func(cmd *exec.Cmd) error {
		return nil
	}

	// When
	err := executeUpdateLdcache(opts, mockLoader, mockRunnerFactory, mockRunner)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create ldconfig runner")
}

func TestExecuteUpdateLdcache_LdconfigRunError(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := updateLdcacheOptions{
		folders:       []string{"/usr/lib64"},
		ldconfigPath:  "/sbin/ldconfig",
		containerSpec: "",
	}
	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockRunnerFactory := func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
		return exec.Command("true"), nil
	}
	mockRunner := func(cmd *exec.Cmd) error {
		return errors.New("ldconfig failed with exit code 1")
	}

	// When
	err := executeUpdateLdcache(opts, mockLoader, mockRunnerFactory, mockRunner)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ldconfig execution failed")
}

func TestExecuteUpdateLdcache_Success(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := updateLdcacheOptions{
		folders:       []string{"/usr/lib64", "/opt/rbln/lib"},
		ldconfigPath:  "/sbin/ldconfig",
		containerSpec: "",
	}

	var capturedLdconfigPath, capturedContainerRoot string
	var capturedDirectories []string

	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockRunnerFactory := func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
		capturedLdconfigPath = ldconfigPath
		capturedContainerRoot = containerRoot
		capturedDirectories = directories
		return exec.Command("true"), nil
	}
	mockRunner := func(cmd *exec.Cmd) error {
		return nil
	}

	// When
	err := executeUpdateLdcache(opts, mockLoader, mockRunnerFactory, mockRunner)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "/sbin/ldconfig", capturedLdconfigPath)
	assert.Equal(t, "/container/root", capturedContainerRoot)
	assert.Equal(t, []string{"/usr/lib64", "/opt/rbln/lib"}, capturedDirectories)
}

func TestExecuteUpdateLdcache_EmptyFolders(t *testing.T) {
	// Given
	tempDir := t.TempDir()
	bundleDir := filepath.Join(tempDir, "bundle")
	require.NoError(t, os.MkdirAll(bundleDir, 0755))

	configJSON := `{"root": {"path": "/container/root"}}`
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "config.json"), []byte(configJSON), 0644))

	opts := updateLdcacheOptions{
		folders:       []string{},
		ldconfigPath:  "/sbin/ldconfig",
		containerSpec: "",
	}

	mockLoader := func(filename string) (*oci.State, error) {
		return &oci.State{Bundle: bundleDir}, nil
	}
	mockRunnerFactory := func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
		return exec.Command("true"), nil
	}
	runnerCalled := false
	mockRunner := func(cmd *exec.Cmd) error {
		runnerCalled = true
		return nil
	}

	// When
	err := executeUpdateLdcache(opts, mockLoader, mockRunnerFactory, mockRunner)

	// Then
	assert.NoError(t, err)
	assert.True(t, runnerCalled, "runner should still be called even with empty folders")
}
