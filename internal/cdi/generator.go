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
	"strconv"
	"strings"

	"tags.cncf.io/container-device-interface/specs-go"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/topology"
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
	cfg      *config.Config
	resolver topology.RsdResolver
}

// NewGenerator creates a new CDI generator. The resolver is consulted while
// building per-NPU device entries to attach each NPU's assigned RSD group
// device; pass topology.NoopResolver{} (or nil) for the K8s path or for
// tests that don't care about RSD topology.
func NewGenerator(cfg *config.Config, resolver topology.RsdResolver) Generator {
	if resolver == nil {
		resolver = topology.NoopResolver{}
	}
	return &generator{cfg: cfg, resolver: resolver}
}

// AllDeviceName is the CDI device entry that selects every discovered device.
// Replaces the previous "runtime" entry from CTK v0.1.x.
const AllDeviceName = "all"

// rsdEntryPrefix is the entry-name prefix for explicit RSD group selection
// (e.g., "rsd0", "rsd1"). Per-NPU entries use the bare numeric index ("0", "1").
const rsdEntryPrefix = "rsd"

// Generate creates a CDI spec from discovery results.
//
// Spec layout:
//   - top-level ContainerEdits: libraries, tools, hooks, env (applied to every
//     selection). No device nodes — RSD attachment is decided per NPU via the
//     resolver so users picking `--device rebellions.ai/npu=N` automatically
//     receive the correct group device without listing it themselves.
//   - per-NPU entries named "0", "1", ... each carrying `/dev/rbln{N}` plus
//     the resolved `/dev/rsd{GroupID}` when the resolver knows the mapping.
//   - per-RSD entries named "rsd0", "rsd1", ... for explicit group selection
//     (kept for debugging / custom-group workflows where the user wants to
//     bypass the auto-mapping).
//   - "all" entry: every discovered NPU + RSD node, replacing the legacy
//     "runtime" entry.
//
// When config.Devices.Disabled is true (Kubernetes path) device discovery is
// suppressed by the caller and we emit only the "all" entry with no device
// nodes; the device-plugin owns per-allocation device injection there.
func (g *generator) Generate(result *discover.DiscoveryResult) (*specs.Spec, error) {
	spec := &specs.Spec{
		Version: CDIVersion,
		Kind:    fmt.Sprintf("%s/%s", g.cfg.CDI.Vendor, g.cfg.CDI.Class),
	}

	spec.ContainerEdits = g.buildCommonEdits(result)

	rblnDevs, rsdDevs := g.classifyDevices(result)
	spec.Devices = g.buildDeviceEntries(rblnDevs, rsdDevs)

	return spec, nil
}

// buildCommonEdits returns ContainerEdits shared across every device selection:
// library/tool mounts, ldcache + symlink hooks, and any required env vars.
// Device nodes are intentionally kept out — RSD attachment is per-NPU (via
// the resolver) and the `all` entry carries the bulk-mount handle, so the
// top-level block stays device-node-free regardless of host topology.
func (g *generator) buildCommonEdits(result *discover.DiscoveryResult) specs.ContainerEdits {
	edits := specs.ContainerEdits{}
	if result == nil {
		return edits
	}

	libPaths := make(map[string]bool)
	mountedPaths := make(map[string]bool)

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
	return edits
}

// classifyDevices partitions discovered devices into RBLN NPU nodes and RSD
// group nodes, sorted by numeric suffix. Unknown device names are dropped.
// Returns empty slices when Devices.Disabled is set (K8s path): the caller
// shouldn't have populated result.Devices in that mode, but defending here
// keeps device-plugin-driven deployments safe against generator misuse.
func (g *generator) classifyDevices(result *discover.DiscoveryResult) (rbln, rsd []discover.Device) {
	if result == nil || g.cfg.Devices.Disabled {
		return nil, nil
	}
	for _, dev := range result.Devices {
		switch parseDeviceClass(dev) {
		case deviceClassRBLN:
			rbln = append(rbln, dev)
		case deviceClassRSD:
			rsd = append(rsd, dev)
		}
	}
	sortDevicesByIndex(rbln)
	sortDevicesByIndex(rsd)
	return rbln, rsd
}

