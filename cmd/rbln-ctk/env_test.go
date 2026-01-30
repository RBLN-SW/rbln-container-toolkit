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
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

// TestEnvVarBinding_RootFlags tests that root flags properly bind to environment variables
func TestEnvVarBinding_RootFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
	}{
		{
			name:     "RBLN_CTK_CONFIG sets config",
			envVar:   "RBLN_CTK_CONFIG",
			viperKey: "config",
			value:    "/custom/config.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_RootBoolFlags tests that root boolean flags properly bind to environment variables
func TestEnvVarBinding_RootBoolFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
		expected bool
	}{
		{
			name:     "RBLN_CTK_DEBUG true",
			envVar:   "RBLN_CTK_DEBUG",
			viperKey: "debug",
			value:    "true",
			expected: true,
		},
		{
			name:     "RBLN_CTK_DEBUG false",
			envVar:   "RBLN_CTK_DEBUG",
			viperKey: "debug",
			value:    "false",
			expected: false,
		},
		{
			name:     "RBLN_CTK_QUIET true",
			envVar:   "RBLN_CTK_QUIET",
			viperKey: "quiet",
			value:    "true",
			expected: true,
		},
		{
			name:     "RBLN_CTK_QUIET false",
			envVar:   "RBLN_CTK_QUIET",
			viperKey: "quiet",
			value:    "false",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.expected, viper.GetBool(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_CDIGenerateFlags tests that cdi generate flags properly bind to environment variables
func TestEnvVarBinding_CDIGenerateFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
	}{
		{
			name:     "RBLN_CTK_OUTPUT sets output",
			envVar:   "RBLN_CTK_OUTPUT",
			viperKey: "output",
			value:    "/custom/output.yaml",
		},
		{
			name:     "RBLN_CTK_FORMAT sets format",
			envVar:   "RBLN_CTK_FORMAT",
			viperKey: "format",
			value:    "json",
		},
		{
			name:     "RBLN_CTK_DRIVER_ROOT sets driver-root",
			envVar:   "RBLN_CTK_DRIVER_ROOT",
			viperKey: "driver-root",
			value:    "/custom/driver",
		},
		{
			name:     "RBLN_CTK_CONTAINER_LIBRARY_PATH sets container-library-path",
			envVar:   "RBLN_CTK_CONTAINER_LIBRARY_PATH",
			viperKey: "container-library-path",
			value:    "/rbln/lib",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_RuntimeConfigureFlags tests that runtime configure flags properly bind to environment variables
func TestEnvVarBinding_RuntimeConfigureFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
	}{
		{
			name:     "RBLN_CTK_RUNTIME sets runtime",
			envVar:   "RBLN_CTK_RUNTIME",
			viperKey: "runtime",
			value:    "containerd",
		},
		{
			name:     "RBLN_CTK_CONFIG_PATH sets config-path",
			envVar:   "RBLN_CTK_CONFIG_PATH",
			viperKey: "config-path",
			value:    "/custom/config.toml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_EnvKeyReplacer tests that hyphens in viper keys are replaced with underscores in env var names
func TestEnvVarBinding_EnvKeyReplacer(t *testing.T) {
	t.Run("hyphen replaced with underscore in env var name", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_DRIVER_ROOT", "/custom/root")

		// When
		initConfig()

		// Then - viper key is "driver-root", env var is RBLN_CTK_DRIVER_ROOT
		assert.Equal(t, "/custom/root", viper.GetString("driver-root"))
	})

	t.Run("container-library-path env var maps to container-library-path viper key", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_CONTAINER_LIBRARY_PATH", "/rbln/lib64")

		// When
		initConfig()

		// Then
		assert.Equal(t, "/rbln/lib64", viper.GetString("container-library-path"))
	})

	t.Run("config-path env var maps to config-path viper key", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_CONFIG_PATH", "/etc/custom.toml")

		// When
		initConfig()

		// Then
		assert.Equal(t, "/etc/custom.toml", viper.GetString("config-path"))
	})
}

// TestEnvVarBinding_AllFlagsComprehensive tests all 9 flags in a single comprehensive test
func TestEnvVarBinding_AllFlagsComprehensive(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
		isBool   bool
	}{
		// Root flags (3)
		{
			name:     "RBLN_CTK_CONFIG",
			envVar:   "RBLN_CTK_CONFIG",
			viperKey: "config",
			value:    "/etc/rbln/config.yaml",
			isBool:   false,
		},
		{
			name:     "RBLN_CTK_DEBUG",
			envVar:   "RBLN_CTK_DEBUG",
			viperKey: "debug",
			value:    "true",
			isBool:   true,
		},
		{
			name:     "RBLN_CTK_QUIET",
			envVar:   "RBLN_CTK_QUIET",
			viperKey: "quiet",
			value:    "true",
			isBool:   true,
		},
		// CDI generate flags (4)
		{
			name:     "RBLN_CTK_OUTPUT",
			envVar:   "RBLN_CTK_OUTPUT",
			viperKey: "output",
			value:    "/var/run/cdi/rbln.yaml",
			isBool:   false,
		},
		{
			name:     "RBLN_CTK_FORMAT",
			envVar:   "RBLN_CTK_FORMAT",
			viperKey: "format",
			value:    "json",
			isBool:   false,
		},
		{
			name:     "RBLN_CTK_DRIVER_ROOT",
			envVar:   "RBLN_CTK_DRIVER_ROOT",
			viperKey: "driver-root",
			value:    "/run/rbln/driver",
			isBool:   false,
		},
		{
			name:     "RBLN_CTK_CONTAINER_LIBRARY_PATH",
			envVar:   "RBLN_CTK_CONTAINER_LIBRARY_PATH",
			viperKey: "container-library-path",
			value:    "/rbln/lib64",
			isBool:   false,
		},
		// Runtime configure flags (2)
		{
			name:     "RBLN_CTK_RUNTIME",
			envVar:   "RBLN_CTK_RUNTIME",
			viperKey: "runtime",
			value:    "containerd",
			isBool:   false,
		},
		{
			name:     "RBLN_CTK_CONFIG_PATH",
			envVar:   "RBLN_CTK_CONFIG_PATH",
			viperKey: "config-path",
			value:    "/etc/containerd/config.toml",
			isBool:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			if tt.isBool {
				expected := tt.value == "true"
				assert.Equal(t, expected, viper.GetBool(tt.viperKey))
			} else {
				assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
			}
		})
	}
}

// TestEnvVarBinding_EnvPrefixRBLN_CTK tests that RBLN_CTK prefix is correctly set
func TestEnvVarBinding_EnvPrefixRBLN_CTK(t *testing.T) {
	t.Run("RBLN_CTK prefix is required", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		// Set env var WITHOUT prefix - should NOT be picked up
		t.Setenv("CONFIG", "/custom/config.yaml")
		// Set env var WITH prefix - should be picked up
		t.Setenv("RBLN_CTK_CONFIG", "/prefixed/config.yaml")

		// When
		initConfig()

		// Then - only the prefixed env var should be used
		assert.Equal(t, "/prefixed/config.yaml", viper.GetString("config"))
	})
}

