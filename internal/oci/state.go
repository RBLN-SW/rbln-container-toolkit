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

// Package oci provides OCI container state parsing for CDI hooks.
package oci

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// specFileName is the standard OCI spec filename.
	specFileName = "config.json"
)

// State represents OCI container state passed to hooks via STDIN.
// This is a minimal representation containing only the fields needed for hook execution.
type State struct {
	// Version is the OCI spec version.
	Version string `json:"ociVersion"`

	// ID is the container ID.
	ID string `json:"id"`

	// Status is the container status: creating, created, running, stopped.
	Status string `json:"status"`

	// Pid is the container process PID (0 if not running).
	Pid int `json:"pid,omitempty"`

	// Bundle is the path to bundle directory containing config.json.
	Bundle string `json:"bundle"`

	// Annotations contains optional container annotations.
	Annotations map[string]string `json:"annotations,omitempty"`
}

// minimalSpec extracts only the root configuration from config.json.
type minimalSpec struct {
	Root *Root `json:"root,omitempty"`
}

// Root represents the container's root filesystem.
type Root struct {
	// Path is the path to the container's root filesystem.
	// Can be absolute or relative to bundle directory.
	Path string `json:"path"`

	// Readonly indicates whether the root is read-only.
	Readonly bool `json:"readonly,omitempty"`
}

// LoadContainerState loads the container state from the specified filename.
// If the filename is empty or "-", the state is loaded from STDIN.
func LoadContainerState(filename string) (*State, error) {
	if filename == "" || filename == "-" {
		return ReadContainerState(os.Stdin)
	}

	inputFile, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer inputFile.Close()

	return ReadContainerState(inputFile)
}

// ReadContainerState reads the container state from the specified reader.
func ReadContainerState(reader io.Reader) (*State, error) {
	var s State

	d := json.NewDecoder(reader)
	if err := d.Decode(&s); err != nil {
		return nil, fmt.Errorf("failed to decode container state: %v", err)
	}

	return &s, nil
}

// GetContainerRoot returns the root filesystem path for the container.
// It loads the minimal spec from config.json and resolves the root path.
// Returns an error if:
// - Bundle path is empty
// - The resolved container root is "/" (system root) or empty
func (s *State) GetContainerRoot() (string, error) {
	// Validate bundle path is not empty
	if s.Bundle == "" {
		return "", fmt.Errorf("bundle path is empty")
	}

	spec, err := s.loadMinimalSpec()
	if err != nil {
		return "", err
	}

	var containerRoot string
	if spec.Root != nil {
		containerRoot = spec.Root.Path
	}

	// Resolve path
	var resolvedRoot string
	if filepath.IsAbs(containerRoot) {
		resolvedRoot = containerRoot
	} else {
		resolvedRoot = filepath.Join(s.Bundle, containerRoot)
	}

	// Validate resolved root is not "/" (system root)
	if resolvedRoot == "/" {
		return "", fmt.Errorf("invalid container root: path is system root")
	}

	// Validate resolved root is not empty
	if resolvedRoot == "" {
		return "", fmt.Errorf("invalid container root: path is empty")
	}

	return resolvedRoot, nil
}

// loadMinimalSpec loads a reduced OCI spec from the bundle's config.json.
func (s *State) loadMinimalSpec() (*minimalSpec, error) {
	specFilePath := GetSpecFilePath(s.Bundle)
	specFile, err := os.Open(specFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open OCI spec file: %v", err)
	}
	defer specFile.Close()

	ms := &minimalSpec{}
	if err := json.NewDecoder(specFile).Decode(ms); err != nil {
		return nil, fmt.Errorf("failed to load minimal OCI spec: %v", err)
	}
	return ms, nil
}

// GetSpecFilePath returns the expected path to the OCI specification file
// for the given bundle directory.
func GetSpecFilePath(bundleDir string) string {
	return filepath.Join(bundleDir, specFileName)
}
