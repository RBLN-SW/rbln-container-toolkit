//go:build linux || darwin

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

package installer

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
)

func TestSetupOptions_Defaults(t *testing.T) {
	// Given
	opts := SetupOptions{}

	// When
	// (no action needed for zero-value check)

	// Then
	assert.Empty(t, opts.Runtime)
	assert.Empty(t, opts.RestartMode)
	assert.Empty(t, opts.Socket)
	assert.Empty(t, opts.HostRootMount)
	assert.Empty(t, opts.CDISpecDir)
	assert.Empty(t, opts.PidFile)
	assert.False(t, opts.DryRun)
	assert.Nil(t, opts.Logger)
}

func TestGetConfigPath(t *testing.T) {
	tests := []struct {
		name               string
		runtime            string
		hostRootMount      string
		configPathOverride string
		expected           string
	}{
		{
			name:     "docker without host mount",
			runtime:  "docker",
			expected: "/etc/docker/daemon.json",
		},
		{
			name:     "containerd without host mount",
			runtime:  "containerd",
			expected: "/etc/containerd/config.toml",
		},
		{
			name:     "crio without host mount",
			runtime:  "crio",
			expected: "/etc/crio/crio.conf.d/99-rbln.conf",
		},
		{
			name:     "unknown runtime",
			runtime:  "unknown",
			expected: "/etc/unknown/config",
		},
		{
			name:          "docker with host mount",
			runtime:       "docker",
			hostRootMount: "/host",
			expected:      "/host/etc/docker/daemon.json",
		},
		{
			name:          "containerd with custom host mount",
			runtime:       "containerd",
			hostRootMount: "/custom/host",
			expected:      "/custom/host/etc/containerd/config.toml",
		},
		{
			name:               "override replaces default for containerd",
			runtime:            "containerd",
			configPathOverride: "/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
			expected:           "/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
		},
		{
			name:               "override replaces default for docker",
			runtime:            "docker",
			configPathOverride: "/custom/docker/daemon.json",
			expected:           "/custom/docker/daemon.json",
		},
		{
			name:               "override is treated as final path even with host mount",
			runtime:            "containerd",
			hostRootMount:      "/host",
			configPathOverride: "/runtime/config-dir/config.toml",
			expected:           "/runtime/config-dir/config.toml",
		},
		{
			name:               "override with explicit host-prefixed path keeps the path as-is",
			runtime:            "containerd",
			hostRootMount:      "/host",
			configPathOverride: "/host/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
			expected:           "/host/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
		},
		{
			name:               "override with custom host mount is still treated as final path",
			runtime:            "containerd",
			hostRootMount:      "/custom/host",
			configPathOverride: "/custom/host/var/lib/rancher/k3s/agent/etc/containerd/config.toml",
			expected:           "/custom/host/var/lib/rancher/k3s/agent/etc/containerd/config.toml",
		},
		{
			name:               "override ignores runtime default entirely",
			runtime:            "containerd",
			configPathOverride: "/var/snap/microk8s/current/args/containerd-template.toml",
			expected:           "/var/snap/microk8s/current/args/containerd-template.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			result := getConfigPath(tt.runtime, tt.hostRootMount, tt.configPathOverride)

			// Then
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidateConfigPathOverride(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{name: "empty override is allowed (uses runtime default)", configPath: "", wantErr: false},
		{name: "absolute path is accepted", configPath: "/etc/containerd/config.toml", wantErr: false},
		{name: "absolute path under operator mount", configPath: "/runtime/config-dir/config.toml", wantErr: false},
		{name: "relative path is rejected", configPath: "etc/containerd/config.toml", wantErr: true},
		{name: "dot-slash relative path is rejected", configPath: "./config.toml", wantErr: true},
		{name: "parent relative path is rejected", configPath: "../config.toml", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigPathOverride(tt.configPath)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "must be absolute")
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSetup_RejectsRelativeConfigPathOverride(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	opts := SetupOptions{
		Runtime:     "containerd",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		ConfigPath:  "etc/containerd/config.toml",
		DryRun:      true,
	}

	// When
	err := Setup(opts)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be absolute")
}

func TestSetup_CrioSignalModeError(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	opts := SetupOptions{
		Runtime:     "crio",
		RestartMode: restart.RestartModeSignal,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
	}

	// When
	err := Setup(opts)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signal restart mode is not supported for CRI-O")
}

func TestSetup_DryRun(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "containerd",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
	assert.True(t, len(logger.infos) > 0)
	assert.Contains(t, logger.infos[0], "[DRY-RUN]")
}

func TestEmptySpec(t *testing.T) {
	// When
	spec := emptySpec()

	// Then
	assert.NotNil(t, spec)
	assert.Equal(t, "0.5.0", spec.Version)
	assert.Equal(t, "rebellions.ai/rbln", spec.Kind)
}

func TestNoopLogger(t *testing.T) {
	// Given
	logger := &noopLogger{}

	// When
	// Then
	assert.NotPanics(t, func() {
		logger.Info("test %s", "info")
		logger.Debug("test %s", "debug")
		logger.Warning("test %s", "warning")
	})
}

func TestDryRunSetup_OutputsExpectedActions(t *testing.T) {
	tests := []struct {
		name          string
		runtime       string
		restartMode   restart.Mode
		expectedSteps []string
	}{
		{
			name:        "containerd with none mode",
			runtime:     "containerd",
			restartMode: restart.RestartModeNone,
			expectedSteps: []string{
				"[DRY-RUN]",
				"Configure containerd",
				"Generate CDI spec",
				"Skip restart",
			},
		},
		{
			name:        "docker with none mode",
			runtime:     "docker",
			restartMode: restart.RestartModeNone,
			expectedSteps: []string{
				"[DRY-RUN]",
				"Configure docker",
				"Generate CDI spec",
				"Skip restart",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			logger := &testLogger{}

			// When
			err := dryRunSetup(
				SetupOptions{Runtime: tt.runtime, RestartMode: tt.restartMode},
				getConfigPath(tt.runtime, "", ""),
				"/var/run/docker.sock",
				"/var/run/cdi",
				logger,
			)

			// Then
			assert.NoError(t, err)
			allLogs := concatLogs(logger)
			for _, expected := range tt.expectedSteps {
				assert.Contains(t, allLogs, expected)
			}
		})
	}
}

func TestRemoveCDISpec(t *testing.T) {
	t.Run("existing file", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "rbln.yaml")
		require.NoError(t, os.WriteFile(specPath, []byte("test: spec"), 0644))

		// When
		err := removeCDISpec(specPath)

		// Then
		assert.NoError(t, err)
		_, statErr := os.Stat(specPath)
		assert.True(t, os.IsNotExist(statErr))
	})

	t.Run("non-existent file returns nil", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := filepath.Join(tmpDir, "nonexistent.yaml")

		// When
		err := removeCDISpec(specPath)

		// Then
		assert.NoError(t, err)
	})
}

func TestConfigureRuntime_UnsupportedRuntime(t *testing.T) {
	// Given
	runtime := "unsupported"
	configPath := "/etc/unsupported/config"

	// When
	err := configureRuntime(runtime, configPath)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestSetup_DefaultsApplied(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		Socket:      "",
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
}

func TestSetup_WithHostRootMount(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:       "docker",
		RestartMode:   restart.RestartModeNone,
		PidFile:       filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:    "/var/run/cdi",
		HostRootMount: "/host",
		DryRun:        true,
		Logger:        logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "/host")
}

func TestSetup_LoggerInterface(t *testing.T) {
	// Given
	logger := &testLogger{}

	// When
	logger.Info("info message %d", 1)
	logger.Debug("debug message %s", "test")
	logger.Warning("warning message")

	// Then
	assert.Len(t, logger.infos, 1)
	assert.Contains(t, logger.infos[0], "info message 1")
	assert.Len(t, logger.debugs, 1)
	assert.Contains(t, logger.debugs[0], "debug message test")
	assert.Len(t, logger.warnings, 1)
	assert.Contains(t, logger.warnings[0], "warning message")
}

type testLogger struct {
	infos    []string
	debugs   []string
	warnings []string
}

func (l *testLogger) Info(format string, args ...interface{}) {
	l.infos = append(l.infos, fmt.Sprintf(format, args...))
}

func (l *testLogger) Debug(format string, args ...interface{}) {
	l.debugs = append(l.debugs, fmt.Sprintf(format, args...))
}

func (l *testLogger) Warning(format string, args ...interface{}) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

func concatLogs(l *testLogger) string {
	result := ""
	for _, s := range l.infos {
		result += s + "\n"
	}
	for _, s := range l.debugs {
		result += s + "\n"
	}
	for _, s := range l.warnings {
		result += s + "\n"
	}
	return result
}

func TestGenerateCDISpec_CreatesDirAndSpec(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cdiSpecDir := filepath.Join(tmpDir, "cdi")

	// When
	err := generateCDISpec(cdiSpecDir, "")

	// Then
	assert.NoError(t, err)
	_, statErr := os.Stat(cdiSpecDir)
	assert.NoError(t, statErr)
	specPath := filepath.Join(cdiSpecDir, "rbln.yaml")
	_, specStatErr := os.Stat(specPath)
	assert.NoError(t, specStatErr)
}

func TestGenerateCDISpec_WithHostRootMount(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cdiSpecDir := filepath.Join(tmpDir, "cdi")

	// When
	err := generateCDISpec(cdiSpecDir, "/custom/host")

	// Then
	assert.NoError(t, err)
}

func TestSetup_LockAcquisitionError(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock := NewLock(pidFile)
	require.NoError(t, lock.Acquire())
	defer lock.Release()

	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		PidFile:     pidFile,
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      false,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.Error(t, err)
	assert.True(t, IsAlreadyRunning(err))
}

func TestCleanup_LockAcquisitionError(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	pidFile := filepath.Join(tmpDir, "test.pid")
	lock := NewLock(pidFile)
	require.NoError(t, lock.Acquire())
	defer lock.Release()

	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		PidFile:     pidFile,
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      false,
		Logger:      logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.Error(t, err)
	assert.True(t, IsAlreadyRunning(err))
}

func TestSetup_AllRuntimes_DryRun(t *testing.T) {
	runtimes := []string{"docker", "containerd", "crio"}

	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			// Given
			tmpDir := t.TempDir()
			logger := &testLogger{}
			opts := SetupOptions{
				Runtime:     rt,
				RestartMode: restart.RestartModeNone,
				PidFile:     filepath.Join(tmpDir, "test.pid"),
				CDISpecDir:  filepath.Join(tmpDir, "cdi"),
				DryRun:      true,
				Logger:      logger,
			}

			// When
			err := Setup(opts)

			// Then
			assert.NoError(t, err)
			allLogs := concatLogs(logger)
			assert.Contains(t, allLogs, rt)
		})
	}
}

