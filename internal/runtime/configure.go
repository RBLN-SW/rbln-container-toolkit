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

// defaultCDISpecDir is the CDI spec directory the configurators enable and the
// daemon writes RBLN specs into. Runtime CDI readiness is judged against it.
const defaultCDISpecDir = "/var/run/cdi"

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
	newContent, err := enableCDIInContainerdConfig(content)
	if err != nil {
		return fmt.Errorf("enable CDI in config %s: %w", c.configPath, err)
	}

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

	newContent, err := enableCDIInContainerdConfig(content)
	if err != nil {
		return "", fmt.Errorf("enable CDI in config %s: %w", c.configPath, err)
	}

	// Return diff
	return fmt.Sprintf("--- %s (original)\n+++ %s (modified)\n\n%s", c.configPath, c.configPath, newContent), nil
}

// enableCDIInContainerdConfig returns containerd config text with CDI enabled
// and the daemon's spec dir covered. It parses the config, sets enable_cdi and
// ensures cdi_spec_dirs includes defaultCDISpecDir, then re-serializes — so a
// pre-existing enable_cdi=true with a custom cdi_spec_dirs that omits our dir is
// remediated (not left as a silent no-op) and the node converges to ready.
func enableCDIInContainerdConfig(content string) (string, error) {
	cfg, err := parseContainerdConfig(content)
	if err != nil {
		return "", err
	}

	cri := cfg.criSection(true)
	cri["enable_cdi"] = true

	if dirs, present := cfg.specDirs(); present {
		// Preserve the operator's list; just guarantee our spec dir is covered.
		if !containsCleanPath(dirs, defaultCDISpecDir) {
			cri["cdi_spec_dirs"] = append(dirs, defaultCDISpecDir)
		}
	} else {
		cri["cdi_spec_dirs"] = []string{"/etc/cdi", defaultCDISpecDir}
	}

	// Fresh config: pin the schema version, matching the previous template.
	if strings.TrimSpace(content) == "" {
		cfg.root["version"] = int64(2)
	}

	return cfg.render()
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

	// Enable CDI feature. The Docker daemon recognizes "cdi" (not "cdi-devices");
	// using the wrong key leaves CDI disabled and `docker run --device` fails with
	// `could not select device driver "cdi"`. Docker 28.2.0+ enables CDI by default,
	// 25.0.0~28.1.x require this flag.
	features["cdi"] = true
}

// CDIReady reports whether the runtime already exposes CDI with no config
// change required — i.e. whether the daemon can skip Configure() and, crucially,
// the runtime restart that follows it.
//
// It is deliberately NOT a plain grep for a config key. The decision reflects
// runtime version defaults: containerd 2.0+ and Docker 28.2.0+ enable CDI by
// default with the standard spec dirs, so their config may legitimately omit
// the key while still being ready. hostRoot locates the runtime binary for that
// version probe (see DetectVersion). When the version can't be determined the
// check falls back to config-content equivalence: ready iff running Configure()
// would leave the file byte-for-byte unchanged.
//
// configPath must be the effective path the configurator would write (already
// host-root-prefixed by the caller).
func CDIReady(rt RuntimeType, configPath, hostRoot string) (bool, error) {
	switch rt {
	case RuntimeContainerd:
		return containerdCDIReady(configPath, hostRoot)
	case RuntimeCRIO:
		return crioCDIReady(configPath)
	case RuntimeDocker:
		return dockerCDIReady(configPath, hostRoot)
	default:
		return false, fmt.Errorf("%w: %s", errors.ErrRuntimeNotFound, rt)
	}
}

// containerdCDIReady reports readiness for containerd. containerd 2.0+ enables
// CDI by default (cdi_spec_dirs defaults include /var/run/cdi), so a 2.0+ node
// is ready unless the config explicitly disables it. Otherwise (older version,
// or version unknown) it is ready only when enabling CDI would not change the
// config file.
func containerdCDIReady(configPath, hostRoot string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read config %s: %w", configPath, err)
	}

	cfg, err := parseContainerdConfig(string(data))
	if err != nil {
		return false, fmt.Errorf("check CDI readiness for %s: %w", configPath, err)
	}

	// Guard against a config that enables CDI but points cdi_spec_dirs away from
	// the dir this daemon writes specs into: the runtime would never see our
	// specs, so we must NOT report ready (which would skip the restart and leave
	// containers silently without devices). An absent cdi_spec_dirs is fine —
	// containerd's default list includes /var/run/cdi.
	if !cfg.scansDefaultSpecDir() {
		return false, nil
	}

	// An explicit enable_cdi setting wins over version defaults.
	if enabled, present := cfg.enableCDI(); present {
		return enabled, nil
	}

	// enable_cdi unset → rely on the runtime version default: containerd 2.0+
	// enables CDI by default; older versions (and an undetectable version) need
	// Configure to write it.
	if v, ok := DetectVersion(RuntimeContainerd, hostRoot); ok && v.AtLeast(2, 0) {
		return true, nil
	}
	return false, nil
}

// crioCDIReady reports readiness for CRI-O. CTK owns a dedicated drop-in file,
// so readiness is simply whether that drop-in already exists with exactly the
// content Configure() would write.
func crioCDIReady(configPath string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read config %s: %w", configPath, err)
	}
	want := (&crioConfigurator{configPath: configPath}).generateConfig()
	return string(data) == want, nil
}

// dockerCDIReady reports readiness for Docker. Docker 28.2.0+ enables CDI by
// default, so it is ready unless features.cdi is explicitly set to false.
// Otherwise readiness requires features.cdi == true in the config.
func dockerCDIReady(configPath, hostRoot string) (bool, error) {
	data, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read config %s: %w", configPath, err)
	}

	config := make(map[string]interface{})
	if len(data) > 0 {
		if err := json.Unmarshal(data, &config); err != nil {
			return false, fmt.Errorf("parse config %s: %w", configPath, err)
		}
	}

	cdi, hasKey := dockerCDIFeature(config)

	if v, ok := DetectVersion(RuntimeDocker, hostRoot); ok && v.AtLeast(28, 2) {
		// Default-on; ready unless explicitly disabled.
		return !hasKey || cdi, nil
	}
	return hasKey && cdi, nil
}

// dockerCDIFeature returns the features.cdi value and whether the key is set.
func dockerCDIFeature(config map[string]interface{}) (value, present bool) {
	features, ok := config["features"].(map[string]interface{})
	if !ok {
		return false, false
	}
	v, ok := features["cdi"].(bool)
	if !ok {
		return false, false
	}
	return v, true
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
		delete(features, "cdi")
		// Also drop the legacy key written by older CTK versions.
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
