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
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/oci"
)

var createSymlinksCmd = &cobra.Command{
	Use:   "create-symlinks",
	Short: "Create symlinks in a container",
	Long: `Creates symlinks in the container's root filesystem by:
1. Reading the OCI container state from STDIN (or specified file)
2. Extracting the container root filesystem path
3. Creating specified symlinks (target::link format)

This command is designed to be used as a CDI createContainer hook,
executed before update-ldcache to recreate symlinks lost during bind mount.`,
	RunE: runCreateSymlinks,
}

func init() {
	rootCmd.AddCommand(createSymlinksCmd)

	createSymlinksCmd.Flags().StringSlice("link", []string{}, "Symlink to create in target::link format (can be specified multiple times)")
	createSymlinksCmd.Flags().String("container-spec", "", "Path to the OCI container spec file (default: STDIN) [$RBLN_CDI_HOOK_CONTAINER_SPEC]")

	_ = viper.BindPFlag("link", createSymlinksCmd.Flags().Lookup("link"))
	_ = viper.BindPFlag("container-spec", createSymlinksCmd.Flags().Lookup("container-spec"))
}

type createSymlinksOptions struct {
	links         []string
	containerSpec string
}

type symlinkCreator func(containerRoot, target, link string) error

func runCreateSymlinks(cmd *cobra.Command, _ []string) error {
	links, err := cmd.Flags().GetStringSlice("link")
	if err != nil {
		return fmt.Errorf("failed to get link flags: %w", err)
	}
	containerSpec, err := cmd.Flags().GetString("container-spec")
	if err != nil {
		return fmt.Errorf("failed to get container-spec flag: %w", err)
	}

	opts := createSymlinksOptions{
		links:         links,
		containerSpec: containerSpec,
	}

	return executeCreateSymlinks(opts, oci.LoadContainerState, createSymlinkInContainer)
}

func executeCreateSymlinks(opts createSymlinksOptions, loadState stateLoader, createLink symlinkCreator) error {
	if len(opts.links) == 0 {
		return nil
	}

	state, err := loadState(opts.containerSpec)
	if err != nil {
		return fmt.Errorf("failed to load container state: %w", err)
	}

	containerRoot, err := state.GetContainerRoot()
	if err != nil {
		return fmt.Errorf("failed to determine container root: %w", err)
	}

	created := make(map[string]bool)
	for _, l := range opts.links {
		if created[l] {
			continue
		}

		parts := strings.SplitN(l, "::", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid symlink specification %q: expected target::link format", l)
		}

		target := parts[0]
		link := parts[1]

		if err := createLink(containerRoot, target, link); err != nil {
			return fmt.Errorf("failed to create symlink %s -> %s: %w", link, target, err)
		}
		created[l] = true
	}

	return nil
}

func createSymlinkInContainer(containerRoot, target, link string) error {
	linkPath := filepath.Join(containerRoot, link)

	currentTarget, err := os.Readlink(linkPath)
	if err == nil && currentTarget == target {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	return forceCreateSymlink(target, linkPath)
}

func forceCreateSymlink(target, linkPath string) error {
	_, err := os.Lstat(linkPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to stat link path: %w", err)
	}
	if !os.IsNotExist(err) {
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("failed to remove existing file: %w", err)
		}
	}
	return os.Symlink(target, linkPath)
}
