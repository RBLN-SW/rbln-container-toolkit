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

package main

import (
	"github.com/spf13/cobra"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/installer"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
)

func newDockerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docker",
		Short: "Configure Docker for RBLN device support",
	}

	cmd.AddCommand(newDockerSetupCmd())
	cmd.AddCommand(newDockerCleanupCmd())

	return cmd
}

func newDockerSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Setup RBLN support for Docker",
		Long: `Configure Docker daemon for CDI support and restart it.

This command:
1. Updates Docker daemon.json to enable CDI
2. Generates CDI spec for RBLN devices
3. Restarts Docker daemon (via SIGHUP by default)`,
		Example: `  # Basic setup
  sudo rbln-ctk-daemon runtime docker setup

  # Setup with systemd restart
  sudo rbln-ctk-daemon runtime docker setup --restart-mode=systemd

  # Setup without restart (for testing)
  rbln-ctk-daemon runtime docker setup --restart-mode=none

  # Setup in Kubernetes DaemonSet
  rbln-ctk-daemon runtime docker setup --host-root-mount=/host`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDockerSetup()
		},
	}
}

func newDockerCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Remove RBLN support from Docker",
		Long: `Remove RBLN CDI configuration from Docker and restart it.

This command:
1. Removes CDI spec for RBLN devices
2. Reverts Docker daemon.json configuration
3. Restarts Docker daemon`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runDockerCleanup()
		},
	}
}

func runDockerSetup() error {
	mode := getEffectiveRestartMode("docker", restartMode)
	socket := getEffectiveSocket("docker", socketPath)

	// Apply defaults if not set
	if mode == "" {
		mode = string(restart.RestartModeSignal)
	}
	if socket == "" {
		socket = "/var/run/docker.sock"
	}

	opts := installer.SetupOptions{
		Runtime:       "docker",
		RestartMode:   restart.Mode(mode),
		Socket:        socket,
		HostRootMount: getEffectiveHostRootMount(),
		CDISpecDir:    getEffectiveCDISpecDir(),
		PidFile:       getEffectivePidFile(),
		DryRun:        dryRun,
		Logger:        &cliLogger{},
	}

	return installer.Setup(opts)
}

func runDockerCleanup() error {
	mode := getEffectiveRestartMode("docker", restartMode)
	socket := getEffectiveSocket("docker", socketPath)

	// Apply defaults if not set
	if mode == "" {
		mode = string(restart.RestartModeSignal)
	}
	if socket == "" {
		socket = "/var/run/docker.sock"
	}

	opts := installer.CleanupOptions{
		Runtime:       "docker",
		RestartMode:   restart.Mode(mode),
		Socket:        socket,
		HostRootMount: getEffectiveHostRootMount(),
		CDISpecDir:    getEffectiveCDISpecDir(),
		PidFile:       getEffectivePidFile(),
		DryRun:        dryRun,
		Logger:        &cliLogger{},
	}

	return installer.Cleanup(opts)
}

// cliLogger implements installer.Logger using CLI output functions.
type cliLogger struct{}

func (l *cliLogger) Info(format string, args ...interface{}) {
	logInfo(format, args...)
}

func (l *cliLogger) Debug(format string, args ...interface{}) {
	logDebug(format, args...)
}

func (l *cliLogger) Warning(format string, args ...interface{}) {
	logWarning(format, args...)
}
