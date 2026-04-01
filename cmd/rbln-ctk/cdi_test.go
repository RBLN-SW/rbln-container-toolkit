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
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/cdi/setup"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

type mockLibraryDiscoverer struct {
	rblnLibs   []discover.Library
	rblnErr    error
	deps       []discover.Library
	depsErr    error
	plugins    []discover.Library
	pluginsErr error
}

func (m *mockLibraryDiscoverer) DiscoverRBLN() ([]discover.Library, error) {
	return m.rblnLibs, m.rblnErr
}

func (m *mockLibraryDiscoverer) DiscoverDependencies(_ []discover.Library) ([]discover.Library, error) {
	return m.deps, m.depsErr
}

func (m *mockLibraryDiscoverer) DiscoverPlugins() ([]discover.Library, error) {
	return m.plugins, m.pluginsErr
}

type mockToolDiscoverer struct {
	tools []discover.Tool
	err   error
}

func (m *mockToolDiscoverer) Discover() ([]discover.Tool, error) {
	return m.tools, m.err
}

func TestDiscoverResourcesWithDeps_Success(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so"},
		},
		deps: []discover.Library{
			{Name: "libc.so.6", Path: "/lib64/libc.so.6"},
		},
		plugins: []discover.Library{
			{Name: "libmlx5.so", Path: "/usr/lib64/libibverbs/libmlx5.so"},
		},
	}
	toolDiscoverer := &mockToolDiscoverer{
		tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi"},
		},
	}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.NoError(t, err)
	assert.Len(t, result.Libraries, 3)
	assert.Len(t, result.Tools, 1)
	assert.Equal(t, "librbln-ml.so", result.Libraries[0].Name)
	assert.Equal(t, "rbln-smi", result.Tools[0].Name)
}

func TestDiscoverResourcesWithDeps_RBLNDiscoveryError(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnErr: errors.New("permission denied"),
	}
	toolDiscoverer := &mockToolDiscoverer{}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "discover RBLN libraries")
}

func TestDiscoverResourcesWithDeps_PluginDiscoveryError(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs:   []discover.Library{},
		deps:       []discover.Library{},
		pluginsErr: errors.New("plugin path not found"),
	}
	toolDiscoverer := &mockToolDiscoverer{}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "discover plugins")
}

func TestDiscoverResourcesWithDeps_ToolDiscoveryError(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{},
		deps:     []discover.Library{},
		plugins:  []discover.Library{},
	}
	toolDiscoverer := &mockToolDiscoverer{
		err: errors.New("tool not found"),
	}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "discover tools")
}

func TestDiscoverResourcesWithDeps_DependencyErrorContinues(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so"},
		},
		depsErr: errors.New("ldd failed"),
		plugins: []discover.Library{},
	}
	toolDiscoverer := &mockToolDiscoverer{
		tools: []discover.Tool{},
	}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "discover dependencies")
}

func TestDiscoverResourcesWithDeps_EmptyResult(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{},
		deps:     []discover.Library{},
		plugins:  []discover.Library{},
	}
	toolDiscoverer := &mockToolDiscoverer{
		tools: []discover.Tool{},
	}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result.Libraries)
	assert.Empty(t, result.Tools)
}

func TestDiscoverResourcesWithDeps_WithDebugLogging(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so"},
		},
		deps:    []discover.Library{},
		plugins: []discover.Library{},
	}
	toolDiscoverer := &mockToolDiscoverer{
		tools: []discover.Tool{},
	}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestDiscoverResourcesWithDeps_MultipleLibraries(t *testing.T) {
	libDiscoverer := &mockLibraryDiscoverer{
		rblnLibs: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so"},
			{Name: "librbln-core.so", Path: "/usr/lib64/librbln-core.so"},
			{Name: "librbln-runtime.so", Path: "/usr/lib64/librbln-runtime.so"},
		},
		deps: []discover.Library{
			{Name: "libstdc++.so.6", Path: "/lib64/libstdc++.so.6"},
			{Name: "libm.so.6", Path: "/lib64/libm.so.6"},
		},
		plugins: []discover.Library{
			{Name: "libmlx5.so", Path: "/usr/lib64/libibverbs/libmlx5.so"},
		},
	}
	toolDiscoverer := &mockToolDiscoverer{
		tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi"},
			{Name: "rbln-stat", Path: "/usr/bin/rbln-stat"},
		},
	}

	result, err := setup.DiscoverResources(libDiscoverer, toolDiscoverer, nil)

	assert.NoError(t, err)
	assert.Len(t, result.Libraries, 6)
	assert.Len(t, result.Tools, 2)
}

