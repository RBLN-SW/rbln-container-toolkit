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

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
)

// libraryDiscoverer implements LibraryDiscoverer interface.
type libraryDiscoverer struct {
	cfg       *config.Config
	cache     LDCache
	lddRunner LDDRunner // For testing; if nil, NewLDDRunner() is used
}

// NewLibraryDiscoverer creates a new library discoverer.
func NewLibraryDiscoverer(cfg *config.Config) LibraryDiscoverer {
	// Create fallback cache using search paths
	cache := NewFallbackLDCache(cfg.DriverRoot, cfg.SearchPaths.Libraries)

	return &libraryDiscoverer{
		cfg:   cfg,
		cache: cache,
	}
}

// DiscoverRBLN discovers RBLN libraries matching configured patterns.
func (d *libraryDiscoverer) DiscoverRBLN() ([]Library, error) {
	var libraries []Library
	seen := make(map[string]bool)

	for _, pattern := range d.cfg.Libraries.Patterns {
		for _, searchPath := range d.cfg.SearchPaths.Libraries {
			searchRoot := d.cfg.SearchRoot
			if searchRoot == "" {
				searchRoot = d.cfg.DriverRoot
			}
			fullPath := filepath.Join(searchRoot, searchPath)
			matches, err := filepath.Glob(filepath.Join(fullPath, pattern))
			if err != nil {
				continue
			}

			for _, match := range matches {
				info, err := os.Lstat(match)
				if err != nil || info.IsDir() {
					continue
				}

				if info.Mode()&os.ModeSymlink != 0 {
					continue
				}

				realPath, err := filepath.EvalSymlinks(match)
				if err != nil {
					realPath = match
				}
				if seen[realPath] {
					continue
				}
				seen[realPath] = true

				hostPath := d.toHostPath(match, searchRoot)
				lib := Library{
					Name:          filepath.Base(match),
					Path:          hostPath,
					ContainerPath: d.toContainerPath(hostPath, searchPath),
					Type:          LibraryTypeRBLN,
				}

				libraries = append(libraries, lib)
			}
		}
	}

	return libraries, nil
}

// toContainerPath converts a host path to the container-visible path.
// This strips the driver-root prefix if present.
func (d *libraryDiscoverer) toContainerPath(hostPath, _ string) string {
	// If ContainerPath is set (isolation mode), use getContainerPath
	if d.cfg.Libraries.ContainerPath != "" {
		return d.getContainerPath(hostPath)
	}

	// If driver root is "/" (default), container path equals host path
	if d.cfg.DriverRoot == "/" || d.cfg.DriverRoot == "" {
		return hostPath
	}

	// Strip the driver root from the host path to get container path
	// e.g., /run/rbln/driver/usr/lib64/librbln-ml.so -> /usr/lib64/librbln-ml.so
	containerPath := strings.TrimPrefix(hostPath, d.cfg.DriverRoot)

	// Ensure it starts with /
	if !strings.HasPrefix(containerPath, "/") {
		containerPath = "/" + containerPath
	}

	return containerPath
}

// toHostPath converts a filesystem path (found via SearchRoot) to a CDI hostPath.
// It replaces SearchRoot with DriverRoot so the resulting path is valid on the actual host.
// e.g., /host/tmp/driver/usr/lib/lib.so -> /tmp/driver/usr/lib/lib.so
func (d *libraryDiscoverer) toHostPath(path, searchRoot string) string {
	if searchRoot == "" || searchRoot == "/" {
		return path
	}

	stripped := strings.TrimPrefix(path, searchRoot)
	if !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}

	if d.cfg.DriverRoot != "" && d.cfg.DriverRoot != "/" {
		return filepath.Join(d.cfg.DriverRoot, stripped)
	}

	return stripped
}

func (d *libraryDiscoverer) getSearchRoot() string {
	if d.cfg.SearchRoot != "" {
		return d.cfg.SearchRoot
	}
	return d.cfg.DriverRoot
}

func (d *libraryDiscoverer) toSearchRootPath(driverRootPath string) string {
	if d.cfg.SearchRoot == "" || d.cfg.SearchRoot == d.cfg.DriverRoot {
		return driverRootPath
	}
	relative := strings.TrimPrefix(driverRootPath, d.cfg.DriverRoot)
	if !strings.HasPrefix(relative, "/") {
		relative = "/" + relative
	}
	return filepath.Join(d.cfg.SearchRoot, relative)
}

