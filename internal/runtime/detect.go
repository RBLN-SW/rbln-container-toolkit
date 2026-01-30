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

package runtime

import (
	"fmt"
	"os"
	"strings"
)

// DetectStrictOptions holds options for strict runtime detection.
type DetectStrictOptions struct {
	// Socket paths for each runtime
	ContainerdSocket string
	CRIOSocket       string
	DockerSocket     string

	// ExplicitRuntime bypasses auto-detection if set.
	// Use this when user specifies --runtime flag explicitly.
	ExplicitRuntime RuntimeType
}

// DetectRuntimeStrict detects the container runtime with strict validation.
// Unlike DetectRuntime which picks by priority, this function:
// - Returns an error if multiple runtimes are detected (unless ExplicitRuntime is set)
// - Returns an error if no runtime is detected
// - Returns the single detected runtime if exactly one is found
//
// This behavior encourages explicit configuration in multi-runtime environments.
func DetectRuntimeStrict(opts *DetectStrictOptions) (RuntimeType, error) {
	if opts == nil {
		opts = &DetectStrictOptions{
			ContainerdSocket: "/run/containerd/containerd.sock",
			CRIOSocket:       "/var/run/crio/crio.sock",
			DockerSocket:     "/var/run/docker.sock",
		}
	}

	// If user explicitly specified a runtime, use it
	if opts.ExplicitRuntime != "" {
		return opts.ExplicitRuntime, nil
	}

	// Detect all available runtimes
	var detected []RuntimeType

	if opts.ContainerdSocket != "" {
		if _, err := os.Stat(opts.ContainerdSocket); err == nil {
			detected = append(detected, RuntimeContainerd)
		}
	}

	if opts.CRIOSocket != "" {
		if _, err := os.Stat(opts.CRIOSocket); err == nil {
			detected = append(detected, RuntimeCRIO)
		}
	}

	if opts.DockerSocket != "" {
		if _, err := os.Stat(opts.DockerSocket); err == nil {
			detected = append(detected, RuntimeDocker)
		}
	}

	switch len(detected) {
	case 0:
		return "", fmt.Errorf("no container runtime detected")
	case 1:
		return detected[0], nil
	default:
		runtimes := make([]string, len(detected))
		for i, rt := range detected {
			runtimes[i] = string(rt)
		}
		return "", fmt.Errorf("multiple runtimes detected: [%s]. Use --runtime flag to specify which one to use", strings.Join(runtimes, ", "))
	}
}