// TestEnvVarBinding_AutomaticEnv tests that AutomaticEnv is enabled
func TestEnvVarBinding_AutomaticEnv(t *testing.T) {
	t.Run("AutomaticEnv binds env vars without explicit BindPFlag", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_OUTPUT", "/custom/output.yaml")

		// When
		initConfig()

		// Then - even without explicit BindPFlag, env var should be bound
		assert.Equal(t, "/custom/output.yaml", viper.GetString("output"))
	})
}

// TestEnvVarBinding_EnvKeyReplacer_Comprehensive tests the env key replacer with various flag names
func TestEnvVarBinding_EnvKeyReplacer_Comprehensive(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
	}{
		{
			name:     "single hyphen: driver-root",
			envVar:   "RBLN_CTK_DRIVER_ROOT",
			viperKey: "driver-root",
			value:    "/driver",
		},
		{
			name:     "multiple hyphens: container-library-path",
			envVar:   "RBLN_CTK_CONTAINER_LIBRARY_PATH",
			viperKey: "container-library-path",
			value:    "/lib",
		},
		{
			name:     "multiple hyphens: config-path",
			envVar:   "RBLN_CTK_CONFIG_PATH",
			viperKey: "config-path",
			value:    "/config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_SetEnvKeyReplacer tests that SetEnvKeyReplacer is correctly configured
func TestEnvVarBinding_SetEnvKeyReplacer(t *testing.T) {
	t.Run("SetEnvKeyReplacer replaces hyphens with underscores", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()

		// When
		initConfig()

		// Then - verify that the replacer is set by checking that hyphenated keys work
		t.Setenv("RBLN_CTK_DRIVER_ROOT", "/test")
		assert.Equal(t, "/test", viper.GetString("driver-root"))
	})
}

// TestEnvVarBinding_BoolFlagVariations tests various boolean value representations
func TestEnvVarBinding_BoolFlagVariations(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
		expected bool
	}{
		{
			name:     "true string",
			envVar:   "RBLN_CTK_DEBUG",
			viperKey: "debug",
			value:    "true",
			expected: true,
		},
		{
			name:     "false string",
			envVar:   "RBLN_CTK_DEBUG",
			viperKey: "debug",
			value:    "false",
			expected: false,
		},
		{
			name:     "1 for true",
			envVar:   "RBLN_CTK_QUIET",
			viperKey: "quiet",
			value:    "1",
			expected: true,
		},
		{
			name:     "0 for false",
			envVar:   "RBLN_CTK_QUIET",
			viperKey: "quiet",
			value:    "0",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.expected, viper.GetBool(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_ViperReset tests that viper.Reset() properly clears state between tests
func TestEnvVarBinding_ViperReset(t *testing.T) {
	t.Run("viper.Reset() clears previous env var bindings", func(t *testing.T) {
		// Given - first test
		viper.Reset()
		t.Setenv("RBLN_CTK_CONFIG", "/first/config.yaml")
		initConfig()
		assert.Equal(t, "/first/config.yaml", viper.GetString("config"))

		// When - reset and set different value
		viper.Reset()
		t.Setenv("RBLN_CTK_CONFIG", "/second/config.yaml")
		initConfig()

		// Then - should have new value, not old
		assert.Equal(t, "/second/config.yaml", viper.GetString("config"))
	})
}

// TestEnvVarBinding_EmptyEnvVar tests behavior when env var is set but empty
func TestEnvVarBinding_EmptyEnvVar(t *testing.T) {
	t.Run("empty env var returns empty string", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_CONFIG", "")

		// When
		initConfig()

		// Then
		assert.Equal(t, "", viper.GetString("config"))
	})
}

// TestEnvVarBinding_SpecialCharacters tests env vars with special characters in values
func TestEnvVarBinding_SpecialCharacters(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
	}{
		{
			name:     "path with spaces",
			envVar:   "RBLN_CTK_OUTPUT",
			viperKey: "output",
			value:    "/var/run/cdi/my spec.yaml",
		},
		{
			name:     "path with dots",
			envVar:   "RBLN_CTK_CONFIG",
			viperKey: "config",
			value:    "/etc/rbln/config.v1.2.3.yaml",
		},
		{
			name:     "path with underscores",
			envVar:   "RBLN_CTK_DRIVER_ROOT",
			viperKey: "driver-root",
			value:    "/run/rbln_driver_root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			viper.Reset()
			defer viper.Reset()
			t.Setenv(tt.envVar, tt.value)

			// When
			initConfig()

			// Then
			assert.Equal(t, tt.value, viper.GetString(tt.viperKey))
		})
	}
}

// TestEnvVarBinding_CaseSensitivity tests that env var names are case-sensitive
func TestEnvVarBinding_CaseSensitivity(t *testing.T) {
	t.Run("env var names are case-sensitive", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		// Set lowercase version - should NOT work
		t.Setenv("rbln_ctk_config", "/lowercase/config.yaml")
		// Set correct uppercase version
		t.Setenv("RBLN_CTK_CONFIG", "/uppercase/config.yaml")

		// When
		initConfig()

		// Then - only uppercase should work
		assert.Equal(t, "/uppercase/config.yaml", viper.GetString("config"))
	})
}

