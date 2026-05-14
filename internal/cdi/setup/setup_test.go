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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/topology"
)

// mockLogger captures log messages for testing. Each method formats msg+args
// via fmt.Sprintf so tests see the substituted text — same contract as the
// production daemonLogger (which goes through log.Printf). Without this,
// resolveTopology's warn closure would store "RSD topology resolver: %v ..."
// verbatim and tests couldn't assert on the underlying error string.
type mockLogger struct {
	infos    []string
	warnings []string
	debugs   []string
}

func (m *mockLogger) Info(msg string, args ...interface{}) {
	m.infos = append(m.infos, fmt.Sprintf(msg, args...))
}
func (m *mockLogger) Warning(msg string, args ...interface{}) {
	m.warnings = append(m.warnings, fmt.Sprintf(msg, args...))
}
func (m *mockLogger) Debug(msg string, args ...interface{}) {
	m.debugs = append(m.debugs, fmt.Sprintf(msg, args...))
}

// mockLibraryDiscoverer implements discover.LibraryDiscoverer for testing.
type mockLibraryDiscoverer struct {
	rblnLibs   []discover.Library
	rblnErr    error
	depsLibs   []discover.Library
	depsErr    error
	pluginLibs []discover.Library
	pluginErr  error
	callOrder  []string
}

func (m *mockLibraryDiscoverer) DiscoverRBLN() ([]discover.Library, error) {
	m.callOrder = append(m.callOrder, "rbln")
	return m.rblnLibs, m.rblnErr
}

func (m *mockLibraryDiscoverer) DiscoverDependencies(_ []discover.Library) ([]discover.Library, error) {
	m.callOrder = append(m.callOrder, "deps")
	return m.depsLibs, m.depsErr
}

func (m *mockLibraryDiscoverer) DiscoverPlugins() ([]discover.Library, error) {
	m.callOrder = append(m.callOrder, "plugins")
	return m.pluginLibs, m.pluginErr
}

func TestGenerateCDISpec_StrictMode_FailsOnError(t *testing.T) {
	// Given a setup with strict error mode and a discoverer that returns an error
	logger := &mockLogger{}
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
	}
	failingDiscoverer := &mockLibraryDiscoverer{
		rblnErr: assert.AnError,
	}
	opts := &Options{
		Config:            cfg,
		ErrorMode:         ErrorModeStrict,
		Logger:            logger,
		LibraryDiscoverer: failingDiscoverer,
	}

	// When GenerateCDISpec is called with a failing discoverer
	err := GenerateCDISpec(opts)

	// Then an error should be returned
	assert.Error(t, err, "strict mode should return error on discovery failure")
}

func TestGenerateCDISpec_LenientMode_ContinuesOnError(t *testing.T) {
	// Given a setup with lenient error mode and a discoverer that returns an error
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	logger := &mockLogger{}
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor:     "rebellions.ai",
			Class:      "npu",
			OutputPath: outputPath,
		},
	}
	failingDiscoverer := &mockLibraryDiscoverer{
		rblnErr: assert.AnError,
	}
	opts := &Options{
		Config:            cfg,
		OutputPath:        outputPath,
		Format:            "yaml",
		ErrorMode:         ErrorModeLenient,
		Logger:            logger,
		LibraryDiscoverer: failingDiscoverer,
	}

	// When GenerateCDISpec is called with a failing discoverer in lenient mode
	err := GenerateCDISpec(opts)

	// Then no error should be returned and warnings should be logged
	assert.NoError(t, err, "lenient mode should not return error on discovery failure")
	assert.Greater(t, len(logger.warnings), 0, "lenient mode should log warnings")
}

func TestGenerateCDISpec_WritesToOutputPath(t *testing.T) {
	// Given a valid configuration with a temporary output path
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor:     "rebellions.ai",
			Class:      "npu",
			OutputPath: outputPath,
			Format:     "yaml",
		},
	}
	opts := &Options{
		Config:     cfg,
		OutputPath: outputPath,
		Format:     "yaml",
		ErrorMode:  ErrorModeLenient,
		Logger:     &mockLogger{},
	}

	// When GenerateCDISpec is called
	err := GenerateCDISpec(opts)

	// Then the CDI spec file should be written to the output path
	require.NoError(t, err, "GenerateCDISpec should succeed")
	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr, "CDI spec file should exist at output path")
}

