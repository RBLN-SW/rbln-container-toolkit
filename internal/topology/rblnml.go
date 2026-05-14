//go:build with_rblnml

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
	"errors"
	"fmt"

	"github.com/RBLN-SW/go-rbln-ml/rblnml"
)

// LoadRblnmlResolver builds an RsdResolver by walking every NPU device via
// the rblnml C bindings and recording the GroupID each one belongs to. The
// driver handles are opened just long enough to take this snapshot — we call
// rblnmlInit / rblnmlShutdown synchronously inside this function so the
// returned resolver carries only an in-memory map. Holding /dev/rbln* opens
// across the daemon's regen loop would block other consumers and was
// explicitly rejected in the design.
//
// Failures during init or DeviceGetCount return an error so the caller can
// degrade to NoopResolver{} with a user-visible warning. Per-device errors
// (DeviceGetHandleByIndex / GetDeviceInfo) are recorded but don't abort the
// whole load — the resulting resolver just won't have a mapping for the
// affected NPU, which the generator handles by emitting an NPU-only entry.
func LoadRblnmlResolver() (RsdResolver, error) {
	r, err := rblnml.New()
	if err != nil {
		return nil, fmt.Errorf("rblnmlInit: %w", err)
	}
	defer func() { _ = r.Shutdown() }()

	count, err := r.DeviceGetCount()
	if err != nil {
		return nil, fmt.Errorf("rblnmlDeviceGetCount: %w", err)
	}

	mapping := make(map[uint32]uint32, count)
	var perDeviceErrs []error
	for i := uint32(0); i < count; i++ {
		handle, err := r.DeviceGetHandleByIndex(i)
		if err != nil {
			perDeviceErrs = append(perDeviceErrs, fmt.Errorf("DeviceGetHandleByIndex(%d): %w", i, err))
			continue
		}
		info, err := r.GetDeviceInfo(handle)
		if err != nil {
			perDeviceErrs = append(perDeviceErrs, fmt.Errorf("GetDeviceInfo(%d): %w", i, err))
			continue
		}
		mapping[i] = info.GroupID
	}

	resolver := NewCachedResolver(mapping)
	if len(perDeviceErrs) > 0 {
		// Return the resolver alongside an error so callers can still use the
		// partial mapping but surface a warning for the NPUs we couldn't read.
		// loadError carries the per-NPU failure count so LoadOrFallbackWithStats
		// can populate LoadStats.FailedNPUs without re-parsing the joined chain.
		return resolver, loadError{
			Failed: len(perDeviceErrs),
			Cause:  fmt.Errorf("partial RSD mapping: %w", errors.Join(perDeviceErrs...)),
		}
	}
	return resolver, nil
}