// buildDeviceEntries produces the per-NPU, per-RSD, and "all" CDI device
// entries. Each per-NPU entry carries the bare `/dev/rbln{N}` plus — when the
// resolver supplies a mapping — the `/dev/rsd{GroupID}` assigned to that NPU,
// so `docker run --device rebellions.ai/npu=N` is functional on its own.
// Always emits "all" as the named handle for library/tool injection even when
// no device nodes are present (K8s path).
func (g *generator) buildDeviceEntries(rblnDevs, rsdDevs []discover.Device) []specs.Device {
	rsdByIndex := indexRSDDevices(rsdDevs)
	// Pre-size for the worst case: every NPU + every RSD + the trailing
	// `all` umbrella entry. Saves a few growslice rounds on hosts with
	// dense NPU populations and silences golangci-lint's prealloc check.
	devices := make([]specs.Device, 0, len(rblnDevs)+len(rsdDevs)+1)

	for _, dev := range rblnDevs {
		npuNode := g.createDeviceNode(dev)
		edits := specs.ContainerEdits{
			DeviceNodes: []*specs.DeviceNode{&npuNode},
		}
		if rsdDev, ok := g.resolveRSDFor(dev, rsdByIndex); ok {
			rsdNode := g.createDeviceNode(rsdDev)
			edits.DeviceNodes = append(edits.DeviceNodes, &rsdNode)
		}
		devices = append(devices, specs.Device{
			Name:           deviceIndex(dev),
			ContainerEdits: edits,
		})
	}

	for _, dev := range rsdDevs {
		node := g.createDeviceNode(dev)
		devices = append(devices, specs.Device{
			Name: rsdEntryPrefix + deviceIndex(dev),
			ContainerEdits: specs.ContainerEdits{
				DeviceNodes: []*specs.DeviceNode{&node},
			},
		})
	}

	allEdits := specs.ContainerEdits{}
	for _, dev := range rblnDevs {
		node := g.createDeviceNode(dev)
		allEdits.DeviceNodes = append(allEdits.DeviceNodes, &node)
	}
	for _, dev := range rsdDevs {
		node := g.createDeviceNode(dev)
		allEdits.DeviceNodes = append(allEdits.DeviceNodes, &node)
	}
	devices = append(devices, specs.Device{
		Name:           AllDeviceName,
		ContainerEdits: allEdits,
	})

	return devices
}

// resolveRSDFor consults the configured resolver to find the RSD group device
// assigned to the given NPU. Returns ok=false when the resolver doesn't know
// the mapping or when the indicated RSD wasn't actually discovered on the
// host — both cases yield a per-NPU entry that carries only the rbln node,
// which the user must complement with an explicit `--device =rsdM` flag.
func (g *generator) resolveRSDFor(npu discover.Device, rsdByIndex map[uint32]discover.Device) (discover.Device, bool) {
	idx, err := strconv.ParseUint(deviceIndex(npu), 10, 32)
	if err != nil {
		return discover.Device{}, false
	}
	rsdIdx, ok := g.resolver.Resolve(uint32(idx))
	if !ok {
		return discover.Device{}, false
	}
	dev, found := rsdByIndex[rsdIdx]
	return dev, found
}

// indexRSDDevices builds a numeric-index → discover.Device map so per-NPU
// entry construction can look up "rsd K" in O(1). Devices whose suffix isn't
// numeric are skipped (parseDeviceClass already filters those upstream, this
// is defensive).
func indexRSDDevices(rsdDevs []discover.Device) map[uint32]discover.Device {
	out := make(map[uint32]discover.Device, len(rsdDevs))
	for _, dev := range rsdDevs {
		idx, err := strconv.ParseUint(deviceIndex(dev), 10, 32)
		if err != nil {
			continue
		}
		out[uint32(idx)] = dev
	}
	return out
}

// deviceClass identifies whether a device node is an RBLN NPU or an RSD group.
type deviceClass int

const (
	deviceClassUnknown deviceClass = iota
	deviceClassRBLN
	deviceClassRSD
)

// parseDeviceClass returns the class of a discovered device by inspecting the
// basename of its container path (e.g., "rbln0", "rsd1"). Devices whose names
// don't match either prefix or whose suffix isn't numeric are classified as
// unknown and skipped by the generator — they shouldn't reach this code given
// config.Devices.Patterns, but defending keeps a future pattern change from
// silently producing malformed CDI entries.
func parseDeviceClass(dev discover.Device) deviceClass {
	base := filepath.Base(dev.ContainerPath)
	if rest, ok := strings.CutPrefix(base, "rbln"); ok {
		if _, err := strconv.Atoi(rest); err == nil {
			return deviceClassRBLN
		}
	}
	if rest, ok := strings.CutPrefix(base, "rsd"); ok {
		if _, err := strconv.Atoi(rest); err == nil {
			return deviceClassRSD
		}
	}
	return deviceClassUnknown
}

// deviceIndex returns the numeric suffix of a device basename
// (e.g., "/dev/rbln3" → "3"). Assumes parseDeviceClass already accepted dev.
func deviceIndex(dev discover.Device) string {
	base := filepath.Base(dev.ContainerPath)
	if rest, ok := strings.CutPrefix(base, "rbln"); ok {
		return rest
	}
	if rest, ok := strings.CutPrefix(base, "rsd"); ok {
		return rest
	}
	return base
}

// sortDevicesByIndex orders devices by their numeric suffix so per-device
// entries appear as "0", "1", "2", ... rather than in glob-walk order.
func sortDevicesByIndex(devs []discover.Device) {
	sort.SliceStable(devs, func(i, j int) bool {
		ii, _ := strconv.Atoi(deviceIndex(devs[i]))
		jj, _ := strconv.Atoi(deviceIndex(devs[j]))
		return ii < jj
	})
}

// createDeviceNode creates a CDI device node spec from a discovered device.
func (g *generator) createDeviceNode(dev discover.Device) specs.DeviceNode {
	return specs.DeviceNode{
		Path:        dev.ContainerPath,
		HostPath:    dev.Path,
		Permissions: "rw",
	}
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
