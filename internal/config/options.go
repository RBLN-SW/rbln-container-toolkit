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

package config

// Option is a function that modifies Config.
type Option func(*Config)

// WithConfigFile sets the configuration file path.
// Note: This is handled by the loader, not as a Config option.
func WithConfigFile(_ string) Option {
	return func(_ *Config) {
		// ConfigFile is handled separately by the loader
	}
}

// WithDriverRoot sets the driver root path.
func WithDriverRoot(root string) Option {
	return func(c *Config) {
		if root != "" {
			c.DriverRoot = root
		}
	}
}

// WithOutputPath sets the CDI output path.
func WithOutputPath(path string) Option {
	return func(c *Config) {
		if path != "" {
			c.CDI.OutputPath = path
		}
	}
}

// WithFormat sets the CDI output format.
func WithFormat(format string) Option {
	return func(c *Config) {
		if format != "" {
			c.CDI.Format = format
		}
	}
}

// WithDebug sets the debug mode.
func WithDebug(debug bool) Option {
	return func(c *Config) {
		c.Debug = debug
	}
}

// WithVendor sets the CDI vendor name.
func WithVendor(vendor string) Option {
	return func(c *Config) {
		if vendor != "" {
			c.CDI.Vendor = vendor
		}
	}
}

// WithClass sets the CDI device class.
func WithClass(class string) Option {
	return func(c *Config) {
		if class != "" {
			c.CDI.Class = class
		}
	}
}

// WithSELinux sets the SELinux configuration.
func WithSELinux(enabled bool) Option {
	return func(c *Config) {
		c.SELinux.Enabled = enabled
	}
}

// WithSELinuxContext sets the SELinux mount context option.
// Valid values are "z" (shared) or "Z" (private).
func WithSELinuxContext(context string) Option {
	return func(c *Config) {
		if context != "" {
			c.SELinux.MountContext = context
		}
	}
}

// WithContainerLibraryPath sets the container path for library isolation.
// When set, libraries are mounted to this path instead of their host paths,
// and LD_LIBRARY_PATH is configured to include this path.
func WithContainerLibraryPath(path string) Option {
	return func(c *Config) {
		if path != "" {
			c.Libraries.ContainerPath = path
		}
	}
}
