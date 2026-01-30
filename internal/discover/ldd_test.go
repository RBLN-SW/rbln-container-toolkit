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

func TestNewLDDRunner(t *testing.T) {
	// Given: Nothing (testing factory function)
	// When: Creating a new LDD runner
	runner := NewLDDRunner()

	// Then: Should return non-nil LDDRunner
	assert.NotNil(t, runner)
	_, ok := runner.(*lddRunner)
	assert.True(t, ok, "NewLDDRunner should return *lddRunner")
}

func TestLDDRunner_Run_InvalidPath(t *testing.T) {
	// Given: A non-existent library path
	runner := NewLDDRunner()

	// When: Running ldd on non-existent file
	_, err := runner.Run("/nonexistent/library.so")

	// Then: Should return error
	assert.Error(t, err)
}

func TestResolveSymlink_RegularFile(t *testing.T) {
	// Given: A regular file (not a symlink)
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "regular.txt")
	f, err := os.Create(filePath)
	require.NoError(t, err)
	f.Close()

	// When: Resolving the path
	resolved, err := ResolveSymlink(filePath)

	// Then: Should return resolved path without error (may differ due to symlinks like /var -> /private/var)
	require.NoError(t, err)
	assert.Contains(t, resolved, "regular.txt")
}

func TestResolveSymlink_Symlink(t *testing.T) {
	// Given: A symlink pointing to a real file
	tmpDir := t.TempDir()
	realPath := filepath.Join(tmpDir, "real.so")
	symlinkPath := filepath.Join(tmpDir, "link.so")

	f, err := os.Create(realPath)
	require.NoError(t, err)
	f.Close()

	require.NoError(t, os.Symlink(realPath, symlinkPath))

	// When: Resolving the symlink
	resolved, err := ResolveSymlink(symlinkPath)

	// Then: Should return the real file name (path may differ due to OS symlinks)
	require.NoError(t, err)
	assert.Contains(t, resolved, "real.so")
}

func TestResolveSymlink_ChainedSymlinks(t *testing.T) {
	// Given: A chain of symlinks: link2 -> link1 -> real
	tmpDir := t.TempDir()
	realPath := filepath.Join(tmpDir, "libfoo.so.1.0.0")
	link1Path := filepath.Join(tmpDir, "libfoo.so.1")
	link2Path := filepath.Join(tmpDir, "libfoo.so")

	f, err := os.Create(realPath)
	require.NoError(t, err)
	f.Close()

	require.NoError(t, os.Symlink("libfoo.so.1.0.0", link1Path))
	require.NoError(t, os.Symlink("libfoo.so.1", link2Path))

	// When: Resolving the chain
	resolved, err := ResolveSymlink(link2Path)

	// Then: Should return the real file at the end of the chain
	require.NoError(t, err)
	assert.Contains(t, resolved, "libfoo.so.1.0.0")
}

func TestResolveSymlink_BrokenSymlink(t *testing.T) {
	// Given: A broken symlink
	tmpDir := t.TempDir()
	brokenLink := filepath.Join(tmpDir, "broken.so")
	require.NoError(t, os.Symlink("/nonexistent/target", brokenLink))

	// When: Resolving broken symlink
	_, err := ResolveSymlink(brokenLink)

	// Then: Should return error
	assert.Error(t, err)
}

func TestResolveSymlink_NonExistent(t *testing.T) {
	// Given: A non-existent path
	// When: Resolving non-existent path
	_, err := ResolveSymlink("/nonexistent/path")

	// Then: Should return error
	assert.Error(t, err)
}

func TestParseLDDOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected []string
	}{
		{
			name: "typical ldd output",
			output: `	linux-vdso.so.1 (0x00007ffe79b34000)
	librbln-thunk.so.3 => /lib/librbln-thunk.so.3 (0x000075307584e000)
	libstdc++.so.6 => /lib/x86_64-linux-gnu/libstdc++.so.6 (0x0000753075600000)
	libbz2.so.1.0 => /lib/x86_64-linux-gnu/libbz2.so.1.0 (0x000075307723b000)
	libz.so.1 => /lib/x86_64-linux-gnu/libz.so.1 (0x000075307721f000)
	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x0000753075200000)
	/lib64/ld-linux-x86-64.so.2 (0x000075307725b000)`,
			expected: []string{
				"/lib/librbln-thunk.so.3",
				"/lib/x86_64-linux-gnu/libstdc++.so.6",
				"/lib/x86_64-linux-gnu/libbz2.so.1.0",
				"/lib/x86_64-linux-gnu/libz.so.1",
				"/lib/x86_64-linux-gnu/libc.so.6",
				"/lib64/ld-linux-x86-64.so.2",
			},
		},
		{
			name: "with not found",
			output: `	libfoo.so => not found
	libbar.so.1 => /lib/libbar.so.1 (0x00007f...)`,
			expected: []string{
				"/lib/libbar.so.1",
			},
		},
		{
			name:     "empty output",
			output:   "",
			expected: nil,
		},
		{
			name: "only virtual libraries",
			output: `	linux-vdso.so.1 (0x00007ffe...)
	linux-gate.so.1 (0x00007ffe...)`,
			expected: nil,
		},
		{
			name: "duplicate libraries",
			output: `	libfoo.so.1 => /lib/libfoo.so.1 (0x00007f...)
	libfoo.so.1 => /lib/libfoo.so.1 (0x00007f...)`,
			expected: []string{
				"/lib/libfoo.so.1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			result := parseLDDOutput([]byte(tt.output))

			// Then
			assert.Len(t, result, len(tt.expected))
			for i, path := range result {
				assert.Equal(t, tt.expected[i], path)
			}
		})
	}
}

func TestExtractLibraryPath(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected string
	}{
		{
			name:     "arrow format with address",
			line:     "libfoo.so.1 => /lib/x86_64-linux-gnu/libfoo.so.1 (0x00007f1234567000)",
			expected: "/lib/x86_64-linux-gnu/libfoo.so.1",
		},
		{
			name:     "arrow format without address",
			line:     "libfoo.so.1 => /lib/libfoo.so.1",
			expected: "/lib/libfoo.so.1",
		},
		{
			name:     "not found",
			line:     "libfoo.so => not found",
			expected: "",
		},
		{
			name:     "absolute path at start",
			line:     "/lib64/ld-linux-x86-64.so.2 (0x00007f...)",
			expected: "/lib64/ld-linux-x86-64.so.2",
		},
		{
			name:     "virtual library (no path)",
			line:     "linux-vdso.so.1 (0x00007ffe...)",
			expected: "",
		},
		{
			name:     "empty line",
			line:     "",
			expected: "",
		},
		{
			name:     "whitespace only",
			line:     "   ",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			result := extractLibraryPath(tt.line)

			// Then
			assert.Equal(t, tt.expected, result)
		})
	}
}
