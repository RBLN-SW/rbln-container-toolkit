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

package cdi

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tags.cncf.io/container-device-interface/specs-go"
)

func TestValidator_Validate_ValidSpec(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
		Devices: []specs.Device{
			{
				Name: "runtime",
				ContainerEdits: specs.ContainerEdits{
					Mounts: []*specs.Mount{
						{
							HostPath:      "/usr/lib64/librbln-ml.so",
							ContainerPath: "/usr/lib64/librbln-ml.so",
							Options:       []string{"ro", "bind"},
						},
					},
				},
			},
		},
	}
	validator := NewValidator()

	// When
	err := validator.Validate(spec)

	// Then
	assert.NoError(t, err)
}

func TestValidator_Validate_NilSpec(t *testing.T) {
	// Given
	validator := NewValidator()

	// When
	err := validator.Validate(nil)

	// Then
	assert.Error(t, err)
}

func TestValidator_Validate_MissingVersion(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Kind: "rebellions.ai/npu",
		Devices: []specs.Device{
			{Name: "runtime"},
		},
	}
	validator := NewValidator()

	// When
	err := validator.Validate(spec)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cdiVersion")
}

func TestValidator_Validate_InvalidKind(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "invalid-kind",
		Devices: []specs.Device{
			{Name: "runtime"},
		},
	}
	validator := NewValidator()

	// When
	err := validator.Validate(spec)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "kind")
}

func TestValidator_Validate_NoDevices(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
		Devices: []specs.Device{},
	}
	validator := NewValidator()

	// When
	err := validator.Validate(spec)

	// Then
	assert.NoError(t, err)
}

func TestValidator_Validate_EmptyDeviceName(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
		Devices: []specs.Device{
			{Name: ""},
		},
	}
	validator := NewValidator()

	// When
	err := validator.Validate(spec)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty name")
}

func TestValidator_ValidateFile_ValidFile(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "rbln.yaml")
	specContent := `version: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
    containeredits:
      mounts:
        - hostpath: "/usr/lib64/librbln-ml.so"
          containerpath: "/usr/lib64/librbln-ml.so"
          options: ["ro", "bind"]
`
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0644))
	validator := NewValidator()

	// When
	err := validator.ValidateFile(specPath)

	// Then
	assert.NoError(t, err)
}

func TestValidator_ValidateFile_NonExistent(t *testing.T) {
	// Given
	nonExistentPath := "/nonexistent/path/rbln.yaml"
	validator := NewValidator()

	// When
	err := validator.ValidateFile(nonExistentPath)

	// Then
	assert.Error(t, err)
}

func TestValidator_ValidateFile_InvalidYAML(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "rbln.yaml")
	invalidContent := `cdiVersion: [invalid yaml`
	require.NoError(t, os.WriteFile(specPath, []byte(invalidContent), 0644))
	validator := NewValidator()

	// When
	err := validator.ValidateFile(specPath)

	// Then
	assert.Error(t, err)
}

func TestValidator_ValidateFile_InvalidSpec(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "rbln.yaml")
	invalidSpec := `kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
`
	require.NoError(t, os.WriteFile(specPath, []byte(invalidSpec), 0644))
	validator := NewValidator()

	// When
	err := validator.ValidateFile(specPath)

	// Then
	assert.Error(t, err)
}
