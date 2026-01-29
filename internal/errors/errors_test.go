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

package errors

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorTypes_AreDistinct(t *testing.T) {
	// Given: All error types defined
	allErrors := []error{
		ErrConfigNotFound,
		ErrInvalidConfig,
		ErrNoLibrariesFound,
		ErrNoToolsFound,
		ErrLdcacheParseFailed,
		ErrLddFailed,
		ErrInvalidCDISpec,
		ErrFileNotFound,
		ErrWriteFailed,
		ErrRuntimeNotFound,
		ErrConfigureFailed,
		ErrPermissionDenied,
	}
	seen := make(map[string]bool)

	// When: Checking all errors for distinctness
	for _, err := range allErrors {
		msg := err.Error()

		// Then: All errors should be distinct
		assert.False(t, seen[msg], "duplicate error message: %s", msg)
		seen[msg] = true
	}
}

func TestErrorTypes_CanBeWrapped(t *testing.T) {
	// Given: A base error
	baseErr := ErrNoLibrariesFound

	// When: Wrapping with context
	wrappedErr := fmt.Errorf("discover RBLN libraries: %w", baseErr)

	// Then: Can be unwrapped and identified
	assert.True(t, errors.Is(wrappedErr, ErrNoLibrariesFound))
	assert.Contains(t, wrappedErr.Error(), "discover RBLN libraries")
}

func TestConfigErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "config not found",
			err:      ErrConfigNotFound,
			expected: "configuration file not found",
		},
		{
			name:     "invalid config",
			err:      ErrInvalidConfig,
			expected: "invalid configuration",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := tt.err
			expected := tt.expected

			// When
			result := err.Error()

			// Then
			assert.Equal(t, expected, result)
		})
	}
}

func TestDiscoveryErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "no libraries found",
			err:      ErrNoLibrariesFound,
			expected: "no RBLN libraries found",
		},
		{
			name:     "no tools found",
			err:      ErrNoToolsFound,
			expected: "no tools found",
		},
		{
			name:     "ldcache parse failed",
			err:      ErrLdcacheParseFailed,
			expected: "failed to parse ldcache",
		},
		{
			name:     "ldd failed",
			err:      ErrLddFailed,
			expected: "ldd execution failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := tt.err
			expected := tt.expected

			// When
			result := err.Error()

			// Then
			assert.Equal(t, expected, result)
		})
	}
}

func TestCDIErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "invalid CDI spec",
			err:      ErrInvalidCDISpec,
			expected: "invalid CDI specification",
		},
		{
			name:     "file not found",
			err:      ErrFileNotFound,
			expected: "file not found",
		},
		{
			name:     "write failed",
			err:      ErrWriteFailed,
			expected: "failed to write file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := tt.err
			expected := tt.expected

			// When
			result := err.Error()

			// Then
			assert.Equal(t, expected, result)
		})
	}
}

func TestRuntimeErrors(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "runtime not found",
			err:      ErrRuntimeNotFound,
			expected: "runtime not found",
		},
		{
			name:     "configure failed",
			err:      ErrConfigureFailed,
			expected: "failed to configure runtime",
		},
		{
			name:     "permission denied",
			err:      ErrPermissionDenied,
			expected: "permission denied",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			err := tt.err
			expected := tt.expected

			// When
			result := err.Error()

			// Then
			assert.Equal(t, expected, result)
		})
	}
}
