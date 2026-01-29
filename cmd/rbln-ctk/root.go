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

// Package main provides the rbln-ctk CLI entry point.
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	version   = "dev"
	buildDate = "unknown"
	gitCommit = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "rbln-ctk",
	Short: "Container toolkit for Rebellions NPU devices",
	Long:  `rbln-ctk is a container toolkit for managing CDI specifications and runtime configuration for Rebellions NPU devices.`,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})

	rootCmd.PersistentFlags().StringP("config", "c", "", "Path to configuration file [$RBLN_CTK_CONFIG]")
	rootCmd.PersistentFlags().BoolP("debug", "d", false, "Enable debug logging [$RBLN_CTK_DEBUG]")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-error output [$RBLN_CTK_QUIET]")

	// Bind flags to Viper
	_ = viper.BindPFlag("config", rootCmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
	_ = viper.BindPFlag("quiet", rootCmd.PersistentFlags().Lookup("quiet"))

	// Add version command
	rootCmd.AddCommand(versionCmd)
}

func initConfig() {
	// Set environment variable prefix
	// All flags will be mapped to RBLN_CTK_* environment variables
	viper.SetEnvPrefix("RBLN_CTK")

	// Enable automatic environment variable binding
	// --driver-root -> RBLN_CTK_DRIVER_ROOT
	// --output -> RBLN_CTK_OUTPUT
	viper.AutomaticEnv()

	// Replace hyphens with underscores in env var names
	// --driver-root -> RBLN_CTK_DRIVER_ROOT (not RBLN_CTK_DRIVER-ROOT)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Printf("rbln-ctk version %s\n", version)
		fmt.Printf("  Build Date: %s\n", buildDate)
		fmt.Printf("  Git Commit: %s\n", gitCommit)
		fmt.Printf("  Go Version: %s\n", runtime.Version())
	},
}

// Execute runs the root command
func Execute() {
	// Silence Cobra's default error and usage output
	// This gives us full control over error presentation
	rootCmd.SilenceErrors = true
	rootCmd.SilenceUsage = true

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
