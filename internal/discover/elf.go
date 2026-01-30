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
	"debug/elf"
	"os"
	"path/filepath"
)

// elfResolver implements LDDRunner using ELF DT_NEEDED parsing
// instead of the system ldd command. This avoids musl/glibc
// incompatibility when running in Alpine containers.
type elfResolver struct {
	driverRoot  string
	searchPaths []string
	// readNeeded reads DT_NEEDED entries from an ELF binary.
	// Injected for testing (defaults to elfReadNeeded).
	readNeeded func(path string) ([]string, error)
}

// NewELFResolver creates a new ELF-based dependency resolver.
// driverRoot: prefix for host filesystem (e.g., "/run/rbln/driver")
// searchPaths: directories to search for libraries (e.g., ["/usr/lib64", "/lib"])
func NewELFResolver(driverRoot string, searchPaths []string) LDDRunner {
	return &elfResolver{
		driverRoot:  driverRoot,
		searchPaths: searchPaths,
		readNeeded:  elfReadNeeded,
	}
}

// Run resolves all transitive dependencies of the given library.
// Input: libraryPath WITH DriverRoot prefix
// Output: dependency paths WITHOUT DriverRoot prefix
func (r *elfResolver) Run(libraryPath string) ([]string, error) {
	visited := make(map[string]bool)
	var result []string

	visited[libraryPath] = true
	queue := []string{libraryPath}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		needed, err := r.readNeeded(current)
		if err != nil {
			continue
		}

		for _, name := range needed {
			resolved := r.resolveLibrary(name)
			if resolved == "" {
				continue
			}

			fullPath := filepath.Join(r.driverRoot, resolved)
			if visited[fullPath] {
				continue
			}
			visited[fullPath] = true

			result = append(result, resolved)
			queue = append(queue, fullPath)
		}
	}

	return result, nil
}

// resolveLibrary finds a library by name in search paths under DriverRoot.
// Returns the path WITHOUT DriverRoot prefix, or empty string if not found.
// If the library is a symlink, returns the resolved real path.
func (r *elfResolver) resolveLibrary(name string) string {
	for _, searchPath := range r.searchPaths {
		candidate := filepath.Join(r.driverRoot, searchPath, name)
		info, err := os.Lstat(candidate)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			realPath, err := filepath.EvalSymlinks(candidate)
			if err != nil {
				continue
			}
			rel, err := filepath.Rel(r.driverRoot, realPath)
			if err != nil {
				continue
			}
			return "/" + rel
		}
		return filepath.Join(searchPath, name)
	}
	return ""
}

// elfReadNeeded reads DT_NEEDED entries from an ELF binary.
func elfReadNeeded(path string) ([]string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	needed, err := f.DynString(elf.DT_NEEDED)
	if err != nil {
		return nil, err
	}

	return needed, nil
}
