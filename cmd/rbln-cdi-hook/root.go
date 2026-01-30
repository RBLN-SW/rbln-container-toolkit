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

// Package main provides the rbln-cdi-hook CLI entry point.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version is set at build time
	Version = "dev"
)

var rootCmd = &cobra.Command{
	Use:     "rbln-cdi-hook",
	Short:   "CDI hook for RBLN container toolkit",
	Version: Version,
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.CompletionOptions.HiddenDefaultCmd = true
}

func initConfig() {
	// Set environment variable prefix for CDI hook
	// All flags will be mapped to RBLN_CDI_HOOK_* environment variables
	viper.SetEnvPrefix("RBLN_CDI_HOOK")

	// Enable automatic environment variable binding
	viper.AutomaticEnv()

	// Replace hyphens with underscores in env var names
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
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
