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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
)

func TestDeviceDiscoverer_Discover(t *testing.T) {
	// Given: A temp directory with device nodes
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	// Create mock device files
	for _, name := range []string{"rbln0", "rbln1", "rsd0"} {
		f, err := os.Create(filepath.Join(devDir, name))
		require.NoError(t, err)
		f.Close()
	}

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*", "/dev/rsd*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then
	require.NoError(t, err)
	assert.Len(t, devices, 3)
	assert.Equal(t, filepath.Join(tmpDir, "dev", "rbln0"), devices[0].Path)
	assert.Equal(t, filepath.Join(tmpDir, "dev", "rbln1"), devices[1].Path)
	assert.Equal(t, filepath.Join(tmpDir, "dev", "rsd0"), devices[2].Path)
}

func TestDeviceDiscoverer_Discover_NoDevices(t *testing.T) {
	// Given: A temp directory with no matching devices
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*", "/dev/rsd*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then
	require.NoError(t, err)
	assert.Empty(t, devices)
}

func TestDeviceDiscoverer_Discover_SkipsDirectories(t *testing.T) {
	// Given: A directory matching the glob pattern
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(filepath.Join(devDir, "rbln_dir"), 0755))

	// Also create a real file
	f, err := os.Create(filepath.Join(devDir, "rbln0"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: Should only find the file, not the directory
	require.NoError(t, err)
	assert.Len(t, devices, 1)
	assert.Equal(t, filepath.Join(tmpDir, "dev", "rbln0"), devices[0].Path)
}

func TestDeviceDiscoverer_Discover_Deduplication(t *testing.T) {
	// Given: Overlapping patterns that match the same file
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	f, err := os.Create(filepath.Join(devDir, "rbln0"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*", "/dev/rbln0"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: Should deduplicate
	require.NoError(t, err)
	assert.Len(t, devices, 1)
}

func TestDeviceDiscoverer_Discover_Sorted(t *testing.T) {
	// Given: Devices that would be unsorted
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	for _, name := range []string{"rbln2", "rbln0", "rbln1"} {
		f, err := os.Create(filepath.Join(devDir, name))
		require.NoError(t, err)
		f.Close()
	}

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: Should be sorted by path
	require.NoError(t, err)
	require.Len(t, devices, 3)
	assert.Contains(t, devices[0].Path, "rbln0")
	assert.Contains(t, devices[1].Path, "rbln1")
	assert.Contains(t, devices[2].Path, "rbln2")
}

func TestDeviceDiscoverer_Discover_WithDriverRoot(t *testing.T) {
	// Given: A driver root structure
	tmpDir := t.TempDir()
	driverRoot := filepath.Join(tmpDir, "run", "rbln", "driver")
	devDir := filepath.Join(driverRoot, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	f, err := os.Create(filepath.Join(devDir, "rbln0"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = driverRoot
	cfg.Devices.Patterns = []string{"/dev/rbln*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: Host path includes driver root, container path doesn't
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, filepath.Join(driverRoot, "dev", "rbln0"), devices[0].Path)
	assert.Equal(t, "/dev/rbln0", devices[0].ContainerPath)
}

func TestDeviceDiscoverer_Discover_WithSearchRoot(t *testing.T) {
	// Given: SearchRoot differs from DriverRoot
	searchDir := t.TempDir()
	devDir := filepath.Join(searchDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	f, err := os.Create(filepath.Join(devDir, "rbln0"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.SearchRoot = searchDir
	cfg.DriverRoot = "/run/rbln/driver"
	cfg.Devices.Patterns = []string{"/dev/rbln*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: Host path uses DriverRoot, container path strips it
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, "/run/rbln/driver/dev/rbln0", devices[0].Path)
	assert.Equal(t, "/dev/rbln0", devices[0].ContainerPath)
}

func TestDeviceDiscoverer_Discover_EmptyPatterns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Devices.Patterns = []string{}

	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	require.NoError(t, err)
	assert.Empty(t, devices)
}
