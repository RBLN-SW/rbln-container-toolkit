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
)

func TestEnvVarBinding_HookStringFlags(t *testing.T) {
	tests := []struct {
		name     string
		envVar   string
		viperKey string
		value    string
	}{
		{
			name:     "RBLN_CDI_HOOK_LDCONFIG_PATH sets ldconfig-path",
			envVar:   "RBLN_CDI_HOOK_LDCONFIG_PATH",
			viperKey: "ldconfig-path",
			value:    "/custom/ldconfig",
		},
		{
			name:     "RBLN_CDI_HOOK_CONTAINER_SPEC sets container-spec",
			envVar:   "RBLN_CDI_HOOK_CONTAINER_SPEC",
			viperKey: "container-spec",
			value:    "/custom/spec.json",
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

func TestEnvVarBinding_HookSliceFlag(t *testing.T) {
	t.Run("RBLN_CDI_HOOK_FOLDER sets folder as comma-separated string", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CDI_HOOK_FOLDER", "/lib1,/lib2,/lib3")

		// When
		initConfig()

		// Then
		result := viper.GetStringSlice("folder")
		assert.Len(t, result, 1)
		assert.Equal(t, "/lib1,/lib2,/lib3", result[0])
	})
}

func TestEnvVarBinding_EnvKeyReplacer(t *testing.T) {
	t.Run("hyphen replaced with underscore in env var name", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CDI_HOOK_LDCONFIG_PATH", "/custom/ldconfig")

		// When
		initConfig()

		// Then
		assert.Equal(t, "/custom/ldconfig", viper.GetString("ldconfig-path"))
	})
}

func TestEnvVarBinding_AllFlagsIntegration(t *testing.T) {
	t.Run("all three flags bind correctly from environment", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CDI_HOOK_FOLDER", "/usr/lib,/usr/local/lib")
		t.Setenv("RBLN_CDI_HOOK_LDCONFIG_PATH", "/sbin/ldconfig")
		t.Setenv("RBLN_CDI_HOOK_CONTAINER_SPEC", "/var/run/spec.json")

		// When
		initConfig()

		// Then
		ldconfigPath := viper.GetString("ldconfig-path")
		containerSpec := viper.GetString("container-spec")
		folder := viper.GetStringSlice("folder")

		assert.Equal(t, "/sbin/ldconfig", ldconfigPath)
		assert.Equal(t, "/var/run/spec.json", containerSpec)
		assert.Len(t, folder, 1)
		assert.Equal(t, "/usr/lib,/usr/local/lib", folder[0])
	})
}

func TestEnvVarBinding_EnvPrefixApplied(t *testing.T) {
	t.Run("RBLN_CDI_HOOK prefix is required", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("LDCONFIG_PATH", "/wrong/path")
		t.Setenv("RBLN_CDI_HOOK_LDCONFIG_PATH", "/correct/path")

		// When
		initConfig()

		// Then
		assert.Equal(t, "/correct/path", viper.GetString("ldconfig-path"))
	})
}

func TestEnvVarBinding_EmptyEnvVars(t *testing.T) {
	t.Run("empty environment variables return empty values", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()

		// When
		initConfig()

		// Then
		assert.Equal(t, "", viper.GetString("ldconfig-path"))
		assert.Equal(t, "", viper.GetString("container-spec"))
		assert.Equal(t, []string(nil), viper.GetStringSlice("folder"))
	})
}

func TestEnvVarBinding_MultipleSliceValues(t *testing.T) {
	t.Run("RBLN_CDI_HOOK_FOLDER with multiple comma-separated paths", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CDI_HOOK_FOLDER", "/lib64,/usr/lib64,/opt/rbln/lib")

		// When
		initConfig()

		// Then
		result := viper.GetStringSlice("folder")
		assert.Len(t, result, 1)
		assert.Equal(t, "/lib64,/usr/lib64,/opt/rbln/lib", result[0])
	})
}

func TestEnvVarBinding_SpecialCharactersInPaths(t *testing.T) {
	t.Run("paths with special characters are preserved", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		specialPath := "/usr/lib/x86_64-linux-gnu"
		t.Setenv("RBLN_CDI_HOOK_LDCONFIG_PATH", specialPath)

		// When
		initConfig()

		// Then
		assert.Equal(t, specialPath, viper.GetString("ldconfig-path"))
	})
}

func TestEnvVarBinding_ViperResetBetweenTests(t *testing.T) {
	t.Run("first test sets value", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CDI_HOOK_LDCONFIG_PATH", "/first/path")

		// When
		initConfig()

		// Then
		assert.Equal(t, "/first/path", viper.GetString("ldconfig-path"))
	})

	t.Run("second test has clean state", func(t *testing.T) {
		// Given
		viper.Reset()
		defer viper.Reset()
		t.Setenv("RBLN_CDI_HOOK_LDCONFIG_PATH", "/second/path")

		// When
		initConfig()

		// Then
		assert.Equal(t, "/second/path", viper.GetString("ldconfig-path"))
	})
}
