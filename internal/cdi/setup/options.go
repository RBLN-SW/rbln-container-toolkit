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

package setup

import (
	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

// ErrorMode defines how errors are handled during CDI setup.
type ErrorMode int

const (
	// ErrorModeStrict returns error on any discovery failure (except dependencies which always warn).
	ErrorModeStrict ErrorMode = iota
	// ErrorModeLenient logs warning and continues on failures.
	ErrorModeLenient
)

// Logger defines the logging interface for setup operations.
type Logger interface {
	// Info logs an informational message.
	Info(msg string, args ...interface{})
	// Warning logs a warning message.
	Warning(msg string, args ...interface{})
	// Debug logs a debug message.
	Debug(msg string, args ...interface{})
}

// Options contains configuration for CDI setup.
type Options struct {
	// Config is the CDI configuration.
	Config *config.Config
	// OutputPath is the path where the CDI spec will be written.
	OutputPath string
	// Format is the output format (yaml or json).
	Format string
	// ErrorMode defines how errors are handled.
	ErrorMode ErrorMode
	// Logger is used for logging during setup.
	Logger Logger
	// LibraryDiscoverer is used for library discovery (optional, for testing).
	LibraryDiscoverer discover.LibraryDiscoverer
	// ToolDiscoverer is used for tool discovery (optional, for testing).
	ToolDiscoverer discover.ToolDiscoverer
	// DeviceDiscoverer is used for device node discovery (optional, for testing).
	DeviceDiscoverer discover.DeviceDiscoverer
}

// Option is a function that modifies SetupOptions.
type Option func(*Options)

// WithConfig sets the CDI configuration.
func WithConfig(cfg *config.Config) Option {
	return func(opts *Options) {
		if cfg != nil {
			opts.Config = cfg
		}
	}
}

// WithOutputPath sets the output path for the CDI spec.
func WithOutputPath(path string) Option {
	return func(opts *Options) {
		if path != "" {
			opts.OutputPath = path
		}
	}
}

// WithFormat sets the output format (yaml or json).
func WithFormat(format string) Option {
	return func(opts *Options) {
		if format != "" {
			opts.Format = format
		}
	}
}

// WithErrorMode sets the error handling mode.
func WithErrorMode(mode ErrorMode) Option {
	return func(opts *Options) {
		opts.ErrorMode = mode
	}
}

// WithLogger sets the logger for setup operations.
func WithLogger(logger Logger) Option {
	return func(opts *Options) {
		if logger != nil {
			opts.Logger = logger
		}
	}
}

// DefaultOptions returns a SetupOptions with default values.
func DefaultOptions() *Options {
	return &Options{
		Config:     &config.Config{},
		OutputPath: "/var/run/cdi/rbln.yaml",
		Format:     "yaml",
		ErrorMode:  ErrorModeStrict,
		Logger:     &noopLogger{},
	}
}

// noopLogger is a no-op implementation of Logger.
type noopLogger struct{}

func (n *noopLogger) Info(_ string, _ ...interface{})    {}
func (n *noopLogger) Warning(_ string, _ ...interface{}) {}
func (n *noopLogger) Debug(_ string, _ ...interface{})   {}
