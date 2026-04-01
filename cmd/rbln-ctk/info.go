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
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/cdi/setup"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Display system information",
	RunE:  runInfo,
}

func init() {
	// Add info command to root
	rootCmd.AddCommand(infoCmd)

	// info flags
	infoCmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")
	_ = viper.BindPFlag("info-format", infoCmd.Flags().Lookup("format"))
}

func runInfo(_ *cobra.Command, _ []string) error {
	// Load configuration
	loader := config.NewLoader()
	if configPath := viper.GetString("config"); configPath != "" {
		loader = loader.WithFile(configPath)
	}

	cfg, err := loader.Load(
		config.WithDebug(viper.GetBool("debug")),
	)
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	libDiscoverer := discover.NewLibraryDiscoverer(cfg)
	toolDiscoverer := discover.NewToolDiscoverer(cfg)
	deviceDiscoverer := discover.NewDeviceDiscoverer(cfg)

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, deviceDiscoverer)
	if err != nil {
		return fmt.Errorf("discover resources: %w", err)
	}

	// Output info
	fmt.Println("RBLN Container Toolkit")
	fmt.Printf("  Version:        %s\n", version)
	fmt.Printf("  Build Date:     %s\n", buildDate)
	fmt.Printf("  Git Commit:     %s\n", gitCommit)
	fmt.Println()
	fmt.Println("System:")
	fmt.Printf("  OS:             %s\n", runtime.GOOS)
	fmt.Printf("  Architecture:   %s\n", runtime.GOARCH)
	fmt.Println()
	fmt.Println("RBLN Driver:")

	libCount := 0
	toolCount := 0
	if result != nil {
		libCount = len(result.Libraries)
		toolCount = len(result.Tools)
	}

	fmt.Printf("  Libraries:      %d found\n", libCount)
	fmt.Printf("  Tools:          %d found\n", toolCount)
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Config File:    %s\n", getConfigPath())
	fmt.Printf("  CDI Output:     %s\n", cfg.CDI.OutputPath)
	fmt.Printf("  Driver Root:    %s\n", cfg.DriverRoot)

	return nil
}

func getConfigPath() string {
	if path := viper.GetString("config"); path != "" {
		return path
	}
	if path := os.Getenv("RBLN_CTK_CONFIG"); path != "" {
		return path
	}
	return "/etc/rbln/container-toolkit.yaml"
}