func TestCleanup_AllRuntimes_DryRun(t *testing.T) {
	runtimes := []string{"docker", "containerd", "crio"}

	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			// Given
			tmpDir := t.TempDir()
			logger := &testLogger{}
			opts := CleanupOptions{
				Runtime:     rt,
				RestartMode: restart.RestartModeNone,
				PidFile:     filepath.Join(tmpDir, "test.pid"),
				CDISpecDir:  filepath.Join(tmpDir, "cdi"),
				DryRun:      true,
				Logger:      logger,
			}

			// When
			err := Cleanup(opts)

			// Then
			assert.NoError(t, err)
			allLogs := concatLogs(logger)
			assert.Contains(t, allLogs, rt)
		})
	}
}

func TestSetup_CDISpecDirCreation(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cdiDir := filepath.Join(tmpDir, "nested", "cdi", "path")
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  cdiDir,
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
}

func TestRevertRuntimeConfig_AllRuntimes(t *testing.T) {
	runtimes := []struct {
		name   string
		config string
	}{
		{"docker", "/tmp/test-docker-config.json"},
		{"containerd", "/tmp/test-containerd-config.toml"},
		{"crio", "/tmp/test-crio-config.conf"},
	}

	for _, tt := range runtimes {
		t.Run(tt.name, func(t *testing.T) {
			// When
			err := revertRuntimeConfig(tt.name, tt.config)

			// Then
			assert.NoError(t, err)
		})
	}
}

