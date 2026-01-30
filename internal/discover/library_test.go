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
	"github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

func TestLibraryDiscoverer_DiscoverRBLN(t *testing.T) {
	// Given: A temp directory with RBLN libraries
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create mock RBLN libraries
	rblnLibs := []string{"librbln-ml.so", "librbln-thunk.so", "librbln-ccl.so"}
	for _, lib := range rblnLibs {
		f, err := os.Create(filepath.Join(libDir, lib))
		require.NoError(t, err)
		f.Close()
	}

	// Create non-RBLN library
	f, err := os.Create(filepath.Join(libDir, "libc.so.6"))
	require.NoError(t, err)
	f.Close()

	// Setup config
	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Libraries = []string{"/usr/lib64"}

	// When: Discovering RBLN libraries
	discoverer := NewLibraryDiscoverer(cfg)
	libs, err := discoverer.DiscoverRBLN()

	// Then: Should find only RBLN libraries
	require.NoError(t, err)
	assert.Len(t, libs, 3)

	names := make([]string, len(libs))
	for i, lib := range libs {
		names[i] = lib.Name
		assert.Equal(t, LibraryTypeRBLN, lib.Type)
	}
	assert.Contains(t, names, "librbln-ml.so")
	assert.Contains(t, names, "librbln-thunk.so")
	assert.Contains(t, names, "librbln-ccl.so")
}

