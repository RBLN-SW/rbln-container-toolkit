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

// toolDiscoverer implements ToolDiscoverer interface.
type toolDiscoverer struct {
	cfg *config.Config
}

// NewToolDiscoverer creates a new tool discoverer.
func NewToolDiscoverer(cfg *config.Config) ToolDiscoverer {
	return &toolDiscoverer{cfg: cfg}
}

// Discover finds the configured tools in search paths.
func (d *toolDiscoverer) Discover() ([]Tool, error) {
	var tools []Tool
	found := make(map[string]bool)

	for _, toolName := range d.cfg.Tools {
		if found[toolName] {
			continue
		}

		tool := d.findTool(toolName)
		if tool != nil {
			tools = append(tools, *tool)
			found[toolName] = true
		}
	}

	return tools, nil
}

// findTool searches for a tool in the configured binary paths.
func (d *toolDiscoverer) findTool(name string) *Tool {
	searchRoot := d.getSearchRoot()
	for _, binPath := range d.cfg.SearchPaths.Binaries {
		fullPath := filepath.Join(searchRoot, binPath, name)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			if info.Mode()&0o111 != 0 {
				hostPath := d.toHostPath(fullPath, searchRoot)
				return &Tool{
					Name:          name,
					Path:          hostPath,
					ContainerPath: d.toContainerPath(hostPath),
				}
			}
		}
	}
	return nil
}

func (d *toolDiscoverer) getSearchRoot() string {
	if d.cfg.SearchRoot != "" {
		return d.cfg.SearchRoot
	}
	return d.cfg.DriverRoot
}

// toContainerPath converts a host path to the container-visible path.
// This strips the driver-root prefix if present.
func (d *toolDiscoverer) toContainerPath(hostPath string) string {
	// If driver root is "/" (default), container path equals host path
	if d.cfg.DriverRoot == "/" || d.cfg.DriverRoot == "" {
		return hostPath
	}

	// Strip the driver root from the host path to get container path
	// e.g., /run/rbln/driver/usr/bin/rbln-smi -> /usr/bin/rbln-smi
	containerPath := strings.TrimPrefix(hostPath, d.cfg.DriverRoot)

	// Ensure it starts with /
	if !strings.HasPrefix(containerPath, "/") {
		containerPath = "/" + containerPath
	}

	return containerPath
}

func (d *toolDiscoverer) toHostPath(path, searchRoot string) string {
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
