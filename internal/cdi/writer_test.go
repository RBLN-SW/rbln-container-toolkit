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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tags.cncf.io/container-device-interface/specs-go"
)

func TestWriter_Write_YAML(t *testing.T) {
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
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "yaml")

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "cdiVersion")
	assert.Contains(t, string(content), "rebellions.ai/npu")
	assert.Contains(t, string(content), "runtime")
}

func TestWriter_Write_JSON(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
		Devices: []specs.Device{
			{Name: "runtime"},
		},
	}
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.json")
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "json")

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "\"cdiVersion\"")
	assert.Contains(t, string(content), "\"rebellions.ai/npu\"")
}

func TestWriter_Write_CreatesDirectory(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "var", "run", "cdi", "rbln.yaml")
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
	}
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "yaml")

	// Then
	require.NoError(t, err)
	assert.FileExists(t, outputPath)
}

func TestWriter_Write_OverwritesExisting(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	require.NoError(t, os.WriteFile(outputPath, []byte("old content"), 0644))
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
	}
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "yaml")

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "old content")
	assert.Contains(t, string(content), "rebellions.ai/npu")
}

func TestWriter_Write_Permissions(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
	}
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "yaml")
	require.NoError(t, err)

	// Then
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}

func TestWriter_WriteToStdout_YAML(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
		Devices: []specs.Device{
			{Name: "runtime"},
		},
	}
	writer := NewWriter()
	var buf bytes.Buffer

	// When
	err := writer.WriteToWriter(spec, &buf, "yaml")

	// Then
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "cdiVersion")
	assert.Contains(t, output, "rebellions.ai/npu")
}

func TestWriter_WriteToStdout_JSON(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
		Devices: []specs.Device{
			{Name: "runtime"},
		},
	}
	writer := NewWriter()
	var buf bytes.Buffer

	// When
	err := writer.WriteToWriter(spec, &buf, "json")

	// Then
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "\"cdiVersion\"")
	assert.Contains(t, output, "\"rebellions.ai/npu\"")
}

func TestWriter_Write_InvalidFormat(t *testing.T) {
	// Given
	spec := &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/npu",
	}
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "invalid")

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "format")
}

func TestWriter_Write_NilSpec(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	writer := NewWriter()

	// When
	err := writer.Write(nil, outputPath, "yaml")

	// Then
	assert.Error(t, err)
}

func TestWriter_Write_MountOptionsFlowStyle(t *testing.T) {
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
							Options:       []string{"ro", "nosuid", "nodev", "bind"},
						},
					},
				},
			},
		},
	}
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "yaml")

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	// Verify flow style: options should be inline array format [ro, nosuid, nodev, bind]
	assert.Contains(t, string(content), "options: [ro,")
}
