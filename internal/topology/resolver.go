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

//go:generate moq -rm -fmt=goimports -stub -out resolver_mock.go . RsdResolver

import (
	"errors"
	"fmt"
	"time"
)

// RsdResolver maps an NPU device index to its assigned RSD group index.
// Implementations may consult the driver (via librbln-ml), sysfs, or return a
// "no mapping" answer for environments without driver access.
//
// The resolver is the only piece of CTK that needs to talk to the driver to
// answer "which RSD group does NPU N belong to?" — keeping it behind an
// interface lets the rest of the CDI pipeline stay pure-Go and lets tests
// exercise the generator with deterministic mock topologies.
type RsdResolver interface {
	// Resolve returns the RSD group index for the given NPU index. ok=false
	// signals that the mapping is unknown — the caller treats this as "no
	// RSD attachment for this NPU" and emits a per-NPU CDI entry containing
	// only the rbln device node.
	Resolve(npuIndex uint32) (rsdIndex uint32, ok bool)
}

// NoopResolver is the zero-value resolver: it answers "no mapping" for every
// NPU. Used as the default when callers don't supply one (e.g., the
// pure-Go stub build, the K8s path where device-plugin owns allocation, or
// any test that doesn't care about RSD topology). Production builds plug in
// a real librbln-ml-backed resolver via internal/topology/rblnml.
type NoopResolver struct{}

// Resolve always returns ok=false — the NoopResolver carries no topology.
func (NoopResolver) Resolve(uint32) (uint32, bool) { return 0, false }

// cachedResolver is the simple map-backed implementation that production
// loaders (librbln-ml, future sysfs) and tests share. Keeping it
// unexported funnels construction through NewCachedResolver so the data
// stays immutable after the snapshot is taken.
type cachedResolver struct {
	mapping map[uint32]uint32
}

// Resolve returns the cached RSD group index for the given NPU.
func (c cachedResolver) Resolve(npu uint32) (uint32, bool) {
	rsd, ok := c.mapping[npu]
	return rsd, ok
}

// NewCachedResolver wraps an NPU→RSD index map as an RsdResolver. Used by
// the librbln-ml-backed loader after it walks the device list, and by
// integration tests that need a deterministic topology without cgo. Passing
// nil yields a resolver equivalent to NoopResolver{}, which keeps callers
// from having to special-case the "no devices discovered" path.
func NewCachedResolver(mapping map[uint32]uint32) RsdResolver {
	if len(mapping) == 0 {
		return NoopResolver{}
	}
	// Defensive copy so later mutations to the caller's map don't bleed into
	// the resolver. Cheap given the typical scale (≤ a few dozen NPUs).
	copied := make(map[uint32]uint32, len(mapping))
	for k, v := range mapping {
		copied[k] = v
	}
	return cachedResolver{mapping: copied}
}

// LoadStats records what happened during a resolver load so callers can
// surface it in operator-visible logs without re-walking the resolver's
// internals. Empty values mean "didn't run" — e.g., the stub build returns
// zeroed stats alongside its ErrRblnmlUnavailable.
type LoadStats struct {
	// MappedNPUs is the number of NPUs for which a GroupID was successfully
	// recorded. Reads 0 in stub builds, in the K8s path, and when
	// rblnmlInit failed before any device was walked.
	MappedNPUs int
	// FailedNPUs counts per-device failures during the walk (handle / info
	// errors that were swallowed in favor of a partial mapping). A non-zero
	// value indicates the resolver is usable but incomplete.
	FailedNPUs int
	// Duration is the wall-clock time spent in LoadRblnmlResolver, including
	// rblnmlInit + walk + rblnmlShutdown. Helps spot rblnml regressions in
	// the daemon regen path.
	Duration time.Duration
	// Fallback is true when LoadRblnmlResolver returned an error and the
	// caller therefore got NoopResolver{}. Distinct from MappedNPUs==0
	// because a successful load on a 0-NPU host is also empty.
	Fallback bool
}