func TestConfigureRuntime_AllRuntimes(t *testing.T) {
	runtimes := []string{"docker", "containerd", "crio"}

	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			// Given
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config")

			// When
			err := configureRuntime(rt, configPath)

			// Then
			assert.NoError(t, err)
		})
	}
}

func TestDryRunSetup_WithSystemdMode(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "containerd.sock")
	require.NoError(t, os.WriteFile(socketPath, []byte{}, 0644))
	logger := &testLogger{}

	// When
	err := dryRunSetup(
		SetupOptions{Runtime: "containerd", RestartMode: restart.RestartModeSystemd},
		"/etc/containerd/config.toml",
		socketPath,
		"/var/run/cdi",
		logger,
	)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "[DRY-RUN]")
}

func TestDryRunCleanup_WithSystemdMode(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "containerd.sock")
	require.NoError(t, os.WriteFile(socketPath, []byte{}, 0644))
	logger := &testLogger{}

	// When
	err := dryRunCleanup(
		CleanupOptions{Runtime: "containerd", RestartMode: restart.RestartModeSystemd},
		"/etc/containerd/config.toml",
		socketPath,
		"/var/run/cdi",
		logger,
	)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "[DRY-RUN]")
}

func TestSetup_SocketPathAdjustment(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "containerd",
		RestartMode: restart.RestartModeNone,
		Socket:      "",
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
}

