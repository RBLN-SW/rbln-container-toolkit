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
	"bytes"
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunInfo_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	assert.NoError(t, err)
	assert.NotEmpty(t, output)
}

func TestRunInfo_VersionDisplay(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	assert.Contains(t, output, "RBLN Container Toolkit")
	assert.Contains(t, output, "Version:")
	assert.Contains(t, output, "Build Date:")
	assert.Contains(t, output, "Git Commit:")
}

func TestRunInfo_SystemInfoDisplay(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	assert.Contains(t, output, "System:")
	assert.Contains(t, output, "OS:")
	assert.Contains(t, output, "Architecture:")
}

func TestRunInfo_DriverInfoDisplay(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	assert.Contains(t, output, "RBLN Driver:")
	assert.Contains(t, output, "Libraries:")
	assert.Contains(t, output, "Tools:")
}

func TestRunInfo_ConfigPathDisplay(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	assert.Contains(t, output, "Configuration:")
	assert.Contains(t, output, "Config File:")
	assert.Contains(t, output, "CDI Output:")
	assert.Contains(t, output, "Driver Root:")
}

func TestRunInfo_OutputFormatting(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	// Verify output has proper formatting with indentation
	assert.Contains(t, output, "  Version:")
	assert.Contains(t, output, "  Build Date:")
	assert.Contains(t, output, "  Git Commit:")
	assert.Contains(t, output, "  OS:")
	assert.Contains(t, output, "  Architecture:")
	assert.Contains(t, output, "  Libraries:")
	assert.Contains(t, output, "  Tools:")
	assert.Contains(t, output, "  Config File:")
	assert.Contains(t, output, "  CDI Output:")
	assert.Contains(t, output, "  Driver Root:")
}

func TestRunInfo_WithConfigPath(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	configPath := tmpDir + "/test-config.yaml"

	// Create a minimal config file
	configContent := `
cdi:
  output-path: /var/run/cdi/rbln.yaml
driver-root: /
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", configPath, "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", configPath)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err = runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	assert.Contains(t, output, "Config File:")
	assert.Contains(t, output, configPath)
}

func TestRunInfo_WithDebugFlag(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", true, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", true)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	assert.NotEmpty(t, output)
	assert.Contains(t, output, "RBLN Container Toolkit")
}

func TestGetConfigPath_DefaultPath(t *testing.T) {
	// Given
	viper.Reset()
	os.Unsetenv("RBLN_CTK_CONFIG")

	// When
	path := getConfigPath()

	// Then
	assert.Equal(t, "/etc/rbln/container-toolkit.yaml", path)
}

func TestGetConfigPath_FromViper(t *testing.T) {
	// Given
	viper.Reset()
	viper.Set("config", "/custom/config.yaml")
	os.Unsetenv("RBLN_CTK_CONFIG")

	// When
	path := getConfigPath()

	// Then
	assert.Equal(t, "/custom/config.yaml", path)
}

func TestGetConfigPath_FromEnvironment(t *testing.T) {
	// Given
	viper.Reset()
	viper.Set("config", "")
	os.Setenv("RBLN_CTK_CONFIG", "/env/config.yaml")
	defer os.Unsetenv("RBLN_CTK_CONFIG")

	// When
	path := getConfigPath()

	// Then
	assert.Equal(t, "/env/config.yaml", path)
}

func TestGetConfigPath_ViperPrecedence(t *testing.T) {
	// Given - Viper should take precedence over environment
	viper.Reset()
	viper.Set("config", "/viper/config.yaml")
	os.Setenv("RBLN_CTK_CONFIG", "/env/config.yaml")
	defer os.Unsetenv("RBLN_CTK_CONFIG")

	// When
	path := getConfigPath()

	// Then
	assert.Equal(t, "/viper/config.yaml", path)
}

func TestRunInfo_OutputContainsAllSections(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("format", "table")
	viper.Set("debug", false)
	viper.Set("config", "")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// When
	err := runInfo(cmd, []string{})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Then
	require.NoError(t, err)
	sections := []string{
		"RBLN Container Toolkit",
		"System:",
		"RBLN Driver:",
		"Configuration:",
	}
	for _, section := range sections {
		assert.Contains(t, output, section, "Output should contain section: %s", section)
	}
}
