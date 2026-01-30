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
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRuntimeCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newRuntimeCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "runtime", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
	})

	t.Run("has expected persistent flags", func(t *testing.T) {
		// Given
		flags := []string{
			"restart-mode",
			"host-root-mount",
			"socket",
			"cdi-spec-dir",
			"pid-file",
			"dry-run",
		}

		// When
		cmd := newRuntimeCmd()

		// Then
		for _, flag := range flags {
			f := cmd.PersistentFlags().Lookup(flag)
			assert.NotNil(t, f, "persistent flag %q should exist", flag)
		}
	})

	t.Run("has runtime subcommands", func(t *testing.T) {
		// Given
		expectedSubcommands := []string{"docker", "containerd", "crio"}

		// When
		cmd := newRuntimeCmd()

		// Then
		actualSubcommands := make(map[string]bool)
		for _, sub := range cmd.Commands() {
			actualSubcommands[sub.Use] = true
		}
		for _, expected := range expectedSubcommands {
			assert.True(t, actualSubcommands[expected], "should have %q subcommand", expected)
		}
	})
}

func TestRuntimeCmdFlagDefaults(t *testing.T) {
	tests := []struct {
		name     string
		flag     string
		expected string
	}{
		{
			name:     "cdi-spec-dir flag has default value",
			flag:     "cdi-spec-dir",
			expected: "/var/run/cdi",
		},
		{
			name:     "pid-file flag has default value",
			flag:     "pid-file",
			expected: "/run/rbln/toolkit.pid",
		},
		{
			name:     "restart-mode flag has empty default",
			flag:     "restart-mode",
			expected: "",
		},
		{
			name:     "host-root-mount flag has empty default",
			flag:     "host-root-mount",
			expected: "",
		},
		{
			name:     "socket flag has empty default",
			flag:     "socket",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cmd := newRuntimeCmd()

			// When
			f := cmd.PersistentFlags().Lookup(tt.flag)

			// Then
			require.NotNil(t, f)
			assert.Equal(t, tt.expected, f.DefValue)
		})
	}
}

func TestGetEffectiveRestartMode(t *testing.T) {
	t.Run("returns flag value when provided", func(t *testing.T) {
		// Given
		originalValue := viper.GetString("restart_mode")
		defer viper.Set("restart_mode", originalValue)
		viper.Set("restart_mode", "systemd")
		flagValue := "signal"

		// When
		result := getEffectiveRestartMode("containerd", flagValue)

		// Then
		assert.Equal(t, "signal", result)
	})

	t.Run("returns env value when no flag provided", func(t *testing.T) {
		// Given
		originalValue := viper.GetString("restart_mode")
		defer viper.Set("restart_mode", originalValue)
		viper.Set("restart_mode", "systemd")
		flagValue := ""

		// When
		result := getEffectiveRestartMode("containerd", flagValue)

		// Then
		assert.Equal(t, "systemd", result)
	})

	t.Run("returns empty when no flag and no env", func(t *testing.T) {
		// Given
		originalValue := viper.GetString("restart_mode")
		defer viper.Set("restart_mode", originalValue)
		viper.Set("restart_mode", "")
		flagValue := ""

		// When
		result := getEffectiveRestartMode("containerd", flagValue)

		// Then
		assert.Equal(t, "", result)
	})
}

func TestGetEffectiveSocket(t *testing.T) {
	t.Run("returns flag value when provided", func(t *testing.T) {
		// Given
		originalValue := viper.GetString("socket")
		defer viper.Set("socket", originalValue)
		viper.Set("socket", "/some/socket.sock")
		flagValue := "/custom/socket.sock"

		// When
		result := getEffectiveSocket("docker", flagValue)

		// Then
		assert.Equal(t, "/custom/socket.sock", result)
	})

	t.Run("returns env value when no flag provided", func(t *testing.T) {
		// Given
		originalValue := viper.GetString("socket")
		defer viper.Set("socket", originalValue)
		viper.Set("socket", "/env/socket.sock")
		flagValue := ""

		// When
		result := getEffectiveSocket("docker", flagValue)

		// Then
		assert.Equal(t, "/env/socket.sock", result)
	})

	t.Run("returns empty when no flag and no env", func(t *testing.T) {
		// Given
		originalValue := viper.GetString("socket")
		defer viper.Set("socket", originalValue)
		viper.Set("socket", "")
		flagValue := ""

		// When
		result := getEffectiveSocket("docker", flagValue)

		// Then
		assert.Equal(t, "", result)
	})
}