func TestLibraryDiscoverer_DiscoverRBLN_NoLibraries(t *testing.T) {
	// Given: A temp directory without RBLN libraries
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create only non-RBLN library
	f, err := os.Create(filepath.Join(libDir, "libc.so.6"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Libraries = []string{"/usr/lib64"}

	// When: Discovering RBLN libraries
	discoverer := NewLibraryDiscoverer(cfg)
	libs, err := discoverer.DiscoverRBLN()

	// Then: Should return empty list without error
	require.NoError(t, err)
	assert.Empty(t, libs)
}

func TestLibraryDiscoverer_IsGlibc(t *testing.T) {
	cfg := config.DefaultConfig()
	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	tests := []struct {
		name     string
		libName  string
		expected bool
	}{
		{"libc is glibc", "libc.so.6", true},
		{"libm is glibc", "libm.so.6", true},
		{"libpthread is glibc", "libpthread.so.0", true},
		{"libdl is glibc", "libdl.so.2", true},
		{"ld-linux is glibc", "ld-linux-x86-64.so.2", true},
		{"libgcc_s is excluded", "libgcc_s.so.1", true},
		{"libstdc++ is excluded", "libstdc++.so.6", true},
		{"librbln is not glibc", "librbln-ml.so", false},
		{"libibverbs is not glibc", "libibverbs.so.1", false},
		{"libbz2 is not glibc", "libbz2.so.1.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := discoverer.isGlibc(tt.libName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// CoreOS driver-root path handling tests

func TestLibraryDiscoverer_DriverRoot_CoreOS(t *testing.T) {
	// Given: CoreOS-style driver root at /run/rbln/driver
	tmpDir := t.TempDir()
	driverRoot := filepath.Join(tmpDir, "run", "rbln", "driver")

	// Create CoreOS-like structure with multiple lib paths
	libPaths := []string{
		filepath.Join(driverRoot, "usr", "lib64"),
		filepath.Join(driverRoot, "usr", "lib", "x86_64-linux-gnu"),
	}

	for _, libPath := range libPaths {
		require.NoError(t, os.MkdirAll(libPath, 0755))
	}

	// Create RBLN libraries in different locations
	libs := map[string]string{
		"librbln-ml.so":    libPaths[0],
		"librbln-thunk.so": libPaths[1],
	}

	for lib, path := range libs {
		f, err := os.Create(filepath.Join(path, lib))
		require.NoError(t, err)
		f.Close()
	}

	cfg := config.DefaultConfig()
	cfg.DriverRoot = driverRoot
	cfg.SearchPaths.Libraries = []string{"/usr/lib64", "/usr/lib/x86_64-linux-gnu"}

	// When: Discovering with CoreOS driver root
	discoverer := NewLibraryDiscoverer(cfg)
	result, err := discoverer.DiscoverRBLN()

	// Then: Should find libraries from both paths under driver root
	require.NoError(t, err)
	assert.Len(t, result, 2)

	// All paths should be under driver root
	for _, lib := range result {
		assert.True(t, strings.HasPrefix(lib.Path, driverRoot),
			"Library path %s should be under driver root %s", lib.Path, driverRoot)
	}
}

func TestLibraryDiscoverer_DriverRoot_ContainerPath(t *testing.T) {
	// Given: Libraries mounted from driver container to host path
	// In CoreOS, driver container mounts to /run/rbln/driver
	// Container sees /usr/lib64/librbln-ml.so
	// Host sees /run/rbln/driver/usr/lib64/librbln-ml.so

	tmpDir := t.TempDir()
	driverRoot := filepath.Join(tmpDir, "run", "rbln", "driver")
	containerLibDir := filepath.Join(driverRoot, "usr", "lib64")
	require.NoError(t, os.MkdirAll(containerLibDir, 0755))

	// Create library in driver container mount
	f, err := os.Create(filepath.Join(containerLibDir, "librbln-ml.so"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = driverRoot
	cfg.SearchPaths.Libraries = []string{"/usr/lib64"}

	// When: Discovering libraries
	discoverer := NewLibraryDiscoverer(cfg)
	libs, err := discoverer.DiscoverRBLN()

	// Then: Host path should point to driver root location
	// Container path should be relative (for CDI spec)
	require.NoError(t, err)
	require.Len(t, libs, 1)

	lib := libs[0]
	assert.Equal(t, filepath.Join(containerLibDir, "librbln-ml.so"), lib.Path)
	// ContainerPath should be the path as seen inside container
	assert.Equal(t, "/usr/lib64/librbln-ml.so", lib.ContainerPath)
}

func TestLibraryDiscoverer_DriverRoot_PluginsUnderDriverRoot(t *testing.T) {
	// Given: Plugin libraries under driver root (CoreOS scenario)
	tmpDir := t.TempDir()
	driverRoot := filepath.Join(tmpDir, "run", "rbln", "driver")
	pluginDir := filepath.Join(driverRoot, "usr", "lib64", "libibverbs")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	// Create plugin files
	f, err := os.Create(filepath.Join(pluginDir, "libmlx5.so"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = driverRoot
	cfg.Libraries.PluginPaths = []string{"/usr/lib64/libibverbs"}

	// When: Discovering plugins with driver root
	discoverer := NewLibraryDiscoverer(cfg)
	plugins, err := discoverer.DiscoverPlugins()

	// Then: Should find plugins under driver root
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.True(t, strings.HasPrefix(plugins[0].Path, driverRoot))
	assert.Equal(t, "/usr/lib64/libibverbs/libmlx5.so", plugins[0].ContainerPath)
}

// getContainerPath method tests

func TestLibraryDiscoverer_GetContainerPath_DefaultMode(t *testing.T) {
	// Given: ContainerPath not set (default mode)
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "" // Empty = default mode

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	// When: Getting container path for a library
	hostPath := "/usr/lib64/librbln-ml.so"
	containerPath := discoverer.getContainerPath(hostPath)

	// Then: Container path should equal host path
	assert.Equal(t, hostPath, containerPath)
}

func TestLibraryDiscoverer_GetContainerPath_IsolationMode(t *testing.T) {
	// Given: ContainerPath set (isolation mode)
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	// When: Getting container path for a library
	hostPath := "/usr/lib64/librbln-ml.so"
	containerPath := discoverer.getContainerPath(hostPath)

	// Then: Container path should be under isolation path
	assert.Equal(t, "/usr/lib64/rbln/librbln-ml.so", containerPath)
}

func TestLibraryDiscoverer_GetContainerPath_IsolationMode_DifferentSourceDirs(t *testing.T) {
	// Given: ContainerPath set (isolation mode)
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	tests := []struct {
		name         string
		hostPath     string
		expectedPath string
	}{
		{
			name:         "lib64 path",
			hostPath:     "/usr/lib64/librbln-ml.so",
			expectedPath: "/usr/lib64/rbln/librbln-ml.so",
		},
		{
			name:         "debian-style path",
			hostPath:     "/lib/x86_64-linux-gnu/libbz2.so.1.0",
			expectedPath: "/usr/lib64/rbln/libbz2.so.1.0",
		},
		{
			name:         "deep path",
			hostPath:     "/opt/rbln/lib/librbln-ccl.so",
			expectedPath: "/usr/lib64/rbln/librbln-ccl.so",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			containerPath := discoverer.getContainerPath(tt.hostPath)
			assert.Equal(t, tt.expectedPath, containerPath)
		})
	}
}

func TestLibraryDiscoverer_GetContainerPath_Plugins_IsolationMode(t *testing.T) {
	// Given: ContainerPath set (isolation mode)
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	// When: Getting container path for a plugin library
	hostPath := "/usr/lib64/libibverbs/libmlx5.so"
	containerPath := discoverer.getPluginContainerPath(hostPath, "libibverbs")

	// Then: Plugin should be under isolation path with subdirectory
	assert.Equal(t, "/usr/lib64/rbln/libibverbs/libmlx5.so", containerPath)
}

func TestLibraryDiscoverer_DiscoverRBLN_WithContainerPath(t *testing.T) {
	// Given: A temp directory with RBLN libraries and ContainerPath set
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create mock RBLN library
	f, err := os.Create(filepath.Join(libDir, "librbln-ml.so"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Libraries = []string{"/usr/lib64"}
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"

	// When: Discovering RBLN libraries
	discoverer := NewLibraryDiscoverer(cfg)
	libs, err := discoverer.DiscoverRBLN()

	// Then: Libraries should have isolated container path
	require.NoError(t, err)
	require.Len(t, libs, 1)
	assert.Equal(t, "/usr/lib64/rbln/librbln-ml.so", libs[0].ContainerPath)
}

func TestLibraryDiscoverer_DiscoverPlugins_WithContainerPath(t *testing.T) {
	// Given: A temp directory with plugin libraries and ContainerPath set
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "usr", "lib64", "libibverbs")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	f, err := os.Create(filepath.Join(pluginDir, "libmlx5.so"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Libraries.PluginPaths = []string{"/usr/lib64/libibverbs"}
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"

	// When: Discovering plugins
	discoverer := NewLibraryDiscoverer(cfg)
	plugins, err := discoverer.DiscoverPlugins()

	// Then: Plugins should have isolated container path with subdirectory
	require.NoError(t, err)
	require.Len(t, plugins, 1)
	assert.Equal(t, "/usr/lib64/rbln/libibverbs/libmlx5.so", plugins[0].ContainerPath)
}

// Additional edge case tests for library discoverer

func TestLibraryDiscoverer_DiscoverRBLN_SkipsDirectories(t *testing.T) {
	// Given: A temp directory with a directory that matches the pattern
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	fakeLibDir := filepath.Join(libDir, "librbln-fake.so")
	require.NoError(t, os.MkdirAll(fakeLibDir, 0755))

	// Also create a real library file
	f, err := os.Create(filepath.Join(libDir, "librbln-ml.so"))
	require.NoError(t, err)
	f.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Libraries = []string{"/usr/lib64"}

	// When: Discovering RBLN libraries
	discoverer := NewLibraryDiscoverer(cfg)
	libs, err := discoverer.DiscoverRBLN()

	// Then: Should skip directories and only find files
	require.NoError(t, err)
	assert.Len(t, libs, 1)
	assert.Equal(t, "librbln-ml.so", libs[0].Name)
}

func TestLibraryDiscoverer_DiscoverRBLN_HandlesDuplicates(t *testing.T) {
	// Given: Same library in multiple search paths
	tmpDir := t.TempDir()
	lib64Dir := filepath.Join(tmpDir, "usr", "lib64")
	localLibDir := filepath.Join(tmpDir, "usr", "local", "lib64")
	require.NoError(t, os.MkdirAll(lib64Dir, 0755))
	require.NoError(t, os.MkdirAll(localLibDir, 0755))

	// Create library in first path
	f1, err := os.Create(filepath.Join(lib64Dir, "librbln-ml.so"))
	require.NoError(t, err)
	f1.Close()

	// Create library in second path
	f2, err := os.Create(filepath.Join(localLibDir, "librbln-ml.so"))
	require.NoError(t, err)
	f2.Close()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.SearchPaths.Libraries = []string{"/usr/lib64", "/usr/local/lib64"}

	// When: Discovering RBLN libraries
	discoverer := NewLibraryDiscoverer(cfg)
	libs, err := discoverer.DiscoverRBLN()

	// Then: Should find both (different paths)
	require.NoError(t, err)
	assert.Len(t, libs, 2)
}

func TestLibraryDiscoverer_DiscoverPlugins_SkipsNonSoFiles(t *testing.T) {
	// Given: A plugin directory with mixed files
	tmpDir := t.TempDir()
	pluginDir := filepath.Join(tmpDir, "usr", "lib64", "libibverbs")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))

	files := []string{
		"libmlx5.so",
		"librdmavt.so.1",
		"readme.txt",
		"config.json",
	}
	for _, file := range files {
		f, err := os.Create(filepath.Join(pluginDir, file))
		require.NoError(t, err)
		f.Close()
	}

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Libraries.PluginPaths = []string{"/usr/lib64/libibverbs"}

	// When: Discovering plugins
	discoverer := NewLibraryDiscoverer(cfg)
	plugins, err := discoverer.DiscoverPlugins()

	// Then: Should only find .so files
	require.NoError(t, err)
	assert.Len(t, plugins, 2)
}

func TestLibraryDiscoverer_DiscoverPlugins_EmptyPluginPath(t *testing.T) {
	// Given: No plugin paths configured
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Libraries.PluginPaths = []string{}

	// When: Discovering plugins
	discoverer := NewLibraryDiscoverer(cfg)
	plugins, err := discoverer.DiscoverPlugins()

	// Then: Should return empty list without error
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestLibraryDiscoverer_DiscoverPlugins_NonExistentPath(t *testing.T) {
	// Given: A non-existent plugin path
	tmpDir := t.TempDir()

	cfg := config.DefaultConfig()
	cfg.DriverRoot = tmpDir
	cfg.Libraries.PluginPaths = []string{"/nonexistent/plugins"}

	// When: Discovering plugins
	discoverer := NewLibraryDiscoverer(cfg)
	plugins, err := discoverer.DiscoverPlugins()

	// Then: Should return empty list without error
	require.NoError(t, err)
	assert.Empty(t, plugins)
}

func TestLibraryDiscoverer_ToContainerPath_EmptyDriverRoot(t *testing.T) {
	// Given: Empty driver root (same as "/")
	cfg := config.DefaultConfig()
	cfg.DriverRoot = ""
	cfg.Libraries.ContainerPath = ""

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	// When: Converting host path to container path
	hostPath := "/usr/lib64/libfoo.so"
	containerPath := discoverer.toContainerPath(hostPath, "/usr/lib64")

	// Then: Container path should equal host path
	assert.Equal(t, hostPath, containerPath)
}

func TestLibraryDiscoverer_GetPluginContainerPath_DefaultMode(t *testing.T) {
	// Given: No ContainerPath set (default mode) with non-root driver root
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/run/rbln/driver"
	cfg.Libraries.ContainerPath = ""

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	// When: Getting plugin container path
	hostPath := "/run/rbln/driver/usr/lib64/libibverbs/libmlx5.so"
	containerPath := discoverer.getPluginContainerPath(hostPath, "libibverbs")

	// Then: Should strip driver root
	assert.Equal(t, "/usr/lib64/libibverbs/libmlx5.so", containerPath)
}

func TestLibraryDiscoverer_GetPluginContainerPath_DefaultModeEmptyDriverRoot(t *testing.T) {
	// Given: No ContainerPath and empty driver root
	cfg := config.DefaultConfig()
	cfg.DriverRoot = ""
	cfg.Libraries.ContainerPath = ""

	discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)

	// When: Getting plugin container path
	hostPath := "/usr/lib64/libibverbs/libmlx5.so"
	containerPath := discoverer.getPluginContainerPath(hostPath, "libibverbs")

	// Then: Container path should equal host path
	assert.Equal(t, hostPath, containerPath)
}

func TestLibraryDiscoverer_DiscoverDependencies(t *testing.T) {
	tests := []struct {
		name          string
		inputLibs     []Library
		mockRunFunc   func(libraryPath string) ([]string, error)
		expectedCount int
		expectedNames []string
	}{
		{
			name: "success: ldd returns dependencies",
			inputLibs: []Library{
				{
					Name: "librbln-ml.so",
					Path: "/usr/lib64/librbln-ml.so",
					Type: LibraryTypeRBLN,
				},
			},
			mockRunFunc: func(libraryPath string) ([]string, error) {
				if libraryPath == "/usr/lib64/librbln-ml.so" {
					return []string{
						"/usr/lib64/libbz2.so.1.0",
						"/usr/lib64/libibverbs.so.1",
						"/lib64/libc.so.6",
					}, nil
				}
				return []string{}, nil
			},
			expectedCount: 2,
			expectedNames: []string{"libbz2.so.1.0", "libibverbs.so.1"},
		},
		{
			name: "empty result: ldd returns no dependencies",
			inputLibs: []Library{
				{
					Name: "librbln-thunk.so",
					Path: "/usr/lib64/librbln-thunk.so",
					Type: LibraryTypeRBLN,
				},
			},
			mockRunFunc: func(_ string) ([]string, error) {
				return []string{}, nil
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name: "ldd failure: ldd command errors",
			inputLibs: []Library{
				{
					Name: "librbln-ccl.so",
					Path: "/usr/lib64/librbln-ccl.so",
					Type: LibraryTypeRBLN,
				},
			},
			mockRunFunc: func(_ string) ([]string, error) {
				return nil, errors.ErrLddFailed
			},
			expectedCount: 0,
			expectedNames: []string{},
		},
		{
			name: "multiple libraries with mixed results",
			inputLibs: []Library{
				{
					Name: "librbln-ml.so",
					Path: "/usr/lib64/librbln-ml.so",
					Type: LibraryTypeRBLN,
				},
				{
					Name: "librbln-thunk.so",
					Path: "/usr/lib64/librbln-thunk.so",
					Type: LibraryTypeRBLN,
				},
			},
			mockRunFunc: func(libraryPath string) ([]string, error) {
				if libraryPath == "/usr/lib64/librbln-ml.so" {
					return []string{
						"/usr/lib64/libbz2.so.1.0",
						"/usr/lib64/libibverbs.so.1",
					}, nil
				}
				if libraryPath == "/usr/lib64/librbln-thunk.so" {
					return []string{
						"/usr/lib64/libbz2.so.1.0",
						"/usr/lib64/libfoo.so",
					}, nil
				}
				return []string{}, nil
			},
			expectedCount: 3,
			expectedNames: []string{"libbz2.so.1.0", "libibverbs.so.1", "libfoo.so"},
		},
		{
			name: "skip input libraries from dependencies",
			inputLibs: []Library{
				{
					Name: "librbln-ml.so",
					Path: "/usr/lib64/librbln-ml.so",
					Type: LibraryTypeRBLN,
				},
			},
			mockRunFunc: func(libraryPath string) ([]string, error) {
				if libraryPath == "/usr/lib64/librbln-ml.so" {
					return []string{
						"/usr/lib64/librbln-ml.so",
						"/usr/lib64/libbz2.so.1.0",
					}, nil
				}
				return []string{}, nil
			},
			expectedCount: 1,
			expectedNames: []string{"libbz2.so.1.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: A library discoverer with mocked ldd runner
			cfg := config.DefaultConfig()
			cfg.DriverRoot = ""
			cfg.Libraries.ContainerPath = ""

			discoverer := NewLibraryDiscoverer(cfg).(*libraryDiscoverer)
			discoverer.lddRunner = &LDDRunnerMock{
				RunFunc: tt.mockRunFunc,
			}

			// When: Calling DiscoverDependencies with mocked ldd
			result, err := discoverer.DiscoverDependencies(tt.inputLibs)

			// Then: Verify results match expected
			require.NoError(t, err)
			assert.Len(t, result, tt.expectedCount)

			if tt.expectedCount > 0 {
				resultNames := make([]string, len(result))
				for i, dep := range result {
					resultNames[i] = dep.Name
					assert.Equal(t, LibraryTypeDependency, dep.Type)
				}
				assert.ElementsMatch(t, tt.expectedNames, resultNames)
			}
		})
	}
}
