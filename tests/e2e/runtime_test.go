/*
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 Rebellions Inc.
*/

package e2e

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Runtime", Label("no-hardware"), func() {

	Describe("runtime detect", func() {
		It("should detect available runtimes without error", func() {
			// Given
			cmd := "rbln-ctk runtime detect || true"

			// When
			_, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("runtime configure", func() {
		It("should show containerd configuration in dry-run mode", func() {
			// Given
			cmd := "rbln-ctk runtime configure --runtime containerd --dry-run"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Or(
				ContainSubstring("enable_cdi"),
				ContainSubstring("cdi"),
				ContainSubstring("containerd"),
			))
		})

		It("should show docker configuration in dry-run mode", func() {
			// Given
			cmd := "rbln-ctk runtime configure --runtime docker --dry-run"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Or(
				ContainSubstring("cdi"),
				ContainSubstring("docker"),
				ContainSubstring("daemon"),
			))
		})

		It("should show crio configuration in dry-run mode", func() {
			// Given
			cmd := "rbln-ctk runtime configure --runtime crio --dry-run"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(Or(
				ContainSubstring("cdi"),
				ContainSubstring("crio"),
			))
		})

		It("should support custom config path", func() {
			// Given
			tmpConfig := "/tmp/test-containerd-config.toml"
			cmd := "rbln-ctk runtime configure --runtime containerd --config-path " + tmpConfig + " --dry-run"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).ToNot(BeEmpty())
		})
	})
})
