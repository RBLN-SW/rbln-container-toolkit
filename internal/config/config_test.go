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

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	// Given: No configuration provided

	// When: Getting default config
	cfg := DefaultConfig()

	// Then: All defaults should be set correctly
	assert.Equal(t, "/var/run/cdi/rbln.yaml", cfg.CDI.OutputPath)
	assert.Equal(t, "yaml", cfg.CDI.Format)
	assert.Equal(t, "rebellions.ai", cfg.CDI.Vendor)
	assert.Equal(t, "npu", cfg.CDI.Class)
	assert.Contains(t, cfg.Libraries.Patterns, "librbln-*.so*")
	assert.Contains(t, cfg.Tools, "rbln-smi")
	assert.Equal(t, "/", cfg.DriverRoot)
	assert.False(t, cfg.Debug)
}

func TestLoader_LoadWithDefaults(t *testing.T) {
	// Given: A loader with no config file
	loader := NewLoader()

	// When: Loading config
	cfg, err := loader.Load()

	// Then: Should return default config without error
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "/var/run/cdi/rbln.yaml", cfg.CDI.OutputPath)
}

func TestLoader_LoadFromFile(t *testing.T) {
	// Given: A config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
cdi:
  output-path: "/custom/path/rbln.yaml"
  format: "json"
  vendor: "test.vendor"
  class: "test-class"
libraries:
  patterns:
    - "libtest-*.so*"
tools:
  - "test-tool"
debug: true
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// When: Loading from file
	loader := NewLoader().WithFile(configPath)
	cfg, err := loader.Load()

	// Then: Should use file values
	require.NoError(t, err)
	assert.Equal(t, "/custom/path/rbln.yaml", cfg.CDI.OutputPath)
	assert.Equal(t, "json", cfg.CDI.Format)
	assert.Equal(t, "test.vendor", cfg.CDI.Vendor)
	assert.Equal(t, "test-class", cfg.CDI.Class)
	assert.Contains(t, cfg.Libraries.Patterns, "libtest-*.so*")
	assert.Contains(t, cfg.Tools, "test-tool")
	assert.True(t, cfg.Debug)
}

func TestLoader_LoadFromEnvVars(t *testing.T) {
	// Given: Environment variables set
	t.Setenv(EnvDebug, "true")
	t.Setenv(EnvCDIOutput, "/env/output.yaml")
	t.Setenv(EnvCDIFormat, "json")
	t.Setenv(EnvDriverRoot, "/custom/root")

	// When: Loading config
	loader := NewLoader()
	cfg, err := loader.Load()

	// Then: Should apply env vars
	require.NoError(t, err)
	assert.True(t, cfg.Debug)
	assert.Equal(t, "/env/output.yaml", cfg.CDI.OutputPath)
	assert.Equal(t, "json", cfg.CDI.Format)
	assert.Equal(t, "/custom/root", cfg.DriverRoot)
}

func TestLoader_OptionsOverrideEnvVars(t *testing.T) {
	// Given: Environment variables and CLI options
	t.Setenv(EnvCDIOutput, "/env/output.yaml")
	t.Setenv(EnvDriverRoot, "/env/root")

	// When: Loading with CLI options
	loader := NewLoader()
	cfg, err := loader.Load(
		WithOutputPath("/cli/output.yaml"),
		WithDriverRoot("/cli/root"),
	)

	// Then: CLI options should win
	require.NoError(t, err)
	assert.Equal(t, "/cli/output.yaml", cfg.CDI.OutputPath)
	assert.Equal(t, "/cli/root", cfg.DriverRoot)
}

func TestLoader_PriorityOrder(t *testing.T) {
	// Given: Config file, env var, and CLI option for the same setting
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
cdi:
  output-path: "/file/output.yaml"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Setenv(EnvCDIOutput, "/env/output.yaml")

	// When: Loading with all sources
	loader := NewLoader().WithFile(configPath)
	cfg, err := loader.Load(WithOutputPath("/cli/output.yaml"))

	// Then: Priority should be CLI > env > file > defaults
	require.NoError(t, err)
	assert.Equal(t, "/cli/output.yaml", cfg.CDI.OutputPath)
}

func TestLoader_EnvVarOverridesFile(t *testing.T) {
	// Given: Config file and env var for the same setting
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
cdi:
  output-path: "/file/output.yaml"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Setenv(EnvCDIOutput, "/env/output.yaml")

	// When: Loading without CLI options
	loader := NewLoader().WithFile(configPath)
	cfg, err := loader.Load()

	// Then: Env var should override file
	require.NoError(t, err)
	assert.Equal(t, "/env/output.yaml", cfg.CDI.OutputPath)
}