func TestDiscoverResources_CorrectOrder(t *testing.T) {
	// Given a library discoverer that tracks call order
	mockLibDisc := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
		depsLibs: []discover.Library{
			{Name: "libstdc++.so.6", Path: "/usr/lib64/libstdc++.so.6", Type: discover.LibraryTypeDependency},
		},
		pluginLibs: []discover.Library{
			{Name: "libmlx5.so", Path: "/usr/lib64/libibverbs/libmlx5.so", Type: discover.LibraryTypePlugin},
		},
	}

	// When DiscoverResources is called
	result, err := DiscoverResources(mockLibDisc, nil, nil)

	// Then discovery should happen in order: RBLN → Dependencies → Plugins
	require.NoError(t, err, "DiscoverResources should succeed")
	require.NotNil(t, result, "result should not be nil")
	assert.Equal(t, []string{"rbln", "deps", "plugins"}, mockLibDisc.callOrder,
		"discovery order should be RBLN → Dependencies → Plugins")
}

func TestGenerateCDISpec_RequiresConfig(t *testing.T) {
	// Given setup options with nil config
	opts := &Options{
		Config:     nil,
		OutputPath: "/tmp/test.yaml",
		Format:     "yaml",
		ErrorMode:  ErrorModeStrict,
		Logger:     &mockLogger{},
	}

	// When GenerateCDISpec is called
	err := GenerateCDISpec(opts)

	// Then an error should be returned indicating config is required
	assert.Error(t, err, "should return error when config is nil")
	assert.Contains(t, err.Error(), "config", "error message should mention config")
}

type mockToolDiscoverer struct {
	tools []discover.Tool
	err   error
}

func (m *mockToolDiscoverer) Discover() ([]discover.Tool, error) {
	return m.tools, m.err
}

type mockDeviceDiscoverer struct {
	devices []discover.Device
	err     error
	calls   int
}

func (m *mockDeviceDiscoverer) Discover() ([]discover.Device, error) {
	m.calls++
	return m.devices, m.err
}

func TestGenerateCDISpecToWriter_Success(t *testing.T) {
	// Given a valid configuration and a buffer to write to
	var buf bytes.Buffer
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
	}
	mockLibDisc := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so"},
		},
	}
	opts := &Options{
		Config:            cfg,
		Format:            "yaml",
		ErrorMode:         ErrorModeLenient,
		Logger:            &mockLogger{},
		LibraryDiscoverer: mockLibDisc,
	}

	// When GenerateCDISpecToWriter is called
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then no error should be returned and output should contain CDI spec
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "rebellions.ai/npu")
}

func TestGenerateCDISpecToWriter_RequiresConfig(t *testing.T) {
	// Given setup options with nil config
	var buf bytes.Buffer
	opts := &Options{
		Config: nil,
		Format: "yaml",
	}

	// When GenerateCDISpecToWriter is called
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then an error should be returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config")
}

func TestGenerateCDISpecToWriter_StrictMode_FailsOnError(t *testing.T) {
	// Given strict mode with a failing discoverer
	var buf bytes.Buffer
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
	}
	failingDiscoverer := &mockLibraryDiscoverer{
		rblnErr: assert.AnError,
	}
	opts := &Options{
		Config:            cfg,
		Format:            "yaml",
		ErrorMode:         ErrorModeStrict,
		Logger:            &mockLogger{},
		LibraryDiscoverer: failingDiscoverer,
	}

	// When GenerateCDISpecToWriter is called
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then an error should be returned
	assert.Error(t, err)
}

func TestGenerateCDISpecToWriter_LenientMode_ContinuesOnError(t *testing.T) {
	// Given lenient mode with a failing discoverer
	var buf bytes.Buffer
	logger := &mockLogger{}
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
	}
	failingDiscoverer := &mockLibraryDiscoverer{
		rblnErr: assert.AnError,
	}
	opts := &Options{
		Config:            cfg,
		Format:            "yaml",
		ErrorMode:         ErrorModeLenient,
		Logger:            logger,
		LibraryDiscoverer: failingDiscoverer,
	}

	// When GenerateCDISpecToWriter is called
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then no error should be returned and warnings should be logged
	assert.NoError(t, err)
	assert.Greater(t, len(logger.warnings), 0)
}