func TestRunCDIGenerate_DryRun_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", true, "test")
	cmd.Flags().String("output", "/var/run/cdi/rbln.yaml", "test")
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/", "test")
	cmd.Flags().String("container-library-path", "", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", true)
	viper.Set("output", "/var/run/cdi/rbln.yaml")
	viper.Set("format", "yaml")
	viper.Set("driver-root", "/")
	viper.Set("container-library-path", "")
	viper.Set("debug", false)
	viper.Set("quiet", false)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIGenerate_OutputToStdout_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", false, "test")
	cmd.Flags().String("output", "-", "test")
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/", "test")
	cmd.Flags().String("container-library-path", "", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", false)
	viper.Set("output", "-")
	viper.Set("format", "yaml")
	viper.Set("driver-root", "/")
	viper.Set("container-library-path", "")
	viper.Set("debug", false)
	viper.Set("quiet", false)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIGenerate_OutputToFile_Success(t *testing.T) {
	// Given
	tmpDir := t.TempDir()
	outputPath := tmpDir + "/rbln.yaml"

	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", false, "test")
	cmd.Flags().String("output", outputPath, "test")
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/", "test")
	cmd.Flags().String("container-library-path", "", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", true, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", false)
	viper.Set("output", outputPath)
	viper.Set("format", "yaml")
	viper.Set("driver-root", "/")
	viper.Set("container-library-path", "")
	viper.Set("debug", false)
	viper.Set("quiet", true)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIGenerate_InvalidFormat_Error(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", false, "test")
	cmd.Flags().String("output", "/var/run/cdi/rbln.yaml", "test")
	cmd.Flags().String("format", "invalid-format", "test")
	cmd.Flags().String("driver-root", "/", "test")
	cmd.Flags().String("container-library-path", "", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", false)
	viper.Set("output", "/var/run/cdi/rbln.yaml")
	viper.Set("format", "invalid-format")
	viper.Set("driver-root", "/")
	viper.Set("container-library-path", "")
	viper.Set("debug", false)
	viper.Set("quiet", false)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestRunCDIGenerate_DryRunWithJSON_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", true, "test")
	cmd.Flags().String("output", "/var/run/cdi/rbln.json", "test")
	cmd.Flags().String("format", "json", "test")
	cmd.Flags().String("driver-root", "/", "test")
	cmd.Flags().String("container-library-path", "", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", true)
	viper.Set("output", "/var/run/cdi/rbln.json")
	viper.Set("format", "json")
	viper.Set("driver-root", "/")
	viper.Set("container-library-path", "")
	viper.Set("debug", false)
	viper.Set("quiet", false)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIGenerate_WithDriverRoot_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", true, "test")
	cmd.Flags().String("output", "/var/run/cdi/rbln.yaml", "test")
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/run/rbln/driver", "test")
	cmd.Flags().String("container-library-path", "", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", true)
	viper.Set("output", "/var/run/cdi/rbln.yaml")
	viper.Set("format", "yaml")
	viper.Set("driver-root", "/run/rbln/driver")
	viper.Set("container-library-path", "")
	viper.Set("debug", false)
	viper.Set("quiet", false)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIGenerate_WithLibraryIsolation_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().Bool("dry-run", true, "test")
	cmd.Flags().String("output", "/var/run/cdi/rbln.yaml", "test")
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/", "test")
	cmd.Flags().String("container-library-path", "/rbln/lib64", "test")
	cmd.Flags().Bool("debug", false, "test")
	cmd.Flags().Bool("quiet", false, "test")
	cmd.Flags().String("config", "", "test")

	viper.Set("dry-run", true)
	viper.Set("output", "/var/run/cdi/rbln.yaml")
	viper.Set("format", "yaml")
	viper.Set("driver-root", "/")
	viper.Set("container-library-path", "/rbln/lib64")
	viper.Set("debug", false)
	viper.Set("quiet", false)
	viper.Set("config", "")

	// When
	err := runCDIGenerate(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_FormatTable_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().String("driver-root", "/", "test")

	viper.Set("debug", false)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_FormatJSON_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "json", "test")
	cmd.Flags().String("driver-root", "/", "test")

	viper.Set("debug", false)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_FormatYAML_Success(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/", "test")

	viper.Set("debug", false)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_EmptyResults(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().String("driver-root", "/", "test")

	viper.Set("debug", false)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_MultipleResults(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "json", "test")
	cmd.Flags().String("driver-root", "/", "test")

	viper.Set("debug", false)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_WithDriverRoot(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "yaml", "test")
	cmd.Flags().String("driver-root", "/run/rbln/driver", "test")

	viper.Set("debug", false)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}

func TestRunCDIList_WithDebugLogging(t *testing.T) {
	// Given
	cmd := &cobra.Command{}
	cmd.Flags().String("format", "table", "test")
	cmd.Flags().String("driver-root", "/", "test")

	viper.Set("debug", true)
	viper.Set("config", "")

	// When
	err := runCDIList(cmd, []string{})

	// Then
	assert.NoError(t, err)
}
