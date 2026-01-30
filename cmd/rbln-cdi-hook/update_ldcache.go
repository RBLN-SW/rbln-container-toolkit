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
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/ldconfig"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/oci"
)

var updateLdcacheCmd = &cobra.Command{
	Use:   "update-ldcache",
	Short: "Update ldcache in a container by running ldconfig",
	Long: `Updates the ldcache in the container's root filesystem by:
1. Reading the OCI container state from STDIN (or specified file)
2. Extracting the container root filesystem path
3. Creating /etc/ld.so.conf.d/00-rbln-*.conf with specified library directories
4. Running ldconfig to update the cache

This command is designed to be used as a CDI createContainer hook.`,
	RunE: runUpdateLdcache,
}

func init() {
	rootCmd.AddCommand(updateLdcacheCmd)

	updateLdcacheCmd.Flags().StringSlice("folder", []string{}, "Library directory to add to ldcache (can be specified multiple times) [$RBLN_CDI_HOOK_FOLDER]")
	updateLdcacheCmd.Flags().String("ldconfig-path", "/sbin/ldconfig", "Path to the ldconfig binary [$RBLN_CDI_HOOK_LDCONFIG_PATH]")
	updateLdcacheCmd.Flags().String("container-spec", "", "Path to the OCI container spec file (default: STDIN) [$RBLN_CDI_HOOK_CONTAINER_SPEC]")

	_ = viper.BindPFlag("folder", updateLdcacheCmd.Flags().Lookup("folder"))
	_ = viper.BindPFlag("ldconfig-path", updateLdcacheCmd.Flags().Lookup("ldconfig-path"))
	_ = viper.BindPFlag("container-spec", updateLdcacheCmd.Flags().Lookup("container-spec"))
}

type updateLdcacheOptions struct {
	folders       []string
	ldconfigPath  string
	containerSpec string
}

type stateLoader func(filename string) (*oci.State, error)

type ldconfigRunnerFactory func(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error)

type cmdRunner func(cmd *exec.Cmd) error

func runUpdateLdcache(_ *cobra.Command, _ []string) error {
	opts := updateLdcacheOptions{
		folders:       viper.GetStringSlice("folder"),
		ldconfigPath:  viper.GetString("ldconfig-path"),
		containerSpec: viper.GetString("container-spec"),
	}

	return executeUpdateLdcache(opts, oci.LoadContainerState, ldconfig.NewRunner, ldconfig.Run)
}

func executeUpdateLdcache(opts updateLdcacheOptions, loadState stateLoader, newRunner ldconfigRunnerFactory, runCmd cmdRunner) error {
	state, err := loadState(opts.containerSpec)
	if err != nil {
		return fmt.Errorf("failed to load container state: %w", err)
	}

	containerRoot, err := state.GetContainerRoot()
	if err != nil {
		return fmt.Errorf("failed to determine container root: %w", err)
	}

	runner, err := newRunner(opts.ldconfigPath, containerRoot, opts.folders...)
	if err != nil {
		return fmt.Errorf("failed to create ldconfig runner: %w", err)
	}

	if err := runCmd(runner); err != nil {
		return fmt.Errorf("ldconfig execution failed: %w", err)
	}

	return nil
}