// String renders LoadStats as a single-line operator-friendly summary.
// Used by the default logging helper; tests rely on this format too.
func (s LoadStats) String() string {
	if s.Fallback {
		return fmt.Sprintf("RSD topology: fallback to no-op resolver (took %s)", s.Duration)
	}
	return fmt.Sprintf("RSD topology: %d NPU(s) mapped, %d failed (took %s)",
		s.MappedNPUs, s.FailedNPUs, s.Duration)
}

// LoadOrFallback attempts to build the rblnml-backed resolver and returns
// NoopResolver{} with a warning when it can't — covers (a) stub builds
// without the with_rblnml tag, (b) librbln-ml unreachable / driver missing,
// and (c) partial-load cases where some NPUs failed but a usable mapping
// was still produced. The warn callback is invoked at most once per call
// with a human-readable diagnostic so callers can route it through their
// preferred logger without dragging a logger interface into this package.
//
// Stats describing the load (NPU count, failure count, duration, fallback
// flag) are accessible via LoadOrFallbackWithStats; this convenience wrapper
// discards them for callers that don't care.
func LoadOrFallback(warn func(format string, args ...any)) RsdResolver {
	r, _ := LoadOrFallbackWithStats(warn)
	return r
}

// LoadOrFallbackWithStats is LoadOrFallback that also reports what happened
// so daemon code can log the outcome (mapped/failed/duration). Stats are
// always populated, even on the fallback path — callers can correlate "RSD
// auto-attachment off" warnings with the underlying cause.
func LoadOrFallbackWithStats(warn func(format string, args ...any)) (RsdResolver, LoadStats) {
	start := time.Now()
	resolver, err := LoadRblnmlResolver()
	stats := LoadStats{Duration: time.Since(start)}
	if err == nil {
		stats.MappedNPUs = countMappings(resolver)
		return resolver, stats
	}
	// Distinguish partial success (some NPUs mapped, some failed during the
	// walk) from total failure: full failure means every per-NPU entry
	// drops RSD, partial means only the unmapped NPUs do. Wording the
	// warning differently lets operators correlate it with `LoadStats` and
	// avoid hunting a "no auto-attach" report when half of their NPUs work.
	if warn != nil {
		if resolver != nil {
			warn("RSD topology resolver: %v — some per-NPU CDI entries may not auto-attach RSD; affected NPUs need `--device rebellions.ai/npu=rsdM` added explicitly", err)
		} else {
			warn("RSD topology resolver: %v — per-NPU CDI entries will not auto-attach RSD; users must add `--device rebellions.ai/npu=rsdM` explicitly", err)
		}
	}
	if resolver != nil {
		// Partial mapping: keep what we got, surface the warning above, and
		// fill in stats so observers see "N mapped, M failed" instead of a
		// silent fallback line.
		stats.MappedNPUs = countMappings(resolver)
		// FailedNPUs comes from the error chain — best-effort, since stub
		// builds also reach this branch via the partial-error path. The
		// rblnml loader sets FailedNPUs via a typed LoadError below; we
		// extract that here without importing the build-tagged file.
		var lerr loadError
		if errors.As(err, &lerr) {
			stats.FailedNPUs = lerr.Failed
		}
		return resolver, stats
	}
	stats.Fallback = true
	return NoopResolver{}, stats
}

// loadError is the error type returned by LoadRblnmlResolver when the walk
// produced a partial mapping. Keeping it unexported here (and constructed
// only by the build-tagged loader) means the stub build can still satisfy
// errors.As checks without needing its own implementation.
type loadError struct {
	Failed int
	Cause  error
}

func (e loadError) Error() string { return e.Cause.Error() }
func (e loadError) Unwrap() error { return e.Cause }

// countMappings reports how many NPUs the resolver knows about. Used to
// populate LoadStats.MappedNPUs without exposing cachedResolver's internals.
// NoopResolver returns 0 by virtue of every Resolve call missing.
func countMappings(r RsdResolver) int {
	if cr, ok := r.(cachedResolver); ok {
		return len(cr.mapping)
	}
	return 0
}