func TestDiscoverResources_DependencyError(t *testing.T) {
	// Given a discoverer that fails on dependencies
	mockLibDisc := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so"},
		},
		depsErr: assert.AnError,
	}

	// When DiscoverResources is called
	result, err := DiscoverResources(mockLibDisc, nil, nil)

	// Then an error should be returned
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "dependencies")
}

func TestDiscoverResources_PluginError(t *testing.T) {
	// Given a discoverer that fails on plugins
	mockLibDisc := &mockLibraryDiscoverer{
		rblnLibs:  []discover.Library{},
		depsLibs:  []discover.Library{},
		pluginErr: assert.AnError,
	}

	// When DiscoverResources is called
	result, err := DiscoverResources(mockLibDisc, nil, nil)

	// Then an error should be returned
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "plugins")
}

func TestDiscoverResources_ToolError(t *testing.T) {
	// Given a tool discoverer that fails
	mockLibDisc := &mockLibraryDiscoverer{
		rblnLibs:   []discover.Library{},
		depsLibs:   []discover.Library{},
		pluginLibs: []discover.Library{},
	}
	mockToolDisc := &mockToolDiscoverer{
		err: assert.AnError,
	}

	// When DiscoverResources is called
	result, err := DiscoverResources(mockLibDisc, mockToolDisc, nil)

	// Then an error should be returned
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tools")
}

func TestDiscoverResources_WithTools(t *testing.T) {
	// Given discoverers that return libraries and tools
	mockLibDisc := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so"},
		},
	}
	mockToolDisc := &mockToolDiscoverer{
		tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi"},
		},
	}

	// When DiscoverResources is called
	result, err := DiscoverResources(mockLibDisc, mockToolDisc, nil)

	// Then result should contain both libraries and tools
	require.NoError(t, err)
	assert.Len(t, result.Libraries, 1)
	assert.Len(t, result.Tools, 1)
}

func TestGenerateCDISpec_NilOpts_UsesDefaults(t *testing.T) {
	// Given nil options (defaults include output path that may not be writable)
	// When GenerateCDISpec is called with nil
	err := GenerateCDISpec(nil)

	// Then it should use default options (may fail on write permissions, but not on config)
	if err != nil {
		assert.NotContains(t, err.Error(), "config is required")
	}
}

func TestGenerateCDISpecToWriter_NilOpts(t *testing.T) {
	// Given nil options
	var buf bytes.Buffer

	// When GenerateCDISpecToWriter is called with nil
	err := GenerateCDISpecToWriter(&buf, nil)

	// Then it should use default options and succeed
	assert.NoError(t, err)
}

func TestGenerateCDISpec_DefaultFormat(t *testing.T) {
	// Given options without format specified
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "rbln.yaml")
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
	}
	opts := &Options{
		Config:     cfg,
		OutputPath: outputPath,
		ErrorMode:  ErrorModeLenient,
		Logger:     &mockLogger{},
	}

	// When GenerateCDISpec is called
	err := GenerateCDISpec(opts)

	// Then it should succeed using default yaml format
	require.NoError(t, err)
	_, statErr := os.Stat(outputPath)
	assert.NoError(t, statErr)
}

func TestGenerateCDISpec_DevicesDisabled_SkipsDeviceDiscovery(t *testing.T) {
	// Given: Devices.Disabled=true (Kubernetes path) and no explicit DeviceDiscoverer.
	// The setup must NOT auto-construct a DeviceDiscoverer that would scan the
	// host's /dev/* and pin nodes (notably /dev/rsd0) onto the runtime device.
	var buf bytes.Buffer
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
		Devices: config.DeviceConfig{
			Disabled: true,
		},
	}
	opts := &Options{
		Config:            cfg,
		Format:            "yaml",
		ErrorMode:         ErrorModeLenient,
		Logger:            &mockLogger{},
		LibraryDiscoverer: &mockLibraryDiscoverer{},
		ToolDiscoverer:    &mockToolDiscoverer{},
		// DeviceDiscoverer intentionally nil
	}

	// When
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then: spec is written and contains no deviceNodes block.
	require.NoError(t, err)
	output := buf.String()
	assert.NotContains(t, output, "deviceNodes:",
		"Devices.Disabled=true must suppress all deviceNodes emission")
}

