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

// Package config provides configuration loading and management.
package config

// Config represents the complete configuration for RBLN Container Toolkit.
type Config struct {
	CDI          CDIConfig        `yaml:"cdi"`
	RDS          RDSConfig        `yaml:"rds"`
	Libraries    LibraryConfig    `yaml:"libraries"`
	Tools        []string         `yaml:"tools"`
	Devices      DeviceConfig     `yaml:"devices"`
	SearchPaths  SearchPathConfig `yaml:"search-paths"`
	GlibcExclude []string         `yaml:"glibc-exclude"`
	SELinux      SELinuxConfig    `yaml:"selinux"`
	Hooks        HookConfig       `yaml:"hooks"`
	Debug        bool             `yaml:"debug"`

	// Runtime options (not from config file)
	DriverRoot string `yaml:"-"`
	SearchRoot string `yaml:"-"` // Prefix for file access (e.g., /host when running in container)

	// DeviceRoot is the filesystem root under which device nodes (/dev/*) are
	// discovered. Kernel device nodes live in the host's real /dev, never under
	// the driver install directory, so device discovery must NOT use SearchRoot
	// (= hostRoot + DriverRoot) — that would look under DriverRoot on
	// driver-container deployments and miss /dev/rblnfs* etc. DeviceRoot is the
	// host filesystem root only: "/host" for a containerized daemon, "" or "/"
	// on bare metal / CLI. Empty is treated as "/".
	DeviceRoot string `yaml:"-"`
}

// CDIConfig represents CDI output settings.
type CDIConfig struct {
	// OutputPath is the conventional location of the NPU CDI spec. It is
	// informational: the actual write path is controlled by `--output`
	// (rbln-ctk) or derived from `--cdi-spec-dir` (rbln-ctk-daemon), NOT read
	// back from this field, so setting `cdi.output-path` in the config file has
	// no runtime effect. It is surfaced by `rbln-ctk info` so operators know
	// where the spec lands.
	OutputPath string `yaml:"output-path"`
	Format     string `yaml:"format"`
	Vendor     string `yaml:"vendor"`
	Class      string `yaml:"class"`
}

// RDSConfig represents the separate CDI device class used to inject the RDS
// (Rebellions Datastore) char device /dev/rblnfs* into opt-in containers.
//
// RDS uses its own CDI class (rebellions.ai/rds) and its own spec file,
// independent of the NPU class, so injection is opt-in: a container only
// receives /dev/rblnfs* when it explicitly references the RDS device — Docker
// `--device rebellions.ai/rds=all` (or `=rblnfs0`), or a Pod annotation
// `cdi.k8s.io/<key>: rebellions.ai/rds=rblnfs0`. This separation keeps the RDS
// device node out of the NPU `all` selection and bypasses the Kubernetes
// Devices.Disabled gate, which only applies to NPU/RSD nodes whose per-Pod
// injection is owned by device-plugin. Because RDS is only injected into pods
// that reference it, always emitting its device node never masks device-plugin
// allocations (the v0.1.2 regression that motivated Devices.Disabled).
//
// The vendor and output format are shared with CDIConfig (CDI.Vendor / CDI.Format).
type RDSConfig struct {
	// Class is the CDI device class for RDS (vendor is shared with CDI.Vendor).
	Class string `yaml:"class"`
	// OutputPath is the conventional location of the RDS CDI spec. Like
	// CDIConfig.OutputPath, it is informational: the actual write path is
	// controlled by `--rds-output` (rbln-ctk) or derived from `--cdi-spec-dir`
	// (rbln-ctk-daemon), NOT read back from this field. Setting
	// `rds.output-path` in the config file therefore has no runtime effect — it
	// documents the default so operators know where the spec lands. Wherever it
	// is written, it must sit in a CDI spec directory the runtime scans so the
	// separate-Kind file is discovered alongside the NPU spec.
	OutputPath string `yaml:"output-path"`
	// Patterns are glob patterns used to discover RDS char devices (e.g.
	// "/dev/rblnfs*").
	Patterns []string `yaml:"patterns"`
}

// LibraryConfig represents library discovery settings.
type LibraryConfig struct {
	Patterns     []string `yaml:"patterns"`
	Dependencies []string `yaml:"dependencies"`
	PluginPaths  []string `yaml:"plugin-paths"`
	// ContainerPath specifies the container path for library isolation.
	// When set, libraries are mounted to this path instead of their host paths,
	// and LD_LIBRARY_PATH is configured to include this path.
	// Empty string (default) means libraries use the same path as on the host.
	ContainerPath string `yaml:"container-path"`
}

// SearchPathConfig represents search path settings.
type SearchPathConfig struct {
	Libraries []string `yaml:"libraries"`
	Binaries  []string `yaml:"binaries"`
}

// SELinuxConfig represents SELinux settings for CDI mounts.
type SELinuxConfig struct {
	// Enabled controls whether SELinux context is added to mounts.
	// When enabled, the "z" option is added to bind mounts for shared context.
	Enabled bool `yaml:"enabled"`

	// MountContext specifies the mount context option to use.
	// Values: "z" (shared), "Z" (private), or empty (disabled)
	// Default: "z" (shared) which allows multiple containers to access the mount.
	MountContext string `yaml:"mount-context"`
}

// DeviceConfig represents device node discovery settings.
type DeviceConfig struct {
	// Patterns are glob patterns to discover device nodes (e.g., "/dev/rbln*").
	Patterns []string `yaml:"patterns"`

	// Disabled turns off device-node discovery and emission into the CDI spec.
	// Default false matches Docker-style usage where the CDI runtime device
	// must inject /dev/rbln* itself. Kubernetes deployments set this to true
	// so that device-plugin/DRA owns per-allocation device injection without
	// CTK statically pinning device nodes (notably /dev/rsd0) into every Pod.
	Disabled bool `yaml:"disabled"`
}

// HookConfig represents CDI hook settings.
type HookConfig struct {
	// Path is the path to the rbln-cdi-hook binary.
	// Default: /usr/local/bin/rbln-cdi-hook
	Path string `yaml:"path"`

	// LdconfigPath is the path to the ldconfig binary used by the hook.
	// Default: /sbin/ldconfig
	LdconfigPath string `yaml:"ldconfig-path"`
}
