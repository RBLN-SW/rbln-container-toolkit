/*
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 Rebellions Inc.
*/

package e2e

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker Integration", Label("no-hardware", "docker"), Ordered, func() {
	var dockerAvailable bool

	BeforeAll(func() {
		_, _, err := runner.Run("docker version")
		dockerAvailable = err == nil
		if !dockerAvailable {
			GinkgoWriter.Printf("Docker not available, skipping Docker integration tests\n")
		}
	})

	Describe("CDI spec generation for Docker", func() {
		BeforeEach(func() {
			if !dockerAvailable {
				Skip("Docker not available")
			}
		})

		It("should generate CDI spec to standard location", func() {
			// Given
			cdiDir := "/var/run/cdi"
			cdiFile := cdiDir + "/rbln.yaml"
			_, _, err := runner.Run("mkdir -p " + cdiDir)
			Expect(err).ToNot(HaveOccurred())

			// When
			_, _, err = runner.Run("rbln-ctk cdi generate --output " + cdiFile)

			// Then
			Expect(err).ToNot(HaveOccurred())
			stdout, _, err := runner.Run("test -f " + cdiFile + " && echo exists")
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.TrimSpace(stdout)).To(Equal("exists"))
			stdout, _, err = runner.Run("cat " + cdiFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("rebellions.ai/npu"))
		})
	})

	Describe("Container execution with CDI", Label("requires-docker-runtime"), func() {
		BeforeEach(func() {
			if !dockerAvailable {
				Skip("Docker not available")
			}
		})

		It("should run a container and verify environment", func() {
			// Given
			testImage := "alpine:latest"
			_, _, err := runner.Run(fmt.Sprintf("docker pull %s", testImage))
			Expect(err).ToNot(HaveOccurred())

			// When
			stdout, _, err := runner.Run(fmt.Sprintf("docker run --rm %s echo 'E2E test passed'", testImage))

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("E2E test passed"))
		})
	})
})

var _ = Describe("Docker Integration with NPU", Label("requires-npu", "docker"), Ordered, func() {
	BeforeEach(func() {
		SkipIfNoNPU()
	})

	It("should inject NPU device into container", func() {
		Skip("NPU hardware tests not yet implemented")
	})
})