func TestGenerateCDISpec_DevicesDisabled_RespectsCallerSuppliedDiscoverer(t *testing.T) {
	// Given: caller supplies a DeviceDiscoverer AND sets Devices.Disabled=true.
	// The discoverer is allowed to run (caller owns lifecycle), but the
	// generator must still drop the devices so K8s deployments are protected
	// even if a future refactor wires a discoverer in by accident.
	var buf bytes.Buffer
	mockDevDisc := &mockDeviceDiscoverer{
		devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
			{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
		},
	}
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
		Devices: config.DeviceConfig{
			Disabled: true,
		},
	}
	opts := &Options{
		Config:            cfg,
		Format:            "yaml",
		ErrorMode:         ErrorModeLenient,
		Logger:            &mockLogger{},
		LibraryDiscoverer: &mockLibraryDiscoverer{},
		ToolDiscoverer:    &mockToolDiscoverer{},
		DeviceDiscoverer:  mockDevDisc,
	}

	// When
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then: even though the discoverer was called, no deviceNodes are emitted.
	require.NoError(t, err)
	assert.NotContains(t, buf.String(), "deviceNodes:",
		"generator-level defense must suppress device-node emission when Devices.Disabled=true")
}

func TestGenerateCDISpec_DevicesEnabled_EmitsDeviceNodes(t *testing.T) {
	// Given: default Devices.Disabled=false (Docker path) with a mock that returns devices.
	var buf bytes.Buffer
	mockDevDisc := &mockDeviceDiscoverer{
		devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
		},
	}
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
		// Devices.Disabled left as zero value (false)
	}
	opts := &Options{
		Config:            cfg,
		Format:            "yaml",
		ErrorMode:         ErrorModeLenient,
		Logger:            &mockLogger{},
		LibraryDiscoverer: &mockLibraryDiscoverer{},
		ToolDiscoverer:    &mockToolDiscoverer{},
		DeviceDiscoverer:  mockDevDisc,
	}

	// When
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then: device nodes are emitted (Docker-compatible v0.1.1 behavior preserved).
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "deviceNodes:")
	assert.Contains(t, output, "/dev/rbln0")
}

func TestGenerateCDISpecToWriter_DefaultFormat(t *testing.T) {
	// Given options without format specified
	var buf bytes.Buffer
	cfg := &config.Config{
		CDI: config.CDIConfig{
			Vendor: "rebellions.ai",
			Class:  "npu",
		},
	}
	opts := &Options{
		Config:    cfg,
		ErrorMode: ErrorModeLenient,
		Logger:    &mockLogger{},
	}

	// When GenerateCDISpecToWriter is called
	err := GenerateCDISpecToWriter(&buf, opts)

	// Then it should succeed using default yaml format
	require.NoError(t, err)
	assert.Contains(t, buf.String(), "cdiVersion")
}

// staticResolver pins a NPU→RSD mapping for tests so we can assert that
// Options.RsdResolver short-circuits the auto-load path.
type staticResolver struct{ m map[uint32]uint32 }

func (s staticResolver) Resolve(npu uint32) (uint32, bool) {
	rsd, ok := s.m[npu]
	return rsd, ok
}

func TestResolveTopology_HonorsExplicitResolver(t *testing.T) {
	// Given a caller-supplied resolver, resolveTopology must hand it back
	// unchanged so tests and future Phase-2 callers can pin a known
	// topology without triggering the librbln-ml auto-load.
	want := staticResolver{m: map[uint32]uint32{0: 7}}
	logger := &mockLogger{}
	opts := &Options{
		Config:      &config.Config{},
		Logger:      logger,
		RsdResolver: want,
	}

	got := resolveTopology(opts)

	rsd, ok := got.Resolve(0)
	require.True(t, ok)
	assert.Equal(t, uint32(7), rsd)
	assert.Empty(t, logger.warnings,
		"explicit resolver path must NOT trigger the librbln-ml fallback warning")
}

