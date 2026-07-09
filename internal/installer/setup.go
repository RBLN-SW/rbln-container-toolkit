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
	ConfigPath    string // Runtime config path override (empty = per-runtime default)
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

	if err := validateConfigPathOverride(opts.ConfigPath); err != nil {
		return err
	}

	// Adjust paths for host root mount
	configPath := getConfigPath(opts.Runtime, opts.HostRootMount, opts.ConfigPath)
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
	logger.Info("CDI spec generated at %s (and %s on RDS-capable hosts)",
		filepath.Join(cdiSpecDir, "rbln.yaml"), filepath.Join(cdiSpecDir, "rbln-rds.yaml"))

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
	logger.Info("  2. Generate CDI spec at %s/rbln.yaml (and rbln-rds.yaml on RDS-capable hosts)", cdiSpecDir)

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

// validateConfigPathOverride ensures an explicit config path override is
// absolute. With the new semantics (override is the final path, not
// host-relative), a relative override would write to the daemon's
// current working directory instead of the intended location — fail
// fast with a clear error rather than letting that happen silently.
func validateConfigPathOverride(configPath string) error {
	if configPath == "" {
		return nil
	}
	if !filepath.IsAbs(configPath) {
		return fmt.Errorf("config path override must be absolute: %q", configPath)
	}
	return nil
}

// getConfigPath resolves the runtime config path that the daemon should
// read/write.
//
// When configPathOverride is set, it is treated as the final path — the
// caller is responsible for mounting the underlying file/directory at
// that location inside the container. hostRootMount is NOT prefixed to
// an override.
//
// When no override is given, the runtime-default host path is used and
// hostRootMount is prefixed so the daemon can reach the host's config
// through its host-root bind mount.
func getConfigPath(runtimeName, hostRootMount, configPathOverride string) string {
	if configPathOverride != "" {
		return configPathOverride
	}

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
	// Device nodes live in the host's real /dev, so root device discovery at the
	// host mount (empty means "/"). This is independent of DriverRoot and ensures
	// the emitted device hostPath is the real /dev path, not one prefixed with
	// the host mount.
	cfg.DeviceRoot = hostRootMount

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

	// Generate CDI spec. The installer path intentionally skips the
	// librbln-ml-backed RSD resolver: the installer typically runs before
	// the driver is healthy / loaded, so loading rblnml here would either
	// fail or return a stale snapshot. Passing nil routes the generator
	// through topology.NoopResolver{}, producing a spec without auto-
	// attached RSD nodes — the daemon/CLI path picks up the real resolver
	// once the driver is up and regenerates the spec.
	gen := cdi.NewGenerator(cfg, nil)
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

	// RDS char device (/dev/rblnfs*): separate opt-in CDI class emitted into its
	// own spec file, independent of the NPU spec above. Mirrors the
	// daemon (regenerateCDISpec) and rbln-ctk paths so `runtime <rt> setup`
	// doesn't silently omit the RDS class.
	if err := generateRDSSpec(cfg, cdiSpecDir, gen); err != nil {
		return fmt.Errorf("failed to generate RDS CDI spec: %w", err)
	}

	return nil
}

// generateRDSSpec discovers /dev/rblnfs* and writes the rebellions.ai/rds spec
// to <cdiSpecDir>/rbln-rds.yaml. RDS discovery is independent of the NPU device
// policy. When no RDS device is present it prunes any stale spec so non-RDS
// hosts don't keep a dangling class. Reuses the caller's generator (NoopResolver
// — RSD attachment is irrelevant to the RDS char device).
func generateRDSSpec(cfg *config.Config, cdiSpecDir string, gen cdi.Generator) error {
	rdsOutputPath := filepath.Join(cdiSpecDir, "rbln-rds.yaml")
	if len(cfg.RDS.Patterns) == 0 {
		return nil
	}

	// Shallow-copy the config with the RDS patterns swapped in (discovery forced
	// on) so the shared device discoverer globs /dev/rblnfs* while keeping the
	// DeviceRoot (host mount) intact.
	rdsCfg := *cfg
	rdsCfg.Devices.Patterns = cfg.RDS.Patterns
	rdsCfg.Devices.Disabled = false

	devices, err := discover.NewDeviceDiscoverer(&rdsCfg).Discover()
	if err != nil {
		return fmt.Errorf("failed to discover RDS devices: %w", err)
	}

	spec, err := gen.GenerateRDS(&discover.DiscoveryResult{Devices: devices})
	if err != nil {
		return fmt.Errorf("failed to generate RDS spec: %w", err)
	}
	if spec == nil {
		// No RDS device on this host — prune any stale spec from a prior run.
		if removeErr := os.Remove(rdsOutputPath); removeErr != nil && !os.IsNotExist(removeErr) {
			return fmt.Errorf("failed to remove stale RDS spec: %w", removeErr)
		}
		return nil
	}

	if err := cdi.NewWriter().Write(spec, rdsOutputPath, "yaml"); err != nil {
		return fmt.Errorf("failed to write RDS CDI spec: %w", err)
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
