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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestWriter_Write_InvalidFormat_PreservesExisting(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	require.NoError(t, os.WriteFile(outputPath, []byte("old content"), 0644))
	spec := &specs.Spec{Version: "0.5.0", Kind: "rebellions.ai/npu"}
	writer := NewWriter()

	// When
	err := writer.Write(spec, outputPath, "invalid")

	// Then
	assert.Error(t, err)
	content, readErr := os.ReadFile(outputPath)
	require.NoError(t, readErr)
	assert.Equal(t, "old content", string(content))
}

func TestWriter_Write_NoLeftoverTempFiles(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	spec := &specs.Spec{Version: "0.5.0", Kind: "rebellions.ai/npu"}
	writer := NewWriter()

	// When
	require.NoError(t, writer.Write(spec, outputPath, "yaml"))

	// Then
	entries, err := os.ReadDir(tmpDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "rbln.yaml", entries[0].Name())
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

// TestWriter_AtomicWrite_NoTornReadsUnderConcurrency stresses the atomic
// write contract: a single writer alternates between two distinct specs at
// max throughput while four concurrent readers parse the file. Every read
// must yield a fully formed CDI spec — never a half-written or empty file.
// This is the regression test that proves the daemon's auto-refresh loop is
// safe for in-flight container starts.
func TestWriter_AtomicWrite_NoTornReadsUnderConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrent stress test in -short mode")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	writer := NewWriter()

	specA := &specs.Spec{
		Version:     "0.5.0",
		Kind:        "rebellions.ai/npu",
		Annotations: map[string]string{"version": "1.0.0"},
		Devices: []specs.Device{
			{Name: "runtime", ContainerEdits: specs.ContainerEdits{Env: []string{"A=1"}}},
		},
	}
	specB := &specs.Spec{
		Version:     "0.5.0",
		Kind:        "rebellions.ai/npu",
		Annotations: map[string]string{"version": "2.0.0"},
		Devices: []specs.Device{
			{Name: "runtime", ContainerEdits: specs.ContainerEdits{Env: []string{"B=2"}}},
		},
	}
	require.NoError(t, writer.Write(specA, outputPath, "yaml"))

	var (
		writes  atomic.Int64
		reads   atomic.Int64
		torn    atomic.Int64
		stopped atomic.Bool
	)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stopped.Load() {
			spec := specA
			if writes.Load()%2 == 1 {
				spec = specB
			}
			if err := writer.Write(spec, outputPath, "yaml"); err != nil {
				t.Errorf("write failed: %v", err)
				return
			}
			writes.Add(1)
		}
	}()

	const readers = 4
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stopped.Load() {
				data, err := os.ReadFile(outputPath)
				if err != nil {
					// os.ErrNotExist would imply the rename briefly exposed a
					// missing target — atomic rename rules that out.
					torn.Add(1)
					continue
				}
				var ys yamlSpec
				if err := yaml.Unmarshal(data, &ys); err != nil {
					torn.Add(1)
					continue
				}
				if ys.CDIVersion == "" || ys.Kind == "" {
					torn.Add(1)
					continue
				}
				reads.Add(1)
			}
		}()
	}

	time.Sleep(300 * time.Millisecond)
	stopped.Store(true)
	wg.Wait()

	t.Logf("writes=%d reads=%d torn=%d", writes.Load(), reads.Load(), torn.Load())
	require.Greater(t, writes.Load(), int64(20), "writer should have looped many times")
	require.Greater(t, reads.Load(), int64(20), "readers should have observed many specs")
	assert.Equal(t, int64(0), torn.Load(), "atomic rename must never expose a torn or missing file to readers")
}