func TestResolveTopology_DevicesDisabled_SkipsLoad(t *testing.T) {
	// Given Devices.Disabled=true (K8s path) — device-plugin owns RSD
	// allocation, so resolveTopology must skip the librbln-ml load entirely
	// rather than open /dev/rbln* and emit a warning the operator can't act on.
	logger := &mockLogger{}
	opts := &Options{
		Config: &config.Config{
			Devices: config.DeviceConfig{Disabled: true},
		},
		Logger: logger,
	}

	got := resolveTopology(opts)

	_, ok := got.Resolve(0)
	assert.False(t, ok, "K8s path must use NoopResolver")
	assert.Empty(t, logger.warnings,
		"K8s path must not emit a fallback warning since it never tried to load")
	assert.IsType(t, topology.NoopResolver{}, got)
}

func TestResolveTopology_AutoLoadFallsBackInStubBuild(t *testing.T) {
	// Given the default (no with_rblnml) build, resolveTopology must invoke
	// LoadOrFallback and surface the warning through the caller's logger so
	// operators see "RSD attachment unavailable" in journald rather than
	// finding it the hard way when a container fails to start.
	logger := &mockLogger{}
	opts := &Options{
		Config: &config.Config{},
		Logger: logger,
	}

	got := resolveTopology(opts)

	_, ok := got.Resolve(0)
	assert.False(t, ok, "stub build must fall back to NoopResolver")
	require.NotEmpty(t, logger.warnings, "fallback must emit a warning")
	assert.True(t, strings.Contains(strings.ToLower(logger.warnings[0]), "rsd"),
		"warning should mention RSD so operators can correlate: %q", logger.warnings[0])
}

func TestResolveTopology_NilLoggerTolerated(t *testing.T) {
	// Defensive: minimal call sites (early bootstrap, tests) may not have a
	// logger configured. resolveTopology must still return a working
	// resolver rather than panic on the nil dereference.
	opts := &Options{Config: &config.Config{}}

	got := resolveTopology(opts)

	_, ok := got.Resolve(0)
	assert.False(t, ok)
}

func TestResolveTopology_LogsLoadStats(t *testing.T) {
	// Operators monitoring the daemon need to see whether the rblnml load
	// succeeded, fell back, or returned a partial mapping — the difference
	// between "0 NPUs mapped because the host has none" and "0 NPUs mapped
	// because the driver call failed" is invisible without this Info line.
	// In the stub build the load always falls back, so the log line should
	// reflect that.
	logger := &mockLogger{}
	opts := &Options{
		Config: &config.Config{},
		Logger: logger,
	}

	_ = resolveTopology(opts)

	require.NotEmpty(t, logger.infos,
		"resolveTopology must emit an Info line summarizing the load outcome")
	assert.Contains(t, strings.ToLower(logger.infos[0]), "rsd topology",
		"summary should start with the recognizable RSD topology prefix: %q", logger.infos[0])
}

func TestResolveTopology_ExplicitResolver_NoInfoLog(t *testing.T) {
	// When the caller pins a resolver explicitly (tests, future bootstrap
	// flows), we shouldn't pretend to have done a load — emitting a stale
	// "fallback to no-op" line would mislead operators reading the logs.
	logger := &mockLogger{}
	opts := &Options{
		Config:      &config.Config{},
		Logger:      logger,
		RsdResolver: staticResolver{m: map[uint32]uint32{0: 1}},
	}

	_ = resolveTopology(opts)

	assert.Empty(t, logger.infos,
		"explicit resolver path must not emit a load-summary log line")
}

func TestResolveTopology_DevicesDisabled_NoInfoLog(t *testing.T) {
	// K8s path skips the load entirely — no log line, since there's nothing
	// to summarize and the operator already knows device-plugin owns RSD.
	logger := &mockLogger{}
	opts := &Options{
		Config: &config.Config{Devices: config.DeviceConfig{Disabled: true}},
		Logger: logger,
	}

	_ = resolveTopology(opts)

	assert.Empty(t, logger.infos,
		"K8s path skips the load, so it must not emit a topology summary")
}
