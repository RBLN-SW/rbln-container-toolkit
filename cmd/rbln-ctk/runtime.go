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
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/runtime"
)

var runtimeCmd = &cobra.Command{
	Use:   "runtime",
	Short: "Container runtime configuration",
}

var runtimeConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure container runtime for CDI support",
	Long: `Configure the container runtime to enable CDI (Container Device Interface) support.

This command modifies the runtime configuration to allow containers to access
RBLN devices via CDI. A backup of the original configuration is created automatically.`,
	Example: `  # Auto-detect runtime and configure (requires root)
  sudo rbln-ctk runtime configure

  # Configure specific runtime
  sudo rbln-ctk runtime configure --runtime containerd

  # Preview changes without applying
  rbln-ctk runtime configure --dry-run

  # Use custom config path
  sudo rbln-ctk runtime configure --config-path /custom/path/config.toml`,
	RunE: runRuntimeConfigure,
}

func init() {
	rootCmd.AddCommand(runtimeCmd)

	runtimeCmd.AddCommand(runtimeConfigureCmd)

	runtimeConfigureCmd.Flags().StringP("runtime", "r", "", "Runtime type: containerd, crio, docker (auto-detected if not specified) [$RBLN_CTK_RUNTIME]")
	runtimeConfigureCmd.Flags().String("config-path", "", "Custom runtime config path (uses default if not specified) [$RBLN_CTK_CONFIG_PATH]")
	runtimeConfigureCmd.Flags().Bool("dry-run", false, "Preview changes without applying")
	runtimeConfigureCmd.Flags().Bool("cdi", true, "Enable CDI support")
	runtimeConfigureCmd.Flags().BoolP("yes", "y", false, "Skip confirmation prompt")

	_ = viper.BindPFlag("runtime", runtimeConfigureCmd.Flags().Lookup("runtime"))
	_ = viper.BindPFlag("config-path", runtimeConfigureCmd.Flags().Lookup("config-path"))
	_ = viper.BindPFlag("runtime-dry-run", runtimeConfigureCmd.Flags().Lookup("dry-run"))
	_ = viper.BindPFlag("cdi", runtimeConfigureCmd.Flags().Lookup("cdi"))
	_ = viper.BindPFlag("yes", runtimeConfigureCmd.Flags().Lookup("yes"))
}

type runtimeConfigureOptions struct {
	runtimeType string
	configPath  string
	cdiEnabled  bool
	dryRun      bool
	skipConfirm bool
	quiet       bool
}

type runtimeDetector func() (runtime.RuntimeType, error)

type configuratorFactory func(rt runtime.RuntimeType, configPath string, opts *runtime.ConfiguratorOptions) (runtime.Configurator, error)

func runRuntimeConfigure(_ *cobra.Command, _ []string) error {
	dryRun := viper.GetBool("runtime-dry-run")
	skipConfirm := viper.GetBool("yes")

	opts := runtimeConfigureOptions{
		runtimeType: viper.GetString("runtime"),
		configPath:  viper.GetString("config-path"),
		cdiEnabled:  viper.GetBool("cdi"),
		dryRun:      dryRun,
		skipConfirm: skipConfirm,
		quiet:       viper.GetBool("quiet"),
	}

	return executeRuntimeConfigure(opts, runtime.DetectRuntime, runtime.NewConfigurator, os.Stdout, os.Stdin)
}

func executeRuntimeConfigure(opts runtimeConfigureOptions, detectRuntime runtimeDetector, newConfigurator configuratorFactory, stdout io.Writer, stdin io.Reader) error {
	var rt runtime.RuntimeType
	if opts.runtimeType != "" {
		rt = runtime.RuntimeType(opts.runtimeType)
		if rt != runtime.RuntimeContainerd && rt != runtime.RuntimeCRIO && rt != runtime.RuntimeDocker {
			return fmt.Errorf("unsupported runtime: %s (supported: containerd, crio, docker)", opts.runtimeType)
		}
	} else {
		detected, err := detectRuntime()
		if err != nil {
			return fmt.Errorf("detect runtime: %w (specify --runtime manually)", err)
		}
		rt = detected
		if !opts.quiet {
			fmt.Fprintf(stdout, "Detected runtime: %s\n", rt)
		}
	}

	configPath := opts.configPath
	if configPath == "" {
		configPath = runtime.DefaultConfigPath(rt)
	}

	configuratorOpts := &runtime.ConfiguratorOptions{
		CDIEnabled: opts.cdiEnabled,
	}
	configurator, err := newConfigurator(rt, configPath, configuratorOpts)
	if err != nil {
		return fmt.Errorf("create configurator: %w", err)
	}

	if opts.dryRun {
		diff, err := configurator.DryRun()
		if err != nil {
			return fmt.Errorf("dry run: %w", err)
		}
		fmt.Fprintln(stdout, "Changes that would be made:")
		fmt.Fprintln(stdout, diff)
		return nil
	}

	if !opts.skipConfirm && !opts.quiet {
		fmt.Fprintf(stdout, "This will modify %s. Continue? [y/N] ", configPath)
		reader := bufio.NewReader(stdin)
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(stdout, "Aborted.")
			return nil
		}
	}

	if err := configurator.Configure(); err != nil {
		return fmt.Errorf("configure runtime: %w", err)
	}

	if !opts.quiet {
		fmt.Fprintf(stdout, "Runtime %s configured for CDI support\n", rt)
		fmt.Fprintf(stdout, "Config file: %s\n", configPath)
		fmt.Fprintf(stdout, "Backup created: %s.backup\n", configPath)
		fmt.Fprintln(stdout, "\nRestart the runtime to apply changes:")
		switch rt {
		case runtime.RuntimeContainerd:
			fmt.Fprintln(stdout, "  sudo systemctl restart containerd")
		case runtime.RuntimeCRIO:
			fmt.Fprintln(stdout, "  sudo systemctl restart crio")
		case runtime.RuntimeDocker:
			fmt.Fprintln(stdout, "  sudo systemctl restart docker")
		}
	}

	return nil
}
