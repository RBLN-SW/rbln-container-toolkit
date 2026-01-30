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

// Package discover provides library and tool discovery functionality.
package discover

//go:generate moq -rm -fmt=goimports -stub -out discoverer_mock.go . Discoverer LibraryDiscoverer ToolDiscoverer

// LibraryType represents the type of a discovered library.
type LibraryType int

const (
	// LibraryTypeRBLN is an RBLN core library (matches librbln-*.so* pattern)
	LibraryTypeRBLN LibraryType = iota
	// LibraryTypeDependency is a library discovered via ldd
	LibraryTypeDependency
	// LibraryTypePlugin is a dlopen plugin library
	LibraryTypePlugin
)

// String returns the string representation of LibraryType.
func (t LibraryType) String() string {
	switch t {
	case LibraryTypeRBLN:
		return "rbln"
	case LibraryTypeDependency:
		return "dependency"
	case LibraryTypePlugin:
		return "plugin"
	default:
		return "unknown"
	}
}

// Library represents a discovered library.
type Library struct {
	Name          string      // File name (e.g., librbln-ml.so)
	Path          string      // Absolute path on host (may include driver-root)
	ContainerPath string      // Path as seen inside container (without driver-root prefix)
	RealPath      string      // Resolved symlink path (empty if not a symlink)
	Type          LibraryType // Library type
}

// Tool represents a discovered CLI tool.
type Tool struct {
	Name          string // Tool name (e.g., rbln-smi)
	Path          string // Absolute path on host (may include driver-root)
	ContainerPath string // Path as seen inside container (without driver-root prefix)
}

// DiscoveryResult holds the complete discovery result.
type DiscoveryResult struct {
	Libraries []Library
	Tools     []Tool
}

// Discoverer is the interface for resource discovery.
type Discoverer interface {
	// Discover performs full discovery and returns the result.
	Discover() (*DiscoveryResult, error)
}

// LibraryDiscoverer discovers libraries.
type LibraryDiscoverer interface {
	// DiscoverRBLN discovers RBLN libraries (librbln-*.so* pattern).
	DiscoverRBLN() ([]Library, error)

	// DiscoverDependencies discovers dependencies of the given libraries using ldd.
	DiscoverDependencies(libs []Library) ([]Library, error)

	// DiscoverPlugins discovers dlopen plugin libraries.
	DiscoverPlugins() ([]Library, error)
}

// ToolDiscoverer discovers tools.
type ToolDiscoverer interface {
	// Discover discovers the configured tools.
	Discover() ([]Tool, error)
}
