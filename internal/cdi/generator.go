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

// Package cdi provides CDI specification generation and validation.
package cdi

//go:generate moq -rm -fmt=goimports -stub -out generator_mock.go . Generator

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"tags.cncf.io/container-device-interface/specs-go"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

const (
	// CDIVersion is the CDI specification version.
	CDIVersion = "0.5.0"
)

// Generator generates CDI specifications.
type Generator interface {
	// Generate creates a CDI spec from discovery results.
	Generate(result *discover.DiscoveryResult) (*specs.Spec, error)
}

// generator implements Generator interface.
type generator struct {
	cfg *config.Config
}

// NewGenerator creates a new CDI generator.
func NewGenerator(cfg *config.Config) Generator {
	return &generator{cfg: cfg}
}

// Generate creates a CDI spec from discovery results.
func (g *generator) Generate(result *discover.DiscoveryResult) (*specs.Spec, error) {
	spec := &specs.Spec{
		Version: CDIVersion,
		Kind:    fmt.Sprintf("%s/%s", g.cfg.CDI.Vendor, g.cfg.CDI.Class),
	}

	// Create runtime device with container edits
	device := specs.Device{
		Name:           "runtime",
		ContainerEdits: g.createContainerEdits(result),
	}

	spec.Devices = []specs.Device{device}

	return spec, nil
}

func (g *generator) createContainerEdits(result *discover.DiscoveryResult) specs.ContainerEdits {
	edits := specs.ContainerEdits{}

	libPaths := make(map[string]bool)
	mountedPaths := make(map[string]bool)

	if result != nil {
		for _, lib := range result.Libraries {
			if lib.RealPath != "" && lib.RealPath != lib.Path {
				// Symlink library: only mount the real file (deduplicated).
				// The symlink will be recreated by create-symlinks hook.
				realContainerPath := g.getSymlinkTargetContainerPath(lib)
				if !mountedPaths[realContainerPath] {
					realMount := g.createMountWithContainerPath(lib.RealPath, realContainerPath)
					edits.Mounts = append(edits.Mounts, &realMount)
					mountedPaths[realContainerPath] = true
					libPaths[filepath.Dir(realContainerPath)] = true
				}
			} else {
				containerPath := lib.ContainerPath
				if !mountedPaths[containerPath] {
					mount := g.createLibraryMount(lib)
					edits.Mounts = append(edits.Mounts, &mount)
					mountedPaths[containerPath] = true
					libPaths[filepath.Dir(containerPath)] = true
				}
			}
		}

		for _, tool := range result.Tools {
			mount := g.createToolMount(tool)
			edits.Mounts = append(edits.Mounts, &mount)
		}

		// create-symlinks hook MUST come before update-ldcache hook
		symlinkSpecs := g.computeSymlinks(result.Libraries)
		if hook := g.createSymlinkHook(symlinkSpecs); hook != nil {
			edits.Hooks = append(edits.Hooks, hook)
		}

		if hook := g.createLdcacheHook(libPaths); hook != nil {
			edits.Hooks = append(edits.Hooks, hook)
		}

		edits.Env = g.createEnvVars(libPaths, len(edits.Hooks) > 0)
	}

	return edits
}

// createLibraryMount creates a bind mount for a library using its ContainerPath.
func (g *generator) createLibraryMount(lib discover.Library) specs.Mount {
	return g.createMountWithContainerPath(lib.Path, lib.ContainerPath)
}

// createToolMount creates a bind mount for a tool using its ContainerPath.
func (g *generator) createToolMount(tool discover.Tool) specs.Mount {
	return g.createMountWithContainerPath(tool.Path, tool.ContainerPath)
}

// createMountWithContainerPath creates a bind mount with specified container path.
func (g *generator) createMountWithContainerPath(hostPath, containerPath string) specs.Mount {
	// Base mount options
	options := []string{"ro", "nosuid", "nodev", "bind"}

	// Add SELinux context if enabled
	if g.cfg.SELinux.Enabled && g.cfg.SELinux.MountContext != "" {
		// "z" - shared label, "Z" - private label
		options = append(options, g.cfg.SELinux.MountContext)
	}

	return specs.Mount{
		HostPath:      hostPath,
		ContainerPath: containerPath,
		Options:       options,
	}
}

// getSymlinkTargetContainerPath returns the container path for a symlink's target.
func (g *generator) getSymlinkTargetContainerPath(lib discover.Library) string {
	if g.cfg.Libraries.ContainerPath != "" {
		// Isolation mode: symlink target also goes to isolation path
		return filepath.Join(g.cfg.Libraries.ContainerPath, filepath.Base(lib.RealPath))
	}

	// Default mode: adjust for driver root
	containerPath := lib.RealPath
	if g.cfg.DriverRoot != "/" && g.cfg.DriverRoot != "" {
		containerPath = strings.TrimPrefix(lib.RealPath, g.cfg.DriverRoot)
		if !strings.HasPrefix(containerPath, "/") {
			containerPath = "/" + containerPath
		}
	}
	return containerPath
}

