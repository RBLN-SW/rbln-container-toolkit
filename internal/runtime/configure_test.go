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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRuntimeType_String(t *testing.T) {
	tests := []struct {
		rt       RuntimeType
		expected string
	}{
		{RuntimeContainerd, "containerd"},
		{RuntimeCRIO, "crio"},
		{RuntimeDocker, "docker"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.rt))
		})
	}
}

func TestNewConfigurator_Containerd(t *testing.T) {
	// Given: A containerd runtime type
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// When: Creating a configurator
	cfg, err := NewConfigurator(RuntimeContainerd, configPath, nil)

	// Then: Should create containerd configurator without error
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestNewConfigurator_CRIO(t *testing.T) {
	// Given: A CRI-O runtime type
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "99-rbln.conf")

	// When: Creating a configurator
	cfg, err := NewConfigurator(RuntimeCRIO, configPath, nil)

	// Then: Should create CRI-O configurator without error
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestNewConfigurator_Docker(t *testing.T) {
	// Given: A Docker runtime type
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "daemon.json")

	// When: Creating a configurator
	cfg, err := NewConfigurator(RuntimeDocker, configPath, nil)

	// Then: Should create Docker configurator without error
	require.NoError(t, err)
	assert.NotNil(t, cfg)
}

func TestNewConfigurator_InvalidRuntime(t *testing.T) {
	// Given: An invalid runtime type
	// When: Creating a configurator
	cfg, err := NewConfigurator(RuntimeType("invalid"), "", nil)

	// Then: Should return error
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestContainerdConfigurator_Configure(t *testing.T) {
	// Given: A containerd config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	initialConfig := `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	cfg, err := NewConfigurator(RuntimeContainerd, configPath, nil)
	require.NoError(t, err)

	// When
	err = cfg.Configure()

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "enable_cdi")
}

