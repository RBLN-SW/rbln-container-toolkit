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

	rbln_errors "github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
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
	specContent := `cdiVersion: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
    containerEdits:
      mounts:
        - hostPath: "/usr/lib64/librbln-ml.so"
          containerPath: "/usr/lib64/librbln-ml.so"
          options: ["ro", "bind"]
containerEdits:
  hooks:
    - hookName: createContainer
      path: /usr/local/bin/rbln-cdi-hook
      args: ["rbln-cdi-hook", "update-ldcache"]
`
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0644))
	validator := NewValidator()

	// When
	err := validator.ValidateFile(specPath)

	// Then
	assert.NoError(t, err)
}

func TestValidator_ValidateFile_JSON(t *testing.T) {
	// Given - CDI runtimes read .json specs with the same decoder as .yaml
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "rbln.json")
	specContent := `{"cdiVersion":"0.5.0","kind":"rebellions.ai/npu",` +
		`"devices":[{"name":"0","containerEdits":{"deviceNodes":[{"path":"/dev/rbln0"}]}}],` +
		`"containerEdits":{"hooks":[{"hookName":"createContainer","path":"/usr/local/bin/rbln-cdi-hook"}]}}`
	require.NoError(t, os.WriteFile(specPath, []byte(specContent), 0644))
	validator := NewValidator()

	// When
	err := validator.ValidateFile(specPath)

	// Then
	assert.NoError(t, err)
}

func TestValidator_ValidateFile_RejectsKeysStrictRuntimesRefuse(t *testing.T) {
	// CDI v1.1.1+ runtimes (CRI-O 1.37, containerd 2.4, Docker 29.10,
	// Podman 6.2) match keys case-sensitively and refuse unknown ones, so a
	// lowercased key fails the whole spec file there and must fail here.
	tests := []struct {
		name    string
		content string
		badKey  string
	}{
		{
			name: "lowercased hookName",
			content: `cdiVersion: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "0"
    containerEdits:
      deviceNodes:
        - path: /dev/rbln0
containerEdits:
  hooks:
    - hookname: createContainer
      path: /usr/local/bin/rbln-cdi-hook
`,
			badKey: "hookname",
		},
		{
			name: "lowercased top-level keys",
			content: `version: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
`,
			badKey: "version",
		},
		{
			name: "lowercased mount keys",
			content: `cdiVersion: "0.5.0"
kind: "rebellions.ai/npu"
devices:
  - name: "runtime"
    containerEdits:
      mounts:
        - hostpath: /usr/lib64/librbln-ml.so
          containerPath: /usr/lib64/librbln-ml.so
`,
			badKey: "hostpath",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			specPath := filepath.Join(t.TempDir(), "rbln.yaml")
			require.NoError(t, os.WriteFile(specPath, []byte(tt.content), 0644))
			validator := NewValidator()

			// When
			err := validator.ValidateFile(specPath)

			// Then
			require.Error(t, err)
			assert.ErrorIs(t, err, rbln_errors.ErrInvalidCDISpec)
			assert.Contains(t, err.Error(), tt.badKey)
		})
	}
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
