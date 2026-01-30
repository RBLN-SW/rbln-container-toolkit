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

package installer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/runtime"
)

// CleanupOptions configures the cleanup operation.
type CleanupOptions struct {
	Runtime       string
	RestartMode   restart.Mode
	Socket        string
	HostRootMount string
	CDISpecDir    string
	PidFile       string
	DryRun        bool
	Logger        Logger
}

// Cleanup performs the full cleanup operation: remove CDI + revert config + restart.
func Cleanup(opts CleanupOptions) error {
	logger := opts.Logger
	if logger == nil {
		logger = &noopLogger{}
	}

	// Resolve runtime defaults
	defaults := restart.GetRuntimeDefaults(opts.Runtime)
	if opts.RestartMode == "" {
		opts.RestartMode = defaults.Mode
	}
	if opts.Socket == "" {
		opts.Socket = defaults.Socket
	}

	// Validate CRI-O doesn't use signal mode
	if opts.Runtime == "crio" && opts.RestartMode == restart.RestartModeSignal {
		return fmt.Errorf("signal restart mode is not supported for CRI-O, use systemd or none")
	}

	// Adjust paths for host root mount
	configPath := getConfigPath(opts.Runtime, opts.HostRootMount)
	socketPath := opts.Socket
	socketUserProvided := opts.Socket != ""
	cdiSpecDir := opts.CDISpecDir
	if opts.HostRootMount != "" {
		if !socketUserProvided {
			socketPath = filepath.Join(opts.HostRootMount, opts.Socket)
		}
		cdiSpecDir = filepath.Join(opts.HostRootMount, opts.CDISpecDir)
	}

	if opts.DryRun {
		return dryRunCleanup(opts, configPath, socketPath, cdiSpecDir, logger)
	}

	// Acquire lock
	lock := NewLock(opts.PidFile)
	if err := lock.Acquire(); err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	logger.Info("Removing RBLN support from %s...", opts.Runtime)

	// Step 1: Remove CDI spec (idempotent - no error if not found)
	cdiSpecPath := filepath.Join(cdiSpecDir, "rbln.yaml")
	logger.Debug("Removing CDI spec at %s", cdiSpecPath)
	if err := removeCDISpec(cdiSpecPath); err != nil {
		// Log but don't fail - might already be removed
		logger.Debug("CDI spec removal: %v", err)
	} else {
		logger.Info("CDI spec removed")
	}

	// Step 2: Revert runtime configuration
	logger.Debug("Reverting runtime configuration at %s", configPath)
	if err := revertRuntimeConfig(opts.Runtime, configPath); err != nil {
		// Log but don't fail - configuration might already be clean
		logger.Debug("Config revert: %v", err)
	} else {
		logger.Info("Runtime configuration reverted")
	}

	// Step 3: Restart runtime
	if opts.RestartMode == restart.RestartModeNone {
		logger.Warning("Restart skipped. To apply changes, manually restart %s:\n  sudo systemctl restart %s",
			opts.Runtime, defaults.Service)
		return nil
	}

	logger.Debug("Restarting %s using %s mode", opts.Runtime, opts.RestartMode)
	restarter, err := restart.NewRestarter(restart.Options{
		Mode:          opts.RestartMode,
		Socket:        socketPath,
		HostRootMount: opts.HostRootMount,
		MaxRetries:    3,
		RetryBackoff:  5 * 1e9, // 5 seconds in nanoseconds
		Timeout:       30 * 1e9,
	})
	if err != nil {
		return fmt.Errorf("failed to create restarter: %w", err)
	}

	if err := restarter.Restart(opts.Runtime); err != nil {
		return &ErrRestartFailed{
			Runtime: opts.Runtime,
			Cause:   err,
			Service: defaults.Service,
		}
	}

	logger.Info("Successfully removed RBLN support from %s", opts.Runtime)
	return nil
}

func dryRunCleanup(opts CleanupOptions, configPath, socketPath, cdiSpecDir string, logger Logger) error {
	defaults := restart.GetRuntimeDefaults(opts.Runtime)
	cdiSpecPath := filepath.Join(cdiSpecDir, "rbln.yaml")

	logger.Info("[DRY-RUN] Would perform the following actions:")
	logger.Info("  1. Remove CDI spec at %s", cdiSpecPath)
	logger.Info("  2. Revert %s configuration at %s", opts.Runtime, configPath)

	if opts.RestartMode == restart.RestartModeNone {
		logger.Info("  3. Skip restart (restart-mode=none)")
	} else {
		restarter, err := restart.NewRestarter(restart.Options{
			Mode:          opts.RestartMode,
			Socket:        socketPath,
			HostRootMount: opts.HostRootMount,
		})
		if err != nil {
			return err
		}
		logger.Info("  3. %s", restarter.DryRun(defaults.Service))
	}

	return nil
}

func removeCDISpec(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil // Already removed - idempotent
	}
	return err
}

func revertRuntimeConfig(runtimeName, configPath string) error {
	var rt runtime.RuntimeType
	switch runtimeName {
	case "docker":
		rt = runtime.RuntimeDocker
	case "containerd":
		rt = runtime.RuntimeContainerd
	case "crio":
		rt = runtime.RuntimeCRIO
	default:
		return fmt.Errorf("unsupported runtime: %s", runtimeName)
	}

	reverter, err := runtime.NewReverter(rt, configPath)
	if err != nil {
		return err
	}

	return reverter.Revert()
}
