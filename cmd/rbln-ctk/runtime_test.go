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
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/runtime"
)

type mockConfigurator struct {
	configureErr error
	dryRunResult string
	dryRunErr    error
	configured   bool
}

func (m *mockConfigurator) Configure() error {
	m.configured = true
	return m.configureErr
}

func (m *mockConfigurator) DryRun() (string, error) {
	return m.dryRunResult, m.dryRunErr
}

func TestExecuteRuntimeConfigure_UnsupportedRuntime(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "invalid",
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, nil, &stdout, &stdin)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported runtime")
}

func TestExecuteRuntimeConfigure_DetectRuntimeError(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "",
	}
	mockDetector := func() (runtime.RuntimeType, error) {
		return "", errors.New("no runtime found")
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, mockDetector, nil, &stdout, &stdin)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "detect runtime")
}

func TestExecuteRuntimeConfigure_DetectRuntimeSuccess(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "",
		dryRun:      true,
	}
	mockDetector := func() (runtime.RuntimeType, error) {
		return runtime.RuntimeContainerd, nil
	}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return &mockConfigurator{dryRunResult: "mock diff"}, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, mockDetector, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Detected runtime: containerd")
}

func TestExecuteRuntimeConfigure_ConfiguratorError(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
	}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return nil, errors.New("failed to create configurator")
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create configurator")
}

func TestExecuteRuntimeConfigure_DryRun(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		dryRun:      true,
	}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return &mockConfigurator{dryRunResult: "--- config.toml\n+++ config.toml\nenable_cdi = true"}, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Changes that would be made:")
	assert.Contains(t, stdout.String(), "enable_cdi = true")
}

func TestExecuteRuntimeConfigure_DryRunError(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		dryRun:      true,
	}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return &mockConfigurator{dryRunErr: errors.New("config parse error")}, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dry run")
}

func TestExecuteRuntimeConfigure_UserAborts(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		skipConfirm: false,
		quiet:       false,
	}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return &mockConfigurator{}, nil
	}
	var stdout bytes.Buffer
	stdin := strings.NewReader("n\n")

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, stdin)

	// Then
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Aborted.")
}

func TestExecuteRuntimeConfigure_UserConfirms(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		skipConfirm: false,
		quiet:       false,
	}
	configurator := &mockConfigurator{}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return configurator, nil
	}
	var stdout bytes.Buffer
	stdin := strings.NewReader("y\n")

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, stdin)

	// Then
	assert.NoError(t, err)
	assert.True(t, configurator.configured)
	assert.Contains(t, stdout.String(), "configured for CDI support")
}

func TestExecuteRuntimeConfigure_SkipConfirm(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		skipConfirm: true,
	}
	configurator := &mockConfigurator{}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return configurator, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.True(t, configurator.configured)
}

func TestExecuteRuntimeConfigure_ConfigureError(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		skipConfirm: true,
	}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return &mockConfigurator{configureErr: errors.New("permission denied")}, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "configure runtime")
}

func TestExecuteRuntimeConfigure_QuietMode(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		skipConfirm: true,
		quiet:       true,
	}
	configurator := &mockConfigurator{}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return configurator, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.True(t, configurator.configured)
	assert.Empty(t, stdout.String())
}

func TestExecuteRuntimeConfigure_AllRuntimes(t *testing.T) {
	runtimes := []string{"containerd", "crio", "docker"}

	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			// Given
			opts := runtimeConfigureOptions{
				runtimeType: rt,
				skipConfirm: true,
			}
			configurator := &mockConfigurator{}
			mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
				return configurator, nil
			}
			var stdout bytes.Buffer
			var stdin bytes.Buffer

			// When
			err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

			// Then
			assert.NoError(t, err)
			assert.True(t, configurator.configured)
			assert.Contains(t, stdout.String(), rt)
		})
	}
}

func TestExecuteRuntimeConfigure_CustomConfigPath(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		configPath:  "/custom/config.toml",
		skipConfirm: true,
	}
	var capturedConfigPath string
	mockFactory := func(_ runtime.RuntimeType, configPath string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		capturedConfigPath = configPath
		return &mockConfigurator{}, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, "/custom/config.toml", capturedConfigPath)
}

func TestExecuteRuntimeConfigure_YesFlagSkipsPrompt(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		skipConfirm: true,
		quiet:       false,
	}
	configurator := &mockConfigurator{}
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return configurator, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.True(t, configurator.configured)
	assert.NotContains(t, stdout.String(), "Continue?")
}

func TestExecuteRuntimeConfigure_DryRunShowsDiff(t *testing.T) {
	// Given
	opts := runtimeConfigureOptions{
		runtimeType: "containerd",
		dryRun:      true,
		skipConfirm: false,
		quiet:       false,
	}
	expectedDiff := "--- config.toml\n+++ config.toml\nenable_cdi = true"
	mockFactory := func(_ runtime.RuntimeType, _ string, _ *runtime.ConfiguratorOptions) (runtime.Configurator, error) {
		return &mockConfigurator{dryRunResult: expectedDiff}, nil
	}
	var stdout bytes.Buffer
	var stdin bytes.Buffer

	// When
	err := executeRuntimeConfigure(opts, nil, mockFactory, &stdout, &stdin)

	// Then
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Changes that would be made:")
	assert.Contains(t, stdout.String(), expectedDiff)
	assert.NotContains(t, stdout.String(), "Continue?")
}