func (d *libraryDiscoverer) toDriverRootPath(searchRootPath string) string {
	searchRoot := d.getSearchRoot()

	// Case 1: No special root handling needed (default install, DriverRoot is "/" or empty)
	if (searchRoot == "" || searchRoot == "/") && (d.cfg.DriverRoot == "" || d.cfg.DriverRoot == "/") {
		return searchRootPath
	}

	// Case 2: SearchRoot is explicitly set and differs from DriverRoot (containerized daemon)
	// e.g., SearchRoot=/host/tmp/driver, DriverRoot=/tmp/driver
	// Path from resolver: /host/tmp/driver/usr/lib/lib.so -> /tmp/driver/usr/lib/lib.so
	if d.cfg.SearchRoot != "" && d.cfg.SearchRoot != d.cfg.DriverRoot {
		relative := strings.TrimPrefix(searchRootPath, searchRoot)
		if !strings.HasPrefix(relative, "/") {
			relative = "/" + relative
		}
		if d.cfg.DriverRoot != "" && d.cfg.DriverRoot != "/" {
			return filepath.Join(d.cfg.DriverRoot, relative)
		}
		return relative
	}

	// Case 3: SearchRoot not set, but DriverRoot is set (host daemon with driver root)
	// e.g., DriverRoot=/run/rbln/driver
	// Path from resolver is relative: /usr/lib/lib.so -> /run/rbln/driver/usr/lib/lib.so
	if d.cfg.DriverRoot != "" && d.cfg.DriverRoot != "/" {
		// If path already starts with DriverRoot, don't double-prefix
		if strings.HasPrefix(searchRootPath, d.cfg.DriverRoot) {
			return searchRootPath
		}
		return filepath.Join(d.cfg.DriverRoot, searchRootPath)
	}

	return searchRootPath
}

// getContainerPath returns the container path for a library.
// In isolation mode, returns ContainerPath/basename.
// In default mode, returns the host path unchanged.
func (d *libraryDiscoverer) getContainerPath(hostPath string) string {
	if d.cfg.Libraries.ContainerPath == "" {
		return hostPath
	}
	return filepath.Join(d.cfg.Libraries.ContainerPath, filepath.Base(hostPath))
}

// getPluginContainerPath returns the container path for a plugin library.
// In isolation mode, returns ContainerPath/subdir/basename to preserve plugin directory structure.
// In default mode, returns the driver-root-adjusted path.
func (d *libraryDiscoverer) getPluginContainerPath(hostPath, subdir string) string {
	if d.cfg.Libraries.ContainerPath != "" {
		return filepath.Join(d.cfg.Libraries.ContainerPath, subdir, filepath.Base(hostPath))
	}

	// Default mode: adjust for driver root
	if d.cfg.DriverRoot == "/" || d.cfg.DriverRoot == "" {
		return hostPath
	}

	containerPath := strings.TrimPrefix(hostPath, d.cfg.DriverRoot)
	if !strings.HasPrefix(containerPath, "/") {
		containerPath = "/" + containerPath
	}
	return containerPath
}

// DiscoverDependencies discovers dependencies of the given libraries using ldd.
// It runs ldd on each library and extracts resolved dependency paths,
// excluding glibc libraries defined in config.
func (d *libraryDiscoverer) DiscoverDependencies(libs []Library) ([]Library, error) {
	var dependencies []Library
	seen := make(map[string]bool)

	for _, lib := range libs {
		seen[lib.Path] = true
		if lib.RealPath != "" {
			seen[lib.RealPath] = true
		}
	}

	searchRoot := d.getSearchRoot()
	lddRunner := d.lddRunner
	if lddRunner == nil {
		lddRunner = NewELFResolver(searchRoot, d.cfg.SearchPaths.Libraries)
	}

	for _, lib := range libs {
		libPathForRead := d.toSearchRootPath(lib.Path)
		deps, err := lddRunner.Run(libPathForRead)
		if err != nil {
			continue
		}

		for _, depPath := range deps {
			hostPath := d.toDriverRootPath(depPath)
			if seen[hostPath] {
				continue
			}
			seen[hostPath] = true

			name := filepath.Base(depPath)
			if d.isGlibc(name) {
				continue
			}

			dependencies = append(dependencies, Library{
				Name:          name,
				Path:          hostPath,
				ContainerPath: d.toContainerPath(hostPath, ""),
				Type:          LibraryTypeDependency,
			})
		}
	}

	return dependencies, nil
}

// DiscoverPlugins discovers dlopen plugin libraries.
func (d *libraryDiscoverer) DiscoverPlugins() ([]Library, error) {
	var plugins []Library
	seen := make(map[string]bool)

	searchRoot := d.getSearchRoot()
	for _, pluginPath := range d.cfg.Libraries.PluginPaths {
		fullPath := filepath.Join(searchRoot, pluginPath)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			name := entry.Name()
			if !strings.HasSuffix(name, ".so") && !strings.Contains(name, ".so.") {
				continue
			}

			filePath := filepath.Join(fullPath, name)
			realPath, err := filepath.EvalSymlinks(filePath)
			if err != nil {
				realPath = filePath
			}
			if seen[realPath] {
				continue
			}
			seen[realPath] = true

			pluginSubdir := filepath.Base(pluginPath)
			hostPath := d.toHostPath(filePath, searchRoot)

			plugins = append(plugins, Library{
				Name:          name,
				Path:          hostPath,
				ContainerPath: d.getPluginContainerPath(hostPath, pluginSubdir),
				Type:          LibraryTypePlugin,
			})
		}
	}

	return plugins, nil
}

// isGlibc checks if the library name matches glibc exclude patterns.
func (d *libraryDiscoverer) isGlibc(name string) bool {
	for _, pattern := range d.cfg.GlibcExclude {
		matched, err := filepath.Match(pattern, name)
		if err == nil && matched {
			return true
		}
	}
	return false
}
