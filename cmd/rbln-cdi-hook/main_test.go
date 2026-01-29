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
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHookCLI_Help(t *testing.T) {
	// Given
	cmd := exec.Command("go", "build", "-o", "/tmp/rbln-cdi-hook-test", ".")
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not build test binary: %v", err)
	}

	// When
	helpCmd := exec.Command("/tmp/rbln-cdi-hook-test", "--help")
	output, err := helpCmd.CombinedOutput()

	// Then
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			assert.Fail(t, "--help failed", err)
		}
	}

	outputStr := string(output)
	assert.Contains(t, outputStr, "rbln-cdi-hook", "--help should contain 'rbln-cdi-hook'")
	assert.Contains(t, outputStr, "update-ldcache", "--help should list 'update-ldcache' command")
}

func TestHookCLI_UpdateLdcacheHelp(t *testing.T) {
	// Given
	cmd := exec.Command("go", "build", "-o", "/tmp/rbln-cdi-hook-test", ".")
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not build test binary: %v", err)
	}

	// When
	helpCmd := exec.Command("/tmp/rbln-cdi-hook-test", "update-ldcache", "--help")
	output, err := helpCmd.CombinedOutput()

	// Then
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			assert.Fail(t, "update-ldcache --help failed", err)
		}
	}

	outputStr := string(output)
	assert.Contains(t, outputStr, "--folder", "update-ldcache --help should show '--folder' flag")
	assert.Contains(t, outputStr, "--ldconfig-path", "update-ldcache --help should show '--ldconfig-path' flag")
	assert.Contains(t, outputStr, "--container-spec", "update-ldcache --help should show '--container-spec' flag")
}

func TestHookCLI_Version(t *testing.T) {
	// Given
	cmd := exec.Command("go", "build", "-o", "/tmp/rbln-cdi-hook-test", ".")
	if err := cmd.Run(); err != nil {
		t.Skipf("Could not build test binary: %v", err)
	}

	// When
	versionCmd := exec.Command("/tmp/rbln-cdi-hook-test", "--version")
	output, err := versionCmd.CombinedOutput()

	// Then
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() != 0 {
			assert.Fail(t, "--version failed", err)
		}
	}

	outputStr := string(output)
	assert.Contains(t, outputStr, "rbln-cdi-hook", "--version should contain 'rbln-cdi-hook'")
}
