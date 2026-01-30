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
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// runInstaller runs the installer binary and returns stdout/stderr
func runInstaller(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "test-installer", ".")
	if err := buildCmd.Run(); err != nil {
		t.Fatalf("Failed to build installer: %v", err)
	}
	defer exec.Command("rm", "-f", "test-installer").Run()

	cmd := exec.Command("./test-installer", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestCLI_Help(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "--help")

	// Then
	assert.NoError(t, err)
	expectedContains := []string{
		"RBLN Container Toolkit Daemon",
		"runtime",
		"--debug",
	}
	for _, expected := range expectedContains {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_Version(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "--version")

	// Then
	assert.NoError(t, err)
	assert.Contains(t, stdout, "rbln-ctk-daemon")
}

func TestCLI_RuntimeHelp(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "runtime", "--help")

	// Then
	assert.NoError(t, err)
	expectedContains := []string{
		"docker",
		"containerd",
		"crio",
		"--host-root-mount",
		"--cdi-spec-dir",
		"--restart-mode",
		"--dry-run",
	}
	for _, expected := range expectedContains {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_DockerHelp(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "runtime", "docker", "--help")

	// Then
	assert.NoError(t, err)
	expectedContains := []string{
		"setup",
		"cleanup",
	}
	for _, expected := range expectedContains {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_DockerSetupHelp(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "runtime", "docker", "setup", "--help")

	// Then
	assert.NoError(t, err)
	expectedContains := []string{
		"Docker",
		"CDI",
		"SIGHUP",
	}
	for _, expected := range expectedContains {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_ContainerdHelp(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "runtime", "containerd", "--help")

	// Then
	assert.NoError(t, err)
	expectedContains := []string{
		"setup",
		"cleanup",
	}
	for _, expected := range expectedContains {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_CrioHelp(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "runtime", "crio", "--help")

	// Then
	assert.NoError(t, err)
	expectedContains := []string{
		"setup",
		"cleanup",
	}
	for _, expected := range expectedContains {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_EnvironmentVariables(t *testing.T) {
	// When
	stdout, _, err := runInstaller(t, "runtime", "--help")

	// Then
	assert.NoError(t, err)
	expectedEnvVars := []string{
		"RBLN_CTK_DAEMON_",
	}
	for _, expected := range expectedEnvVars {
		assert.Contains(t, stdout, expected)
	}
}

func TestCLI_InvalidCommand(t *testing.T) {
	// When
	_, stderr, err := runInstaller(t, "invalid-command")

	// Then
	assert.Error(t, err)
	if !strings.Contains(stderr, "unknown command") {
		t.Logf("stderr: %s", stderr)
	}
}

func TestCLI_CrioSignalModeError(t *testing.T) {
	// When
	_, stderr, err := runInstaller(t, "runtime", "--restart-mode=signal", "crio", "setup", "--dry-run")

	// Then
	assert.Error(t, err)
	combined := stderr
	if !strings.Contains(combined, "signal") || !strings.Contains(combined, "CRI-O") {
		t.Logf("Output: %s", combined)
	}
}
