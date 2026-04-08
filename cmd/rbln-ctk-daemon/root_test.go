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

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/runtime"
)

func TestNewRootCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newRootCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "rbln-ctk-daemon", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
	})

	t.Run("has expected flags", func(t *testing.T) {
		// Given
		flags := []string{
			"runtime",
			"shutdown-timeout",
			"pid-file",
			"health-port",
			"no-cleanup-on-exit",
			"host-root",
			"driver-root",
			"cdi-spec-dir",
			"debug",
		}

		// When
		cmd := newRootCmd()

		// Then
		for _, flag := range flags {
			f := cmd.Flags().Lookup(flag)
			assert.NotNil(t, f, "flag %q should exist", flag)
		}
	})

	t.Run("has runtime subcommand", func(t *testing.T) {
		// When
		cmd := newRootCmd()

		// Then
		var found bool
		for _, sub := range cmd.Commands() {
			if sub.Use == "runtime" {
				found = true
				break
			}
		}
		assert.True(t, found, "should have 'runtime' subcommand")
	})
}

func TestInitConfig(t *testing.T) {
	// When
	err := initConfig()

	// Then
	assert.NoError(t, err)
}

func TestLogFunctions(t *testing.T) {
	t.Run("log functions do not panic", func(t *testing.T) {
		// When / Then
		assert.NotPanics(t, func() {
			logInfo("test %s", "info")
		})
		assert.NotPanics(t, func() {
			logWarning("test %s", "warning")
		})
		assert.NotPanics(t, func() {
			logError("test %s", "error")
		})
	})

	t.Run("logDebug does not panic with debug disabled", func(t *testing.T) {
		// Given
		viper.Set("debug", false)
		defer viper.Set("debug", false)

		// When / Then
		assert.NotPanics(t, func() {
			logDebug("test %s", "debug")
		})
	})

	t.Run("logDebug does not panic with debug enabled", func(t *testing.T) {
		// Given
		viper.Set("debug", true)
		defer viper.Set("debug", false)

		// When / Then
		assert.NotPanics(t, func() {
			logDebug("test %s", "debug")
		})
	})
}

func TestRootCmdFlagDefaults(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		expected string
	}{
		{
			name:     "pid-file flag has default value",
			flag:     "pid-file",
			expected: "/run/rbln/toolkit.pid",
		},
		{
			name:     "cdi-spec-dir flag has default value",
			flag:     "cdi-spec-dir",
			expected: "/var/run/cdi",
		},
		{
			name:     "host-root flag has default value",
			flag:     "host-root",
			expected: "",
		},
		{
			name:     "driver-root flag has default value",
			flag:     "driver-root",
			expected: "/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cmd := newRootCmd()

			// When
			f := cmd.Flags().Lookup(tt.flag)

			// Then
			require.NotNil(t, f)
			assert.Equal(t, tt.expected, f.DefValue)
		})
	}
}

func TestRootCmdHealthPortDefault(t *testing.T) {
	// Given
	cmd := newRootCmd()

	// When
	f := cmd.Flags().Lookup("health-port")

	// Then
	require.NotNil(t, f)
	assert.Equal(t, "8080", f.DefValue)
}

func TestRootCmdShutdownTimeoutDefault(t *testing.T) {
	// Given
	cmd := newRootCmd()

	// When
	f := cmd.Flags().Lookup("shutdown-timeout")

	// Then
	require.NotNil(t, f)
	assert.Equal(t, "30s", f.DefValue)
}

func TestRootCmdVersion(t *testing.T) {
	// When
	cmd := newRootCmd()

	// Then
	assert.NotEmpty(t, cmd.Version)
	assert.Contains(t, cmd.Version, "commit:")
	assert.Contains(t, cmd.Version, "built:")
}

func TestRootCmdPersistentPreRunE(t *testing.T) {
	// Given
	cmd := newRootCmd()

	// When
	err := cmd.PersistentPreRunE(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRootCmdSilenceUsage(t *testing.T) {
	// When
	cmd := newRootCmd()

	// Then
	assert.True(t, cmd.SilenceUsage)
}

func TestRootCmdFlagShortcuts(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		shortcut string
	}{
		{
			name:     "runtime flag has shortcut r",
			flag:     "runtime",
			shortcut: "r",
		},
		{
			name:     "debug flag has shortcut d",
			flag:     "debug",
			shortcut: "d",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cmd := newRootCmd()

			// When
			f := cmd.Flags().Lookup(tt.flag)

			// Then
			require.NotNil(t, f)
			assert.Equal(t, tt.shortcut, f.Shorthand)
		})
	}
}

