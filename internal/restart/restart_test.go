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

package restart

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRestartMode_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		given    Mode
		expected bool
	}{
		{
			name:     "signal mode",
			given:    RestartModeSignal,
			expected: true,
		},
		{
			name:     "systemd mode",
			given:    RestartModeSystemd,
			expected: true,
		},
		{
			name:     "none mode",
			given:    RestartModeNone,
			expected: true,
		},
		{
			name:     "invalid mode",
			given:    Mode("invalid"),
			expected: false,
		},
		{
			name:     "empty mode",
			given:    Mode(""),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			result := tt.given.IsValid()

			// Then
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestValidRestartModes(t *testing.T) {
	t.Run("returns all three modes", func(t *testing.T) {
		// When
		modes := ValidRestartModes()

		// Then
		assert.Len(t, modes, 3)

		expected := map[Mode]bool{
			RestartModeSignal:  false,
			RestartModeSystemd: false,
			RestartModeNone:    false,
		}

		for _, m := range modes {
			assert.Contains(t, expected, m)
			expected[m] = true
		}

		for m, found := range expected {
			assert.True(t, found, "Mode %s not found in result", m)
		}
	})
}

func TestDefaultOptions(t *testing.T) {
	t.Run("returns expected defaults", func(t *testing.T) {
		// When
		opts := DefaultOptions()

		// Then
		assert.Equal(t, RestartModeSignal, opts.Mode)
		assert.Equal(t, 3, opts.MaxRetries)
		assert.Equal(t, 5*time.Second, opts.RetryBackoff)
		assert.Equal(t, 30*time.Second, opts.Timeout)
	})
}

func TestGetRuntimeDefaults(t *testing.T) {
	tests := []struct {
		name            string
		runtime         string
		expectedMode    Mode
		expectedSocket  string
		expectedService string
	}{
		{
			name:            "containerd runtime",
			runtime:         "containerd",
			expectedMode:    RestartModeSignal,
			expectedSocket:  "/run/containerd/containerd.sock",
			expectedService: "containerd",
		},
		{
			name:            "docker runtime",
			runtime:         "docker",
			expectedMode:    RestartModeSignal,
			expectedSocket:  "/var/run/docker.sock",
			expectedService: "docker",
		},
		{
			name:            "crio runtime",
			runtime:         "crio",
			expectedMode:    RestartModeSystemd,
			expectedSocket:  "/var/run/crio/crio.sock",
			expectedService: "crio",
		},
		{
			name:            "unknown runtime",
			runtime:         "unknown-runtime",
			expectedMode:    RestartModeSystemd,
			expectedSocket:  "",
			expectedService: "unknown-runtime",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			defaults := GetRuntimeDefaults(tt.runtime)

			// Then
			assert.Equal(t, tt.expectedMode, defaults.Mode)
			assert.Equal(t, tt.expectedSocket, defaults.Socket)
			assert.Equal(t, tt.expectedService, defaults.Service)
		})
	}
}

func TestNewRestarter(t *testing.T) {
	t.Run("returns NoneRestarter for none mode", func(t *testing.T) {
		// Given
		opts := Options{Mode: RestartModeNone}

		// When
		restarter, err := NewRestarter(opts)

		// Then
		assert.NoError(t, err)
		assert.IsType(t, (*NoneRestarter)(nil), restarter)
	})

	t.Run("returns SystemdRestarter for systemd mode", func(t *testing.T) {
		// Given
		opts := Options{Mode: RestartModeSystemd}

		// When
		restarter, err := NewRestarter(opts)

		// Then
		assert.NoError(t, err)
		assert.IsType(t, (*SystemdRestarter)(nil), restarter)
	})

	t.Run("returns SignalRestarter for signal mode with socket or error on non-Linux", func(t *testing.T) {
		// Given
		opts := Options{
			Mode:   RestartModeSignal,
			Socket: "/var/run/docker.sock",
		}

		// When
		restarter, err := NewRestarter(opts)

		// Then
		if err != nil {
			t.Logf("Signal mode not supported on this platform: %v", err)
			return
		}
		assert.NotNil(t, restarter)
	})

	t.Run("returns error for signal mode without socket", func(t *testing.T) {
		// Given
		opts := Options{Mode: RestartModeSignal}

		// When
		restarter, err := NewRestarter(opts)

		// Then
		assert.Error(t, err)
		assert.Nil(t, restarter)
	})

	t.Run("returns error for invalid mode", func(t *testing.T) {
		// Given
		opts := Options{Mode: Mode("invalid")}

		// When
		restarter, err := NewRestarter(opts)

		// Then
		assert.Error(t, err)
		assert.Nil(t, restarter)
	})
}
