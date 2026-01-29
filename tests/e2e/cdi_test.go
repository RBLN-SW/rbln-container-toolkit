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

var _ = Describe("CDI", Label("no-hardware"), func() {

	Describe("cdi generate", func() {
		It("should generate CDI spec to stdout", func() {
			// Given
			cmd := "rbln-ctk cdi generate --output -"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("cdiVersion"))
			Expect(stdout).To(ContainSubstring("kind"))
			Expect(stdout).To(ContainSubstring("rebellions.ai/npu"))
		})

		It("should generate CDI spec to file", func() {
			// Given
			tmpFile := "/tmp/rbln-e2e-test.yaml"
			defer runner.Run("rm -f " + tmpFile)

			// When
			_, _, err := runner.Run("rbln-ctk cdi generate --output " + tmpFile)

			// Then
			Expect(err).ToNot(HaveOccurred())
			stdout, _, err := runner.Run("cat " + tmpFile)
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("cdiVersion"))
			Expect(stdout).To(ContainSubstring("rebellions.ai/npu"))
		})

		It("should generate CDI spec in JSON format", func() {
			// Given
			cmd := "rbln-ctk cdi generate --output - --format json"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring(`"cdiVersion"`))
			Expect(stdout).To(ContainSubstring(`"kind"`))
		})

		It("should support dry-run mode", func() {
			// Given
			cmd := "rbln-ctk cdi generate --dry-run"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).To(ContainSubstring("cdiVersion"))
		})
	})

	Describe("cdi validate", func() {
		It("should validate a correct CDI spec", func() {
			// Given
			validSpec := `cdiVersion: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
    containerEdits:
      env:
        - "RBLN_VISIBLE_DEVICES=all"
`
			tmpFile := "/tmp/rbln-valid-spec.yaml"
			defer runner.Run("rm -f " + tmpFile)
			_, _, err := runner.Run("cat > " + tmpFile + " << 'EOF'\n" + validSpec + "EOF")
			Expect(err).ToNot(HaveOccurred())

			// When
			stdout, _, err := runner.Run("rbln-ctk cdi validate --input " + tmpFile)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(strings.ToLower(stdout)).To(ContainSubstring("valid"))
		})

		It("should reject an invalid CDI spec", func() {
			// Given
			invalidSpec := `not: valid: yaml: structure`
			tmpFile := "/tmp/rbln-invalid-spec.yaml"
			defer runner.Run("rm -f " + tmpFile)
			_, _, err := runner.Run("cat > " + tmpFile + " << 'EOF'\n" + invalidSpec + "EOF")
			Expect(err).ToNot(HaveOccurred())

			// When
			_, _, err = runner.Run("rbln-ctk cdi validate --input " + tmpFile)

			// Then
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("cdi list", func() {
		It("should list discovered resources", func() {
			// Given
			cmd := "rbln-ctk cdi list"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			Expect(stdout).ToNot(BeEmpty())
		})

		It("should support JSON output format", func() {
			// Given
			cmd := "rbln-ctk cdi list --format json"

			// When
			stdout, _, err := runner.Run(cmd)

			// Then
			Expect(err).ToNot(HaveOccurred())
			trimmed := strings.TrimSpace(stdout)
			Expect(trimmed).To(Or(HavePrefix("{"), HavePrefix("[")))
		})
	})
})
