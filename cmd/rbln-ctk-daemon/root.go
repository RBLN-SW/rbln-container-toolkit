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
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	cdisetup "github.com/RBLN-SW/rbln-container-toolkit/internal/cdi/setup"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/daemon"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/restart"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/runtime"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rbln-ctk-daemon",
		Short: "RBLN Container Toolkit Daemon",
		Long: `RBLN Container Toolkit Daemon configures container runtimes for RBLN device support.

This daemon is designed for Kubernetes DaemonSet deployments. It:
1. Auto-detects the container runtime (or uses --runtime flag)
2. Generates CDI specification
3. Configures the runtime for CDI support
4. Waits for SIGTERM/SIGINT signal
5. Cleans up configuration on shutdown

The daemon ensures containers have access to RBLN NPU devices via CDI.

Flags:
  --host-root: Host filesystem mount point (for containerized daemon)
  --driver-root: Driver installation path on host (for CDI hostPath)`,
		Example: `  # Run with auto-detected runtime
  rbln-ctk-daemon

  # Run with specific runtime
  rbln-ctk-daemon --runtime containerd

  # Run with custom health port
  rbln-ctk-daemon --health-port 9090

  # Kubernetes DaemonSet deployment
  rbln-ctk-daemon --host-root /host --cdi-spec-dir /var/run/cdi

  # CoreOS with driver container
  rbln-ctk-daemon --host-root /host --driver-root /run/rbln/driver`,
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", version, gitCommit, buildDate),
		RunE:    runDaemon,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return initConfig()
		},
		SilenceUsage: true,
	}

	// Daemon flags
	cmd.Flags().StringP("runtime", "r", "", "Container runtime (auto-detect if not specified) [$RBLN_CTK_DAEMON_RUNTIME]")
	cmd.Flags().Duration("shutdown-timeout", 30*time.Second, "Graceful shutdown timeout [$RBLN_CTK_DAEMON_SHUTDOWN_TIMEOUT]")
	cmd.Flags().String("pid-file", "/run/rbln/toolkit.pid", "PID file path [$RBLN_CTK_DAEMON_PID_FILE]")
	cmd.Flags().Int("health-port", 8080, "Health check HTTP port [$RBLN_CTK_DAEMON_HEALTH_PORT]")
	cmd.Flags().Bool("no-cleanup-on-exit", false, "Skip cleanup on exit (debug mode) [$RBLN_CTK_DAEMON_NO_CLEANUP_ON_EXIT]")
	cmd.Flags().String("host-root", "", "Host root mount path (auto-detect if empty) [$RBLN_CTK_DAEMON_HOST_ROOT]")
	cmd.Flags().String("driver-root", "/", "Driver root path for CDI spec (host perspective) [$RBLN_CTK_DAEMON_DRIVER_ROOT]")
	cmd.Flags().String("cdi-spec-dir", "/var/run/cdi", "CDI specification directory [$RBLN_CTK_DAEMON_CDI_SPEC_DIR]")
	cmd.Flags().String("container-library-path", "", "Container library path for isolation (enables LD_LIBRARY_PATH) [$RBLN_CTK_DAEMON_CONTAINER_LIBRARY_PATH]")
	cmd.Flags().String("socket", "", "Runtime socket path (auto-detect if empty) [$RBLN_CTK_DAEMON_SOCKET]")
	cmd.Flags().String("config-path", "", "Runtime config path override (default: per-runtime) [$RBLN_CTK_DAEMON_CONFIG_PATH]")
	cmd.Flags().BoolP("debug", "d", false, "Enable debug logging [$RBLN_CTK_DAEMON_DEBUG]")
	cmd.Flags().BoolP("force", "f", false, "Terminate existing instance before starting [$RBLN_CTK_DAEMON_FORCE]")

	// Bind to viper for env var support
	viper.SetEnvPrefix("RBLN_CTK_DAEMON")
	_ = viper.BindPFlag("runtime", cmd.Flags().Lookup("runtime"))
	_ = viper.BindPFlag("shutdown_timeout", cmd.Flags().Lookup("shutdown-timeout"))
	_ = viper.BindPFlag("pid_file", cmd.Flags().Lookup("pid-file"))
	_ = viper.BindPFlag("health_port", cmd.Flags().Lookup("health-port"))
	_ = viper.BindPFlag("no_cleanup_on_exit", cmd.Flags().Lookup("no-cleanup-on-exit"))
	_ = viper.BindPFlag("host_root", cmd.Flags().Lookup("host-root"))
	_ = viper.BindPFlag("driver_root", cmd.Flags().Lookup("driver-root"))
	_ = viper.BindPFlag("cdi_spec_dir", cmd.Flags().Lookup("cdi-spec-dir"))
	_ = viper.BindPFlag("container_library_path", cmd.Flags().Lookup("container-library-path"))
	_ = viper.BindPFlag("socket", cmd.Flags().Lookup("socket"))
	_ = viper.BindPFlag("config_path", cmd.Flags().Lookup("config-path"))
	_ = viper.BindPFlag("debug", cmd.Flags().Lookup("debug"))
	_ = viper.BindPFlag("force", cmd.Flags().Lookup("force"))

	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.CompletionOptions.HiddenDefaultCmd = true

	cmd.AddCommand(newRuntimeCmd())

	return cmd
}

