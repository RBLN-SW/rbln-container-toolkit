//go:build !with_rblnml

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

// Stub-build tests for the rblnml-backed resolver. The tagged build replaces
// LoadRblnmlResolver with a cgo implementation whose behavior depends on the
// presence of /dev/rbln* and librbln-ml; that path has its own gated tests.

package topology

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadRblnmlResolver_StubReturnsUnavailable(t *testing.T) {
	// Given the pure-Go build (default, no with_rblnml tag). LoadRblnmlResolver
	// must surface a typed sentinel so callers can branch on it cleanly when
	// deciding whether to log a warning vs. surface a hard error.
	resolver, err := LoadRblnmlResolver()

	require.Error(t, err)
	assert.Nil(t, resolver, "stub must not return a partial resolver")
	assert.True(t, errors.Is(err, ErrRblnmlUnavailable),
		"stub LoadRblnmlResolver must return ErrRblnmlUnavailable")
}

func TestLoadOrFallback_StubLogsAndFallsBackToNoop(t *testing.T) {
	// Given stub mode, LoadOrFallback always falls back. Verify both that
	// the warn callback fires (operator-visible) and that the returned
	// resolver is NoopResolver-equivalent.
	var warned string
	r := LoadOrFallback(func(format string, _ ...any) {
		warned = format
	})

	assert.NotEmpty(t, warned, "warn callback must fire with a diagnostic")
	_, ok := r.Resolve(0)
	assert.False(t, ok, "fallback must report no mapping")
}

func TestLoadOrFallback_NilWarnCallbackTolerated(t *testing.T) {
	// Defensive: callers that haven't set up a logger yet shouldn't crash
	// the resolver construction path.
	r := LoadOrFallback(nil)
	_, ok := r.Resolve(0)
	assert.False(t, ok)
}

func TestLoadOrFallbackWithStats_StubReportsFallback(t *testing.T) {
	// Stub-build load always fails → fallback to Noop, stats.Fallback=true.
	// Operators rely on this flag to distinguish "load failed → no RSD" from
	// "load succeeded but host has no NPUs" (both produce empty mappings).
	r, stats := LoadOrFallbackWithStats(nil)

	_, ok := r.Resolve(0)
	assert.False(t, ok)
	assert.True(t, stats.Fallback,
		"stub build must mark stats.Fallback so callers can correlate it with the warning")
	assert.Equal(t, 0, stats.MappedNPUs)
	assert.Equal(t, 0, stats.FailedNPUs)
}

func TestLoadStats_String_FallbackFormatting(t *testing.T) {
	// The string form is what the daemon logs; pin the format so a future
	// refactor doesn't accidentally change operator-facing log lines.
	s := LoadStats{Fallback: true}
	assert.Contains(t, s.String(), "fallback")
	assert.Contains(t, s.String(), "no-op")
}

func TestLoadStats_String_SuccessFormatting(t *testing.T) {
	s := LoadStats{MappedNPUs: 4, FailedNPUs: 1}
	str := s.String()
	assert.Contains(t, str, "4 NPU")
	assert.Contains(t, str, "1 failed")
	assert.NotContains(t, str, "fallback")
}