func TestLoader_InvalidConfigFile(t *testing.T) {
	// Given: An invalid config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	invalidContent := `
cdi:
  output-path: [invalid yaml
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	// When: Loading from invalid file
	loader := NewLoader().WithFile(configPath)
	_, err = loader.Load()

	// Then: Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestLoader_NonExistentConfigFile(t *testing.T) {
	// Given: A non-existent config file path set via loader
	loader := NewLoader().WithFile("/nonexistent/config.yaml")

	// When: Loading config
	cfg, err := loader.Load()

	// Then: Should return default config (non-existent file is not an error)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "/var/run/cdi/rbln.yaml", cfg.CDI.OutputPath)
}

func TestLoader_EnvVarConfigFile(t *testing.T) {
	// Given: Config file path in environment variable
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "env-config.yaml")
	configContent := `
cdi:
  output-path: "/env-file/output.yaml"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	t.Setenv(EnvConfigFile, configPath)

	// When: Loading config
	loader := NewLoader()
	cfg, err := loader.Load()

	// Then: Should use config from env var path
	require.NoError(t, err)
	assert.Equal(t, "/env-file/output.yaml", cfg.CDI.OutputPath)
}

func TestOptions_WithDriverRoot(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()

	// When: Applying WithDriverRoot
	WithDriverRoot("/custom/driver/root")(cfg)

	// Then: Driver root should be set
	assert.Equal(t, "/custom/driver/root", cfg.DriverRoot)
}

func TestOptions_WithDriverRoot_Empty(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()
	original := cfg.DriverRoot

	// When: Applying WithDriverRoot with empty string
	WithDriverRoot("")(cfg)

	// Then: Driver root should remain unchanged
	assert.Equal(t, original, cfg.DriverRoot)
}

func TestOptions_WithDebug(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()
	assert.False(t, cfg.Debug)

	// When: Applying WithDebug(true)
	WithDebug(true)(cfg)

	// Then: Debug should be true
	assert.True(t, cfg.Debug)
}

func TestOptions_WithFormat(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()

	// When: Applying WithFormat
	WithFormat("json")(cfg)

	// Then: Format should be set
	assert.Equal(t, "json", cfg.CDI.Format)
}

func TestOptions_WithVendorAndClass(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()

	// When: Applying WithVendor and WithClass
	WithVendor("custom.vendor")(cfg)
	WithClass("custom-class")(cfg)

	// Then: Values should be set
	assert.Equal(t, "custom.vendor", cfg.CDI.Vendor)
	assert.Equal(t, "custom-class", cfg.CDI.Class)
}

func TestGlibcExcludePatterns(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()

	// Then: glibc exclude list should contain critical patterns
	expectedPatterns := []string{
		"ld-linux*",
		"libc.so*",
		"libm.so*",
		"libpthread*",
		"libdl*",
		"librt*",
		"libgcc_s*",
		"libstdc++*",
	}

	for _, pattern := range expectedPatterns {
		assert.Contains(t, cfg.GlibcExclude, pattern, "should exclude %s", pattern)
	}
}

func TestDefaultConfig_ContainerPath(t *testing.T) {
	// Given: No configuration provided

	// When: Getting default config
	cfg := DefaultConfig()

	// Then: ContainerPath should be empty (disabled by default)
	assert.Equal(t, "", cfg.Libraries.ContainerPath)
}

func TestLoader_LoadFromFile_WithContainerPath(t *testing.T) {
	// Given: A config file with container-path
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	configContent := `
libraries:
  patterns:
    - "librbln-*.so*"
  container-path: "/usr/lib64/rbln"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// When: Loading from file
	loader := NewLoader().WithFile(configPath)
	cfg, err := loader.Load()

	// Then: Should use container-path value
	require.NoError(t, err)
	assert.Equal(t, "/usr/lib64/rbln", cfg.Libraries.ContainerPath)
}

func TestOptions_WithContainerLibraryPath(t *testing.T) {
	// Given: Default config
	cfg := DefaultConfig()

	// When: Applying WithContainerLibraryPath
	WithContainerLibraryPath("/usr/lib64/rbln")(cfg)

	// Then: ContainerPath should be set
	assert.Equal(t, "/usr/lib64/rbln", cfg.Libraries.ContainerPath)
}

func TestOptions_WithContainerLibraryPath_Empty(t *testing.T) {
	// Given: Config with ContainerPath set
	cfg := DefaultConfig()
	cfg.Libraries.ContainerPath = "/some/path"

	// When: Applying WithContainerLibraryPath with empty string
	WithContainerLibraryPath("")(cfg)

	// Then: ContainerPath should remain unchanged (empty string is no-op)
	assert.Equal(t, "/some/path", cfg.Libraries.ContainerPath)
}

func TestLoader_CLIOverridesEnvForContainerPath(t *testing.T) {
	// Given: Environment variable and CLI option for container-library-path
	t.Setenv("RBLN_CONTAINER_LIBRARY_PATH", "/env/path")

	// When: Loading with CLI option
	loader := NewLoader()
	cfg, err := loader.Load(
		WithContainerLibraryPath("/cli/path"),
	)

	// Then: CLI option should win
	require.NoError(t, err)
	assert.Equal(t, "/cli/path", cfg.Libraries.ContainerPath)
}
