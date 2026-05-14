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

package topology

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNoopResolver_AlwaysReportsNoMapping(t *testing.T) {
	// Given
	r := NoopResolver{}

	// When / Then: every NPU index returns ok=false so the generator falls
	// back to "rbln node only" entries — the contract that lets pure-Go
	// builds and the K8s path emit a valid (if RSD-less) CDI spec.
	for _, npu := range []uint32{0, 1, 7, 1024} {
		_, ok := r.Resolve(npu)
		assert.False(t, ok, "NoopResolver must never claim a mapping (npu=%d)", npu)
	}
}

func TestNewCachedResolver_ReturnsMappedRSD(t *testing.T) {
	// Given: a deterministic topology — NPU 0 belongs to group 1, NPU 3 to 7.
	r := NewCachedResolver(map[uint32]uint32{0: 1, 3: 7})

	// When / Then: mapped NPUs hit, unmapped miss.
	rsd, ok := r.Resolve(0)
	assert.True(t, ok)
	assert.Equal(t, uint32(1), rsd)

	rsd, ok = r.Resolve(3)
	assert.True(t, ok)
	assert.Equal(t, uint32(7), rsd)

	_, ok = r.Resolve(2)
	assert.False(t, ok, "unmapped NPU must miss")
}

func TestNewCachedResolver_EmptyMappingDegradesToNoop(t *testing.T) {
	// Given: an empty mapping (e.g., rblnmlDeviceGetCount returned 0).
	// We want callers to treat it just like NoopResolver — generator falls
	// back to NPU-only entries everywhere.
	for _, m := range []map[uint32]uint32{nil, {}} {
		r := NewCachedResolver(m)
		_, ok := r.Resolve(0)
		assert.False(t, ok, "empty mapping must report no resolutions")
	}
}

func TestNewCachedResolver_DefensiveCopy(t *testing.T) {
	// Given
	source := map[uint32]uint32{0: 5}
	r := NewCachedResolver(source)

	// When: the caller mutates the source map after construction. This
	// happens in practice if a loader builds the map incrementally and the
	// resolver gets stashed mid-flight.
	source[0] = 99
	source[1] = 100

	// Then: the resolver still reports the original snapshot.
	rsd, ok := r.Resolve(0)
	require.True(t, ok)
	assert.Equal(t, uint32(5), rsd, "post-construction map mutations must not leak in")
	_, ok = r.Resolve(1)
	assert.False(t, ok, "post-construction insertions must not be visible")
}
