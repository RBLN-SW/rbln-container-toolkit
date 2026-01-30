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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Note: ldcache binary format is complex; we test with mock/fallback approach

func TestLDCache_NewWithInvalidRoot(t *testing.T) {
	// Given: An invalid root path
	root := "/nonexistent/path"

	// When: Creating LDCache with invalid root
	cache, err := NewLDCache(root)

	// Then: Should return error
	assert.Error(t, err)
	assert.Nil(t, cache)
}

func TestLDCache_List_Empty(t *testing.T) {
	// Given: A mock ldcache with no libraries
	tmpDir := t.TempDir()

	// Create empty ldcache structure
	cache := &ldcache{
		root:      tmpDir,
		libraries: []ldcacheEntry{},
	}

	// When: Listing libraries
	libs32, libs64 := cache.List()

	// Then: Should return empty lists
	assert.Empty(t, libs32)
	assert.Empty(t, libs64)
}

func TestLDCache_Lookup_PatternMatch(t *testing.T) {
	// Given: A mock ldcache with libraries
	cache := &ldcache{
		root: "/",
		libraries: []ldcacheEntry{
			{name: "librbln-ml.so", path: "/usr/lib64/librbln-ml.so", is64bit: true},
			{name: "librbln-thunk.so", path: "/usr/lib64/librbln-thunk.so", is64bit: true},
			{name: "libc.so.6", path: "/usr/lib64/libc.so.6", is64bit: true},
			{name: "libm.so.6", path: "/usr/lib64/libm.so.6", is64bit: true},
		},
	}

	// When: Looking up RBLN pattern
	matches, err := cache.Lookup("librbln-*.so*")

	// Then: Should return only RBLN libraries
	require.NoError(t, err)
	assert.Len(t, matches, 2)
	assert.Contains(t, matches, "/usr/lib64/librbln-ml.so")
	assert.Contains(t, matches, "/usr/lib64/librbln-thunk.so")
}

