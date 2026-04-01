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
	"sort"
	"strings"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
)

// deviceDiscoverer implements DeviceDiscoverer interface.
type deviceDiscoverer struct {
	cfg *config.Config
}

// NewDeviceDiscoverer creates a new device node discoverer.
func NewDeviceDiscoverer(cfg *config.Config) DeviceDiscoverer {
	return &deviceDiscoverer{cfg: cfg}
}

// Discover finds device nodes matching the configured glob patterns.
func (d *deviceDiscoverer) Discover() ([]Device, error) {
	found := make(map[string]bool)
	var devices []Device

	searchRoot := d.getSearchRoot()

	for _, pattern := range d.cfg.Devices.Patterns {
		searchPattern := filepath.Join(searchRoot, pattern)
		matches, err := filepath.Glob(searchPattern)
		if err != nil {
			return nil, err
		}

		for _, match := range matches {
			info, err := os.Lstat(match)
			if err != nil {
				continue
			}
			// Skip directories
			if info.IsDir() {
				continue
			}

			hostPath := d.toHostPath(match, searchRoot)
			if found[hostPath] {
				continue
			}
			found[hostPath] = true

			devices = append(devices, Device{
				Path:          hostPath,
				ContainerPath: d.toContainerPath(hostPath),
			})
		}
	}

	sort.Slice(devices, func(i, j int) bool {
		return devices[i].Path < devices[j].Path
	})

	return devices, nil
}

func (d *deviceDiscoverer) getSearchRoot() string {
	if d.cfg.SearchRoot != "" {
		return d.cfg.SearchRoot
	}
	return d.cfg.DriverRoot
}

// toContainerPath converts a host path to the container-visible path.
func (d *deviceDiscoverer) toContainerPath(hostPath string) string {
	if d.cfg.DriverRoot == "/" || d.cfg.DriverRoot == "" {
		return hostPath
	}

	containerPath := strings.TrimPrefix(hostPath, d.cfg.DriverRoot)
	if !strings.HasPrefix(containerPath, "/") {
		containerPath = "/" + containerPath
	}
	return containerPath
}

// toHostPath converts a discovered file path to the host-visible path.
//
// Case 1: No search root (empty or "/") — path is already absolute on host.
// Case 2: Path already has DriverRoot prefix — return as-is (SearchRoot == DriverRoot).
// Case 3: SearchRoot != DriverRoot — strip search root, prepend driver root.
func (d *deviceDiscoverer) toHostPath(path, searchRoot string) string {
	if searchRoot == "" || searchRoot == "/" {
		return path
	}

	stripped := strings.TrimPrefix(path, searchRoot)
	if !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}

	if d.cfg.DriverRoot != "" && d.cfg.DriverRoot != "/" {
		if strings.HasPrefix(path, d.cfg.DriverRoot) {
			return path
		}
		return filepath.Join(d.cfg.DriverRoot, stripped)
	}

	return stripped
}