// TestEnvVarBinding_MultipleEnvVars tests that multiple env vars can be set simultaneously
func TestEnvVarBinding_MultipleEnvVars(t *testing.T) {
	t.Run("multiple env vars bind correctly", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_CONFIG", "/etc/rbln/config.yaml")
		t.Setenv("RBLN_CTK_DEBUG", "true")
		t.Setenv("RBLN_CTK_QUIET", "false")
		t.Setenv("RBLN_CTK_OUTPUT", "/var/run/cdi/rbln.yaml")
		t.Setenv("RBLN_CTK_FORMAT", "json")
		t.Setenv("RBLN_CTK_DRIVER_ROOT", "/run/rbln/driver")
		t.Setenv("RBLN_CTK_CONTAINER_LIBRARY_PATH", "/rbln/lib64")
		t.Setenv("RBLN_CTK_RUNTIME", "containerd")
		t.Setenv("RBLN_CTK_CONFIG_PATH", "/etc/containerd/config.toml")

		// When
		initConfig()

		// Then - all should be bound correctly
		assert.Equal(t, "/etc/rbln/config.yaml", viper.GetString("config"))
		assert.Equal(t, true, viper.GetBool("debug"))
		assert.Equal(t, false, viper.GetBool("quiet"))
		assert.Equal(t, "/var/run/cdi/rbln.yaml", viper.GetString("output"))
		assert.Equal(t, "json", viper.GetString("format"))
		assert.Equal(t, "/run/rbln/driver", viper.GetString("driver-root"))
		assert.Equal(t, "/rbln/lib64", viper.GetString("container-library-path"))
		assert.Equal(t, "containerd", viper.GetString("runtime"))
		assert.Equal(t, "/etc/containerd/config.toml", viper.GetString("config-path"))
	})
}