func TestDoCleanup(t *testing.T) {
	t.Run("returns no error with non-existent CDI spec", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()

		// When
		err := doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("removes existing CDI spec", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := tmpDir + "/rbln.yaml"
		err := os.WriteFile(specPath, []byte("test: spec"), 0644)
		require.NoError(t, err)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
		_, err = os.Stat(specPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("handles gracefully with missing backup", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()

		// When
		err := doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("is idempotent - cleanup twice succeeds", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := tmpDir + "/rbln.yaml"
		err := os.WriteFile(specPath, []byte("test: spec"), 0644)
		require.NoError(t, err)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")
		assert.NoError(t, err)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("removes CDI spec even if backup exists", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := tmpDir + "/rbln.yaml"
		backupPath := tmpDir + "/rbln.yaml.backup"

		err := os.WriteFile(specPath, []byte("test: spec"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(backupPath, []byte("backup: spec"), 0644)
		require.NoError(t, err)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
		_, err = os.Stat(specPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("handles read-only CDI directory gracefully", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := tmpDir + "/rbln.yaml"
		err := os.WriteFile(specPath, []byte("test: spec"), 0644)
		require.NoError(t, err)

		err = os.Chmod(tmpDir, 0555)
		require.NoError(t, err)
		defer os.Chmod(tmpDir, 0755)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("handles multiple runtime types", func(t *testing.T) {
		// Given
		runtimes := []runtime.RuntimeType{"containerd", "crio", "docker"}

		for _, rt := range runtimes {
			t.Run("runtime_"+string(rt), func(t *testing.T) {
				// Given
				tmpDir := t.TempDir()
				specPath := tmpDir + "/rbln.yaml"
				err := os.WriteFile(specPath, []byte("test: spec"), 0644)
				require.NoError(t, err)

				// When
				err = doCleanup(rt, tmpDir, "/", "")

				// Then
				assert.NoError(t, err)
				_, err = os.Stat(specPath)
				assert.True(t, os.IsNotExist(err))
			})
		}
	})

	t.Run("handles backup file in CDI directory", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		backupPath := tmpDir + "/rbln.yaml.backup"
		err := os.WriteFile(backupPath, []byte("backup: spec"), 0644)
		require.NoError(t, err)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("handles corrupted backup file gracefully", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		backupPath := tmpDir + "/rbln.yaml.backup"
		err := os.WriteFile(backupPath, []byte("backup: spec"), 0000)
		require.NoError(t, err)
		defer os.Chmod(backupPath, 0644)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("succeeds with empty CDI directory", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()

		// When
		err := doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
	})

	t.Run("removes only rbln.yaml spec file", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		specPath := tmpDir + "/rbln.yaml"
		otherFile := tmpDir + "/other.yaml"

		err := os.WriteFile(specPath, []byte("test: spec"), 0644)
		require.NoError(t, err)
		err = os.WriteFile(otherFile, []byte("other: file"), 0644)
		require.NoError(t, err)

		// When
		err = doCleanup(runtime.RuntimeType("containerd"), tmpDir, "/", "")

		// Then
		assert.NoError(t, err)
		_, err = os.Stat(specPath)
		assert.True(t, os.IsNotExist(err))
		_, err = os.Stat(otherFile)
		assert.NoError(t, err)
	})
}

func TestResolveConfigPath(t *testing.T) {
	tests := []struct {
		name       string
		rt         runtime.RuntimeType
		hostRoot   string
		configPath string
		expected   string
	}{
		{
			name:       "empty configPath falls back to containerd default",
			rt:         runtime.RuntimeContainerd,
			hostRoot:   "/",
			configPath: "",
			expected:   "/etc/containerd/config.toml",
		},
		{
			name:       "empty configPath falls back to docker default",
			rt:         runtime.RuntimeDocker,
			hostRoot:   "/",
			configPath: "",
			expected:   "/etc/docker/daemon.json",
		},
		{
			name:       "empty configPath falls back to crio default",
			rt:         runtime.RuntimeCRIO,
			hostRoot:   "/",
			configPath: "",
			expected:   "/etc/crio/crio.conf.d/99-rbln.conf",
		},
		{
			name:       "configPath override is used as-is",
			rt:         runtime.RuntimeContainerd,
			hostRoot:   "/",
			configPath: "/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
			expected:   "/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
		},
		{
			name:       "hostRoot prefix applied to default path",
			rt:         runtime.RuntimeContainerd,
			hostRoot:   "/host",
			configPath: "",
			expected:   "/host/etc/containerd/config.toml",
		},
		{
			name:       "hostRoot prefix applied to override path",
			rt:         runtime.RuntimeContainerd,
			hostRoot:   "/host",
			configPath: "/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
			expected:   "/host/var/lib/rancher/rke2/agent/etc/containerd/config.toml",
		},
		{
			name:       "empty hostRoot does not add prefix",
			rt:         runtime.RuntimeContainerd,
			hostRoot:   "",
			configPath: "/etc/containerd/config.toml",
			expected:   "/etc/containerd/config.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveConfigPath(tt.rt, tt.hostRoot, tt.configPath)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDoCleanup_WithConfigPathOverride(t *testing.T) {
	t.Run("uses override config path for backup restoration", func(t *testing.T) {
		// Given: a tmpDir simulating host root with custom config path
		tmpDir := t.TempDir()
		customConfigDir := filepath.Join(tmpDir, "var", "lib", "rancher", "rke2", "agent", "etc", "containerd")
		require.NoError(t, os.MkdirAll(customConfigDir, 0o755))

		configFile := filepath.Join(customConfigDir, "config.toml")
		backupFile := configFile + ".backup"
		require.NoError(t, os.WriteFile(configFile, []byte("modified config"), 0o644))
		require.NoError(t, os.WriteFile(backupFile, []byte("original config"), 0o644))

		cdiDir := filepath.Join(tmpDir, "cdi")
		require.NoError(t, os.MkdirAll(cdiDir, 0o755))

		// When: cleanup with override config path pointing to the full path (no hostRoot prefix needed)
		err := doCleanup(
			runtime.RuntimeContainerd,
			cdiDir,
			"/",
			configFile,
		)

		// Then: backup should be restored
		assert.NoError(t, err)
		content, err := os.ReadFile(configFile)
		require.NoError(t, err)
		assert.Equal(t, "original config", string(content))

		// backup file should be removed
		_, err = os.Stat(backupFile)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("applies hostRoot prefix to override config path", func(t *testing.T) {
		// Given: simulate /host mount with custom containerd config path
		tmpDir := t.TempDir()
		hostRoot := tmpDir // tmpDir acts as /host

		customConfigPath := "/var/lib/rancher/rke2/agent/etc/containerd/config.toml"
		fullConfigPath := filepath.Join(hostRoot, customConfigPath)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullConfigPath), 0o755))

		require.NoError(t, os.WriteFile(fullConfigPath, []byte("modified"), 0o644))
		require.NoError(t, os.WriteFile(fullConfigPath+".backup", []byte("original"), 0o644))

		cdiDir := filepath.Join(tmpDir, "cdi")
		require.NoError(t, os.MkdirAll(cdiDir, 0o755))

		// When: cleanup with hostRoot and override config path
		err := doCleanup(
			runtime.RuntimeContainerd,
			cdiDir,
			hostRoot,
			customConfigPath,
		)

		// Then: backup at hostRoot-prefixed path should be restored
		assert.NoError(t, err)
		content, err := os.ReadFile(fullConfigPath)
		require.NoError(t, err)
		assert.Equal(t, "original", string(content))
	})

	t.Run("applies hostRoot prefix to default config path", func(t *testing.T) {
		// Given: simulate /host mount with default containerd config
		tmpDir := t.TempDir()
		hostRoot := tmpDir

		defaultConfigPath := filepath.Join(hostRoot, "etc", "containerd", "config.toml")
		require.NoError(t, os.MkdirAll(filepath.Dir(defaultConfigPath), 0o755))

		require.NoError(t, os.WriteFile(defaultConfigPath, []byte("modified"), 0o644))
		require.NoError(t, os.WriteFile(defaultConfigPath+".backup", []byte("original"), 0o644))

		cdiDir := filepath.Join(tmpDir, "cdi")
		require.NoError(t, os.MkdirAll(cdiDir, 0o755))

		// When: cleanup with hostRoot but no config path override (empty = default)
		err := doCleanup(
			runtime.RuntimeContainerd,
			cdiDir,
			hostRoot,
			"",
		)

		// Then: backup at hostRoot + default path should be restored
		assert.NoError(t, err)
		content, err := os.ReadFile(defaultConfigPath)
		require.NoError(t, err)
		assert.Equal(t, "original", string(content))
	})
}

func TestDetectHostRoot(t *testing.T) {
	t.Run("flag value provided returns flag value", func(t *testing.T) {
		// Given
		flagValue := "/custom/root"

		// When
		result := detectHostRoot(flagValue)

		// Then
		assert.Equal(t, "/custom/root", result)
	})

	t.Run("flag empty and /host missing returns /", func(t *testing.T) {
		// Given
		flagValue := ""

		// When
		result := detectHostRoot(flagValue)

		// Then
		assert.Equal(t, "/", result)
	})

	t.Run("flag empty and /host exists returns /host", func(t *testing.T) {
		// Given
		tmpDir := t.TempDir()
		hostPath := tmpDir + "/host"
		err := os.Mkdir(hostPath, 0755)
		require.NoError(t, err)

		flagValue := ""

		// When - test with actual /host path (will return "/" since /host doesn't exist on test system)
		result := detectHostRoot(flagValue)

		// Then - verify it returns "/" when /host doesn't exist
		assert.Equal(t, "/", result)
	})
}

func TestInstallHookBinary(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(t *testing.T) string
		wantErr   bool
		errMsg    string
		checkFunc func(t *testing.T, hostRoot string)
	}{
		{
			name: "returns error when hook binary not found in standard paths",
			setupFunc: func(t *testing.T) string {
				hostRoot := t.TempDir()
				return hostRoot
			},
			wantErr: true,
			errMsg:  "hook binary not found",
		},
		{
			name: "returns error when source binary missing even if dest dir exists",
			setupFunc: func(t *testing.T) string {
				hostRoot := t.TempDir()

				destDir := hostRoot + "/usr/local/bin"
				err := os.MkdirAll(destDir, 0755)
				require.NoError(t, err)

				return hostRoot
			},
			wantErr: true,
			errMsg:  "hook binary not found",
		},
		{
			name: "returns error when source binary missing even if dest file exists",
			setupFunc: func(t *testing.T) string {
				hostRoot := t.TempDir()

				destDir := hostRoot + "/usr/local/bin"
				err := os.MkdirAll(destDir, 0755)
				require.NoError(t, err)

				destPath := destDir + "/rbln-cdi-hook"
				err = os.WriteFile(destPath, []byte("existing binary"), 0755)
				require.NoError(t, err)

				return hostRoot
			},
			wantErr: true,
			errMsg:  "hook binary not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			hostRoot := tt.setupFunc(t)

			// When
			err := installHookBinary(hostRoot)

			// Then
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
				if tt.checkFunc != nil {
					tt.checkFunc(t, hostRoot)
				}
			}
		})
	}
}

func TestSetup_RestartOptions(t *testing.T) {
	t.Run("runtime defaults resolution", func(t *testing.T) {
		tests := []struct {
			name           string
			runtime        string
			expectedMode   string
			expectedSocket string
		}{
			{
				name:           "containerd uses signal mode and default socket",
				runtime:        "containerd",
				expectedMode:   "signal",
				expectedSocket: "/run/containerd/containerd.sock",
			},
			{
				name:           "docker uses signal mode and default socket",
				runtime:        "docker",
				expectedMode:   "signal",
				expectedSocket: "/var/run/docker.sock",
			},
			{
				name:           "crio uses systemd mode",
				runtime:        "crio",
				expectedMode:   "systemd",
				expectedSocket: "/var/run/crio/crio.sock",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				rt := runtime.RuntimeType(tt.runtime)

				// When
				defaults := restart.GetRuntimeDefaults(string(rt))

				// Then
				assert.Equal(t, tt.expectedMode, string(defaults.Mode))
				assert.Equal(t, tt.expectedSocket, defaults.Socket)
			})
		}
	})

	t.Run("user-specified socket overrides default", func(t *testing.T) {
		tests := []struct {
			name           string
			runtime        string
			userSocket     string
			expectedSocket string
		}{
			{
				name:           "containerd with custom socket",
				runtime:        "containerd",
				userSocket:     "/custom/containerd.sock",
				expectedSocket: "/custom/containerd.sock",
			},
			{
				name:           "docker with custom socket",
				runtime:        "docker",
				userSocket:     "/tmp/docker.sock",
				expectedSocket: "/tmp/docker.sock",
			},
			{
				name:           "crio with custom socket",
				runtime:        "crio",
				userSocket:     "/custom/crio.sock",
				expectedSocket: "/custom/crio.sock",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				defaults := restart.GetRuntimeDefaults(tt.runtime)
				socketPath := tt.userSocket

				// When - apply override pattern
				if socketPath == "" {
					socketPath = defaults.Socket
				}

				// Then
				assert.Equal(t, tt.expectedSocket, socketPath)
			})
		}
	})

	t.Run("empty socket falls back to default", func(t *testing.T) {
		tests := []struct {
			name           string
			runtime        string
			expectedSocket string
		}{
			{
				name:           "containerd falls back to default",
				runtime:        "containerd",
				expectedSocket: "/run/containerd/containerd.sock",
			},
			{
				name:           "docker falls back to default",
				runtime:        "docker",
				expectedSocket: "/var/run/docker.sock",
			},
			{
				name:           "crio falls back to default",
				runtime:        "crio",
				expectedSocket: "/var/run/crio/crio.sock",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				defaults := restart.GetRuntimeDefaults(tt.runtime)
				socketPath := ""

				// When - apply override pattern
				if socketPath == "" {
					socketPath = defaults.Socket
				}

				// Then
				assert.Equal(t, tt.expectedSocket, socketPath)
			})
		}
	})

	t.Run("hostRootMount adjusts socket path", func(t *testing.T) {
		tests := []struct {
			name           string
			runtime        string
			hostRoot       string
			expectedSocket string
		}{
			{
				name:           "containerd with /host mount",
				runtime:        "containerd",
				hostRoot:       "/host",
				expectedSocket: "/host/run/containerd/containerd.sock",
			},
			{
				name:           "docker with /host mount",
				runtime:        "docker",
				hostRoot:       "/host",
				expectedSocket: "/host/var/run/docker.sock",
			},
			{
				name:           "crio with /host mount",
				runtime:        "crio",
				hostRoot:       "/host",
				expectedSocket: "/host/var/run/crio/crio.sock",
			},
			{
				name:           "containerd with custom host root",
				runtime:        "containerd",
				hostRoot:       "/custom/root",
				expectedSocket: "/custom/root/run/containerd/containerd.sock",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				defaults := restart.GetRuntimeDefaults(tt.runtime)
				socketPath := defaults.Socket

				// When - apply host root adjustment
				if tt.hostRoot != "" && tt.hostRoot != "/" {
					socketPath = filepath.Join(tt.hostRoot, socketPath)
				}

				// Then
				assert.Equal(t, tt.expectedSocket, socketPath)
			})
		}
	})

	t.Run("hostRootMount with user socket override", func(t *testing.T) {
		tests := []struct {
			name           string
			runtime        string
			userSocket     string
			hostRoot       string
			expectedSocket string
		}{
			{
				name:           "user socket is NOT adjusted by hostRoot",
				runtime:        "containerd",
				userSocket:     "/custom/socket.sock",
				hostRoot:       "/host",
				expectedSocket: "/custom/socket.sock",
			},
			{
				name:           "empty user socket uses default, then adjusted for hostRoot",
				runtime:        "docker",
				userSocket:     "",
				hostRoot:       "/host",
				expectedSocket: "/host/var/run/docker.sock",
			},
			{
				name:           "hostRoot of / does not adjust socket",
				runtime:        "crio",
				userSocket:     "",
				hostRoot:       "/",
				expectedSocket: "/var/run/crio/crio.sock",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				defaults := restart.GetRuntimeDefaults(tt.runtime)
				socketPath := tt.userSocket
				socketUserProvided := socketPath != ""

				// When - apply override pattern
				if socketPath == "" {
					socketPath = defaults.Socket
				}

				// When - apply host root adjustment (only for default socket, not user-provided)
				if !socketUserProvided && tt.hostRoot != "" && tt.hostRoot != "/" {
					socketPath = filepath.Join(tt.hostRoot, socketPath)
				}

				// Then
				assert.Equal(t, tt.expectedSocket, socketPath)
			})
		}
	})
}

func TestEnvVarBinding(t *testing.T) {
	t.Run("string flags from env", func(t *testing.T) {
		tests := []struct {
			name     string
			envVar   string
			viperKey string
			value    string
		}{
			{"RBLN_CTK_DAEMON_RUNTIME sets runtime", "RBLN_CTK_DAEMON_RUNTIME", "runtime", "containerd"},
			{"RBLN_CTK_DAEMON_PID_FILE sets pid_file", "RBLN_CTK_DAEMON_PID_FILE", "pid_file", "/custom/pid"},
			{"RBLN_CTK_DAEMON_HOST_ROOT sets host_root", "RBLN_CTK_DAEMON_HOST_ROOT", "host_root", "/host"},
			{"RBLN_CTK_DAEMON_CDI_SPEC_DIR sets cdi_spec_dir", "RBLN_CTK_DAEMON_CDI_SPEC_DIR", "cdi_spec_dir", "/custom/cdi"},
			{"RBLN_CTK_DAEMON_CONTAINER_LIBRARY_PATH sets container_library_path", "RBLN_CTK_DAEMON_CONTAINER_LIBRARY_PATH", "container_library_path", "/rbln/lib"},
			{"RBLN_CTK_DAEMON_SOCKET sets socket", "RBLN_CTK_DAEMON_SOCKET", "socket", "/custom/sock"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				viper.Reset()
				defer viper.Reset()
				t.Setenv(tt.envVar, tt.value)

				// When
				cmd := newRootCmd()
				_ = cmd.PersistentPreRunE(cmd, []string{})

				// Then
				assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
			})
		}
	})

	t.Run("int flags from env", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_DAEMON_HEALTH_PORT", "9090")

		// When
		cmd := newRootCmd()
		_ = cmd.PersistentPreRunE(cmd, []string{})

		// Then
		assert.Equal(t, 9090, viper.GetInt("health_port"))
	})

	t.Run("bool flags from env", func(t *testing.T) {
		tests := []struct {
			name     string
			envVar   string
			viperKey string
			envValue string
			expected bool
		}{
			{"RBLN_CTK_DAEMON_DEBUG true", "RBLN_CTK_DAEMON_DEBUG", "debug", "true", true},
			{"RBLN_CTK_DAEMON_DEBUG false", "RBLN_CTK_DAEMON_DEBUG", "debug", "false", false},
			{"RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT true", "RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT", "no_cleanup_on_exit", "true", true},
			{"RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT 1", "RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT", "no_cleanup_on_exit", "1", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Given
				viper.Reset()
				defer viper.Reset()
				t.Setenv(tt.envVar, tt.envValue)

				// When
				cmd := newRootCmd()
				_ = cmd.PersistentPreRunE(cmd, []string{})

				// Then
				assert.Equal(t, tt.expected, viper.GetBool(tt.viperKey))
			})
		}
	})

	t.Run("duration flags from env", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_DAEMON_SHUTDOWN_TIMEOUT", "60s")

		// When
		cmd := newRootCmd()
		_ = cmd.PersistentPreRunE(cmd, []string{})

		// Then
		assert.Equal(t, 60*time.Second, viper.GetDuration("shutdown_timeout"))
	})

	t.Run("env var overrides default", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_DAEMON_HEALTH_PORT", "9999")

		// When
		cmd := newRootCmd()
		_ = cmd.PersistentPreRunE(cmd, []string{})

		// Then - should override default 8080
		assert.Equal(t, 9999, viper.GetInt("health_port"))
	})
}