func TestGetEffectiveHostRootMount(t *testing.T) {
	t.Run("returns flag value when set", func(t *testing.T) {
		// Given
		originalHostRootMount := hostRootMount
		originalViperValue := viper.GetString("host_root_mount")
		defer func() {
			hostRootMount = originalHostRootMount
			viper.Set("host_root_mount", originalViperValue)
		}()
		hostRootMount = "/host-flag"
		viper.Set("host_root_mount", "/host-env")

		// When
		result := getEffectiveHostRootMount()

		// Then
		assert.Equal(t, "/host-flag", result)
	})

	t.Run("returns env value when no flag", func(t *testing.T) {
		// Given
		originalHostRootMount := hostRootMount
		originalViperValue := viper.GetString("host_root_mount")
		defer func() {
			hostRootMount = originalHostRootMount
			viper.Set("host_root_mount", originalViperValue)
		}()
		hostRootMount = ""
		viper.Set("host_root_mount", "/host-env")

		// When
		result := getEffectiveHostRootMount()

		// Then
		assert.Equal(t, "/host-env", result)
	})

	t.Run("returns empty when no flag and no env", func(t *testing.T) {
		// Given
		originalHostRootMount := hostRootMount
		originalViperValue := viper.GetString("host_root_mount")
		defer func() {
			hostRootMount = originalHostRootMount
			viper.Set("host_root_mount", originalViperValue)
		}()
		hostRootMount = ""
		viper.Set("host_root_mount", "")

		// When
		result := getEffectiveHostRootMount()

		// Then
		assert.Equal(t, "", result)
	})
}

func TestGetEffectiveCDISpecDir(t *testing.T) {
	t.Run("returns flag value when set", func(t *testing.T) {
		// Given
		originalCdiSpecDir := cdiSpecDir
		originalViperValue := viper.GetString("cdi_spec_dir")
		defer func() {
			cdiSpecDir = originalCdiSpecDir
			viper.Set("cdi_spec_dir", originalViperValue)
		}()
		cdiSpecDir = "/custom/cdi"
		viper.Set("cdi_spec_dir", "/env/cdi")

		// When
		result := getEffectiveCDISpecDir()

		// Then
		assert.Equal(t, "/custom/cdi", result)
	})

	t.Run("returns env value when no flag", func(t *testing.T) {
		// Given
		originalCdiSpecDir := cdiSpecDir
		originalViperValue := viper.GetString("cdi_spec_dir")
		defer func() {
			cdiSpecDir = originalCdiSpecDir
			viper.Set("cdi_spec_dir", originalViperValue)
		}()
		cdiSpecDir = ""
		viper.Set("cdi_spec_dir", "/env/cdi")

		// When
		result := getEffectiveCDISpecDir()

		// Then
		assert.Equal(t, "/env/cdi", result)
	})

	t.Run("returns default when no flag and no env", func(t *testing.T) {
		// Given
		originalCdiSpecDir := cdiSpecDir
		originalViperValue := viper.GetString("cdi_spec_dir")
		defer func() {
			cdiSpecDir = originalCdiSpecDir
			viper.Set("cdi_spec_dir", originalViperValue)
		}()
		cdiSpecDir = ""
		viper.Set("cdi_spec_dir", "")

		// When
		result := getEffectiveCDISpecDir()

		// Then
		assert.Equal(t, "/var/run/cdi", result)
	})
}

func TestGetEffectivePidFile(t *testing.T) {
	t.Run("returns flag value when set", func(t *testing.T) {
		// Given
		originalPidFile := pidFile
		originalViperValue := viper.GetString("pid_file")
		defer func() {
			pidFile = originalPidFile
			viper.Set("pid_file", originalViperValue)
		}()
		pidFile = "/custom/pid.pid"
		viper.Set("pid_file", "/env/pid.pid")

		// When
		result := getEffectivePidFile()

		// Then
		assert.Equal(t, "/custom/pid.pid", result)
	})

	t.Run("returns env value when no flag", func(t *testing.T) {
		// Given
		originalPidFile := pidFile
		originalViperValue := viper.GetString("pid_file")
		defer func() {
			pidFile = originalPidFile
			viper.Set("pid_file", originalViperValue)
		}()
		pidFile = ""
		viper.Set("pid_file", "/env/pid.pid")

		// When
		result := getEffectivePidFile()

		// Then
		assert.Equal(t, "/env/pid.pid", result)
	})

	t.Run("returns default when no flag and no env", func(t *testing.T) {
		// Given
		originalPidFile := pidFile
		originalViperValue := viper.GetString("pid_file")
		defer func() {
			pidFile = originalPidFile
			viper.Set("pid_file", originalViperValue)
		}()
		pidFile = ""
		viper.Set("pid_file", "")

		// When
		result := getEffectivePidFile()

		// Then
		assert.Equal(t, "/run/rbln/toolkit.pid", result)
	})
}