// TestEnvVarBinding_InitConfigIdempotent tests that calling initConfig multiple times is safe
func TestEnvVarBinding_InitConfigIdempotent(t *testing.T) {
	t.Run("calling initConfig multiple times is safe", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CTK_CONFIG", "/etc/rbln/config.yaml")

		// When - call initConfig multiple times
		initConfig()
		initConfig()
		initConfig()

		// Then - should still work correctly
		assert.Equal(t, "/etc/rbln/config.yaml", viper.GetString("config"))
	})
}

// TestEnvVarBinding_EnvVarPriority tests that env vars override defaults
func TestEnvVarBinding_EnvVarPriority(t *testing.T) {
	t.Run("env var overrides default value", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		// Set default
		viper.SetDefault("output", "/var/run/cdi/rbln.yaml")
		// Set env var with different value
		t.Setenv("RBLN_CTK_OUTPUT", "/custom/output.yaml")

		// When
		initConfig()

		// Then - env var should take precedence
		assert.Equal(t, "/custom/output.yaml", viper.GetString("output"))
	})
}

// TestEnvVarBinding_VerifyEnvPrefix tests that the env prefix is correctly set to RBLN_CTK
func TestEnvVarBinding_VerifyEnvPrefix(t *testing.T) {
	t.Run("env prefix is RBLN_CTK", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()

		// When
		initConfig()

		// Then - verify by checking that RBLN_CTK_ prefix works
		t.Setenv("RBLN_CTK_CONFIG", "/test/config.yaml")
		assert.Equal(t, "/test/config.yaml", viper.GetString("config"))
	})
}

// TestEnvVarBinding_VerifyEnvKeyReplacer tests that the env key replacer is correctly configured
func TestEnvVarBinding_VerifyEnvKeyReplacer(t *testing.T) {
	t.Run("env key replacer converts hyphens to underscores", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()

		// When
		initConfig()

		// Then - verify by checking that hyphenated keys work with underscored env vars
		t.Setenv("RBLN_CTK_DRIVER_ROOT", "/test")
		assert.Equal(t, "/test", viper.GetString("driver-root"))

		t.Setenv("RBLN_CTK_CONTAINER_LIBRARY_PATH", "/lib")
		assert.Equal(t, "/lib", viper.GetString("container-library-path"))

		t.Setenv("RBLN_CTK_CONFIG_PATH", "/config")
		assert.Equal(t, "/config", viper.GetString("config-path"))
	})
}

// TestEnvVarBinding_VerifyAutomaticEnv tests that AutomaticEnv is enabled
func TestEnvVarBinding_VerifyAutomaticEnv(t *testing.T) {
	t.Run("AutomaticEnv is enabled", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()

		// When
		initConfig()

		// Then - verify by checking that env vars are automatically bound without explicit BindPFlag
		t.Setenv("RBLN_CTK_OUTPUT", "/custom/output.yaml")
		assert.Equal(t, "/custom/output.yaml", viper.GetString("output"))

		t.Setenv("RBLN_CTK_FORMAT", "json")
		assert.Equal(t, "json", viper.GetString("format"))
	})
}

// TestEnvVarBinding_StringReplacer tests the strings.NewReplacer functionality
func TestEnvVarBinding_StringReplacer(t *testing.T) {
	t.Run("strings.NewReplacer correctly replaces hyphens with underscores", func(t *testing.T) {
		// Given
		replacer := strings.NewReplacer("-", "_")

		// When
		result := replacer.Replace("driver-root")

		// Then
		assert.Equal(t, "driver_root", result)
	})

	t.Run("strings.NewReplacer handles multiple hyphens", func(t *testing.T) {
		// Given
		replacer := strings.NewReplacer("-", "_")

		// When
		result := replacer.Replace("container-library-path")

		// Then
		assert.Equal(t, "container_library_path", result)
	})
}
