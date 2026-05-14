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

// Package topology exposes the NPU↔RSD mapping resolver consumed by the CDI
// generator. The package is intentionally small: one interface
// (RsdResolver), one inert default (NoopResolver), one map-backed
// implementation (cachedResolver via NewCachedResolver), and the
// build-tagged librbln-ml loader (LoadRblnmlResolver). Everything else
// lives in the generator that calls into it.
//
// # Lifecycle
//
// Resolver construction is one-shot. LoadRblnmlResolver opens /dev/rbln* via
// rblnmlInit, walks every NPU to record its GroupID, and calls
// rblnmlShutdown before returning — no device handles survive past the
// function. Callers that need a fresh mapping (e.g., after a driver upgrade
// or RSD reshuffle) simply call LoadRblnmlResolver again. We don't cache
// across Generate() invocations on purpose: stale RSD topology would
// silently produce wrong CDI specs, and the synchronous open / close cost
// is O(NPU count), which is small compared to the rest of CDI generation.
//
// # Concurrency
//
// LoadRblnmlResolver assumes callers serialize regen events. The daemon
// today honors this by running the watcher's OnChange callback inside the
// tick loop (no parallel ticks), and one-shot `rbln-ctk cdi generate`
// invocations naturally serialize. Two concurrent LoadRblnmlResolver calls
// would each attempt rblnmlInit; the second would fail with a device-busy
// error and the caller would fall back to NoopResolver{}. That's a
// graceful-degradation contract — never a panic, never a half-formed
// resolver — but it isn't a substitute for the serialization invariant.
//
// # Failure modes
//
// rblnmlInit can fail when the driver isn't loaded, when another process
// holds an exclusive handle, or when a malformed `/dev/rbln*` entry is
// present. Per-NPU failures inside the walk (GetDeviceInfo) are recorded
// but don't abort the whole load — the resulting resolver simply reports
// ok=false for the affected NPU, which the generator handles by emitting an
// NPU-only CDI entry. Use topology.LoadOrFallback to centralize the
// "real resolver, else NoopResolver with a logged warning" pattern.
package topology
