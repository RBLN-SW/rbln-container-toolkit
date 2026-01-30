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
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewELFResolver(t *testing.T) {
	// Given
	driverRoot := "/run/rbln/driver"
	searchPaths := []string{"/usr/lib64", "/lib"}

	// When
	resolver := NewELFResolver(driverRoot, searchPaths)

	// Then
	assert.NotNil(t, resolver)
	_, ok := resolver.(*elfResolver)
	assert.True(t, ok, "NewELFResolver should return *elfResolver")
}

func TestELFResolver_Run_BasicResolution(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	inputLib := filepath.Join(libDir, "librbln-ml.so")
	require.NoError(t, os.WriteFile(inputLib, nil, 0644))

	depLib := filepath.Join(libDir, "libfoo.so.1")
	require.NoError(t, os.WriteFile(depLib, nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
		readNeeded: func(path string) ([]string, error) {
			if path == inputLib {
				return []string{"libfoo.so.1"}, nil
			}
			return nil, nil
		},
	}

	// When
	result, err := resolver.Run(inputLib)

	// Then
	require.NoError(t, err)
	assert.Equal(t, []string{"/usr/lib64/libfoo.so.1"}, result)
}

func TestELFResolver_Run_WithDriverRoot(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "lib", "x86_64-linux-gnu")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	inputLib := filepath.Join(tmpDir, "usr", "lib64", "librbln-ml.so")
	require.NoError(t, os.MkdirAll(filepath.Dir(inputLib), 0755))
	require.NoError(t, os.WriteFile(inputLib, nil, 0644))

	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libbz2.so.1.0"), nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64", "/lib/x86_64-linux-gnu"},
		readNeeded: func(path string) ([]string, error) {
			if path == inputLib {
				return []string{"libbz2.so.1.0"}, nil
			}
			return nil, nil
		},
	}

	// When
	result, err := resolver.Run(inputLib)

	// Then
	require.NoError(t, err)
	assert.Equal(t, []string{"/lib/x86_64-linux-gnu/libbz2.so.1.0"}, result)
}

func TestELFResolver_Run_TransitiveDeps(t *testing.T) {
	// Given: libA needs libB, libB needs libC
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	libA := filepath.Join(libDir, "libA.so")
	require.NoError(t, os.WriteFile(libA, nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libB.so.1"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libC.so.2"), nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
		readNeeded: func(path string) ([]string, error) {
			switch filepath.Base(path) {
			case "libA.so":
				return []string{"libB.so.1"}, nil
			case "libB.so.1":
				return []string{"libC.so.2"}, nil
			default:
				return nil, nil
			}
		},
	}

	// When
	result, err := resolver.Run(libA)

	// Then
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Contains(t, result, "/usr/lib64/libB.so.1")
	assert.Contains(t, result, "/usr/lib64/libC.so.2")
}

func TestELFResolver_Run_CircularDeps(t *testing.T) {
	// Given: libA needs libB, libB needs libA (cycle)
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	libA := filepath.Join(libDir, "libA.so")
	require.NoError(t, os.WriteFile(libA, nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libB.so.1"), nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
		readNeeded: func(path string) ([]string, error) {
			switch filepath.Base(path) {
			case "libA.so":
				return []string{"libB.so.1"}, nil
			case "libB.so.1":
				return []string{"libA.so"}, nil
			default:
				return nil, nil
			}
		},
	}

	// When
	result, err := resolver.Run(libA)

	// Then: should terminate without infinite loop, returning only libB
	require.NoError(t, err)
	assert.Equal(t, []string{"/usr/lib64/libB.so.1"}, result)
}

func TestELFResolver_Run_MissingDep(t *testing.T) {
	// Given: readNeeded returns a library name that doesn't exist on disk
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	inputLib := filepath.Join(libDir, "librbln-ml.so")
	require.NoError(t, os.WriteFile(inputLib, nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
		readNeeded: func(path string) ([]string, error) {
			if path == inputLib {
				return []string{"libmissing.so.1"}, nil
			}
			return nil, nil
		},
	}

	// When
	result, err := resolver.Run(inputLib)

	// Then
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestELFResolver_Run_ReadNeededError(t *testing.T) {
	// Given: readNeeded returns an error
	tmpDir := t.TempDir()

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
		readNeeded: func(_ string) ([]string, error) {
			return nil, errors.New("not an ELF file")
		},
	}

	// When
	result, err := resolver.Run("/some/library.so")

	// Then: graceful degradation, no error propagated
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestELFResolver_Run_DeduplicatesDeps(t *testing.T) {
	// Given: libA needs libB and libC, libB also needs libC (diamond)
	tmpDir := t.TempDir()
	libDir := filepath.Join(tmpDir, "usr", "lib64")
	require.NoError(t, os.MkdirAll(libDir, 0755))

	libA := filepath.Join(libDir, "libA.so")
	require.NoError(t, os.WriteFile(libA, nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libB.so.1"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(libDir, "libC.so.2"), nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
		readNeeded: func(path string) ([]string, error) {
			switch filepath.Base(path) {
			case "libA.so":
				return []string{"libB.so.1", "libC.so.2"}, nil
			case "libB.so.1":
				return []string{"libC.so.2"}, nil
			default:
				return nil, nil
			}
		},
	}

	// When
	result, err := resolver.Run(libA)

	// Then: libC should appear only once
	require.NoError(t, err)
	assert.Len(t, result, 2)

	libCCount := 0
	for _, r := range result {
		if r == "/usr/lib64/libC.so.2" {
			libCCount++
		}
	}
	assert.Equal(t, 1, libCCount, "libC.so.2 should appear exactly once")
}

func TestELFResolver_ResolveLibrary_SearchPathPriority(t *testing.T) {
	// Given: libfoo.so.1 exists in both /usr/lib64 and /lib64
	tmpDir := t.TempDir()
	dir1 := filepath.Join(tmpDir, "usr", "lib64")
	dir2 := filepath.Join(tmpDir, "lib64")
	require.NoError(t, os.MkdirAll(dir1, 0755))
	require.NoError(t, os.MkdirAll(dir2, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "libfoo.so.1"), nil, 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "libfoo.so.1"), nil, 0644))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64", "/lib64"},
	}

	// When
	result := resolver.resolveLibrary("libfoo.so.1")

	// Then: returns the FIRST matching search path
	assert.Equal(t, "/usr/lib64/libfoo.so.1", result)
}

func TestELFResolver_ResolveLibrary_NotFound(t *testing.T) {
	// Given: library doesn't exist in any search path
	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "usr", "lib64"), 0755))

	resolver := &elfResolver{
		driverRoot:  tmpDir,
		searchPaths: []string{"/usr/lib64"},
	}

	// When
	result := resolver.resolveLibrary("libnothere.so")

	// Then
	assert.Equal(t, "", result)
}
