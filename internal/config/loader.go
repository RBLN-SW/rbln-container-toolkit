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

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// Environment variable prefix
const envPrefix = "RBLN_"

// Environment variable names
const (
	EnvConfigFile = envPrefix + "CTK_CONFIG"
	EnvDebug      = envPrefix + "CTK_DEBUG"
	EnvCDIOutput  = envPrefix + "CDI_OUTPUT"
	EnvCDIFormat  = envPrefix + "CDI_FORMAT"
	EnvDriverRoot = envPrefix + "DRIVER_ROOT"
)

// Loader loads configuration with priority: CLI > env > file > defaults
type Loader struct {
	configFile string
}

// NewLoader creates a new configuration loader.
func NewLoader() *Loader {
	return &Loader{}
}

// WithFile sets the configuration file path.
func (l *Loader) WithFile(path string) *Loader {
	l.configFile = path
	return l
}

// Load loads the configuration with the specified options.
// Priority: CLI options > environment variables > config file > defaults
func (l *Loader) Load(opts ...Option) (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Load from config file (if exists)
	configPath := l.resolveConfigPath()
	if configPath != "" {
		if err := l.loadFromFile(cfg, configPath); err != nil {
			return nil, err
		}
	}

	// Apply environment variables
	l.applyEnvVars(cfg)

	// Apply CLI options (highest priority)
	for _, opt := range opts {
		opt(cfg)
	}

	return cfg, nil
}

// resolveConfigPath determines which config file to use.
func (l *Loader) resolveConfigPath() string {
	// 1. Explicit config file
	if l.configFile != "" {
		return l.configFile
	}

	// 2. Environment variable
	if envPath := os.Getenv(EnvConfigFile); envPath != "" {
		return envPath
	}

	// 3. Default paths (only if file exists)
	for _, path := range DefaultConfigPaths() {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// loadFromFile loads configuration from a YAML file.
func (l *Loader) loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Config file not found is not an error (use defaults)
			return nil
		}
		return fmt.Errorf("read config file %s: %w", path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file %s: %w", path, errors.ErrInvalidConfig)
	}

	return nil
}

// applyEnvVars applies environment variables to the configuration.
func (l *Loader) applyEnvVars(cfg *Config) {
	if val := os.Getenv(EnvDebug); val == "true" || val == "1" {
		cfg.Debug = true
	}

	if val := os.Getenv(EnvCDIOutput); val != "" {
		cfg.CDI.OutputPath = val
	}

	if val := os.Getenv(EnvCDIFormat); val != "" {
		cfg.CDI.Format = val
	}

	if val := os.Getenv(EnvDriverRoot); val != "" {
		cfg.DriverRoot = val
	}
}

// LoadDefault loads configuration with default settings.
func LoadDefault() *Config {
	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		return DefaultConfig()
	}
	return cfg
}
