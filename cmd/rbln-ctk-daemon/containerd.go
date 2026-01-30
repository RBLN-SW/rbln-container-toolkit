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

func newContainerdCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "containerd",
		Short: "Configure containerd for RBLN device support",
	}

	cmd.AddCommand(newContainerdSetupCmd())
	cmd.AddCommand(newContainerdCleanupCmd())

	return cmd
}

func newContainerdSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Setup RBLN support for containerd",
		Long: `Configure containerd for CDI support and restart it.

This command:
1. Updates containerd config.toml to enable CDI
2. Generates CDI spec for RBLN devices
3. Restarts containerd (via SIGHUP by default)`,
		Example: `  # Basic setup
  sudo rbln-ctk-daemon runtime containerd setup

  # Setup in Kubernetes DaemonSet
  rbln-ctk-daemon runtime containerd setup --host-root-mount=/host

  # Setup with systemd restart
  sudo rbln-ctk-daemon runtime containerd setup --restart-mode=systemd`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runContainerdSetup()
		},
	}
}

func newContainerdCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Remove RBLN support from containerd",
		Long: `Remove RBLN CDI configuration from containerd and restart it.

This command:
1. Removes CDI spec for RBLN devices
2. Reverts containerd config.toml configuration
3. Restarts containerd`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runContainerdCleanup()
		},
	}
}

func runContainerdSetup() error {
	mode := getEffectiveRestartMode("containerd", restartMode)
	socket := getEffectiveSocket("containerd", socketPath)

	// Apply defaults if not set
	if mode == "" {
		mode = string(restart.RestartModeSignal)
	}
	if socket == "" {
		socket = "/run/containerd/containerd.sock"
	}

	opts := installer.SetupOptions{
		Runtime:       "containerd",
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

func runContainerdCleanup() error {
	mode := getEffectiveRestartMode("containerd", restartMode)
	socket := getEffectiveSocket("containerd", socketPath)

	// Apply defaults if not set
	if mode == "" {
		mode = string(restart.RestartModeSignal)
	}
	if socket == "" {
		socket = "/run/containerd/containerd.sock"
	}

	opts := installer.CleanupOptions{
		Runtime:       "containerd",
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
