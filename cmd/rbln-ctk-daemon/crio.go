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
	"fmt"

	"github.com/spf13/cobra"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/installer"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
)

func newCrioCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "crio",
		Short: "Configure CRI-O for RBLN device support",
	}

	cmd.AddCommand(newCrioSetupCmd())
	cmd.AddCommand(newCrioCleanupCmd())

	return cmd
}

func newCrioSetupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Setup RBLN support for CRI-O",
		Long: `Configure CRI-O for CDI support and restart it.

This command:
1. Creates CRI-O config drop-in for CDI support
2. Generates CDI spec for RBLN devices
3. Restarts CRI-O (via systemctl by default)

Note: CRI-O does not support SIGHUP reload, so systemd restart is used by default.`,
		Example: `  # Basic setup
  sudo rbln-ctk-daemon runtime crio setup

  # Setup in OpenShift
  rbln-ctk-daemon runtime crio setup --host-root-mount=/host`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCrioSetup()
		},
	}
}

func newCrioCleanupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Remove RBLN support from CRI-O",
		Long: `Remove RBLN CDI configuration from CRI-O and restart it.

This command:
1. Removes CDI spec for RBLN devices
2. Removes CRI-O config drop-in
3. Restarts CRI-O`,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCrioCleanup()
		},
	}
}

func runCrioSetup() error {
	mode := getEffectiveRestartMode("crio", restartMode)

	// Apply defaults if not set
	if mode == "" {
		mode = string(restart.RestartModeSystemd)
	}

	// Validate: CRI-O does not support signal restart
	if restart.Mode(mode) == restart.RestartModeSignal {
		return fmt.Errorf("signal restart mode is not supported for CRI-O\n\nCRI-O does not support SIGHUP reload. Use --restart-mode=systemd or --restart-mode=none")
	}

	opts := installer.SetupOptions{
		Runtime:       "crio",
		RestartMode:   restart.Mode(mode),
		Socket:        "", // Not used for CRI-O (systemd only)
		HostRootMount: getEffectiveHostRootMount(),
		CDISpecDir:    getEffectiveCDISpecDir(),
		PidFile:       getEffectivePidFile(),
		DryRun:        dryRun,
		Logger:        &cliLogger{},
	}

	return installer.Setup(opts)
}

func runCrioCleanup() error {
	mode := getEffectiveRestartMode("crio", restartMode)

	// Apply defaults if not set
	if mode == "" {
		mode = string(restart.RestartModeSystemd)
	}

	// Validate: CRI-O does not support signal restart
	if restart.Mode(mode) == restart.RestartModeSignal {
		return fmt.Errorf("signal restart mode is not supported for CRI-O\n\nCRI-O does not support SIGHUP reload. Use --restart-mode=systemd or --restart-mode=none")
	}

	opts := installer.CleanupOptions{
		Runtime:       "crio",
		RestartMode:   restart.Mode(mode),
		Socket:        "", // Not used for CRI-O (systemd only)
		HostRootMount: getEffectiveHostRootMount(),
		CDISpecDir:    getEffectiveCDISpecDir(),
		PidFile:       getEffectivePidFile(),
		DryRun:        dryRun,
		Logger:        &cliLogger{},
	}

	return installer.Cleanup(opts)
}
