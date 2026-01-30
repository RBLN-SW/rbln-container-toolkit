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

//go:generate moq -rm -fmt=goimports -stub -out logger_mock.go . Logger

import (
	"fmt"
	"os"
	"path/filepath"

	"tags.cncf.io/container-device-interface/specs-go"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/cdi"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/runtime"
)

// SetupOptions configures the setup operation.
type SetupOptions struct {
	Runtime       string
	RestartMode   restart.Mode
	Socket        string
	HostRootMount string
	CDISpecDir    string
	PidFile       string
	DryRun        bool
	Logger        Logger
}

// Logger interface for setup operations.
type Logger interface {
	Info(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Warning(format string, args ...interface{})
}

// Setup performs the full setup operation: configure runtime + generate CDI + restart.
func Setup(opts SetupOptions) error {
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
		return dryRunSetup(opts, configPath, socketPath, cdiSpecDir, logger)
	}

	// Acquire lock
	lock := NewLock(opts.PidFile)
	if err := lock.Acquire(); err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	logger.Info("Setting up RBLN support for %s...", opts.Runtime)

	// Step 1: Configure runtime
	logger.Debug("Configuring runtime at %s", configPath)
	if err := configureRuntime(opts.Runtime, configPath); err != nil {
		return fmt.Errorf("failed to configure %s: %w", opts.Runtime, err)
	}
	logger.Info("Runtime configuration updated")

	// Step 2: Generate CDI spec
	logger.Debug("Generating CDI spec at %s", cdiSpecDir)
	if err := generateCDISpec(cdiSpecDir, opts.HostRootMount); err != nil {
		return fmt.Errorf("failed to generate CDI spec: %w", err)
	}
	logger.Info("CDI spec generated at %s", filepath.Join(cdiSpecDir, "rbln.yaml"))

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

	logger.Info("Successfully set up RBLN support for %s", opts.Runtime)
	return nil
}

func dryRunSetup(opts SetupOptions, configPath, socketPath, cdiSpecDir string, logger Logger) error {
	defaults := restart.GetRuntimeDefaults(opts.Runtime)

	logger.Info("[DRY-RUN] Would perform the following actions:")
	logger.Info("  1. Configure %s at %s", opts.Runtime, configPath)
	logger.Info("  2. Generate CDI spec at %s/rbln.yaml", cdiSpecDir)

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

func getConfigPath(runtimeName, hostRootMount string) string {
	var configPath string
	switch runtimeName {
	case "docker":
		configPath = "/etc/docker/daemon.json"
	case "containerd":
		configPath = "/etc/containerd/config.toml"
	case "crio":
		configPath = "/etc/crio/crio.conf.d/99-rbln.conf"
	default:
		configPath = fmt.Sprintf("/etc/%s/config", runtimeName)
	}

	if hostRootMount != "" {
		return filepath.Join(hostRootMount, configPath)
	}
	return configPath
}

func configureRuntime(runtimeName, configPath string) error {
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

	// Use default config path if not specified
	if configPath == "" {
		configPath = runtime.DefaultConfigPath(rt)
	}

	configurer, err := runtime.NewConfigurator(rt, configPath, nil)
	if err != nil {
		return err
	}

	return configurer.Configure()
}

func generateCDISpec(cdiSpecDir, hostRootMount string) error {
	// Ensure CDI spec directory exists
	if err := os.MkdirAll(cdiSpecDir, 0o755); err != nil {
		return fmt.Errorf("failed to create CDI spec directory: %w", err)
	}

	// Create configuration
	cfg := config.DefaultConfig()
	if hostRootMount != "" {
		cfg.DriverRoot = hostRootMount
	}

	// Discover RBLN libraries
	libDisc := discover.NewLibraryDiscoverer(cfg)
	rblnLibs, err := libDisc.DiscoverRBLN()
	if err != nil {
		return fmt.Errorf("failed to discover RBLN libraries: %w", err)
	}

	// Discover dependencies
	depLibs, err := libDisc.DiscoverDependencies(rblnLibs)
	if err != nil {
		// Log but don't fail - dependencies are optional
		depLibs = nil
	}

	// Discover tools
	toolDisc := discover.NewToolDiscoverer(cfg)
	tools, err := toolDisc.Discover()
	if err != nil {
		// Log but don't fail - tools are optional
		tools = nil
	}

	// Create discovery result
	result := &discover.DiscoveryResult{
		Libraries: append(rblnLibs, depLibs...),
		Tools:     tools,
	}

	// Generate CDI spec
	gen := cdi.NewGenerator(cfg)
	spec, err := gen.Generate(result)
	if err != nil {
		return fmt.Errorf("failed to generate CDI spec: %w", err)
	}

	// Write CDI spec
	outputPath := filepath.Join(cdiSpecDir, "rbln.yaml")
	writer := cdi.NewWriter()
	if err := writer.Write(spec, outputPath, "yaml"); err != nil {
		return fmt.Errorf("failed to write CDI spec: %w", err)
	}

	return nil
}

// emptySpec returns an empty CDI spec as a fallback.
func emptySpec() *specs.Spec {
	return &specs.Spec{
		Version: "0.5.0",
		Kind:    "rebellions.ai/rbln",
	}
}

// noopLogger is a no-op logger implementation.
type noopLogger struct{}

func (l *noopLogger) Info(_ string, _ ...interface{})    {}
func (l *noopLogger) Debug(_ string, _ ...interface{})   {}
func (l *noopLogger) Warning(_ string, _ ...interface{}) {}
