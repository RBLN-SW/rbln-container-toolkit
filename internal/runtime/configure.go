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

// Package runtime provides container runtime configuration functionality.
package runtime

//go:generate moq -rm -fmt=goimports -stub -out configure_mock.go . Configurator Reverter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// Type represents the container runtime type.
type Type string

// RuntimeType is an alias for Type for backward compatibility.
//
//nolint:revive // RuntimeType stutter is kept for API compatibility
type RuntimeType = Type

const (
	// RuntimeContainerd is the containerd runtime.
	RuntimeContainerd RuntimeType = "containerd"
	// RuntimeCRIO is the CRI-O runtime.
	RuntimeCRIO RuntimeType = "crio"
	// RuntimeDocker is the Docker runtime.
	RuntimeDocker RuntimeType = "docker"
)

// Configurator configures a container runtime for RBLN CDI support.
type Configurator interface {
	// Configure modifies the runtime configuration to enable CDI.
	Configure() error

	// DryRun returns what would be changed without actually modifying files.
	DryRun() (string, error)
}

// ConfiguratorOptions holds options for creating a configurator.
type ConfiguratorOptions struct {
	// CDIEnabled controls whether CDI should be enabled.
	CDIEnabled bool
}

// NewConfigurator creates a new runtime configurator.
func NewConfigurator(rt RuntimeType, configPath string, opts *ConfiguratorOptions) (Configurator, error) {
	if opts == nil {
		opts = &ConfiguratorOptions{
			CDIEnabled: true,
		}
	}

	switch rt {
	case RuntimeContainerd:
		return &containerdConfigurator{
			configPath: configPath,
			opts:       opts,
		}, nil
	case RuntimeCRIO:
		return &crioConfigurator{
			configPath: configPath,
			opts:       opts,
		}, nil
	case RuntimeDocker:
		return &dockerConfigurator{
			configPath: configPath,
			opts:       opts,
		}, nil
	default:
		return nil, fmt.Errorf("%w: %s", errors.ErrRuntimeNotFound, rt)
	}
}

// DefaultConfigPath returns the default configuration path for a runtime type.
func DefaultConfigPath(rt RuntimeType) string {
	switch rt {
	case RuntimeContainerd:
		return "/etc/containerd/config.toml"
	case RuntimeCRIO:
		return "/etc/crio/crio.conf.d/99-rbln.conf"
	case RuntimeDocker:
		return "/etc/docker/daemon.json"
	default:
		return ""
	}
}

// DetectOptions holds options for runtime detection.
type DetectOptions struct {
	ContainerdSocket string
	CRIOSocket       string
	DockerSocket     string
}

// DetectRuntime detects the installed container runtime.
func DetectRuntime() (RuntimeType, error) {
	return DetectRuntimeWithOptions(nil)
}

// DetectRuntimeWithOptions detects the installed container runtime with custom options.
func DetectRuntimeWithOptions(opts *DetectOptions) (RuntimeType, error) {
	if opts == nil {
		opts = &DetectOptions{
			ContainerdSocket: "/run/containerd/containerd.sock",
			CRIOSocket:       "/var/run/crio/crio.sock",
			DockerSocket:     "/var/run/docker.sock",
		}
	}

	// Check in priority order: containerd > crio > docker
	if opts.ContainerdSocket != "" {
		if _, err := os.Stat(opts.ContainerdSocket); err == nil {
			return RuntimeContainerd, nil
		}
	}

	if opts.CRIOSocket != "" {
		if _, err := os.Stat(opts.CRIOSocket); err == nil {
			return RuntimeCRIO, nil
		}
	}

	if opts.DockerSocket != "" {
		if _, err := os.Stat(opts.DockerSocket); err == nil {
			return RuntimeDocker, nil
		}
	}

	return "", errors.ErrRuntimeNotFound
}

// backupFile creates a backup of the original file.
func backupFile(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Nothing to backup
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	backupPath := path + ".backup"
	return os.WriteFile(backupPath, content, 0o644)
}

// containerdConfigurator configures containerd for CDI.
type containerdConfigurator struct {
	configPath string
	opts       *ConfiguratorOptions
}

