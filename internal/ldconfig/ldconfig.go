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

// Package ldconfig provides ldconfig execution for updating the dynamic linker cache
// in container root filesystems.
package ldconfig

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/moby/sys/reexec"
)

const (
	// reexecCommandName is the reexec function name for ldconfig update.
	reexecCommandName = "rbln-update-ldcache"

	// ldsoconfdFilenamePattern is the pattern for the ldconfig config filename.
	ldsoconfdFilenamePattern = "00-rbln-*.conf"

	// defaultLdsoconfdDir is the standard location for ld.so.conf.d.
	defaultLdsoconfdDir = "/etc/ld.so.conf.d"
)

// Ldconfig holds configuration for running ldconfig in a container root.
type Ldconfig struct {
	// LdconfigPath is the path to ldconfig binary.
	LdconfigPath string

	// ContainerRoot is the container root filesystem path.
	ContainerRoot string

	// Directories are the library directories to add to ldcache.
	Directories []string
}

func init() {
	reexec.Register(reexecCommandName, reexecHandler)
	if reexec.Init() {
		os.Exit(0)
	}
}

// NewRunner creates an exec.Cmd that can be used to run ldconfig update.
// It uses reexec to run the ldconfig update in an isolated process.
func NewRunner(ldconfigPath, containerRoot string, directories ...string) (*exec.Cmd, error) {
	if containerRoot == "" || containerRoot == "/" {
		return nil, fmt.Errorf("container root must be specified and not be the system root")
	}

	args := []string{
		reexecCommandName,
		"--ldconfig-path", ldconfigPath,
		"--container-root", containerRoot,
	}
	args = append(args, directories...)

	cmd := reexec.Command(args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	return cmd, nil
}

// reexecHandler is the handler for the reexec command.
// It parses arguments and runs the ldconfig update in the container root.
func reexecHandler() {
	if err := runLdconfigUpdate(os.Args); err != nil {
		log.Printf("Error updating ldcache: %v", err)
		os.Exit(1)
	}
}

// runLdconfigUpdate runs the ldconfig update with the given arguments.
func runLdconfigUpdate(args []string) error {
	if len(args) < 5 {
		return fmt.Errorf("insufficient arguments: %v", args)
	}

	var ldconfigPath, containerRoot string
	var directories []string

	// Parse arguments: reexecName --ldconfig-path PATH --container-root ROOT [dirs...]
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--ldconfig-path":
			if i+1 >= len(args) {
				return fmt.Errorf("--ldconfig-path requires an argument")
			}
			ldconfigPath = args[i+1]
			i++
		case "--container-root":
			if i+1 >= len(args) {
				return fmt.Errorf("--container-root requires an argument")
			}
			containerRoot = args[i+1]
			i++
		default:
			// Remaining args are directories
			if !hasPrefix(args[i], "--") {
				directories = append(directories, args[i])
			}
		}
	}

	if ldconfigPath == "" {
		return fmt.Errorf("ldconfig path must be specified")
	}
	if containerRoot == "" || containerRoot == "/" {
		return fmt.Errorf("container root must be specified and not be the system root")
	}

	l := &Ldconfig{
		LdconfigPath:  ldconfigPath,
		ContainerRoot: containerRoot,
		Directories:   directories,
	}

	return l.UpdateLDCache()
}

// UpdateLDCache updates the ldcache in the container root.
func (l *Ldconfig) UpdateLDCache() error {
	// Create ld.so.conf.d directory in container root if it doesn't exist
	ldsoconfdDir := filepath.Join(l.ContainerRoot, defaultLdsoconfdDir)
	if err := os.MkdirAll(ldsoconfdDir, 0o755); err != nil {
		return fmt.Errorf("failed to create ld.so.conf.d: %w", err)
	}

	// Create config file with library directories
	if err := l.createLdsoconfdFile(ldsoconfdDir); err != nil {
		return fmt.Errorf("failed to create ldconfig config file: %w", err)
	}

	// Run ldconfig with the container root
	return l.runLdconfig()
}

// createLdsoconfdFile creates a ld.so.conf.d drop-in file with the specified directories.
func (l *Ldconfig) createLdsoconfdFile(ldsoconfdDir string) error {
	if len(l.Directories) == 0 {
		return nil
	}

	configFile, err := os.CreateTemp(ldsoconfdDir, ldsoconfdFilenamePattern)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer configFile.Close()

	if err := writeDirectories(configFile, l.Directories...); err != nil {
		return err
	}

	// Make file readable by all users
	if err := configFile.Chmod(0o644); err != nil {
		return fmt.Errorf("failed to chmod config file: %w", err)
	}

	return nil
}

// writeDirectories writes directory paths to the writer, one per line.
func writeDirectories(w io.Writer, dirs ...string) error {
	seen := make(map[string]bool)
	for _, dir := range dirs {
		if seen[dir] {
			continue
		}
		if _, err := fmt.Fprintf(w, "%s\n", dir); err != nil {
			return fmt.Errorf("failed to write directory: %w", err)
		}
		seen[dir] = true
	}
	return nil
}

// runLdconfig executes ldconfig with the container root.
func (l *Ldconfig) runLdconfig() error {
	// Validate ldconfig binary exists before execution (FR-012)
	if _, err := os.Stat(l.LdconfigPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("ldconfig binary not found: %w", err)
		}
		return fmt.Errorf("failed to stat ldconfig binary: %w", err)
	}

	// Determine the ldconfig path inside container or use host ldconfig with -r flag
	args := []string{
		"-r", l.ContainerRoot,
	}

	cmd := exec.Command(l.LdconfigPath, args...)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ldconfig failed: %w", err)
	}

	return nil
}

// hasPrefix checks if the string has the given prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// Run executes the ldconfig runner command.
func Run(cmd *exec.Cmd) error {
	return cmd.Run()
}
