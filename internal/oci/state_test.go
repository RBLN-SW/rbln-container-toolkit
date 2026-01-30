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

package oci

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadContainerState(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantVersion string
		wantID      string
		wantStatus  string
		wantBundle  string
		wantPid     int
		wantErr     bool
	}{
		{
			name: "valid state with all fields",
			input: `{
				"ociVersion": "1.0.2",
				"id": "test-container-123",
				"status": "creating",
				"pid": 0,
				"bundle": "/run/containerd/io.containerd.runtime.v2.task/default/test-container-123"
			}`,
			wantVersion: "1.0.2",
			wantID:      "test-container-123",
			wantStatus:  "creating",
			wantBundle:  "/run/containerd/io.containerd.runtime.v2.task/default/test-container-123",
			wantPid:     0,
			wantErr:     false,
		},
		{
			name: "valid state with running container",
			input: `{
				"ociVersion": "1.0.2",
				"id": "running-container",
				"status": "running",
				"pid": 12345,
				"bundle": "/var/lib/containers/storage/overlay-containers/abc/userdata"
			}`,
			wantVersion: "1.0.2",
			wantID:      "running-container",
			wantStatus:  "running",
			wantBundle:  "/var/lib/containers/storage/overlay-containers/abc/userdata",
			wantPid:     12345,
			wantErr:     false,
		},
		{
			name: "minimal state",
			input: `{
				"ociVersion": "1.0.0",
				"id": "minimal",
				"status": "created",
				"bundle": "/tmp/bundle"
			}`,
			wantVersion: "1.0.0",
			wantID:      "minimal",
			wantStatus:  "created",
			wantBundle:  "/tmp/bundle",
			wantPid:     0,
			wantErr:     false,
		},
		{
			name:    "invalid json",
			input:   `{invalid json`,
			wantErr: true,
		},
		{
			name:    "empty input",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			reader := strings.NewReader(tt.input)

			// When
			state, err := ReadContainerState(reader)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if state.Version != tt.wantVersion {
				t.Errorf("Version = %q, want %q", state.Version, tt.wantVersion)
			}
			if state.ID != tt.wantID {
				t.Errorf("ID = %q, want %q", state.ID, tt.wantID)
			}
			if state.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", state.Status, tt.wantStatus)
			}
			if state.Bundle != tt.wantBundle {
				t.Errorf("Bundle = %q, want %q", state.Bundle, tt.wantBundle)
			}
			if state.Pid != tt.wantPid {
				t.Errorf("Pid = %d, want %d", state.Pid, tt.wantPid)
			}
		})
	}
}

func TestGetSpecFilePath(t *testing.T) {
	tests := []struct {
		name      string
		bundleDir string
		want      string
	}{
		{
			name:      "absolute path",
			bundleDir: "/run/containerd/bundle",
			want:      "/run/containerd/bundle/config.json",
		},
		{
			name:      "path with trailing slash",
			bundleDir: "/var/lib/containers/",
			want:      "/var/lib/containers/config.json",
		},
		{
			name:      "relative path",
			bundleDir: "bundle",
			want:      "bundle/config.json",
		},
		{
			name:      "empty path",
			bundleDir: "",
			want:      "config.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			bundleDir := tt.bundleDir

			// When
			got := GetSpecFilePath(bundleDir)

			// Then
			if got != tt.want {
				t.Errorf("GetSpecFilePath(%q) = %q, want %q", bundleDir, got, tt.want)
			}
		})
	}
}

func TestGetContainerRoot(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "oci-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name       string
		bundleDir  string
		configJSON string
		wantRoot   string
		wantErr    bool
	}{
		{
			name:      "absolute root path",
			bundleDir: tmpDir,
			configJSON: `{
				"root": {
					"path": "/var/lib/containers/rootfs"
				}
			}`,
			wantRoot: "/var/lib/containers/rootfs",
			wantErr:  false,
		},
		{
			name:      "relative root path",
			bundleDir: tmpDir,
			configJSON: `{
				"root": {
					"path": "rootfs"
				}
			}`,
			wantRoot: filepath.Join(tmpDir, "rootfs"),
			wantErr:  false,
		},
		{
			name:       "no root in config",
			bundleDir:  tmpDir,
			configJSON: `{}`,
			wantRoot:   tmpDir,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			configPath := filepath.Join(tt.bundleDir, "config.json")
			if err := os.WriteFile(configPath, []byte(tt.configJSON), 0644); err != nil {
				t.Fatalf("failed to write config.json: %v", err)
			}
			defer os.Remove(configPath)

			state := &State{Bundle: tt.bundleDir}

			// When
			root, err := state.GetContainerRoot()

			// Then
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if root != tt.wantRoot {
				t.Errorf("GetContainerRoot() = %q, want %q", root, tt.wantRoot)
			}
		})
	}
}

func TestGetContainerRoot_BundleValidation(t *testing.T) {
	tests := []struct {
		name    string
		bundle  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "empty bundle returns error",
			bundle:  "",
			wantErr: true,
			errMsg:  "bundle path is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			state := &State{Bundle: tt.bundle}

			// When
			_, err := state.GetContainerRoot()

			// Then
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetContainerRoot_RootPathValidation(t *testing.T) {
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "oci-root-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name       string
		configJSON string
		wantErr    bool
		errMsg     string
	}{
		{
			name: "root path is system root",
			configJSON: `{
				"root": {
					"path": "/"
				}
			}`,
			wantErr: true,
			errMsg:  "invalid container root",
		},
		{
			name:       "root path is empty string",
			configJSON: `{"root": {"path": ""}}`,
			wantErr:    false,
		},
		{
			name:       "no root in config",
			configJSON: `{}`,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			configPath := filepath.Join(tmpDir, "config.json")
			if err := os.WriteFile(configPath, []byte(tt.configJSON), 0644); err != nil {
				t.Fatalf("failed to write config.json: %v", err)
			}
			defer os.Remove(configPath)

			state := &State{Bundle: tmpDir}

			// When
			root, err := state.GetContainerRoot()

			// Then
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if root == "" || root == "/" {
				t.Errorf("GetContainerRoot() = %q, should not be empty or /", root)
			}
		})
	}
}

func TestLoadContainerState(t *testing.T) {
	t.Run("loads state from file", func(t *testing.T) {
		// Given
		tmpFile, err := os.CreateTemp("", "state-*.json")
		if err != nil {
			t.Fatalf("failed to create temp file: %v", err)
		}
		defer os.Remove(tmpFile.Name())

		stateJSON := `{
			"ociVersion": "1.0.2",
			"id": "file-test-container",
			"status": "creating",
			"bundle": "/tmp/test-bundle"
		}`

		if _, writeErr := tmpFile.WriteString(stateJSON); writeErr != nil {
			t.Fatalf("failed to write temp file: %v", writeErr)
		}
		tmpFile.Close()

		// When
		state, err := LoadContainerState(tmpFile.Name())

		// Then
		if err != nil {
			t.Errorf("LoadContainerState() error = %v", err)
			return
		}

		if state.ID != "file-test-container" {
			t.Errorf("ID = %q, want %q", state.ID, "file-test-container")
		}
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		// Given
		filePath := "/non/existent/file.json"

		// When
		_, err := LoadContainerState(filePath)

		// Then
		if err == nil {
			t.Error("expected error for non-existent file")
		}
	})
}
