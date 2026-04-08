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
	"github.com/spf13/viper"
)

// Runtime command flags (shared across runtime subcommands)
var (
	restartMode    string
	hostRootMount  string
	socketPath     string
	cdiSpecDir     string
	runtimeCfgPath string
	pidFile        string
	dryRun         bool
)

func newRuntimeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "runtime",
		Short: "Configure container runtimes for RBLN device support",
		Long:  `Commands for setting up and cleaning up RBLN configuration in container runtimes.`,
	}

	// Shared flags for all runtime subcommands
	cmd.PersistentFlags().StringVar(&restartMode, "restart-mode", "", "Restart mode: signal, systemd, none (default: per-runtime) [$RBLN_CTK_DAEMON_RESTART_MODE]")
	cmd.PersistentFlags().StringVar(&hostRootMount, "host-root-mount", "", "Host root mount path for containerized deployment [$RBLN_CTK_DAEMON_HOST_ROOT_MOUNT]")
	cmd.PersistentFlags().StringVar(&socketPath, "socket", "", "Runtime socket path (default: per-runtime) [$RBLN_CTK_DAEMON_SOCKET]")
	cmd.PersistentFlags().StringVar(&cdiSpecDir, "cdi-spec-dir", "/var/run/cdi", "CDI spec output directory [$RBLN_CTK_DAEMON_CDI_SPEC_DIR]")
	cmd.PersistentFlags().StringVar(&runtimeCfgPath, "config-path", "", "Runtime config path override (default: per-runtime) [$RBLN_CTK_DAEMON_CONFIG_PATH]")
	cmd.PersistentFlags().StringVar(&pidFile, "pid-file", "/run/rbln/toolkit.pid", "PID file path for locking [$RBLN_CTK_DAEMON_PID_FILE]")
	cmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Preview changes without applying them")

	// Bind environment variables
	_ = viper.BindPFlag("restart_mode", cmd.PersistentFlags().Lookup("restart-mode"))
	_ = viper.BindPFlag("host_root_mount", cmd.PersistentFlags().Lookup("host-root-mount"))
	_ = viper.BindPFlag("socket", cmd.PersistentFlags().Lookup("socket"))
	_ = viper.BindPFlag("cdi_spec_dir", cmd.PersistentFlags().Lookup("cdi-spec-dir"))
	_ = viper.BindPFlag("config_path", cmd.PersistentFlags().Lookup("config-path"))
	_ = viper.BindPFlag("pid_file", cmd.PersistentFlags().Lookup("pid-file"))

	// Add runtime-specific subcommands
	cmd.AddCommand(newDockerCmd())
	cmd.AddCommand(newContainerdCmd())
	cmd.AddCommand(newCrioCmd())

	return cmd
}

// getEffectiveRestartMode returns the restart mode to use, applying defaults.
func getEffectiveRestartMode(_, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := viper.GetString("restart_mode"); envValue != "" {
		return envValue
	}
	// Return empty to use runtime default
	return ""
}

// getEffectiveSocket returns the socket path to use, applying defaults.
func getEffectiveSocket(_, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if envValue := viper.GetString("socket"); envValue != "" {
		return envValue
	}
	// Return empty to use runtime default
	return ""
}

// getEffectiveHostRootMount returns the host root mount path.
func getEffectiveHostRootMount() string {
	if hostRootMount != "" {
		return hostRootMount
	}
	return viper.GetString("host_root_mount")
}

// getEffectiveCDISpecDir returns the CDI spec directory.
func getEffectiveCDISpecDir() string {
	if cdiSpecDir != "" {
		return cdiSpecDir
	}
	if envValue := viper.GetString("cdi_spec_dir"); envValue != "" {
		return envValue
	}
	return "/var/run/cdi"
}

// getEffectiveConfigPath returns the runtime config path override.
func getEffectiveConfigPath() string {
	if runtimeCfgPath != "" {
		return runtimeCfgPath
	}
	return viper.GetString("config_path")
}

// getEffectivePidFile returns the PID file path.
func getEffectivePidFile() string {
	if pidFile != "" {
		return pidFile
	}
	if envValue := viper.GetString("pid_file"); envValue != "" {
		return envValue
	}
	return "/run/rbln/toolkit.pid"
}
