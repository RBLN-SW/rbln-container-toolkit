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

//go:generate moq -rm -fmt=goimports -stub -out ldd_mock.go . LDDRunner

import (
	"bufio"
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
)

// LDDRunner interface for running ldd commands.
// This allows for easy mocking in tests.
type LDDRunner interface {
	Run(libraryPath string) ([]string, error)
}

// lddRunner implements LDDRunner using the system ldd command.
type lddRunner struct{}

// NewLDDRunner creates a new LDD runner.
func NewLDDRunner() LDDRunner {
	return &lddRunner{}
}

// Run executes ldd on a library and returns resolved dependency paths.
// It parses ldd output like:
//
//	linux-vdso.so.1 (0x00007ffc...)
//	libfoo.so.1 => /lib/x86_64-linux-gnu/libfoo.so.1 (0x00007f...)
//	/lib64/ld-linux-x86-64.so.2 (0x00007f...)
func (r *lddRunner) Run(libraryPath string) ([]string, error) {
	cmd := exec.Command("ldd", libraryPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return parseLDDOutput(output), nil
}

// parseLDDOutput parses ldd output and extracts resolved library paths.
func parseLDDOutput(output []byte) []string {
	paths := make([]string, 0, bytes.Count(output, []byte("\n")))
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		path := extractLibraryPath(line)
		if path == "" {
			continue
		}

		// Deduplicate
		if seen[path] {
			continue
		}
		seen[path] = true

		paths = append(paths, path)
	}

	return paths
}

// extractLibraryPath extracts the resolved library path from an ldd output line.
// Handles three formats:
//  1. "libfoo.so.1 => /path/to/libfoo.so.1 (0x...)" -> "/path/to/libfoo.so.1"
//  2. "linux-vdso.so.1 (0x...)" -> "" (no resolved path, skip)
//  3. "/lib64/ld-linux-x86-64.so.2 (0x...)" -> "/lib64/ld-linux-x86-64.so.2"
func extractLibraryPath(line string) string {
	// Format 1: "libname => /path (0x...)"
	if strings.Contains(line, "=>") {
		parts := strings.Split(line, "=>")
		if len(parts) != 2 {
			return ""
		}

		right := strings.TrimSpace(parts[1])

		// "not found" case
		if strings.Contains(right, "not found") {
			return ""
		}

		// Remove address part "(0x...)"
		if idx := strings.Index(right, "("); idx != -1 {
			right = strings.TrimSpace(right[:idx])
		}

		// Validate it's an absolute path
		if !strings.HasPrefix(right, "/") {
			return ""
		}

		return right
	}

	// Format 3: "/lib64/ld-linux-x86-64.so.2 (0x...)" - absolute path at start
	if strings.HasPrefix(line, "/") {
		// Remove address part
		if idx := strings.Index(line, "("); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		return line
	}

	// Format 2: "linux-vdso.so.1 (0x...)" - virtual library, skip
	return ""
}

// ResolveSymlink resolves a symlink to its real path.
// Returns the original path if it's not a symlink.
func ResolveSymlink(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
