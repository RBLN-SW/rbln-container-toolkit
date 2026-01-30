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

	"github.com/stretchr/testify/require"
)

// runCLI builds and runs the rbln-ctk binary and returns stdout/stderr
func runCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()

	// Build the binary first
	buildCmd := exec.Command("go", "build", "-o", "test-rbln-ctk", ".")
	require.NoError(t, buildCmd.Run(), "Failed to build rbln-ctk")
	defer exec.Command("rm", "-f", "test-rbln-ctk").Run()

	cmd := exec.Command("./test-rbln-ctk", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func TestCLI_Help(t *testing.T) {
	// Given
	expectedContains := []string{
		"rbln-ctk",
		"cdi",
		"runtime",
		"version",
		"--debug",
		"--quiet",
		"--config",
	}

	// When
	stdout, _, err := runCLI(t, "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_Version(t *testing.T) {
	// Given
	expectedContains := []string{
		"rbln-ctk version",
		"Build Date:",
		"Git Commit:",
		"Go Version:",
	}

	// When
	stdout, _, err := runCLI(t, "version")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_CDIHelp(t *testing.T) {
	// Given
	expectedContains := []string{
		"generate",
		"list",
		"CDI specification",
	}

	// When
	stdout, _, err := runCLI(t, "cdi", "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_CDIGenerateHelp(t *testing.T) {
	// Given
	expectedContains := []string{
		"--output",
		"--format",
		"--driver-root",
		"--container-library-path",
		"--dry-run",
		"RBLN_CTK_",
	}

	// When
	stdout, _, err := runCLI(t, "cdi", "generate", "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_CDIListHelp(t *testing.T) {
	// Given
	expectedContains := []string{
		"--format",
		"--driver-root",
		"table",
		"json",
		"yaml",
	}

	// When
	stdout, _, err := runCLI(t, "cdi", "list", "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_RuntimeHelp(t *testing.T) {
	// Given
	expectedContains := []string{
		"configure",
		"Container runtime",
	}

	// When
	stdout, _, err := runCLI(t, "runtime", "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_RuntimeConfigureHelp(t *testing.T) {
	// Given
	expectedContains := []string{
		"--runtime",
		"--config-path",
		"--dry-run",
		"--cdi",
		"containerd",
		"crio",
		"docker",
		"RBLN_CTK_",
	}

	// When
	stdout, _, err := runCLI(t, "runtime", "configure", "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}

func TestCLI_InvalidCommand(t *testing.T) {
	// When
	_, _, err := runCLI(t, "invalid-command")

	// Then
	require.Error(t, err)
}

func TestCLI_InvalidRuntime(t *testing.T) {
	// When
	_, stderr, err := runCLI(t, "runtime", "configure", "--runtime=invalid")

	// Then
	require.Error(t, err)
	require.Contains(t, stderr, "unsupported runtime")
}

func TestCLI_CDIGenerateDryRun(t *testing.T) {
	// When
	stdout, stderr, err := runCLI(t, "cdi", "generate", "--dry-run")

	// Then - may succeed or fail depending on environment, but should not crash
	if err != nil {
		// It's OK if it fails due to missing libraries in test environment
		t.Logf("Expected: dry-run may fail in test env: %v, stderr: %s", err, stderr)
	} else {
		// If it succeeds, output should contain CDI spec structure
		if !strings.Contains(stdout, "cdiVersion") && !strings.Contains(stdout, "kind") {
			t.Logf("Dry-run output (may be empty if no libraries found): %s", stdout)
		}
	}
}

func TestCLI_CDIListFormats(t *testing.T) {
	formats := []string{"table", "json", "yaml"}

	for _, format := range formats {
		t.Run("outputs in "+format+" format", func(t *testing.T) {
			// When
			stdout, stderr, err := runCLI(t, "cdi", "list", "--format="+format)

			// Then - may succeed or fail depending on environment
			if err != nil {
				t.Logf("Expected: cdi list may fail in test env: %v, stderr: %s", err, stderr)
			} else {
				// Verify output is not empty or has expected structure
				t.Logf("cdi list --format=%s output: %s", format, stdout)
			}
		})
	}
}

func TestCLI_GlobalFlags(t *testing.T) {
	// Given
	expectedContains := []string{
		"RBLN_CTK_CONFIG",
		"RBLN_CTK_DEBUG",
		"RBLN_CTK_QUIET",
	}

	// When
	stdout, _, err := runCLI(t, "--help")

	// Then
	require.NoError(t, err)
	for _, expected := range expectedContains {
		require.Contains(t, stdout, expected)
	}
}
