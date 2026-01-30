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
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/cdi/setup"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/output"
)

// cdiCmd represents the cdi parent command
var cdiCmd = &cobra.Command{
	Use:   "cdi",
	Short: "CDI specification management",
}

var cdiGenerateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate CDI specification",
	Long: `Generate CDI (Container Device Interface) specification for RBLN devices.

The generated specification enables container runtimes to inject RBLN libraries
and tools into containers.`,
	Example: `  # Generate CDI spec (requires root)
  sudo rbln-ctk cdi generate

  # Preview without writing
  rbln-ctk cdi generate --dry-run

  # Output to stdout
  rbln-ctk cdi generate --output -

  # CoreOS with driver container
  rbln-ctk cdi generate --driver-root /run/rbln/driver

  # Enable library isolation
  rbln-ctk cdi generate --container-library-path /rbln/lib64`,
	RunE: runCDIGenerate,
}

var cdiListCmd = &cobra.Command{
	Use:   "list",
	Short: "List discovered libraries and tools",
	Long:  `List RBLN libraries and tools that would be included in the CDI specification.`,
	Example: `  # List in table format (default)
  rbln-ctk cdi list

  # List in JSON format
  rbln-ctk cdi list --format json

  # List with custom driver root
  rbln-ctk cdi list --driver-root /run/rbln/driver`,
	RunE: runCDIList,
}

func init() {
	// Add cdi command to root
	rootCmd.AddCommand(cdiCmd)

	// Add subcommands to cdi
	cdiCmd.AddCommand(cdiGenerateCmd)
	cdiCmd.AddCommand(cdiListCmd)

	// cdi generate flags
	cdiGenerateCmd.Flags().StringP("output", "o", "/var/run/cdi/rbln.yaml", "Output path (use '-' for stdout) [$RBLN_CTK_OUTPUT]")
	cdiGenerateCmd.Flags().StringP("format", "f", "yaml", "Output format (yaml or json) [$RBLN_CTK_FORMAT]")
	cdiGenerateCmd.Flags().String("driver-root", "/", "Driver root path (for CoreOS driver container) [$RBLN_CTK_DRIVER_ROOT]")
	cdiGenerateCmd.Flags().String("container-library-path", "", "Container path for libraries (enables isolation with LD_LIBRARY_PATH) [$RBLN_CTK_CONTAINER_LIBRARY_PATH]")
	cdiGenerateCmd.Flags().Bool("dry-run", false, "Preview without writing")

	// Bind cdi generate flags to Viper
	_ = viper.BindPFlag("output", cdiGenerateCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("format", cdiGenerateCmd.Flags().Lookup("format"))
	_ = viper.BindPFlag("driver-root", cdiGenerateCmd.Flags().Lookup("driver-root"))
	_ = viper.BindPFlag("container-library-path", cdiGenerateCmd.Flags().Lookup("container-library-path"))
	_ = viper.BindPFlag("dry-run", cdiGenerateCmd.Flags().Lookup("dry-run"))

	// cdi list flags
	cdiListCmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml) [$RBLN_CTK_LIST_FORMAT]")
	cdiListCmd.Flags().String("driver-root", "/", "Driver root path [$RBLN_CTK_LIST_DRIVER_ROOT]")
	_ = viper.BindPFlag("list-format", cdiListCmd.Flags().Lookup("format"))
	_ = viper.BindPFlag("list-driver-root", cdiListCmd.Flags().Lookup("driver-root"))
}

func runCDIGenerate(_ *cobra.Command, _ []string) error {
	loader := config.NewLoader()
	if configPath := viper.GetString("config"); configPath != "" {
		loader = loader.WithFile(configPath)
	}

	cfg, err := loader.Load(
		config.WithDriverRoot(viper.GetString("driver-root")),
		config.WithOutputPath(viper.GetString("output")),
		config.WithFormat(viper.GetString("format")),
		config.WithDebug(viper.GetBool("debug")),
		config.WithContainerLibraryPath(viper.GetString("container-library-path")),
	)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	outputPath := viper.GetString("output")
	format := viper.GetString("format")

	// Handle dry-run and stdout cases
	if viper.GetBool("dry-run") || outputPath == "-" {
		return setup.GenerateCDISpecToWriter(
			os.Stdout,
			&setup.Options{
				Config:    cfg,
				Format:    format,
				ErrorMode: setup.ErrorModeStrict,
			},
		)
	}

	// Generate to file
	err = setup.GenerateCDISpec(
		&setup.Options{
			Config:     cfg,
			OutputPath: outputPath,
			Format:     format,
			ErrorMode:  setup.ErrorModeStrict,
		},
	)
	if err != nil {
		return err
	}

	if !viper.GetBool("quiet") {
		fmt.Printf("CDI spec written to %s\n", outputPath)
	}

	return nil
}

func runCDIList(_ *cobra.Command, _ []string) error {
	loader := config.NewLoader()
	if configPath := viper.GetString("config"); configPath != "" {
		loader = loader.WithFile(configPath)
	}

	formatFlag := viper.GetString("list-format")
	driverRootFlag := viper.GetString("list-driver-root")

	cfg, err := loader.Load(
		config.WithDriverRoot(driverRootFlag),
		config.WithDebug(viper.GetBool("debug")),
	)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	libDiscoverer := discover.NewLibraryDiscoverer(cfg)
	toolDiscoverer := discover.NewToolDiscoverer(cfg)

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer)
	if err != nil {
		return fmt.Errorf("discover resources: %w", err)
	}

	formatter := output.NewFormatter(os.Stdout)
	return formatter.Format(result, formatFlag)
}
