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
)

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

	result, err := DiscoverResources(libDiscoverer, toolDiscoverer)
	if err != nil {
		if opts.ErrorMode == ErrorModeStrict {
			return fmt.Errorf("discover resources: %w", err)
		}
		if opts.Logger != nil {
			opts.Logger.Warning("Discovery failed: %v", err)
		}
		result = &discover.DiscoveryResult{}
	}

	generator := cdi.NewGenerator(opts.Config)
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

	result, err := DiscoverResources(libDiscoverer, toolDiscoverer)
	if err != nil {
		if opts.ErrorMode == ErrorModeStrict {
			return fmt.Errorf("discover resources: %w", err)
		}
		if opts.Logger != nil {
			opts.Logger.Warning("Discovery failed: %v", err)
		}
		result = &discover.DiscoveryResult{}
	}

	generator := cdi.NewGenerator(opts.Config)
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

// DiscoverResources discovers libraries and tools using the provided discoverers.
func DiscoverResources(libDisc discover.LibraryDiscoverer, toolDisc discover.ToolDiscoverer) (*discover.DiscoveryResult, error) {
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

	return result, nil
}
