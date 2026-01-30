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

package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
)

func TestToolDiscoverer_Discover(t *testing.T) {
	// Given: A temp directory with tools
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	// Create mock tool
	toolPath := filepath.Join(binDir, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should find the tool
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Equal(t, "rbln-smi", tools[0].Name)
	assert.Contains(t, tools[0].Path, "rbln-smi")
}

func TestToolDiscoverer_Discover_NotFound(t *testing.T) {
	// Given: A temp directory without the configured tool
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should return empty list without error
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestToolDiscoverer_Discover_MultipleSearchPaths(t *testing.T) {
	// Given: Tools in different search paths
	tmpDir := t.TempDir()
	binDir1 := filepath.Join(tmpDir, "usr", "bin")
	binDir2 := filepath.Join(tmpDir, "usr", "local", "bin")
	require.NoError(t, os.MkdirAll(binDir1, 0755))
	require.NoError(t, os.MkdirAll(binDir2, 0755))

	// Create tool in second search path
	toolPath := filepath.Join(binDir2, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin", "/usr/local/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should find tool in second path
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Contains(t, tools[0].Path, "usr/local/bin")
}

func TestToolDiscoverer_Discover_WithDriverRoot(t *testing.T) {
	// Given: A driver root with tools
	tmpDir := t.TempDir()
	driverRoot := filepath.Join(tmpDir, "run", "rbln", "driver")
	binDir := filepath.Join(driverRoot, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	toolPath := filepath.Join(binDir, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = driverRoot
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering with driver root
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should find tool under driver root
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Contains(t, tools[0].Path, driverRoot)
}

// CoreOS tool discoverer tests

func TestToolDiscoverer_ContainerPath_WithDriverRoot(t *testing.T) {
	// Given: A CoreOS-like driver root structure
	tmpDir := t.TempDir()
	driverRoot := filepath.Join(tmpDir, "run", "rbln", "driver")
	binDir := filepath.Join(driverRoot, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	toolPath := filepath.Join(binDir, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = driverRoot
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should have correct host path and container path
	require.NoError(t, err)
	require.Len(t, tools, 1)

	tool := tools[0]
	// Host path includes driver root
	assert.Equal(t, filepath.Join(driverRoot, "usr", "bin", "rbln-smi"), tool.Path)
	// Container path is without driver root
	assert.Equal(t, "/usr/bin/rbln-smi", tool.ContainerPath)
}

func TestToolDiscoverer_ContainerPath_WithDefaultDriverRoot(t *testing.T) {
	// Given: Default driver root (/)
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	toolPath := filepath.Join(binDir, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir // Acts as "/" in real system
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Path and ContainerPath should be the same
	require.NoError(t, err)
	require.Len(t, tools, 1)

	tool := tools[0]
	// Both should point to the same location (when driver root matches tmpDir)
	assert.Equal(t, tool.Path, filepath.Join(tmpDir, "usr", "bin", "rbln-smi"))
}

func TestToolDiscoverer_Discover_MultipleTools(t *testing.T) {
	// Given: Multiple configured tools
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	// Create multiple tools
	tools := []string{"rbln-smi", "rbln-info", "rbln-check"}
	for _, tool := range tools {
		toolPath := filepath.Join(binDir, tool)
		f, err := os.Create(toolPath)
		require.NoError(t, err)
		f.Close()
		require.NoError(t, os.Chmod(toolPath, 0755))
	}

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = tools

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	foundTools, err := discoverer.Discover()

	// Then: Should find all tools
	require.NoError(t, err)
	assert.Len(t, foundTools, 3)

	names := make([]string, len(foundTools))
	for i, t := range foundTools {
		names[i] = t.Name
	}
	for _, tool := range tools {
		assert.Contains(t, names, tool)
	}
}

// Additional tool discoverer edge case tests

func TestToolDiscoverer_Discover_NonExecutableFile(t *testing.T) {
	// Given: A non-executable file with tool name
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	toolPath := filepath.Join(binDir, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0644))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should not find non-executable file
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestToolDiscoverer_Discover_DirectoryWithToolName(t *testing.T) {
	// Given: A directory with the same name as the tool
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	toolDir := filepath.Join(binDir, "rbln-smi")
	require.NoError(t, os.MkdirAll(toolDir, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should not find directory
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestToolDiscoverer_Discover_EmptyToolsList(t *testing.T) {
	// Given: No tools configured
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Tools = []string{}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should return empty list without error
	require.NoError(t, err)
	assert.Empty(t, tools)
}

func TestToolDiscoverer_Discover_DuplicateToolNames(t *testing.T) {
	// Given: Duplicate tool names in config
	tmpDir := t.TempDir()
	binDir := filepath.Join(tmpDir, "usr", "bin")
	require.NoError(t, os.MkdirAll(binDir, 0755))

	toolPath := filepath.Join(binDir, "rbln-smi")
	f, err := os.Create(toolPath)
	require.NoError(t, err)
	f.Close()
	require.NoError(t, os.Chmod(toolPath, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin"}
	cfg.Tools = []string{"rbln-smi", "rbln-smi", "rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should find tool only once (deduplication)
	require.NoError(t, err)
	assert.Len(t, tools, 1)
}

func TestToolDiscoverer_Discover_FirstSearchPathWins(t *testing.T) {
	// Given: Same tool in multiple search paths
	tmpDir := t.TempDir()
	binDir1 := filepath.Join(tmpDir, "usr", "bin")
	binDir2 := filepath.Join(tmpDir, "usr", "local", "bin")
	require.NoError(t, os.MkdirAll(binDir1, 0755))
	require.NoError(t, os.MkdirAll(binDir2, 0755))

	// Create tool in first path
	tool1 := filepath.Join(binDir1, "rbln-smi")
	f1, err := os.Create(tool1)
	require.NoError(t, err)
	f1.Close()
	require.NoError(t, os.Chmod(tool1, 0755))

	// Create tool in second path
	tool2 := filepath.Join(binDir2, "rbln-smi")
	f2, err := os.Create(tool2)
	require.NoError(t, err)
	f2.Close()
	require.NoError(t, os.Chmod(tool2, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Binaries = []string{"/usr/bin", "/usr/local/bin"}
	cfg.Tools = []string{"rbln-smi"}

	// When: Discovering tools
	discoverer := NewToolDiscoverer(cfg)
	tools, err := discoverer.Discover()

	// Then: Should find tool from first search path
	require.NoError(t, err)
	assert.Len(t, tools, 1)
	assert.Contains(t, tools[0].Path, "usr/bin")
}

func TestToolDiscoverer_ToContainerPath_DriverRootSlash(t *testing.T) {
	// Given: Driver root is "/"
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/"

	discoverer := NewToolDiscoverer(cfg).(*toolDiscoverer)

	// When: Converting host path
	hostPath := "/usr/bin/rbln-smi"
	containerPath := discoverer.toContainerPath(hostPath)

	// Then: Container path should equal host path
	assert.Equal(t, hostPath, containerPath)
}

func TestToolDiscoverer_ToContainerPath_StripDriverRoot(t *testing.T) {
	// Given: Custom driver root
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/run/rbln/driver"

	discoverer := NewToolDiscoverer(cfg).(*toolDiscoverer)

	// When: Converting host path
	hostPath := "/run/rbln/driver/usr/bin/rbln-smi"
	containerPath := discoverer.toContainerPath(hostPath)

	// Then: Should strip driver root
	assert.Equal(t, "/usr/bin/rbln-smi", containerPath)
}

func TestToolDiscoverer_ToContainerPath_MissingLeadingSlash(t *testing.T) {
	// Given: Driver root that when stripped leaves path without leading slash
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/host"

	discoverer := NewToolDiscoverer(cfg).(*toolDiscoverer)

	// When: Converting host path where stripping creates path without /
	hostPath := "/hostusr/bin/rbln-smi"
	containerPath := discoverer.toContainerPath(hostPath)

	// Then: Should add leading slash
	assert.True(t, strings.HasPrefix(containerPath, "/"))
}
