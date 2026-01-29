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

package daemon

import (
	"fmt"
	"time"
)

// RuntimeType represents a container runtime type.
type RuntimeType string

const (
	// RuntimeContainerd represents the containerd runtime.
	RuntimeContainerd RuntimeType = "containerd"
	// RuntimeCRIO represents the CRI-O runtime.
	RuntimeCRIO RuntimeType = "crio"
	// RuntimeDocker represents the Docker runtime.
	RuntimeDocker RuntimeType = "docker"
)

// socketPaths maps runtime types to their socket paths.
var socketPaths = map[RuntimeType]string{
	RuntimeContainerd: "/run/containerd/containerd.sock",
	RuntimeCRIO:       "/var/run/crio/crio.sock",
	RuntimeDocker:     "/var/run/docker.sock",
}

// validRuntimes is the set of valid runtime types.
var validRuntimes = map[RuntimeType]bool{
	RuntimeContainerd: true,
	RuntimeCRIO:       true,
	RuntimeDocker:     true,
	RuntimeType(""):   true, // empty means auto-detect
}

// IsValid returns true if the runtime type is valid.
func (r RuntimeType) IsValid() bool {
	return validRuntimes[r]
}

// SocketPath returns the socket path for this runtime.
func (r RuntimeType) SocketPath() string {
	return socketPaths[r]
}

// Config holds configuration for the daemon process.
type Config struct {
	// Runtime is the target container runtime (empty for auto-detect).
	Runtime RuntimeType

	// ShutdownTimeout is the graceful shutdown timeout.
	ShutdownTimeout time.Duration

	// PidFile is the PID file location.
	PidFile string

	// HealthPort is the health check HTTP port.
	HealthPort int

	// NoCleanupOnExit skips cleanup on exit (debug mode).
	NoCleanupOnExit bool

	// HostRootMount is the host root mount path.
	HostRootMount string

	// DriverRoot is the driver installation path (host perspective).
	// Used for CDI spec hostPath generation.
	DriverRoot string

	// CDISpecDir is the CDI spec output directory.
	CDISpecDir string

	// ContainerLibraryPath is the container library path for isolation.
	// When set, libraries are mounted to this path and LD_LIBRARY_PATH is configured.
	ContainerLibraryPath string

	// Socket is the runtime socket path.
	// When empty, auto-detection is used based on runtime type.
	Socket string

	// Debug enables debug logging.
	Debug bool

	// Force terminates existing daemon instance before starting.
	Force bool
}

// NewDaemonConfig creates a new DaemonConfig with default values.
func NewDaemonConfig() *Config {
	return &Config{
		Runtime:              RuntimeType(""), // auto-detect
		ShutdownTimeout:      30 * time.Second,
		PidFile:              "/run/rbln/toolkit.pid",
		HealthPort:           8080,
		NoCleanupOnExit:      false,
		HostRootMount:        "/host",
		DriverRoot:           "/",
		CDISpecDir:           "/var/run/cdi",
		ContainerLibraryPath: "",
		Socket:               "",
		Debug:                false,
	}
}

// Validate validates the daemon configuration.
func (c *Config) Validate() error {
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("shutdown timeout must be greater than 0")
	}

	if c.ShutdownTimeout > 300*time.Second {
		return fmt.Errorf("shutdown timeout must be <= 300s")
	}

	if c.HealthPort < 1 || c.HealthPort > 65535 {
		return fmt.Errorf("health port must be between 1 and 65535")
	}

	if !c.Runtime.IsValid() {
		return fmt.Errorf("invalid runtime: %s", c.Runtime)
	}

	if c.PidFile == "" {
		return fmt.Errorf("pid file path is required")
	}

	return nil
}
