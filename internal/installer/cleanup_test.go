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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
)

func TestCleanupOptions_Defaults(t *testing.T) {
	// Given
	opts := CleanupOptions{}

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

func TestCleanup_CrioSignalModeError(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	opts := CleanupOptions{
		Runtime:     "crio",
		RestartMode: restart.RestartModeSignal,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signal restart mode is not supported for CRI-O")
}

func TestCleanup_DryRun(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "containerd",
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
	assert.True(t, len(logger.infos) > 0)
	assert.Contains(t, logger.infos[0], "[DRY-RUN]")
}

func TestDryRunCleanup_OutputsExpectedActions(t *testing.T) {
	tests := []struct {
		name          string
		runtime       string
		restartMode   restart.Mode
		expectedSteps []string
	}{
		{
			name:        "containerd_with_none_mode",
			runtime:     "containerd",
			restartMode: restart.RestartModeNone,
			expectedSteps: []string{
				"[DRY-RUN]",
				"Remove CDI spec",
				"Revert containerd configuration",
				"Skip restart",
			},
		},
		{
			name:        "docker_with_none_mode",
			runtime:     "docker",
			restartMode: restart.RestartModeNone,
			expectedSteps: []string{
				"[DRY-RUN]",
				"Remove CDI spec",
				"Revert docker configuration",
				"Skip restart",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			logger := &testLogger{}

			// When
			err := dryRunCleanup(
				CleanupOptions{Runtime: tt.runtime, RestartMode: tt.restartMode},
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

func TestCleanup_WithHostRootMount(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:       "docker",
		RestartMode:   restart.RestartModeNone,
		PidFile:       filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:    "/var/run/cdi",
		HostRootMount: "/host",
		DryRun:        true,
		Logger:        logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "/host")
}

func TestRevertRuntimeConfig_UnsupportedRuntime(t *testing.T) {
	// Given
	runtime := "unsupported"
	configPath := "/etc/unsupported/config"

	// When
	err := revertRuntimeConfig(runtime, configPath)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestRemoveCDISpec_Idempotent(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "nonexistent.yaml")

	// When
	err1 := removeCDISpec(specPath)
	err2 := removeCDISpec(specPath)

	// Then
	assert.NoError(t, err1)
	assert.NoError(t, err2)
}

func TestCleanup_DefaultsApplied(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "docker",
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

func TestCleanup_NilLogger(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	opts := CleanupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      nil,
	}

	// When
	// Then
	assert.NotPanics(t, func() {
		_ = Cleanup(opts)
	})
}

func TestSetup_NilLogger(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	opts := SetupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  filepath.Join(tmpDir, "cdi"),
		DryRun:      true,
		Logger:      nil,
	}

	// When
	// Then
	assert.NotPanics(t, func() {
		_ = Setup(opts)
	})
}

func TestCleanup_RemovesCDISpec(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	cdiDir := filepath.Join(tmpDir, "cdi")
	specPath := filepath.Join(cdiDir, "rbln.yaml")
	os.MkdirAll(cdiDir, 0755)
	os.WriteFile(specPath, []byte("test: spec"), 0644)
	logger := &testLogger{}
	opts := CleanupOptions{
		Runtime:     "docker",
		RestartMode: restart.RestartModeNone,
		PidFile:     filepath.Join(tmpDir, "test.pid"),
		CDISpecDir:  cdiDir,
		DryRun:      true,
		Logger:      logger,
	}

	// When
	err := Cleanup(opts)

	// Then
	assert.NoError(t, err)
	allLogs := concatLogs(logger)
	assert.Contains(t, allLogs, "Remove CDI spec")
}
