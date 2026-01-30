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

package runtime

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectRuntimeStrict_SingleRuntime(t *testing.T) {
	tests := []struct {
		name       string
		createSock RuntimeType
		expected   RuntimeType
	}{
		{
			name:       "containerd only",
			createSock: RuntimeContainerd,
			expected:   RuntimeContainerd,
		},
		{
			name:       "crio only",
			createSock: RuntimeCRIO,
			expected:   RuntimeCRIO,
		},
		{
			name:       "docker only",
			createSock: RuntimeDocker,
			expected:   RuntimeDocker,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			tmpDir := t.TempDir()
			var sockPath string
			switch tt.createSock {
			case RuntimeContainerd:
				sockPath = filepath.Join(tmpDir, "containerd.sock")
			case RuntimeCRIO:
				sockPath = filepath.Join(tmpDir, "crio.sock")
			case RuntimeDocker:
				sockPath = filepath.Join(tmpDir, "docker.sock")
			}
			err := os.WriteFile(sockPath, []byte{}, 0644)
			require.NoError(t, err)

			opts := &DetectStrictOptions{
				ContainerdSocket: filepath.Join(tmpDir, "containerd.sock"),
				CRIOSocket:       filepath.Join(tmpDir, "crio.sock"),
				DockerSocket:     filepath.Join(tmpDir, "docker.sock"),
			}

			// When
			rt, err := DetectRuntimeStrict(opts)

			// Then
			require.NoError(t, err)
			assert.Equal(t, tt.expected, rt)
		})
	}
}

func TestDetectRuntimeStrict_MultipleRuntimes(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	containerdSock := filepath.Join(tmpDir, "containerd.sock")
	dockerSock := filepath.Join(tmpDir, "docker.sock")

	err := os.WriteFile(containerdSock, []byte{}, 0644)
	require.NoError(t, err)
	err = os.WriteFile(dockerSock, []byte{}, 0644)
	require.NoError(t, err)

	opts := &DetectStrictOptions{
		ContainerdSocket: containerdSock,
		CRIOSocket:       filepath.Join(tmpDir, "crio.sock"),
		DockerSocket:     dockerSock,
	}

	// When
	rt, err := DetectRuntimeStrict(opts)

	// Then
	require.Error(t, err)
	assert.Empty(t, rt)
	assert.Contains(t, err.Error(), "multiple runtimes detected")
	assert.Contains(t, err.Error(), "containerd")
	assert.Contains(t, err.Error(), "docker")
}

func TestDetectRuntimeStrict_NoRuntime(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	opts := &DetectStrictOptions{
		ContainerdSocket: filepath.Join(tmpDir, "containerd.sock"),
		CRIOSocket:       filepath.Join(tmpDir, "crio.sock"),
		DockerSocket:     filepath.Join(tmpDir, "docker.sock"),
	}

	// When
	rt, err := DetectRuntimeStrict(opts)

	// Then
	require.Error(t, err)
	assert.Empty(t, rt)
	assert.Contains(t, err.Error(), "no container runtime detected")
}

func TestDetectRuntimeStrict_OverrideWithExplicit(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	containerdSock := filepath.Join(tmpDir, "containerd.sock")
	dockerSock := filepath.Join(tmpDir, "docker.sock")

	err := os.WriteFile(containerdSock, []byte{}, 0644)
	require.NoError(t, err)
	err = os.WriteFile(dockerSock, []byte{}, 0644)
	require.NoError(t, err)

	opts := &DetectStrictOptions{
		ContainerdSocket: containerdSock,
		CRIOSocket:       filepath.Join(tmpDir, "crio.sock"),
		DockerSocket:     dockerSock,
		ExplicitRuntime:  RuntimeDocker,
	}

	// When
	rt, err := DetectRuntimeStrict(opts)

	// Then
	require.NoError(t, err)
	assert.Equal(t, RuntimeDocker, rt)
}

func TestDetectRuntimeStrict_AllThreeRuntimes(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	containerdSock := filepath.Join(tmpDir, "containerd.sock")
	crioSock := filepath.Join(tmpDir, "crio.sock")
	dockerSock := filepath.Join(tmpDir, "docker.sock")

	err := os.WriteFile(containerdSock, []byte{}, 0644)
	require.NoError(t, err)
	err = os.WriteFile(crioSock, []byte{}, 0644)
	require.NoError(t, err)
	err = os.WriteFile(dockerSock, []byte{}, 0644)
	require.NoError(t, err)

	opts := &DetectStrictOptions{
		ContainerdSocket: containerdSock,
		CRIOSocket:       crioSock,
		DockerSocket:     dockerSock,
	}

	// When
	rt, err := DetectRuntimeStrict(opts)

	// Then
	require.Error(t, err)
	assert.Empty(t, rt)
	assert.Contains(t, err.Error(), "multiple runtimes detected")
}

func TestDetectRuntimeStrict_DefaultOptions(t *testing.T) {
	// Given: nil options (uses default paths)

	// When
	rt, err := DetectRuntimeStrict(nil)

	// Then
	if err != nil {
		assert.Empty(t, rt)
	} else {
		assert.NotEmpty(t, rt)
	}
}
