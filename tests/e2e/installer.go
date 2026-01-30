/*
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 Rebellions Inc.
*/

package e2e

import (
	"fmt"
	"path/filepath"
)

// ToolkitInstaller handles installation of toolkit binaries for testing.
type ToolkitInstaller struct {
	BinaryDir string
}

// NewToolkitInstaller creates a new installer.
func NewToolkitInstaller(binaryDir string) *ToolkitInstaller {
	return &ToolkitInstaller{
		BinaryDir: binaryDir,
	}
}

// Install installs the toolkit binaries to standard locations.
func (i *ToolkitInstaller) Install(runner Runner) error {
	binaries := []struct {
		name   string
		target string
	}{
		{"rbln-ctk", "/usr/local/bin/rbln-ctk"},
		{"rbln-cdi-hook", "/usr/bin/rbln-cdi-hook"},
		{"rbln-ctk-daemon", "/usr/local/bin/rbln-ctk-daemon"},
	}

	for _, bin := range binaries {
		srcPath := filepath.Join(i.BinaryDir, bin.name)

		// Check if binary exists
		checkScript := fmt.Sprintf(`test -f "%s" && echo "exists"`, srcPath)
		stdout, _, err := runner.Run(checkScript)
		if err != nil || stdout != "exists\n" {
			return fmt.Errorf("binary not found: %s", srcPath)
		}

		// Copy binary to target location
		copyScript := fmt.Sprintf(`cp "%s" "%s" && chmod 755 "%s"`,
			srcPath, bin.target, bin.target)
		_, stderr, err := runner.Run(copyScript)
		if err != nil {
			return fmt.Errorf("failed to install %s: %w, stderr: %s", bin.name, err, stderr)
		}
	}

	// Verify installation
	_, _, err := runner.Run("rbln-ctk version")
	if err != nil {
		return fmt.Errorf("failed to verify installation: %w", err)
	}

	return nil
}

// InstallToContainer installs binaries into a container via docker cp.
func (i *ToolkitInstaller) InstallToContainer(containerName string) error {
	localRunner := NewLocalRunner()

	binaries := []struct {
		name   string
		target string
	}{
		{"rbln-ctk", "/usr/local/bin/rbln-ctk"},
		{"rbln-cdi-hook", "/usr/bin/rbln-cdi-hook"},
		{"rbln-ctk-daemon", "/usr/local/bin/rbln-ctk-daemon"},
	}

	for _, bin := range binaries {
		srcPath := filepath.Join(i.BinaryDir, bin.name)

		// Docker cp the binary
		cpScript := fmt.Sprintf(`docker cp "%s" "%s:%s"`, srcPath, containerName, bin.target)
		_, stderr, err := localRunner.Run(cpScript)
		if err != nil {
			return fmt.Errorf("failed to copy %s to container: %w, stderr: %s", bin.name, err, stderr)
		}

		// Make executable
		chmodScript := fmt.Sprintf(`docker exec "%s" chmod 755 "%s"`, containerName, bin.target)
		_, stderr, err = localRunner.Run(chmodScript)
		if err != nil {
			return fmt.Errorf("failed to chmod %s: %w, stderr: %s", bin.name, err, stderr)
		}
	}

	return nil
}
