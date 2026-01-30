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

//go:generate moq -rm -fmt=goimports -stub -out ldcache_mock.go . LDCache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// ldcacheEntry represents a single entry in ldcache.
type ldcacheEntry struct {
	name    string
	path    string
	is64bit bool
}

// ldcache implements LDCache interface.
type ldcache struct {
	root      string
	libraries []ldcacheEntry
}

// LDCache is the interface for /etc/ld.so.cache operations.
type LDCache interface {
	// List returns all cached libraries.
	// First return: 32-bit, Second return: 64-bit
	List() ([]string, []string)

	// Lookup finds libraries matching the pattern.
	// Pattern supports glob format (e.g., "librbln-*.so*")
	Lookup(pattern string) ([]string, error)
}

// NewLDCache creates a new LDCache instance.
// Note: This is a simplified implementation that uses fallback search
// instead of parsing the binary ldcache format.
func NewLDCache(root string) (LDCache, error) {
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil, fmt.Errorf("root path does not exist: %s: %w", root, errors.ErrLdcacheParseFailed)
	}

	// For now, return a cache that will use fallback search paths
	return &ldcache{
		root:      root,
		libraries: []ldcacheEntry{},
	}, nil
}

// List returns all cached libraries separated by architecture.
func (c *ldcache) List() (libs32, libs64 []string) {
	for _, entry := range c.libraries {
		if entry.is64bit {
			libs64 = append(libs64, entry.path)
		} else {
			libs32 = append(libs32, entry.path)
		}
	}
	return libs32, libs64
}

// Lookup finds libraries matching the pattern (64-bit only).
func (c *ldcache) Lookup(pattern string) ([]string, error) {
	var matches []string
	for _, entry := range c.libraries {
		if !entry.is64bit {
			continue
		}
		matched, err := filepath.Match(pattern, entry.name)
		if err == nil && matched {
			matches = append(matches, entry.path)
		}
	}
	return matches, nil
}

// FallbackLDCache implements LDCache using directory search.
type FallbackLDCache struct {
	root        string
	searchPaths []string
	libraries   []ldcacheEntry
}

// NewFallbackLDCache creates a fallback cache that searches directories.
func NewFallbackLDCache(root string, searchPaths []string) *FallbackLDCache {
	cache := &FallbackLDCache{
		root:        root,
		searchPaths: searchPaths,
		libraries:   []ldcacheEntry{},
	}
	cache.scan()
	return cache
}

// scan searches for libraries in the configured paths.
func (c *FallbackLDCache) scan() {
	for _, searchPath := range c.searchPaths {
		fullPath := filepath.Join(c.root, searchPath)
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasSuffix(name, ".so") || strings.Contains(name, ".so.") {
				c.libraries = append(c.libraries, ldcacheEntry{
					name:    name,
					path:    filepath.Join(fullPath, name),
					is64bit: true, // Assume 64-bit for fallback
				})
			}
		}
	}
}

// List returns all cached libraries.
func (c *FallbackLDCache) List() (libs32, libs64 []string) {
	libs64 = make([]string, 0, len(c.libraries))
	for _, entry := range c.libraries {
		libs64 = append(libs64, entry.path)
	}
	return nil, libs64
}

// Lookup finds libraries matching the pattern.
func (c *FallbackLDCache) Lookup(pattern string) ([]string, error) {
	var matches []string
	seen := make(map[string]bool)

	for _, entry := range c.libraries {
		matched, err := filepath.Match(pattern, entry.name)
		if err == nil && matched {
			if !seen[entry.path] {
				matches = append(matches, entry.path)
				seen[entry.path] = true
			}
		}
	}
	return matches, nil
}
