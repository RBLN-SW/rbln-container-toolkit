/*
SPDX-License-Identifier: Apache-2.0
Copyright (c) 2026 Rebellions Inc.
*/

package e2e

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CLI", Label("no-hardware"), func() {

	Describe("version command", func() {
		It("should display version information", func() {
			// Given
			cmd := "rbln-ctk version"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("rbln-ctk"))
			Expect(stdout).To(Or(
				ContainSubstring("version"),
				ContainSubstring("Version"),
			))
		})
	})

	Describe("info command", func() {
		It("should display system information", func() {
			// Given
			cmd := "rbln-ctk info"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("RBLN Container Toolkit"))
		})
	})

	Describe("help command", func() {
		It("should display help for main command", func() {
			// Given
			cmd := "rbln-ctk --help"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.ToLower(stdout)).To(ContainSubstring("usage"))
			Expect(strings.ToLower(stdout)).To(ContainSubstring("commands"))
		})

		It("should display help for cdi subcommand", func() {
			// Given
			cmd := "rbln-ctk cdi --help"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.ToLower(stdout)).To(ContainSubstring("generate"))
		})

		It("should display help for runtime subcommand", func() {
			// Given
			cmd := "rbln-ctk runtime --help"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.ToLower(stdout)).To(ContainSubstring("configure"))
		})
	})

	Describe("global flags", func() {
		It("should support --debug flag", func() {
			// Given
			cmd := "rbln-ctk --debug version"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).ToNot(BeEmpty())
		})

		It("should support --quiet flag", func() {
			// Given
			tmpFile := "/tmp/rbln-quiet-test.yaml"
			defer runner.Run("rm -f " + tmpFile)

			// When
			_, _, err := runner.Run("rbln-ctk --quiet cdi generate --output " + tmpFile)

			// Then
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