func (c *containerdConfigurator) Configure() error {
	// Backup existing config
	if err := backupFile(c.configPath); err != nil {
		return fmt.Errorf("backup config: %v", err)
	}

	// Read existing config or create new
	content := ""
	if data, err := os.ReadFile(c.configPath); err == nil {
		content = string(data)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read config %s: %v", c.configPath, err)
	}

	// Enable CDI in config
	newContent := enableCDIInContainerdConfig(content)

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %v", filepath.Dir(c.configPath), err)
	}

	// Write new config
	if err := os.WriteFile(c.configPath, []byte(newContent), 0o644); err != nil {
		return fmt.Errorf("write config %s: %v", c.configPath, err)
	}

	return nil
}

func (c *containerdConfigurator) DryRun() (string, error) {
	content := ""
	if data, err := os.ReadFile(c.configPath); err == nil {
		content = string(data)
	}

	newContent := enableCDIInContainerdConfig(content)

	// Return diff
	return fmt.Sprintf("--- %s (original)\n+++ %s (modified)\n\n%s", c.configPath, c.configPath, newContent), nil
}

// enableCDIInContainerdConfig adds CDI configuration to containerd config.
func enableCDIInContainerdConfig(content string) string {
	// Check if CDI is already enabled
	if strings.Contains(content, "enable_cdi = true") {
		return content
	}

	// If config is empty, create a basic config
	if strings.TrimSpace(content) == "" {
		return `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
`
	}

	// Find the CRI plugin section and add CDI settings
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines)+4)
	inCRISection := false
	cdiAdded := false

	for i, line := range lines {
		result = append(result, line)

		// Look for CRI plugin section
		if strings.Contains(line, `[plugins."io.containerd.grpc.v1.cri"]`) {
			inCRISection = true
			continue
		}

		// If we're in CRI section and haven't added CDI yet
		if inCRISection && !cdiAdded {
			// Check if next line starts a new section
			nextIsSection := i+1 < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i+1]), "[")
			if nextIsSection || i == len(lines)-1 {
				// Add CDI settings before moving to next section
				result = append(result,
					`    enable_cdi = true`,
					`    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]`)
				cdiAdded = true
			}
		}
	}

	// If no CRI section found, append one
	if !cdiAdded {
		result = append(result,
			"",
			`[plugins."io.containerd.grpc.v1.cri"]`,
			`  enable_cdi = true`,
			`  cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]`)
	}

	return strings.Join(result, "\n")
}

// crioConfigurator configures CRI-O for CDI.
type crioConfigurator struct {
	configPath string
	opts       *ConfiguratorOptions
}

func (c *crioConfigurator) Configure() error {
	// Backup existing config
	if err := backupFile(c.configPath); err != nil {
		return fmt.Errorf("backup config: %v", err)
	}

	// Generate config content
	content := c.generateConfig()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %v", filepath.Dir(c.configPath), err)
	}

	// Write config
	if err := os.WriteFile(c.configPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write config %s: %v", c.configPath, err)
	}

	return nil
}

func (c *crioConfigurator) DryRun() (string, error) {
	content := c.generateConfig()
	return fmt.Sprintf("--- %s (new file)\n\n%s", c.configPath, content), nil
}

func (c *crioConfigurator) generateConfig() string {
	return `# RBLN Container Toolkit CRI-O Configuration
# This file enables CDI support for Rebellions NPU

[crio.runtime]
# Enable CDI (Container Device Interface) support
enable_cdi = true

# CDI specification directories
cdi_spec_dirs = [
    "/etc/cdi",
    "/var/run/cdi"
]
`
}

// dockerConfigurator configures Docker for CDI.
type dockerConfigurator struct {
	configPath string
	opts       *ConfiguratorOptions
}

func (c *dockerConfigurator) Configure() error {
	// Backup existing config
	if err := backupFile(c.configPath); err != nil {
		return fmt.Errorf("backup config: %v", err)
	}

	// Read existing config or create new
	config := make(map[string]interface{})
	data, readErr := os.ReadFile(c.configPath)
	if readErr == nil {
		if len(data) > 0 {
			if err := json.Unmarshal(data, &config); err != nil {
				return fmt.Errorf("parse config %s: %v", c.configPath, err)
			}
		}
	} else if !os.IsNotExist(readErr) {
		return fmt.Errorf("read config %s: %v", c.configPath, readErr)
	}

	// Add CDI feature
	c.enableCDI(config)

	// Write config
	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %v", err)
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(c.configPath), 0o755); err != nil {
		return fmt.Errorf("create config directory %s: %v", filepath.Dir(c.configPath), err)
	}

	if err := os.WriteFile(c.configPath, newContent, 0o644); err != nil {
		return fmt.Errorf("write config %s: %v", c.configPath, err)
	}

	return nil
}

