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

func TestSystemdRestarter_DryRun(t *testing.T) {
	tests := []struct {
		name             string
		hostRootMount    string
		runtime          string
		expectedContains []string
	}{
		{
			name:          "no host root mount",
			hostRootMount: "",
			runtime:       "docker",
			expectedContains: []string{
				"systemctl restart docker",
			},
		},
		{
			name:          "with host root mount",
			hostRootMount: "/host",
			runtime:       "containerd",
			expectedContains: []string{
				"chroot /host",
				"systemctl restart containerd",
			},
		},
		{
			name:          "crio runtime",
			hostRootMount: "",
			runtime:       "crio",
			expectedContains: []string{
				"systemctl restart crio",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			opts := Options{
				Mode:          RestartModeSystemd,
				HostRootMount: tt.hostRootMount,
			}
			restarter := newSystemdRestarter(opts)

			// When
			result := restarter.DryRun(tt.runtime)

			// Then
			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestSystemdRestarter_DefaultTimeout(t *testing.T) {
	t.Run("uses 30 second default for zero timeout", func(t *testing.T) {
		// Given
		opts := Options{
			Mode:    RestartModeSystemd,
			Timeout: 0,
		}

		// When
		restarter := newSystemdRestarter(opts)

		// Then
		assert.Equal(t, 30*time.Second, restarter.timeout)
	})

	t.Run("uses 30 second default for negative timeout", func(t *testing.T) {
		// Given
		opts := Options{
			Mode:    RestartModeSystemd,
			Timeout: -5 * time.Second,
		}

		// When
		restarter := newSystemdRestarter(opts)

		// Then
		assert.Equal(t, 30*time.Second, restarter.timeout)
	})

	t.Run("uses provided timeout for positive value", func(t *testing.T) {
		// Given
		opts := Options{
			Mode:    RestartModeSystemd,
			Timeout: 60 * time.Second,
		}

		// When
		restarter := newSystemdRestarter(opts)

		// Then
		assert.Equal(t, 60*time.Second, restarter.timeout)
	})
}

func TestSystemdRestarter_HostRootMount(t *testing.T) {
	// Given
	opts := Options{
		Mode:          RestartModeSystemd,
		HostRootMount: "/host",
	}

	// When
	restarter := newSystemdRestarter(opts)

	// Then
	assert.Equal(t, "/host", restarter.hostRootMount)
}

func TestSystemdRestarter_ImplementsInterface(t *testing.T) {
	// Given/When
	var restarter Restarter = newSystemdRestarter(Options{Mode: RestartModeSystemd})

	// Then
	assert.NotNil(t, restarter)
}
