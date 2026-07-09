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
	cfg.DeviceRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*", "/dev/rsd*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: paths are the real host paths with DeviceRoot stripped.
	require.NoError(t, err)
	assert.Len(t, devices, 3)
	assert.Equal(t, "/dev/rbln0", devices[0].Path)
	assert.Equal(t, "/dev/rbln1", devices[1].Path)
	assert.Equal(t, "/dev/rsd0", devices[2].Path)
}

func TestDeviceDiscoverer_Discover_NoDevices(t *testing.T) {
	// Given: A temp directory with no matching devices
	tmpDir := t.TempDir()
	devDir := filepath.Join(tmpDir, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	cfg := config.DefaultConfig()
	cfg.DeviceRoot = tmpDir
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
	cfg.DeviceRoot = tmpDir
	cfg.Devices.Patterns = []string{"/dev/rbln*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: Should only find the file, not the directory
	require.NoError(t, err)
	assert.Len(t, devices, 1)
	assert.Equal(t, "/dev/rbln0", devices[0].Path)
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
	cfg.DeviceRoot = tmpDir
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
	cfg.DeviceRoot = tmpDir
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

func TestDeviceDiscoverer_Discover_DriverContainer(t *testing.T) {
	// Regression: on driver-container deployments the daemon sets
	// DeviceRoot=hostRoot (e.g. /host) and DriverRoot=/run/rbln/driver. Kernel
	// device nodes live in the host's real /dev (hostRoot/dev), NOT under the
	// driver install dir, so discovery must root at DeviceRoot and emit the real
	// host path — never re-rooted under DriverRoot.
	hostRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(hostRoot, "dev"), 0755))
	f, err := os.Create(filepath.Join(hostRoot, "dev", "rblnfs0"))
	require.NoError(t, err)
	f.Close()

	// Decoy under the driver install dir: if discovery wrongly used
	// SearchRoot/DriverRoot it would pick this up instead.
	driverDev := filepath.Join(hostRoot, "run", "rbln", "driver", "dev")
	require.NoError(t, os.MkdirAll(driverDev, 0755))
	decoy, err := os.Create(filepath.Join(driverDev, "rblnfs9"))
	require.NoError(t, err)
	decoy.Close()

	cfg := config.DefaultConfig()
	cfg.DeviceRoot = hostRoot
	cfg.SearchRoot = filepath.Join(hostRoot, "run", "rbln", "driver") // libs/tools root
	cfg.DriverRoot = "/run/rbln/driver"
	cfg.Devices.Patterns = []string{"/dev/rblnfs*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: only the real /dev node is found, with the host-absolute path.
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, "/dev/rblnfs0", devices[0].Path)
	assert.Equal(t, "/dev/rblnfs0", devices[0].ContainerPath)
}

func TestDeviceDiscoverer_Discover_IgnoresDriverAndSearchRoot(t *testing.T) {
	// SearchRoot/DriverRoot are for driver-installed libraries and tools; they
	// must not influence device discovery. Devices resolve purely from
	// DeviceRoot, and their host path is DeviceRoot-relative (real /dev path).
	deviceRoot := t.TempDir()
	devDir := filepath.Join(deviceRoot, "dev")
	require.NoError(t, os.MkdirAll(devDir, 0755))

	f, err := os.Create(filepath.Join(devDir, "rbln0"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DeviceRoot = deviceRoot
	cfg.SearchRoot = "/some/other/search/root"
	cfg.DriverRoot = "/run/rbln/driver"
	cfg.Devices.Patterns = []string{"/dev/rbln*"}

	// When
	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	// Then: found under DeviceRoot, emitted as the real host path.
	require.NoError(t, err)
	require.Len(t, devices, 1)
	assert.Equal(t, "/dev/rbln0", devices[0].Path)
	assert.Equal(t, "/dev/rbln0", devices[0].ContainerPath)
}

func TestDeviceDiscoverer_Discover_BareMetalDefaultRoot(t *testing.T) {
	// Empty DeviceRoot means the host root "/": device paths are returned as-is
	// and DriverRoot (a --driver-root override for libraries) is not applied to
	// device nodes.
	d := &deviceDiscoverer{cfg: &config.Config{DriverRoot: "/run/rbln/driver"}}
	assert.Equal(t, "/", d.getSearchRoot())
	assert.Equal(t, "/dev/rbln0", d.toHostPath("/dev/rbln0", "/"))
	assert.Equal(t, "/dev/rbln0", d.toContainerPath("/dev/rbln0"))
}

func TestDeviceDiscoverer_Discover_EmptyPatterns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Devices.Patterns = []string{}

	discoverer := NewDeviceDiscoverer(cfg)
	devices, err := discoverer.Discover()

	require.NoError(t, err)
	assert.Empty(t, devices)
}
