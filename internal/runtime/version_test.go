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

package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantPatch int
		wantOK    bool
	}{
		{"containerd 2.0", "containerd github.com/containerd/containerd/v2 v2.0.0 abc123", 2, 0, 0, true},
		{"containerd 1.7", "containerd github.com/containerd/containerd 1.7.13 def456", 1, 7, 13, true},
		{"docker 28.2", "Docker version 28.2.0, build 1234567", 28, 2, 0, true},
		{"major.minor only", "runtime v3.5 build", 3, 5, 0, true},
		{"no version", "no digits here", 0, 0, 0, false},
		{"empty", "", 0, 0, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, ok := parseVersion(tt.input)
			assert.Equal(t, tt.wantOK, ok)
			if tt.wantOK {
				assert.Equal(t, tt.wantMajor, v.Major)
				assert.Equal(t, tt.wantMinor, v.Minor)
				assert.Equal(t, tt.wantPatch, v.Patch)
			}
		})
	}
}

func TestVersionAtLeast(t *testing.T) {
	tests := []struct {
		v            Version
		major, minor int
		want         bool
	}{
		{Version{2, 0, 0}, 2, 0, true},
		{Version{2, 1, 0}, 2, 0, true},
		{Version{1, 7, 13}, 2, 0, false},
		{Version{3, 0, 0}, 2, 0, true},
		{Version{28, 2, 0}, 28, 2, true},
		{Version{28, 1, 5}, 28, 2, false},
		{Version{27, 9, 9}, 28, 2, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.v.AtLeast(tt.major, tt.minor),
			"%v.AtLeast(%d,%d)", tt.v, tt.major, tt.minor)
	}
}

func TestDetectVersion(t *testing.T) {
	orig := commandRunner
	t.Cleanup(func() { commandRunner = orig })

	t.Run("parses containerd version and chroots for host root", func(t *testing.T) {
		var gotName string
		var gotArgs []string
		commandRunner = func(_ context.Context, name string, args ...string) ([]byte, error) {
			gotName, gotArgs = name, args
			return []byte("containerd github.com/containerd/containerd/v2 v2.0.1 x"), nil
		}

		v, ok := DetectVersion(RuntimeContainerd, "/host")
		assert.True(t, ok)
		assert.Equal(t, 2, v.Major)
		assert.Equal(t, "chroot", gotName)
		assert.Equal(t, []string{"/host", "containerd", "--version"}, gotArgs)
	})

	t.Run("runs directly when host root is /", func(t *testing.T) {
		var gotName string
		commandRunner = func(_ context.Context, name string, _ ...string) ([]byte, error) {
			gotName = name
			return []byte("Docker version 28.3.0, build z"), nil
		}

		v, ok := DetectVersion(RuntimeDocker, "/")
		assert.True(t, ok)
		assert.Equal(t, 28, v.Major)
		assert.Equal(t, "dockerd", gotName)
	})

	t.Run("returns false on command error", func(t *testing.T) {
		commandRunner = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
			return nil, errors.New("binary not found")
		}
		_, ok := DetectVersion(RuntimeContainerd, "/")
		assert.False(t, ok)
	})

	t.Run("returns false for runtime without a version probe", func(t *testing.T) {
		_, ok := DetectVersion(RuntimeCRIO, "/")
		assert.False(t, ok)
	})
}
