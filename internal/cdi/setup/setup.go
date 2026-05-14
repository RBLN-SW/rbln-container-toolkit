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

package setup

import (
	"fmt"
	"io"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/cdi"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/topology"
)

// resolveTopology returns the resolver the generator should consult for
// per-NPU RSD attachment. Callers may pin a specific implementation via
// Options.RsdResolver (tests typically do); otherwise we attempt to load the
// librbln-ml-backed resolver and fall back to NoopResolver{} when the driver
// isn't reachable or the binary was built without the with_rblnml tag — in
// either case the warning is routed through the caller's logger so a missing
// RSD mapping is visible in operator logs rather than silently ignored.
//
// The outcome of the load — NPUs mapped, NPUs that failed, wall-clock cost,
// and whether we fell back — is emitted as an Info log line on the success
// path so operators monitoring the daemon (or just tailing CLI output) can
// confirm the resolver is doing what they expect. Without this signal, a
// regression where the librbln-ml call silently returns an empty mapping
// would look identical to "no NPUs on the host" — both produce per-NPU
// entries without RSD attachment, with no visible distinction.
func resolveTopology(opts *Options) topology.RsdResolver {
	if opts.RsdResolver != nil {
		return opts.RsdResolver
	}
	// K8s path: device-plugin owns RSD allocation, so skip the rblnml load
	// entirely — opening /dev/rbln* here would be wasted work and might
	// race with device-plugin's own handles.
	if opts.Config != nil && opts.Config.Devices.Disabled {
		return topology.NoopResolver{}
	}
	warn := func(format string, args ...any) {
		if opts.Logger != nil {
			opts.Logger.Warning(format, args...)
		}
	}
	resolver, stats := topology.LoadOrFallbackWithStats(warn)
	if opts.Logger != nil {
		// stats.String() never embeds format codes, so passing it as the
		// message with no args is safe even for loggers that go through
		// fmt.Printf-family functions internally.
		opts.Logger.Info("%s", stats.String())
	}
	return resolver
}

// GenerateCDISpec discovers resources and generates a CDI specification.
func GenerateCDISpec(opts *Options) error {
	if opts == nil {
		opts = DefaultOptions()
	}

	if opts.Config == nil {
		return fmt.Errorf("config is required")
	}

	libDiscoverer := opts.LibraryDiscoverer
	if libDiscoverer == nil {
		libDiscoverer = discover.NewLibraryDiscoverer(opts.Config)
	}
	toolDiscoverer := opts.ToolDiscoverer
	if toolDiscoverer == nil {
		toolDiscoverer = discover.NewToolDiscoverer(opts.Config)
	}
	deviceDiscoverer := opts.DeviceDiscoverer
	if deviceDiscoverer == nil && !opts.Config.Devices.Disabled {
		deviceDiscoverer = discover.NewDeviceDiscoverer(opts.Config)
	}

	result, err := DiscoverResources(libDiscoverer, toolDiscoverer, deviceDiscoverer)
	if err != nil {
		if opts.ErrorMode == ErrorModeStrict {
			return fmt.Errorf("discover resources: %w", err)
		}
		if opts.Logger != nil {
			opts.Logger.Warning("Discovery failed: %v", err)
		}
		result = &discover.DiscoveryResult{}
	}

	generator := cdi.NewGenerator(opts.Config, resolveTopology(opts))
	spec, err := generator.Generate(result)
	if err != nil {
		return fmt.Errorf("generate CDI spec: %w", err)
	}

	if opts.OutputPath != "" {
		writer := cdi.NewWriter()
		format := opts.Format
		if format == "" {
			format = "yaml"
		}
		if err := writer.Write(spec, opts.OutputPath, format); err != nil {
			return fmt.Errorf("write CDI spec: %w", err)
		}
		if opts.Logger != nil {
			opts.Logger.Info("CDI specification written to %s", opts.OutputPath)
		}
	}

	return nil
}

// GenerateCDISpecToWriter generates CDI spec and writes to io.Writer (for dry-run/stdout).
func GenerateCDISpecToWriter(w io.Writer, opts *Options) error {
	if opts == nil {
		opts = DefaultOptions()
	}

	if opts.Config == nil {
		return fmt.Errorf("config is required")
	}

	libDiscoverer := opts.LibraryDiscoverer
	if libDiscoverer == nil {
		libDiscoverer = discover.NewLibraryDiscoverer(opts.Config)
	}
	toolDiscoverer := opts.ToolDiscoverer
	if toolDiscoverer == nil {
		toolDiscoverer = discover.NewToolDiscoverer(opts.Config)
	}
	deviceDiscoverer := opts.DeviceDiscoverer
	if deviceDiscoverer == nil && !opts.Config.Devices.Disabled {
		deviceDiscoverer = discover.NewDeviceDiscoverer(opts.Config)
	}

	result, err := DiscoverResources(libDiscoverer, toolDiscoverer, deviceDiscoverer)
	if err != nil {
		if opts.ErrorMode == ErrorModeStrict {
			return fmt.Errorf("discover resources: %w", err)
		}
		if opts.Logger != nil {
			opts.Logger.Warning("Discovery failed: %v", err)
		}
		result = &discover.DiscoveryResult{}
	}

	generator := cdi.NewGenerator(opts.Config, resolveTopology(opts))
	spec, err := generator.Generate(result)
	if err != nil {
		return fmt.Errorf("generate CDI spec: %w", err)
	}

	writer := cdi.NewWriter()
	format := opts.Format
	if format == "" {
		format = "yaml"
	}
	if err := writer.WriteToWriter(spec, w, format); err != nil {
		return fmt.Errorf("write CDI spec: %w", err)
	}

	return nil
}

// DiscoverResources discovers libraries, tools, and devices using the provided discoverers.
func DiscoverResources(libDisc discover.LibraryDiscoverer, toolDisc discover.ToolDiscoverer, devDisc discover.DeviceDiscoverer) (*discover.DiscoveryResult, error) {
	result := &discover.DiscoveryResult{}

	rblnLibs, err := libDisc.DiscoverRBLN()
	if err != nil {
		return nil, fmt.Errorf("discover RBLN libraries: %w", err)
	}
	result.Libraries = append(result.Libraries, rblnLibs...)

	deps, err := libDisc.DiscoverDependencies(rblnLibs)
	if err != nil {
		return nil, fmt.Errorf("discover dependencies: %w", err)
	}
	result.Libraries = append(result.Libraries, deps...)

	plugins, err := libDisc.DiscoverPlugins()
	if err != nil {
		return nil, fmt.Errorf("discover plugins: %w", err)
	}
	result.Libraries = append(result.Libraries, plugins...)

	if toolDisc != nil {
		tools, err := toolDisc.Discover()
		if err != nil {
			return nil, fmt.Errorf("discover tools: %w", err)
		}
		result.Tools = tools
	}

	if devDisc != nil {
		devices, err := devDisc.Discover()
		if err != nil {
			return nil, fmt.Errorf("discover devices: %w", err)
		}
		result.Devices = devices
	}

	return result, nil
}
