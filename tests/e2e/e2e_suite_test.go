/*
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 Rebellions Inc.
*/

package e2e

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	runner           Runner
	binaryDir        string
	baseImage        string
	runMode          string
	requireNPU       bool
	containerName    string
	toolkitInstaller *ToolkitInstaller
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RBLN Container Toolkit E2E Suite")
}

var _ = BeforeSuite(func() {
	// Given
	binaryDir = getEnvOrDefault("E2E_BINARY_DIR", "./bin")
	baseImage = getEnvOrDefault("E2E_BASE_IMAGE", "ubuntu:22.04")
	runMode = getEnvOrDefault("E2E_RUN_MODE", "container")
	requireNPU = getEnvOrDefault("E2E_REQUIRE_NPU", "false") == "true"
	containerName = "rbln-e2e-test"

	GinkgoWriter.Printf("E2E Configuration:\n")
	GinkgoWriter.Printf("  Binary Dir: %s\n", binaryDir)
	GinkgoWriter.Printf("  Base Image: %s\n", baseImage)
	GinkgoWriter.Printf("  Run Mode: %s\n", runMode)
	GinkgoWriter.Printf("  Require NPU: %v\n", requireNPU)

	localRunner := NewLocalRunner()
	for _, bin := range []string{"rbln-ctk", "rbln-cdi-hook", "rbln-ctk-daemon"} {
		path := binaryDir + "/" + bin
		_, _, err := localRunner.Run("test -f " + path)
		Expect(err).ToNot(HaveOccurred(), "Binary not found: %s", path)
	}

	// When
	var err error
	if runMode == "local" {
		runner = localRunner
		GinkgoWriter.Printf("Using local runner\n")
	} else {
		runner, err = NewNestedContainerRunner(localRunner, baseImage, containerName)
		Expect(err).ToNot(HaveOccurred(), "Failed to create nested container runner")
		GinkgoWriter.Printf("Using nested container runner: %s\n", containerName)
	}

	toolkitInstaller = NewToolkitInstaller(binaryDir)

	// Then
	if runMode != "local" {
		err = toolkitInstaller.InstallToContainer(containerName)
		Expect(err).ToNot(HaveOccurred(), "Failed to install toolkit to container")
	}
})

var _ = AfterSuite(func() {
	// Given
	// When
	// Then
	if runMode != "local" && runner != nil {
		if ncr, ok := runner.(*nestedContainerRunner); ok {
			_ = ncr.Cleanup()
		}
	}
})

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func SkipIfNoNPU() {
	if !requireNPU {
		Skip("Skipping: E2E_REQUIRE_NPU not set (no NPU hardware)")
	}
}