func (c *dockerConfigurator) DryRun() (string, error) {
	config := make(map[string]interface{})
	if data, err := os.ReadFile(c.configPath); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &config); err != nil {
			return "", fmt.Errorf("parse config: %w", err)
		}
	}

	c.enableCDI(config)

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal config: %w", err)
	}

	return fmt.Sprintf("--- %s\n\n%s", c.configPath, string(newContent)), nil
}

func (c *dockerConfigurator) enableCDI(config map[string]interface{}) {
	// Docker uses "features" for experimental features
	features, ok := config["features"].(map[string]interface{})
	if !ok {
		features = make(map[string]interface{})
		config["features"] = features
	}

	// Enable CDI devices feature
	features["cdi-devices"] = true
}

// Reverter reverts runtime configuration changes.
type Reverter interface {
	// Revert removes CDI configuration from the runtime config.
	// It first tries to restore from backup, then falls back to removing CDI settings.
	Revert() error
}

// NewReverter creates a new runtime configuration reverter.
func NewReverter(rt RuntimeType, configPath string) (Reverter, error) {
	if configPath == "" {
		configPath = DefaultConfigPath(rt)
	}

	switch rt {
	case RuntimeContainerd:
		return &containerdReverter{configPath: configPath}, nil
	case RuntimeCRIO:
		return &crioReverter{configPath: configPath}, nil
	case RuntimeDocker:
		return &dockerReverter{configPath: configPath}, nil
	default:
		return nil, fmt.Errorf("%w: %s", errors.ErrRuntimeNotFound, rt)
	}
}

// containerdReverter reverts containerd configuration.
type containerdReverter struct {
	configPath string
}

func (r *containerdReverter) Revert() error {
	// Try backup first
	backupPath := r.configPath + ".backup"
	if _, err := os.Stat(backupPath); err == nil {
		content, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if err := os.WriteFile(r.configPath, content, 0o644); err != nil {
			return fmt.Errorf("restore backup: %w", err)
		}
		_ = os.Remove(backupPath)
		return nil
	}

	// No backup - remove CDI settings from config
	content, err := os.ReadFile(r.configPath)
	if os.IsNotExist(err) {
		return nil // Nothing to revert
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	newContent := disableCDIInContainerdConfig(string(content))
	if newContent == string(content) {
		return nil // No changes needed
	}

	return os.WriteFile(r.configPath, []byte(newContent), 0o644)
}

// disableCDIInContainerdConfig removes CDI settings from containerd config.
func disableCDIInContainerdConfig(content string) string {
	lines := strings.Split(content, "\n")
	result := make([]string, 0, len(lines))

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip CDI-related lines
		if strings.HasPrefix(trimmed, "enable_cdi") ||
			strings.HasPrefix(trimmed, "cdi_spec_dirs") {
			continue
		}
		result = append(result, line)
	}

	return strings.Join(result, "\n")
}

// crioReverter reverts CRI-O configuration.
type crioReverter struct {
	configPath string
}

func (r *crioReverter) Revert() error {
	// For CRI-O, just remove the drop-in config file
	err := os.Remove(r.configPath)
	if os.IsNotExist(err) {
		return nil // Already removed
	}
	return err
}

// dockerReverter reverts Docker configuration.
type dockerReverter struct {
	configPath string
}

func (r *dockerReverter) Revert() error {
	// Try backup first
	backupPath := r.configPath + ".backup"
	if _, err := os.Stat(backupPath); err == nil {
		content, err := os.ReadFile(backupPath)
		if err != nil {
			return fmt.Errorf("read backup: %w", err)
		}
		if err := os.WriteFile(r.configPath, content, 0o644); err != nil {
			return fmt.Errorf("restore backup: %w", err)
		}
		_ = os.Remove(backupPath)
		return nil
	}

	// No backup - remove CDI settings from config
	data, err := os.ReadFile(r.configPath)
	if os.IsNotExist(err) {
		return nil // Nothing to revert
	}
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	var config map[string]interface{}
	if unmarshalErr := json.Unmarshal(data, &config); unmarshalErr != nil {
		return fmt.Errorf("parse config: %w", unmarshalErr)
	}

	if features, ok := config["features"].(map[string]interface{}); ok {
		delete(features, "cdi-devices")
		if len(features) == 0 {
			delete(config, "features")
		}
	}

	newContent, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	return os.WriteFile(r.configPath, newContent, 0o644)
}