// createEnvVars creates environment variables for the container.
// When hooks are enabled, LD_LIBRARY_PATH is NOT added because ldcache handles it.
// ldconfig hooks are preferred over LD_LIBRARY_PATH for setuid binary support.
func (g *generator) createEnvVars(libPaths map[string]bool, hasHooks bool) []string {
	var envVars []string

	// LD_LIBRARY_PATH - only when hooks are NOT available (fallback)
	// When hooks are present, ldcache handles library discovery (supports setuid)
	if !hasHooks && len(libPaths) > 0 {
		paths := make([]string, 0, len(libPaths))
		for p := range libPaths {
			paths = append(paths, p)
		}
		envVars = append(envVars, fmt.Sprintf("LD_LIBRARY_PATH=%s", strings.Join(paths, ":")))
	}

	return envVars
}

// createLdcacheHook creates a CDI hook to update ldcache via ldconfig.
// This ensures proper library discovery for setuid binaries.
func (g *generator) createLdcacheHook(libPaths map[string]bool) *specs.Hook {
	if len(libPaths) == 0 {
		return nil
	}

	folders := make([]string, 0, len(libPaths))
	for p := range libPaths {
		folders = append(folders, p)
	}
	sort.Strings(folders)

	args := []string{
		"rbln-cdi-hook",
		"update-ldcache",
	}
	for _, folder := range folders {
		args = append(args, "--folder", folder)
	}

	env := []string{
		fmt.Sprintf("RBLN_CDI_HOOK_LDCONFIG_PATH=%s", g.cfg.Hooks.LdconfigPath),
		fmt.Sprintf("RBLN_CDI_HOOK_DEBUG=%v", g.cfg.Debug),
	}

	return &specs.Hook{
		HookName: "createContainer",
		Path:     g.cfg.Hooks.Path,
		Args:     args,
		Env:      env,
	}
}

func (g *generator) computeSymlinks(libs []discover.Library) []string {
	var links []string
	seen := make(map[string]bool)

	for _, lib := range libs {
		// Skip symlink entries (after discovery refactor, there won't be any)
		if lib.RealPath != "" && lib.RealPath != lib.Path {
			continue
		}

		hostPathForRead := g.getHostPathForRead(lib.Path)
		soname, err := discover.ReadSONAME(hostPathForRead)
		if err != nil {
			continue
		}

		hostDir := filepath.Dir(lib.Path)
		libraryName := filepath.Base(lib.Path)
		containerDir := filepath.Dir(lib.ContainerPath)
		if g.cfg.Libraries.ContainerPath != "" {
			containerDir = g.cfg.Libraries.ContainerPath
		}

		// Create SONAME -> libraryName symlink
		// e.g., librbln-ccl.so.3.0.0::/usr/lib/librbln-ccl.so.3
		// creates: /usr/lib/librbln-ccl.so.3 -> librbln-ccl.so.3.0.0
		if soname != libraryName && g.linkExistsOnHost(hostDir, soname) {
			spec := fmt.Sprintf("%s::%s", libraryName, filepath.Join(containerDir, soname))
			if !seen[spec] {
				links = append(links, spec)
				seen[spec] = true
			}
		}

		// Create .so -> SONAME symlink
		// e.g., librbln-ccl.so.3::/usr/lib/librbln-ccl.so
		// creates: /usr/lib/librbln-ccl.so -> librbln-ccl.so.3
		soTarget := soname
		if soTarget == "" {
			soTarget = libraryName
		}
		if soLink := discover.GetSoLink(soTarget); soLink != "" && soLink != soTarget && g.linkExistsOnHost(hostDir, soLink) {
			spec := fmt.Sprintf("%s::%s", soTarget, filepath.Join(containerDir, soLink))
			if !seen[spec] {
				links = append(links, spec)
				seen[spec] = true
			}
		}
	}

	return links
}

func (g *generator) getHostPathForRead(path string) string {
	if g.cfg.SearchRoot == "" || g.cfg.SearchRoot == "/" || g.cfg.SearchRoot == g.cfg.DriverRoot {
		return path
	}
	relative := strings.TrimPrefix(path, g.cfg.DriverRoot)
	if !strings.HasPrefix(relative, "/") {
		relative = "/" + relative
	}
	return filepath.Join(g.cfg.SearchRoot, relative)
}

func (g *generator) linkExistsOnHost(dir, linkName string) bool {
	if linkName == "" {
		return false
	}
	linkPath := filepath.Join(dir, linkName)
	hostLinkPath := g.getHostPathForRead(linkPath)
	exists, err := discover.LinkExists(hostLinkPath)
	if err != nil {
		return false
	}
	return exists
}

func (g *generator) createSymlinkHook(links []string) *specs.Hook {
	if len(links) == 0 {
		return nil
	}

	args := []string{
		"rbln-cdi-hook",
		"create-symlinks",
	}
	for _, link := range links {
		args = append(args, "--link", link)
	}

	return &specs.Hook{
		HookName: "createContainer",
		Path:     g.cfg.Hooks.Path,
		Args:     args,
	}
}