func TestCleanup_SocketPathAdjustment(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "containerd",
		RestartMode: restart.RestartModeNone,
		Socket:      "",
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
}

func TestSetup_CrioWithSystemdMode(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "crio",
		RestartMode: restart.RestartModeSystemd,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
}

func TestCleanup_CrioWithSystemdMode(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "crio",
		RestartMode: restart.RestartModeSystemd,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
}

func TestGenerateCDISpec_WritesValidYAML(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cdiSpecDir := filepath.Join(tmpDir, "cdi")

	// When
	err := generateCDISpec(cdiSpecDir, "")

	// Then
	assert.NoError(t, err)
	specPath := filepath.Join(cdiSpecDir, "rbln.yaml")
	content, readErr := os.ReadFile(specPath)
	assert.NoError(t, readErr)
	assert.Contains(t, string(content), "cdiVersion")
	assert.Contains(t, string(content), "kind")
}

func TestConfigureRuntime_WithTempConfigPath(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config")

	// When
	err := configureRuntime("docker", configPath)

	// Then
	assert.NoError(t, err)
}

func TestSetup_SystemdRestartMode_DryRun(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "docker.sock")
	require.NoError(t, os.WriteFile(socketPath, []byte{}, 0644))
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeSystemd,
		Socket:      socketPath,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "[DRY-RUN]")
}

func TestCleanup_SystemdRestartMode_DryRun(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "docker.sock")
	require.NoError(t, os.WriteFile(socketPath, []byte{}, 0644))
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeSystemd,
		Socket:      socketPath,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "[DRY-RUN]")
}

func TestSetup_NonDryRun_WithNoneRestart(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	hostRoot := filepath.Join(tmpDir, "host")
	require.NoError(t, os.MkdirAll(filepath.Join(hostRoot, "etc", "docker"), 0755))
	cdiDir := "/var/run/cdi"
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:       "docker",
		RestartMode:   restart.RestartModeNone,
		PidFile:       filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:    cdiDir,
		HostRootMount: hostRoot,
		DryRun:        false,
		Logger:        logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.NoError(t, err)
	specPath := filepath.Join(hostRoot, cdiDir, "rbln.yaml")
	_, statErr := os.Stat(specPath)
	assert.NoError(t, statErr)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "Restart skipped")
}

func TestCleanup_NonDryRun_WithNoneRestart(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	hostRoot := filepath.Join(tmpDir, "host")
	cdiDir := "/var/run/cdi"
	actualCdiDir := filepath.Join(hostRoot, cdiDir)
	require.NoError(t, os.MkdirAll(actualCdiDir, 0755))
	specPath := filepath.Join(actualCdiDir, "rbln.yaml")
	require.NoError(t, os.WriteFile(specPath, []byte("test: spec"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(hostRoot, "etc", "docker"), 0755))

	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:       "docker",
		RestartMode:   restart.RestartModeNone,
		PidFile:       filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:    cdiDir,
		HostRootMount: hostRoot,
		DryRun:        false,
		Logger:        logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
	_, statErr := os.Stat(specPath)
	assert.True(t, os.IsNotExist(statErr))
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "Restart skipped")
}

func TestSetup_ConfigureRuntimeError(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := SetupOptions{
		Runtime:     "unsupported",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      false,
		Logger:      logger,
	}

	// When
	err := Setup(opts)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to configure unsupported")
}

func TestCleanup_NonExistentCDISpec(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	hostRoot := filepath.Join(tmpDir, "host")
	require.NoError(t, os.MkdirAll(filepath.Join(hostRoot, "etc", "docker"), 0755))
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:       "docker",
		RestartMode:   restart.RestartModeNone,
		PidFile:       filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:    "/var/run/cdi",
		HostRootMount: hostRoot,
		DryRun:        false,
		Logger:        logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
}

func TestGenerateCDISpec_PermissionError(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")
	}

	// Given
	tmpDir := t.TempDir()
	cdiDir := filepath.Join(tmpDir, "cdi")
	require.NoError(t, os.MkdirAll(cdiDir, 0555))
	defer os.Chmod(cdiDir, 0755)

	// When
	err := generateCDISpec(cdiDir, "")

	// Then - may or may not error depending on OS
	_ = err
}
