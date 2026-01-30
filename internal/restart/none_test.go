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

	"github.com/stretchr/testify/assert"
)

func TestNoneRestarter_Restart(t *testing.T) {
	tests := []struct {
		name    string
		runtime string
	}{
		{
			name:    "docker runtime",
			runtime: "docker",
		},
		{
			name:    "containerd runtime",
			runtime: "containerd",
		},
		{
			name:    "crio runtime",
			runtime: "crio",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			restarter := newNoneRestarter()

			// When
			err := restarter.Restart(tt.runtime)

			// Then
			assert.NoError(t, err)
		})
	}
}

func TestNoneRestarter_DryRun(t *testing.T) {
	tests := []struct {
		name            string
		runtime         string
		expectedContain string
	}{
		{
			name:            "docker runtime",
			runtime:         "docker",
			expectedContain: "docker",
		},
		{
			name:            "containerd runtime",
			runtime:         "containerd",
			expectedContain: "restart-mode=none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			restarter := newNoneRestarter()

			// When
			result := restarter.DryRun(tt.runtime)

			// Then
			assert.Contains(t, result, tt.expectedContain)
		})
	}
}

func TestNoneRestarter_SkipMessage(t *testing.T) {
	tests := []struct {
		name             string
		runtime          string
		expectedContains []string
	}{
		{
			name:    "docker runtime",
			runtime: "docker",
			expectedContains: []string{
				"Restart skipped",
				"systemctl restart docker",
			},
		},
		{
			name:    "crio runtime",
			runtime: "crio",
			expectedContains: []string{
				"crio",
				"systemctl restart crio",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			restarter := newNoneRestarter()

			// When
			result := restarter.SkipMessage(tt.runtime)

			// Then
			for _, expected := range tt.expectedContains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

func TestNoneRestarter_ImplementsInterface(t *testing.T) {
	// Given/When
	var restarter Restarter = newNoneRestarter()

	// Then
	assert.NotNil(t, restarter)
}