func initConfig() error {
	viper.SetEnvPrefix("RBLN_CTK_DAEMON")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	return nil
}

func runDaemon(_ *cobra.Command, _ []string) error {
	// Get configuration from flags/env via viper
	runtimeFlag := viper.GetString("runtime")
	shutdownTimeout := viper.GetDuration("shutdown_timeout")
	pidFile := viper.GetString("pid_file")
	healthPort := viper.GetInt("health_port")
	noCleanup := viper.GetBool("no_cleanup_on_exit")
	hostRoot := viper.GetString("host_root")
	driverRoot := viper.GetString("driver_root")
	cdiDir := viper.GetString("cdi_spec_dir")
	containerLibraryPath := viper.GetString("container_library_path")
	socketPath := viper.GetString("socket")
	configPath := viper.GetString("config_path")
	debugFlag := viper.GetBool("debug")

	if configPath != "" && !filepath.IsAbs(configPath) {
		return fmt.Errorf("config path override must be absolute: %q", configPath)
	}

	// Auto-detect host root mount path
	hostRoot = detectHostRoot(hostRoot)
	log.Printf("INFO: Using host root mount: %s", hostRoot)

	// Detect or validate runtime
	log.Println("INFO: Detecting container runtime...")
	var rt runtime.RuntimeType
	var err error

	if runtimeFlag != "" {
		rt = runtime.RuntimeType(runtimeFlag)
		log.Printf("INFO: Using specified runtime: %s", rt)
	} else {
		detectOpts := buildDetectOptions(socketPath)
		rt, err = runtime.DetectRuntimeStrict(detectOpts)
		if err != nil {
			return fmt.Errorf("runtime detection failed: %w", err)
		}
		log.Printf("INFO: Detected runtime: %s", rt)
	}

	// Create daemon config
	cfg := &daemon.Config{
		Runtime:              daemon.RuntimeType(rt),
		ShutdownTimeout:      shutdownTimeout,
		PidFile:              pidFile,
		HealthPort:           healthPort,
		NoCleanupOnExit:      noCleanup,
		HostRootMount:        hostRoot,
		CDISpecDir:           cdiDir,
		ContainerLibraryPath: containerLibraryPath,
		Socket:               socketPath,
		Debug:                debugFlag,
		Force:                viper.GetBool("force"),
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	cleanup := func() error {
		return doCleanup(rt, cdiDir, hostRoot, configPath)
	}

	d := daemon.NewDaemon(cfg, cleanup)

	if err := d.AcquirePIDLock(); err != nil {
		return fmt.Errorf("acquire PID lock: %w", err)
	}
	defer func() { _ = d.ReleasePIDLock() }()

	if err := setup(rt, cdiDir, hostRoot, driverRoot, containerLibraryPath, socketPath, configPath); err != nil {
		return fmt.Errorf("setup failed: %w", err)
	}

	ctx := context.Background()
	return d.Run(ctx)
}

// daemonLogger implements setup.Logger interface for daemon logging.
type daemonLogger struct{}

func (l *daemonLogger) Info(msg string, args ...interface{}) {
	log.Printf("INFO: "+msg, args...)
}

func (l *daemonLogger) Warning(msg string, args ...interface{}) {
	log.Printf("WARNING: "+msg, args...)
}

func (l *daemonLogger) Debug(msg string, args ...interface{}) {
	if viper.GetBool("debug") {
		log.Printf("DEBUG: "+msg, args...)
	}
}

func setup(rt runtime.RuntimeType, cdiDir, hostRoot, driverRoot, containerLibraryPath, socketPath, configPath string) error {
	if hostRoot != "/" && hostRoot != "" {
		if err := installHookBinary(hostRoot); err != nil {
			log.Printf("WARNING: Failed to install hook binary: %v", err)
		}
	}

	log.Println("INFO: Generating CDI specification...")

	cfg := config.LoadDefault()
	cfg.DriverRoot = driverRoot

	// When running in container (hostRoot != "/"), set SearchRoot for file access
	// SearchRoot is used to find files, while DriverRoot is used for CDI paths
	if hostRoot != "/" && hostRoot != "" {
		cfg.SearchRoot = filepath.Join(hostRoot, driverRoot)
		log.Printf("DEBUG: Search root set to: %s", cfg.SearchRoot)
	}

	if containerLibraryPath != "" {
		cfg.Libraries.ContainerPath = containerLibraryPath
	}

	specPath := cdiDir + "/rbln.yaml"
	if err := os.MkdirAll(cdiDir, 0o755); err != nil {
		return fmt.Errorf("create CDI dir: %w", err)
	}

	opts := &cdisetup.Options{
		Config:     cfg,
		OutputPath: specPath,
		Format:     "yaml",
		ErrorMode:  cdisetup.ErrorModeLenient,
		Logger:     &daemonLogger{},
	}

	if err := cdisetup.GenerateCDISpec(opts); err != nil {
		return fmt.Errorf("generate CDI spec: %w", err)
	}

	// Configure runtime
	log.Println("INFO: Configuring runtime...")
	configPath = resolveConfigPath(rt, hostRoot, configPath)
	log.Printf("INFO: Using runtime config path: %s", configPath)
	configurator, err := runtime.NewConfigurator(rt, configPath, nil)
	if err != nil {
		return fmt.Errorf("create configurator: %w", err)
	}

	if configErr := configurator.Configure(); configErr != nil {
		return fmt.Errorf("configure runtime: %w", configErr)
	}
	log.Printf("INFO: Runtime %s configured", rt)

	// Restart runtime
	log.Println("INFO: Restarting runtime...")

	// Resolve runtime defaults
	defaults := restart.GetRuntimeDefaults(string(rt))
	restartMode := defaults.Mode
	socketUserProvided := socketPath != ""
	if socketPath == "" {
		socketPath = defaults.Socket
	}

	// Adjust socket path for host root mount (only for default socket, not user-provided)
	if !socketUserProvided && hostRoot != "" && hostRoot != "/" {
		socketPath = filepath.Join(hostRoot, socketPath)
	}

	restartOpts := restart.Options{
		Mode:          restartMode,
		Socket:        socketPath,
		HostRootMount: hostRoot,
		MaxRetries:    3,
		RetryBackoff:  5 * 1e9, // 5 seconds in nanoseconds
		Timeout:       30 * 1e9,
	}
	restarter, err := restart.NewRestarter(restartOpts)
	if err != nil {
		return fmt.Errorf("create restarter: %w", err)
	}

	if err := restarter.Restart(string(rt)); err != nil {
		log.Printf("WARNING: Runtime restart failed: %v (may need manual restart)", err)
	} else {
		log.Println("INFO: Runtime restarted")
	}

	return nil
}

func doCleanup(rt runtime.RuntimeType, cdiDir, hostRoot, configPath string) error {
	log.Println("INFO: Removing CDI specification...")

	// Remove CDI spec
	specPath := cdiDir + "/rbln.yaml"
	if err := os.Remove(specPath); err != nil && !os.IsNotExist(err) {
		log.Printf("WARNING: Failed to remove CDI spec: %v", err)
	}

	log.Println("INFO: Reverting runtime configuration...")

	// Revert runtime config (restore backup)
	configPath = resolveConfigPath(rt, hostRoot, configPath)
	backupPath := configPath + ".backup"

	if _, err := os.Stat(backupPath); err == nil {
		backup, err := os.ReadFile(backupPath)
		if err == nil {
			_ = os.WriteFile(configPath, backup, 0o644)
			os.Remove(backupPath)
		}
	}

	// Restart runtime
	log.Println("INFO: Restarting runtime...")

	// Resolve runtime defaults
	defaults := restart.GetRuntimeDefaults(string(rt))
	restartMode := defaults.Mode
	socketPath := defaults.Socket

	// Apply host root prefix for containerized deployments
	if hostRoot != "" && hostRoot != "/" {
		socketPath = filepath.Join(hostRoot, socketPath)
	}

	restartOpts := restart.Options{
		Mode:          restartMode,
		Socket:        socketPath,
		HostRootMount: hostRoot,
		MaxRetries:    3,
		RetryBackoff:  5 * 1e9, // 5 seconds in nanoseconds
		Timeout:       30 * 1e9,
	}
	restarter, err := restart.NewRestarter(restartOpts)
	if err != nil {
		log.Printf("WARNING: Could not create restarter: %v", err)
		return nil
	}

	if err := restarter.Restart(string(rt)); err != nil {
		log.Printf("WARNING: Runtime restart failed: %v", err)
	}

	return nil
}

// Helper functions for subcommands (used by runtime.go etc.)
func logInfo(format string, args ...interface{}) {
	log.Printf("INFO: "+format, args...)
}

func logDebug(format string, args ...interface{}) {
	if viper.GetBool("debug") {
		log.Printf("DEBUG: "+format, args...)
	}
}

func logWarning(format string, args ...interface{}) {
	log.Printf("WARNING: "+format, args...)
}

func logError(format string, args ...interface{}) {
	log.Printf("ERROR: "+format, args...)
}

// resolveConfigPath returns the effective runtime config path.
//
// When configPath is set, it is used as-is — the caller is responsible
// for ensuring it points to a writable location inside the daemon's
// filesystem. hostRoot is NOT prefixed to an override.
//
// When configPath is empty, the runtime's default host path is used and
// hostRoot is prefixed so the daemon can reach the host config through
// its host-root bind mount.
func resolveConfigPath(rt runtime.RuntimeType, hostRoot, configPath string) string {
	if configPath != "" {
		return configPath
	}
	configPath = runtime.DefaultConfigPath(rt)
	if hostRoot != "/" && hostRoot != "" {
		configPath = filepath.Join(hostRoot, configPath)
	}
	return configPath
}

func detectHostRoot(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if _, err := os.Stat("/host"); err == nil {
		return "/host"
	}
	return "/"
}

func buildDetectOptions(socketPath string) *runtime.DetectStrictOptions {
	if socketPath == "" {
		return nil
	}
	opts := &runtime.DetectStrictOptions{
		ContainerdSocket: "/run/containerd/containerd.sock",
		CRIOSocket:       "/var/run/crio/crio.sock",
		DockerSocket:     "/var/run/docker.sock",
	}
	switch {
	case strings.Contains(socketPath, "containerd"):
		opts.ContainerdSocket = socketPath
	case strings.Contains(socketPath, "crio"):
		opts.CRIOSocket = socketPath
	case strings.Contains(socketPath, "docker"):
		opts.DockerSocket = socketPath
	}
	return opts
}

func installHookBinary(hostRoot string) error {
	src := "/usr/local/bin/rbln-cdi-hook"
	if _, err := os.Stat(src); err != nil {
		src = "/usr/bin/rbln-cdi-hook"
		if _, err := os.Stat(src); err != nil {
			return fmt.Errorf("hook binary not found")
		}
	}

	destDir := filepath.Join(hostRoot, "usr", "local", "bin")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	dest := filepath.Join(destDir, "rbln-cdi-hook")

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, srcFile); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	log.Printf("INFO: Hook binary installed/updated at %s", dest)
	return nil
}