func TestContainerdConfigurator_DryRun(t *testing.T) {
	// Given: A containerd config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	initialConfig := `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
`
	require.NoError(t, os.WriteFile(configPath, []byte(initialConfig), 0644))

	cfg, err := NewConfigurator(RuntimeContainerd, configPath, nil)
	require.NoError(t, err)

	// When
	diff, err := cfg.DryRun()

	// Then
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
	assert.Contains(t, diff, "enable_cdi")
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "enable_cdi")
}

func TestCRIOConfigurator_Configure(t *testing.T) {
	// Given: A CRI-O config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "crio.conf.d")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	configPath := filepath.Join(configDir, "99-rbln.conf")

	cfg, err := NewConfigurator(RuntimeCRIO, configPath, nil)
	require.NoError(t, err)

	// When
	err = cfg.Configure()

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "[crio.runtime]")
}

func TestCRIOConfigurator_DryRun(t *testing.T) {
	// Given: A CRI-O config directory
	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "crio.conf.d")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	configPath := filepath.Join(configDir, "99-rbln.conf")

	cfg, err := NewConfigurator(RuntimeCRIO, configPath, nil)
	require.NoError(t, err)

	// When
	diff, err := cfg.DryRun()

	// Then
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
	_, err = os.Stat(configPath)
	assert.True(t, os.IsNotExist(err))
}

func TestDockerConfigurator_Configure(t *testing.T) {
	// Given: A Docker daemon.json file (or empty)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "daemon.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0644))

	cfg, err := NewConfigurator(RuntimeDocker, configPath, nil)
	require.NoError(t, err)

	// When
	err = cfg.Configure()

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "cdi-devices")
}

func TestDockerConfigurator_Configure_ExistingConfig(t *testing.T) {
	// Given: An existing Docker daemon.json with other settings
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "daemon.json")

	existingConfig := `{
  "log-driver": "json-file",
  "storage-driver": "overlay2"
}`
	require.NoError(t, os.WriteFile(configPath, []byte(existingConfig), 0644))

	cfg, err := NewConfigurator(RuntimeDocker, configPath, nil)
	require.NoError(t, err)

	// When
	err = cfg.Configure()

	// Then
	require.NoError(t, err)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "log-driver")
	assert.Contains(t, string(content), "cdi-devices")
}

func TestDockerConfigurator_DryRun(t *testing.T) {
	// Given: A Docker daemon.json file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "daemon.json")
	require.NoError(t, os.WriteFile(configPath, []byte("{}"), 0644))

	cfg, err := NewConfigurator(RuntimeDocker, configPath, nil)
	require.NoError(t, err)

	// When
	diff, err := cfg.DryRun()

	// Then
	require.NoError(t, err)
	assert.NotEmpty(t, diff)
	content, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.Equal(t, "{}", string(content))
}

func TestConfigurator_BackupCreated(t *testing.T) {
	// Given: An existing config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	originalContent := `version = 2
[plugins]
`
	require.NoError(t, os.WriteFile(configPath, []byte(originalContent), 0644))

	cfg, err := NewConfigurator(RuntimeContainerd, configPath, nil)
	require.NoError(t, err)

	// When
	err = cfg.Configure()

	// Then
	require.NoError(t, err)
	backupPath := configPath + ".backup"
	content, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	assert.Equal(t, originalContent, string(content))
}

func TestDetectRuntime_Containerd(t *testing.T) {
	// Given: A system with containerd socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "containerd.sock")
	f, err := os.Create(socketPath)
	require.NoError(t, err)
	f.Close()

	opts := &DetectOptions{
		ContainerdSocket: socketPath,
	}

	// When
	rt, err := DetectRuntimeWithOptions(opts)

	// Then
	require.NoError(t, err)
	assert.Equal(t, RuntimeContainerd, rt)
}

func TestDetectRuntime_CRIO(t *testing.T) {
	// Given: A system with CRI-O socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "crio.sock")
	f, err := os.Create(socketPath)
	require.NoError(t, err)
	f.Close()

	opts := &DetectOptions{
		CRIOSocket: socketPath,
	}

	// When
	rt, err := DetectRuntimeWithOptions(opts)

	// Then
	require.NoError(t, err)
	assert.Equal(t, RuntimeCRIO, rt)
}

func TestDetectRuntime_Docker(t *testing.T) {
	// Given: A system with Docker socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "docker.sock")
	f, err := os.Create(socketPath)
	require.NoError(t, err)
	f.Close()

	opts := &DetectOptions{
		DockerSocket: socketPath,
	}

	// When
	rt, err := DetectRuntimeWithOptions(opts)

	// Then
	require.NoError(t, err)
	assert.Equal(t, RuntimeDocker, rt)
}

func TestDetectRuntime_NotFound(t *testing.T) {
	// Given: A system without any runtime sockets
	opts := &DetectOptions{
		ContainerdSocket: "/nonexistent/containerd.sock",
		CRIOSocket:       "/nonexistent/crio.sock",
		DockerSocket:     "/nonexistent/docker.sock",
	}

	// When
	rt, err := DetectRuntimeWithOptions(opts)

	// Then
	assert.Error(t, err)
	assert.Equal(t, RuntimeType(""), rt)
}

func TestDetectRuntime_Priority(t *testing.T) {
	// Given: A system with multiple runtimes
	tmpDir := t.TempDir()
	containerdSocket := filepath.Join(tmpDir, "containerd.sock")
	dockerSocket := filepath.Join(tmpDir, "docker.sock")

	f1, _ := os.Create(containerdSocket)
	f1.Close()
	f2, _ := os.Create(dockerSocket)
	f2.Close()

	opts := &DetectOptions{
		ContainerdSocket: containerdSocket,
		DockerSocket:     dockerSocket,
	}

	// When
	rt, err := DetectRuntimeWithOptions(opts)

	// Then
	require.NoError(t, err)
	assert.Equal(t, RuntimeContainerd, rt)
}

func TestDefaultConfigPath(t *testing.T) {
	tests := []struct {
		rt       RuntimeType
		expected string
	}{
		{RuntimeContainerd, "/etc/containerd/config.toml"},
		{RuntimeCRIO, "/etc/crio/crio.conf.d/99-rbln.conf"},
		{RuntimeDocker, "/etc/docker/daemon.json"},
	}

	for _, tt := range tests {
		t.Run(string(tt.rt), func(t *testing.T) {
			path := DefaultConfigPath(tt.rt)
			assert.Equal(t, tt.expected, path)
		})
	}
}

func TestContainerdConfig_EnableCDI(t *testing.T) {
	// Given: A containerd config without CDI enabled
	config := `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
`

	// When
	result := enableCDIInContainerdConfig(config)

	// Then
	assert.Contains(t, result, "enable_cdi = true")
	assert.Contains(t, result, "version = 2")
	assert.Contains(t, result, "default_runtime_name")
}

func TestContainerdConfig_AlreadyEnabled(t *testing.T) {
	// Given: A containerd config with CDI already enabled
	config := `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
`

	// When
	result := enableCDIInContainerdConfig(config)

	// Then
	count := strings.Count(result, "enable_cdi = true")
	assert.Equal(t, 1, count)
}

func TestNewReverter(t *testing.T) {
	tests := []struct {
		name    string
		rt      RuntimeType
		wantErr bool
	}{
		{"containerd", RuntimeContainerd, false},
		{"crio", RuntimeCRIO, false},
		{"docker", RuntimeDocker, false},
		{"invalid", "invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reverter, err := NewReverter(tt.rt, "")
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, reverter)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reverter)
			}
		})
	}
}

func TestContainerdReverter_WithBackup(t *testing.T) {
	// Given: A config file with a backup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")
	backupPath := configPath + ".backup"

	originalContent := "version = 2\n"
	modifiedContent := "version = 2\nenable_cdi = true\n"

	require.NoError(t, os.WriteFile(backupPath, []byte(originalContent), 0644))
	require.NoError(t, os.WriteFile(configPath, []byte(modifiedContent), 0644))

	reverter, err := NewReverter(RuntimeContainerd, configPath)
	require.NoError(t, err)

	// When
	err = reverter.Revert()

	// Then
	assert.NoError(t, err)
	content, _ := os.ReadFile(configPath)
	assert.Equal(t, originalContent, string(content))
	_, err = os.Stat(backupPath)
	assert.True(t, os.IsNotExist(err))
}

func TestContainerdReverter_WithoutBackup(t *testing.T) {
	// Given: A config file without backup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	content := `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi"]
`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	reverter, err := NewReverter(RuntimeContainerd, configPath)
	require.NoError(t, err)

	// When
	err = reverter.Revert()

	// Then
	assert.NoError(t, err)
	result, _ := os.ReadFile(configPath)
	assert.NotContains(t, string(result), "enable_cdi")
	assert.NotContains(t, string(result), "cdi_spec_dirs")
	assert.Contains(t, string(result), "version = 2")
}

func TestCrioReverter(t *testing.T) {
	// Given: A CRI-O drop-in config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "99-rbln.conf")
	require.NoError(t, os.WriteFile(configPath, []byte("enable_cdi = true"), 0644))

	reverter, err := NewReverter(RuntimeCRIO, configPath)
	require.NoError(t, err)

	// When
	err = reverter.Revert()

	// Then
	assert.NoError(t, err)
	_, err = os.Stat(configPath)
	assert.True(t, os.IsNotExist(err))
}

func TestDockerReverter_WithBackup(t *testing.T) {
	// Given: A Docker config with backup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "daemon.json")
	backupPath := configPath + ".backup"

	originalContent := `{}`
	modifiedContent := `{"features":{"cdi-devices":true}}`

	require.NoError(t, os.WriteFile(backupPath, []byte(originalContent), 0644))
	require.NoError(t, os.WriteFile(configPath, []byte(modifiedContent), 0644))

	reverter, err := NewReverter(RuntimeDocker, configPath)
	require.NoError(t, err)

	// When
	err = reverter.Revert()

	// Then
	assert.NoError(t, err)
	content, _ := os.ReadFile(configPath)
	assert.Equal(t, originalContent, string(content))
}

func TestDockerReverter_WithoutBackup(t *testing.T) {
	// Given: A Docker config without backup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "daemon.json")

	content := `{
  "features": {
    "cdi-devices": true,
    "other-feature": true
  },
  "storage-driver": "overlay2"
}`
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	reverter, err := NewReverter(RuntimeDocker, configPath)
	require.NoError(t, err)

	// When
	err = reverter.Revert()

	// Then
	assert.NoError(t, err)
	result, _ := os.ReadFile(configPath)
	assert.NotContains(t, string(result), "cdi-devices")
	assert.Contains(t, string(result), "other-feature")
	assert.Contains(t, string(result), "storage-driver")
}
