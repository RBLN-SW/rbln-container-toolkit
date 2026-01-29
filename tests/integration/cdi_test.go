//go:build integration

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

// Package integration provides integration tests for rbln-container-toolkit.
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCDIGenerateCommand tests the full CDI generate workflow.
func TestCDIGenerateCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Build the binary first
	binaryPath := buildBinary(t)

	t.Run("generate with dry-run", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "cdi", "generate", "--dry-run")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err, "Command should not error: %s", string(output))
	})

	t.Run("generate to file", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "rbln.yaml")

		// When
		cmd := exec.Command(binaryPath, "cdi", "generate", "--output", outputPath)
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err, "Command should not error: %s", string(output))
		assert.FileExists(t, outputPath)
		content, err := os.ReadFile(outputPath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "cdiVersion")
		assert.Contains(t, string(content), "kind")
	})

	t.Run("generate to stdout", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "cdi", "generate", "--output", "-")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		assert.Contains(t, string(output), "cdiVersion")
	})

	t.Run("generate with json format", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "cdi", "generate", "--output", "-", "--format", "json")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		assert.Contains(t, string(output), `"cdiVersion"`)
	})
}

// TestCDIValidateCommand tests the CDI validate workflow.
func TestCDIValidateCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := buildBinary(t)

	t.Run("validate valid spec", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "valid.yaml")
		validSpec := `cdiVersion: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
    containerEdits:
      env:
        - "RBLN_VISIBLE_DEVICES=all"
`
		require.NoError(t, os.WriteFile(specPath, []byte(validSpec), 0644))

		// When
		cmd := exec.Command(binaryPath, "cdi", "validate", "--input", specPath)
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err, "Command output: %s", string(output))
		assert.Contains(t, string(output), "valid")
	})

	t.Run("validate invalid spec", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "invalid.yaml")
		invalidSpec := `not: valid: yaml: content`
		require.NoError(t, os.WriteFile(specPath, []byte(invalidSpec), 0644))

		// When
		cmd := exec.Command(binaryPath, "cdi", "validate", "--input", specPath)
		err := cmd.Run()

		// Then
		assert.Error(t, err)
	})
}

// TestCDIListCommand tests the CDI list workflow.
func TestCDIListCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := buildBinary(t)

	t.Run("list with table format", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "cdi", "list")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err, "Command output: %s", string(output))
	})

	t.Run("list with json format", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "cdi", "list", "--format", "json")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		trimmed := strings.TrimSpace(string(output))
		assert.True(t, strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["),
			"Output should be JSON: %s", trimmed)
	})
}

// TestRuntimeCommands tests runtime-related commands.
func TestRuntimeCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := buildBinary(t)

	t.Run("runtime detect", func(_ *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "runtime", "detect")
		_, _ = cmd.CombinedOutput()

		// Then
		// (no assertions - just verify it doesn't crash)
	})

	t.Run("runtime configure dry-run", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "runtime", "configure",
			"--runtime", "containerd",
			"--config-path", "/tmp/test-config.toml",
			"--dry-run")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err, "Command output: %s", string(output))
		assert.Contains(t, string(output), "enable_cdi")
	})
}

// TestInfoCommand tests the info command.
func TestInfoCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := buildBinary(t)

	t.Run("info displays system information", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "info")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		assert.Contains(t, string(output), "RBLN Container Toolkit")
		assert.Contains(t, string(output), "Version")
		assert.Contains(t, string(output), "Architecture")
	})
}

// TestVersionCommand tests the version command.
func TestVersionCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := buildBinary(t)

	t.Run("version displays version info", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "version")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		assert.Contains(t, string(output), "rbln-ctk version")
		assert.Contains(t, string(output), "Build Date")
		assert.Contains(t, string(output), "Git Commit")
	})
}

// TestGlobalFlags tests global CLI flags.
func TestGlobalFlags(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := buildBinary(t)

	t.Run("help flag", func(t *testing.T) {
		// Given
		// (no setup needed)

		// When
		cmd := exec.Command(binaryPath, "--help")
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		assert.Contains(t, string(output), "USAGE")
		assert.Contains(t, string(output), "COMMANDS")
	})

	t.Run("quiet flag suppresses output", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		outputPath := filepath.Join(tmpDir, "rbln.yaml")

		// When
		cmd := exec.Command(binaryPath, "--quiet", "cdi", "generate", "--output", outputPath)
		output, err := cmd.CombinedOutput()

		// Then
		assert.NoError(t, err)
		assert.NotContains(t, string(output), "CDI spec written")
	})
}

func TestCDILifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Given
	binaryPath := buildBinary(t)
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "rbln.yaml")

	// When
	generateCmd := exec.Command(binaryPath, "cdi", "generate", "--output", specPath)
	generateOutput, generateErr := generateCmd.CombinedOutput()
	validateCmd := exec.Command(binaryPath, "cdi", "validate", "--input", specPath)
	validateOutput, validateErr := validateCmd.CombinedOutput()
	content, readErr := os.ReadFile(specPath)
	listCmd := exec.Command(binaryPath, "cdi", "list", "--format", "json")
	listOutput, listErr := listCmd.CombinedOutput()

	// Then
	require.NoError(t, generateErr, "Generate failed: %s", string(generateOutput))
	assert.FileExists(t, specPath)
	assert.NoError(t, validateErr, "Validate failed: %s", string(validateOutput))
	assert.Contains(t, strings.ToLower(string(validateOutput)), "valid")
	require.NoError(t, readErr)
	assert.Contains(t, string(content), "cdiVersion")
	assert.Contains(t, string(content), "kind")
	assert.Contains(t, string(content), "rebellions.ai")
	assert.NoError(t, listErr, "List failed: %s", string(listOutput))
}

// buildBinary builds the rbln-ctk binary for testing.
func buildBinary(t *testing.T) string {
	t.Helper()

	projectRoot := filepath.Join("..", "..")
	binaryPath := filepath.Join(projectRoot, "bin", "rbln-ctk-test")

	cmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/rbln-ctk")
	cmd.Dir = projectRoot
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "Failed to build binary: %s", string(output))

	return binaryPath
}
