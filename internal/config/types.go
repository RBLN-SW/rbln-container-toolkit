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
}

// CDIConfig represents CDI output settings.
type CDIConfig struct {
	OutputPath string `yaml:"output-path"`
	Format     string `yaml:"format"`
	Vendor     string `yaml:"vendor"`
	Class      string `yaml:"class"`
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
