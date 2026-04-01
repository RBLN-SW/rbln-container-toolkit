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

// DefaultConfig returns the default configuration with hardcoded values.
func DefaultConfig() *Config {
	return &Config{
		CDI: CDIConfig{
			OutputPath: "/var/run/cdi/rbln.yaml",
			Format:     "yaml",
			Vendor:     "rebellions.ai",
			Class:      "npu",
		},
		Libraries: LibraryConfig{
			Patterns: []string{"librbln-*.so*"},
			PluginPaths: []string{
				"/usr/lib64/libibverbs",
				"/usr/lib/x86_64-linux-gnu/libibverbs",
			},
			ContainerPath: "", // Empty = default mode (hostPath == containerPath)
		},
		Devices: DeviceConfig{
			Patterns: []string{"/dev/rbln*", "/dev/rsd*"},
		},
		Tools: []string{"rbln-smi", "rbln-stat", "rblnBandwidthLatencyTest"},
		SearchPaths: SearchPathConfig{
			Libraries: []string{
				"/usr/lib64",
				"/usr/lib/x86_64-linux-gnu",
				"/usr/lib",
				"/usr/local/lib",
				"/usr/local/lib64",
				"/lib",
				"/lib64",
				"/lib/x86_64-linux-gnu",
			},
			Binaries: []string{
				"/usr/bin",
				"/usr/local/bin",
			},
		},
		GlibcExclude: []string{
			"ld-linux*",
			"linux-vdso*",
			"libc.so*",
			"libm.so*",
			"libpthread*",
			"libdl*",
			"librt*",
			"libgcc_s*",
			"libstdc++*",
			"libresolv*",
			"libnss_*",
		},
		SELinux: SELinuxConfig{
			// Disabled by default, auto-detected on RHEL/CoreOS
			Enabled:      false,
			MountContext: "z", // "z" for shared context
		},
		Hooks: HookConfig{
			Path:         "/usr/local/bin/rbln-cdi-hook",
			LdconfigPath: "/sbin/ldconfig",
		},
		Debug:      false,
		DriverRoot: "/",
	}
}

// DefaultConfigPaths returns the default configuration file search paths.
func DefaultConfigPaths() []string {
	return []string{
		"/etc/rbln/container-toolkit.yaml",
		"/etc/rbln/container-toolkit.yml",
	}
}
