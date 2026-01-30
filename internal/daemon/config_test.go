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

package daemon

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDaemonConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with defaults",
			config: &Config{
				ShutdownTimeout: 30 * time.Second,
				HealthPort:      8080,
				PidFile:         "/run/rbln/toolkit.pid",
				HostRootMount:   "/host",
				CDISpecDir:      "/var/run/cdi",
			},
			wantErr: false,
		},
		{
			name: "valid config with explicit runtime",
			config: &Config{
				Runtime:         RuntimeContainerd,
				ShutdownTimeout: 30 * time.Second,
				HealthPort:      8080,
				PidFile:         "/run/rbln/toolkit.pid",
				HostRootMount:   "/host",
				CDISpecDir:      "/var/run/cdi",
			},
			wantErr: false,
		},
		{
			name: "invalid shutdown timeout - zero",
			config: &Config{
				ShutdownTimeout: 0,
				HealthPort:      8080,
				PidFile:         "/run/rbln/toolkit.pid",
			},
			wantErr: true,
			errMsg:  "shutdown timeout must be greater than 0",
		},
		{
			name: "invalid shutdown timeout - too long",
			config: &Config{
				ShutdownTimeout: 301 * time.Second,
				HealthPort:      8080,
				PidFile:         "/run/rbln/toolkit.pid",
			},
			wantErr: true,
			errMsg:  "shutdown timeout must be <= 300s",
		},
		{
			name: "invalid health port - zero",
			config: &Config{
				ShutdownTimeout: 30 * time.Second,
				HealthPort:      0,
				PidFile:         "/run/rbln/toolkit.pid",
			},
			wantErr: true,
			errMsg:  "health port must be between 1 and 65535",
		},
		{
			name: "invalid health port - too high",
			config: &Config{
				ShutdownTimeout: 30 * time.Second,
				HealthPort:      65536,
				PidFile:         "/run/rbln/toolkit.pid",
			},
			wantErr: true,
			errMsg:  "health port must be between 1 and 65535",
		},
		{
			name: "invalid runtime",
			config: &Config{
				Runtime:         "invalid-runtime",
				ShutdownTimeout: 30 * time.Second,
				HealthPort:      8080,
				PidFile:         "/run/rbln/toolkit.pid",
			},
			wantErr: true,
			errMsg:  "invalid runtime",
		},
		{
			name: "empty pid file",
			config: &Config{
				ShutdownTimeout: 30 * time.Second,
				HealthPort:      8080,
				PidFile:         "",
			},
			wantErr: true,
			errMsg:  "pid file path is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			cfg := tt.config

			// When
			err := cfg.Validate()

			// Then
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDaemonConfig_DefaultValues(t *testing.T) {
	// Given
	// When
	cfg := NewDaemonConfig()

	// Then
	assert.Equal(t, RuntimeType(""), cfg.Runtime)
	assert.Equal(t, 30*time.Second, cfg.ShutdownTimeout)
	assert.Equal(t, 8080, cfg.HealthPort)
	assert.Equal(t, "/run/rbln/toolkit.pid", cfg.PidFile)
	assert.Equal(t, "/host", cfg.HostRootMount)
	assert.Equal(t, "/var/run/cdi", cfg.CDISpecDir)
	assert.False(t, cfg.NoCleanupOnExit)
	assert.False(t, cfg.Debug)
}

func TestRuntimeType_IsValid(t *testing.T) {
	tests := []struct {
		runtime RuntimeType
		valid   bool
	}{
		{RuntimeContainerd, true},
		{RuntimeCRIO, true},
		{RuntimeDocker, true},
		{RuntimeType(""), true},
		{RuntimeType("invalid"), false},
		{RuntimeType("podman"), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			// Given
			rt := tt.runtime

			// When
			result := rt.IsValid()

			// Then
			assert.Equal(t, tt.valid, result)
		})
	}
}

func TestRuntimeType_SocketPath(t *testing.T) {
	tests := []struct {
		runtime    RuntimeType
		socketPath string
	}{
		{RuntimeContainerd, "/run/containerd/containerd.sock"},
		{RuntimeCRIO, "/var/run/crio/crio.sock"},
		{RuntimeDocker, "/var/run/docker.sock"},
		{RuntimeType(""), ""},
	}

	for _, tt := range tests {
		t.Run(string(tt.runtime), func(t *testing.T) {
			// Given
			rt := tt.runtime

			// When
			result := rt.SocketPath()

			// Then
			assert.Equal(t, tt.socketPath, result)
		})
	}
}