func TestLDCache_Lookup_NoMatch(t *testing.T) {
	// Given: A mock ldcache without matching libraries
	cache := &ldcache{
		root: "/",
		libraries: []ldcacheEntry{
			{name: "libc.so.6", path: "/usr/lib64/libc.so.6", is64bit: true},
		},
	}

	// When: Looking up RBLN pattern
	matches, err := cache.Lookup("librbln-*.so*")

	// Then: Should return empty list without error
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestLDCache_Lookup_64BitOnly(t *testing.T) {
	// Given: A mock ldcache with 32-bit and 64-bit libraries
	cache := &ldcache{
		root: "/",
		libraries: []ldcacheEntry{
			{name: "librbln-ml.so", path: "/usr/lib/librbln-ml.so", is64bit: false},
			{name: "librbln-ml.so", path: "/usr/lib64/librbln-ml.so", is64bit: true},
		},
	}

	// When: Looking up RBLN pattern
	matches, err := cache.Lookup("librbln-*.so*")

	// Then: Should return only 64-bit library
	require.NoError(t, err)
	assert.Len(t, matches, 1)
	assert.Contains(t, matches, "/usr/lib64/librbln-ml.so")
}

func TestFallbackLDCache_FromSearchPaths(t *testing.T) {
	// Given: A temp directory with library files
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create mock library files
	libs := []string{"librbln-ml.so", "librbln-thunk.so", "libc.so.6"}
	for _, lib := range libs {
		f, err := os.Create(filepath.Join(libDir, lib))
		require.NoError(t, err)
		f.Close()
	}

	// When: Creating fallback cache from search paths
	cache := NewFallbackLDCache(tmpDir, []string{"/usr/lib64"})

	// Then: Should find libraries in search paths
	matches, err := cache.Lookup("librbln-*.so*")
	require.NoError(t, err)
	assert.Len(t, matches, 2)
}

func TestFallbackLDCache_List(t *testing.T) {
	// Given: A temp directory with library files
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	libs := []string{"libfoo.so", "libbar.so.1", "libbaz.so.2.0"}
	for _, lib := range libs {
		f, err := os.Create(filepath.Join(libDir, lib))
		require.NoError(t, err)
		f.Close()
	}

	// When: Listing libraries
	cache := NewFallbackLDCache(tmpDir, []string{"/usr/lib64"})
	libs32, libs64 := cache.List()

	// Then: Should return all 64-bit libraries (fallback assumes 64-bit)
	assert.Nil(t, libs32)
	assert.Len(t, libs64, 3)
}

func TestFallbackLDCache_List_Empty(t *testing.T) {
	// Given: A temp directory with no libraries
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// When: Listing libraries
	cache := NewFallbackLDCache(tmpDir, []string{"/usr/lib64"})
	libs32, libs64 := cache.List()

	// Then: Should return empty lists
	assert.Nil(t, libs32)
	assert.Empty(t, libs64)
}

func TestFallbackLDCache_Lookup_Deduplication(t *testing.T) {
	// Given: A fallback cache with libraries
	tmpDir := t.TempDir()
	lib64Dir := filepath.Join(tmpDir, "usr", "lib64")
	libDir := filepath.Join(tmpDir, "usr", "lib")
	require.NoError(t, os.MkdirAll(lib64Dir, 0755))
	require.NoError(t, os.MkdirAll(libDir, 0755))

	// Create same library name in different paths
	for _, dir := range []string{lib64Dir, libDir} {
		f, err := os.Create(filepath.Join(dir, "libfoo.so"))
		require.NoError(t, err)
		f.Close()
	}

	// When: Looking up pattern that matches both
	cache := NewFallbackLDCache(tmpDir, []string{"/usr/lib64", "/usr/lib"})
	matches, err := cache.Lookup("libfoo.so")

	// Then: Should return both paths (deduplication by path, not name)
	require.NoError(t, err)
	assert.Len(t, matches, 2)
}

func TestFallbackLDCache_Scan_SkipsDirectories(t *testing.T) {
	// Given: A temp directory with libraries and subdirectories
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	subDir := filepath.Join(libDir, "libibverbs")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	f, err := os.Create(filepath.Join(libDir, "libfoo.so"))
	require.NoError(t, err)
	f.Close()

	// When: Creating cache
	cache := NewFallbackLDCache(tmpDir, []string{"/usr/lib64"})
	_, libs64 := cache.List()

	// Then: Should only contain files, not directories
	assert.Len(t, libs64, 1)
}

func TestFallbackLDCache_Scan_NonExistentPath(t *testing.T) {
	// Given: A search path that doesn't exist
	tmpDir := t.TempDir()

	// When: Creating cache with non-existent search path
	cache := NewFallbackLDCache(tmpDir, []string{"/nonexistent/path"})
	_, libs64 := cache.List()

	// Then: Should return empty list without error
	assert.Empty(t, libs64)
}

func TestFallbackLDCache_Scan_FiltersNonLibraries(t *testing.T) {
	// Given: A directory with mixed files
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	files := []string{
		"libfoo.so",     // Valid: .so suffix
		"libbar.so.1",   // Valid: contains .so.
		"libbaz.so.2.0", // Valid: contains .so.
		"readme.txt",    // Invalid: not a library
		"config.yml",    // Invalid: not a library
		"binary",        // Invalid: no .so
	}
	for _, file := range files {
		f, err := os.Create(filepath.Join(libDir, file))
		require.NoError(t, err)
		f.Close()
	}

	// When: Creating cache
	cache := NewFallbackLDCache(tmpDir, []string{"/usr/lib64"})
	_, libs64 := cache.List()

	// Then: Should only contain .so files
	assert.Len(t, libs64, 3)
}

func TestLDCache_List_SeparatesArchitectures(t *testing.T) {
	// Given: A mock ldcache with 32-bit and 64-bit libraries
	cache := &ldcache{
		root: "/",
		libraries: []ldcacheEntry{
			{name: "lib32.so", path: "/usr/lib/lib32.so", is64bit: false},
			{name: "lib64a.so", path: "/usr/lib64/lib64a.so", is64bit: true},
			{name: "lib64b.so", path: "/usr/lib64/lib64b.so", is64bit: true},
		},
	}

	// When: Listing libraries
	libs32, libs64 := cache.List()

	// Then: Should separate by architecture
	assert.Len(t, libs32, 1)
	assert.Len(t, libs64, 2)
	assert.Contains(t, libs32, "/usr/lib/lib32.so")
	assert.Contains(t, libs64, "/usr/lib64/lib64a.so")
	assert.Contains(t, libs64, "/usr/lib64/lib64b.so")
}

func TestNewLDCache_ValidRoot(t *testing.T) {
	// Given: A valid root directory
	tmpDir := t.TempDir()

	// When: Creating LDCache with valid root
	cache, err := NewLDCache(tmpDir)

	// Then: Should return cache without error
	require.NoError(t, err)
	assert.NotNil(t, cache)
}
