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

// Package restart provides runtime restart functionality.
package restart

//go:generate moq -rm -fmt=goimports -stub -out restart_mock.go . Restarter

import (
	"fmt"
	"time"
)

// Mode defines how to restart a container runtime.
type Mode string

const (
	// RestartModeSignal sends SIGHUP via Unix socket.
	RestartModeSignal Mode = "signal"
	// RestartModeSystemd uses systemctl restart.
	RestartModeSystemd Mode = "systemd"
	// RestartModeNone skips restart (warning only).
	RestartModeNone Mode = "none"
)

// ValidRestartModes returns all valid restart modes.
func ValidRestartModes() []Mode {
	return []Mode{RestartModeSignal, RestartModeSystemd, RestartModeNone}
}

// IsValid checks if the restart mode is valid.
func (m Mode) IsValid() bool {
	switch m {
	case RestartModeSignal, RestartModeSystemd, RestartModeNone:
		return true
	default:
		return false
	}
}

// Options configures the restart operation.
type Options struct {
	Mode          Mode
	Socket        string
	HostRootMount string
	MaxRetries    int
	RetryBackoff  time.Duration
	Timeout       time.Duration
}

// DefaultOptions returns default restart options.
func DefaultOptions() Options {
	return Options{
		Mode:         RestartModeSignal,
		MaxRetries:   3,
		RetryBackoff: 5 * time.Second,
		Timeout:      30 * time.Second,
	}
}

// Restarter performs runtime restart operations.
type Restarter interface {
	// Restart performs the restart operation for the given runtime.
	Restart(runtime string) error
	// DryRun returns a description of what would happen.
	DryRun(runtime string) string
}

// NewRestarter creates a Restarter based on the options.
func NewRestarter(opts Options) (Restarter, error) {
	switch opts.Mode {
	case RestartModeSignal:
		r, err := newSignalRestarter(opts)
		if err != nil {
			return nil, err
		}
		return r, nil
	case RestartModeSystemd:
		return newSystemdRestarter(opts), nil
	case RestartModeNone:
		return newNoneRestarter(), nil
	default:
		return nil, fmt.Errorf("invalid restart mode: %s", opts.Mode)
	}
}

// RuntimeDefaults holds default values for each container runtime.
type RuntimeDefaults struct {
	Mode    Mode
	Socket  string
	Service string
}

// GetRuntimeDefaults returns the default configuration for a runtime.
func GetRuntimeDefaults(runtime string) RuntimeDefaults {
	switch runtime {
	case "containerd":
		return RuntimeDefaults{
			Mode:    RestartModeSignal,
			Socket:  "/run/containerd/containerd.sock",
			Service: "containerd",
		}
	case "docker":
		return RuntimeDefaults{
			Mode:    RestartModeSignal,
			Socket:  "/var/run/docker.sock",
			Service: "docker",
		}
	case "crio":
		return RuntimeDefaults{
			Mode:    RestartModeSystemd,
			Socket:  "/var/run/crio/crio.sock",
			Service: "crio",
		}
	default:
		return RuntimeDefaults{
			Mode:    RestartModeSystemd,
			Service: runtime,
		}
	}
}
