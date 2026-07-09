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

// getSearchRoot returns the filesystem root under which device nodes are
// globbed. Devices always live in the host's real /dev, so this is DeviceRoot
// (the host root: "/host" in a containerized daemon, "/" otherwise) and never
// the driver install directory. An empty DeviceRoot means the host root "/".
func (d *deviceDiscoverer) getSearchRoot() string {
	if d.cfg.DeviceRoot == "" {
		return "/"
	}
	return d.cfg.DeviceRoot
}

// toContainerPath converts a host path to the container-visible path. Device
// nodes are bound into the container at the same absolute path they occupy on
// the host (/dev/rblnfs0 -> /dev/rblnfs0), so this is the identity mapping.
// DriverRoot is deliberately not involved: kernel device nodes never live under
// the driver install directory.
func (d *deviceDiscoverer) toContainerPath(hostPath string) string {
	return hostPath
}

// toHostPath converts a discovered path back to its real host path by stripping
// the DeviceRoot prefix. The result is the host-absolute device path (e.g.
// /dev/rblnfs0) that a container runtime binds into workloads. DriverRoot is
// deliberately not prepended: device nodes are addressed by their real /dev
// path on the host regardless of where driver libraries were installed.
func (d *deviceDiscoverer) toHostPath(path, searchRoot string) string {
	if searchRoot == "" || searchRoot == "/" {
		return path
	}

	stripped := strings.TrimPrefix(path, searchRoot)
	if !strings.HasPrefix(stripped, "/") {
		stripped = "/" + stripped
	}
	return stripped
}
