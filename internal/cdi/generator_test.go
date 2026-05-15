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

package cdi

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	specs "tags.cncf.io/container-device-interface/specs-go"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/config"
	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

func TestGenerator_Generate_EmptyResult(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{},
		Tools:     []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	assert.NotNil(t, spec)
	assert.Equal(t, "0.5.0", spec.Version)
	assert.Equal(t, "rebellions.ai/npu", spec.Kind)
	assert.NotEmpty(t, spec.Devices)
}

func TestGenerator_Generate_WithLibraries(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
			{Name: "librbln-thunk.so", Path: "/usr/lib64/librbln-thunk.so", ContainerPath: "/usr/lib64/librbln-thunk.so", Type: discover.LibraryTypeRBLN},
		},
		Tools: []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	assertDeviceNames(t, spec, "all", "runtime")
	assert.GreaterOrEqual(t, len(spec.ContainerEdits.Mounts), 2)
	for _, mount := range spec.ContainerEdits.Mounts {
		assert.Contains(t, mount.Options, "ro")
		assert.Contains(t, mount.Options, "bind")
	}
}

func TestGenerator_Generate_WithTools(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi", ContainerPath: "/usr/bin/rbln-smi"},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	var hasToolMount bool
	for _, mount := range spec.ContainerEdits.Mounts {
		if mount.HostPath == "/usr/bin/rbln-smi" {
			hasToolMount = true
			break
		}
	}
	assert.True(t, hasToolMount)
	for _, env := range spec.ContainerEdits.Env {
		assert.False(t, strings.HasPrefix(env, "PATH="))
	}
}

func TestGenerator_Generate_EnvVars(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
		Tools: []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	var hasLDPath bool
	for _, env := range spec.ContainerEdits.Env {
		if len(env) > 16 && env[:16] == "LD_LIBRARY_PATH=" {
			hasLDPath = true
			break
		}
	}
	assert.False(t, hasLDPath)
	assert.NotEmpty(t, spec.ContainerEdits.Hooks)
}

func TestGenerator_Generate_CustomVendorClass(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{}
	cfg := config.DefaultConfig()
	cfg.CDI.Vendor = "custom.vendor"
	cfg.CDI.Class = "custom-class"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	assert.Equal(t, "custom.vendor/custom-class", spec.Kind)
}

func TestGenerator_Generate_MountOptions(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount.HostPath)
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount.ContainerPath)
	assert.Contains(t, mount.Options, "ro")
	assert.Contains(t, mount.Options, "nosuid")
	assert.Contains(t, mount.Options, "nodev")
	assert.Contains(t, mount.Options, "bind")
}

func TestGenerator_Generate_SymlinkHookFromSONAME(t *testing.T) {
	// Given - versioned file with symlinks on host
	originalReadSONAME := discover.ReadSONAME
	originalLinkExists := discover.LinkExists
	defer func() {
		discover.ReadSONAME = originalReadSONAME
		discover.LinkExists = originalLinkExists
	}()

	discover.ReadSONAME = func(_ string) (string, error) {
		return "librbln-ml.so.1", nil
	}
	discover.LinkExists = func(path string) (bool, error) {
		if strings.HasSuffix(path, "librbln-ml.so.1") || strings.HasSuffix(path, "librbln-ml.so") {
			return true, nil
		}
		return false, nil
	}

	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so.1.0.0",
				Path:          "/usr/lib64/librbln-ml.so.1.0.0",
				ContainerPath: "/usr/lib64/librbln-ml.so.1.0.0",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)

	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/usr/lib64/librbln-ml.so.1.0.0", mount.HostPath)
	assert.Equal(t, "/usr/lib64/librbln-ml.so.1.0.0", mount.ContainerPath)

	var symlinkHook *specs.Hook
	for _, hook := range spec.ContainerEdits.Hooks {
		if strings.Contains(strings.Join(hook.Args, " "), "create-symlinks") {
			symlinkHook = hook
			break
		}
	}
	require.NotNil(t, symlinkHook, "Should have create-symlinks hook")
	hookArgs := strings.Join(symlinkHook.Args, " ")
	assert.Contains(t, hookArgs, "librbln-ml.so.1.0.0::/usr/lib64/librbln-ml.so.1")
	assert.Contains(t, hookArgs, "librbln-ml.so.1::/usr/lib64/librbln-ml.so")
}

func TestGenerator_Generate_NoSymlinkHookWhenLinksNotOnHost(t *testing.T) {
	// Given
	originalReadSONAME := discover.ReadSONAME
	originalLinkExists := discover.LinkExists
	defer func() {
		discover.ReadSONAME = originalReadSONAME
		discover.LinkExists = originalLinkExists
	}()

	discover.ReadSONAME = func(_ string) (string, error) {
		return "librbln-ml.so.1", nil
	}
	discover.LinkExists = func(_ string) (bool, error) {
		return false, nil
	}

	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so.1.0.0",
				Path:          "/usr/lib64/librbln-ml.so.1.0.0",
				ContainerPath: "/usr/lib64/librbln-ml.so.1.0.0",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)

	for _, hook := range spec.ContainerEdits.Hooks {
		if strings.Contains(strings.Join(hook.Args, " "), "create-symlinks") {
			t.Error("Should NOT have create-symlinks hook when links don't exist on host")
		}
	}
}

func TestGenerator_Generate_WithSELinuxDisabled(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
	}
	cfg := config.DefaultConfig()
	cfg.SELinux.Enabled = false
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.NotContains(t, mount.Options, "z")
	assert.NotContains(t, mount.Options, "Z")
}

func TestGenerator_Generate_WithSELinuxEnabled_SharedContext(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
	}
	cfg := config.DefaultConfig()
	cfg.SELinux.Enabled = true
	cfg.SELinux.MountContext = "z"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Contains(t, mount.Options, "z")
	assert.Contains(t, mount.Options, "ro")
	assert.Contains(t, mount.Options, "bind")
}

func TestGenerator_Generate_WithSELinuxEnabled_PrivateContext(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
	}
	cfg := config.DefaultConfig()
	cfg.SELinux.Enabled = true
	cfg.SELinux.MountContext = "Z"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Contains(t, mount.Options, "Z")
	assert.NotContains(t, mount.Options, "z")
}

func TestGenerator_Generate_SELinuxAppliedToAllMounts(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
			{Name: "librbln-thunk.so", Path: "/usr/lib64/librbln-thunk.so", ContainerPath: "/usr/lib64/librbln-thunk.so", Type: discover.LibraryTypeRBLN},
		},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi", ContainerPath: "/usr/bin/rbln-smi"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.SELinux.Enabled = true
	cfg.SELinux.MountContext = "z"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	for _, mount := range spec.ContainerEdits.Mounts {
		assert.Contains(t, mount.Options, "z")
	}
}

func TestGenerator_Generate_WithContainerPath_NoLDLibraryPath(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	for _, env := range spec.ContainerEdits.Env {
		if strings.HasPrefix(env, "LD_LIBRARY_PATH=") {
			assert.NotContains(t, env, "/rbln:")
		}
	}
}

func TestGenerator_Generate_WithContainerPath_NoLDLibraryPath_WithHook(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	for _, env := range spec.ContainerEdits.Env {
		assert.False(t, strings.HasPrefix(env, "LD_LIBRARY_PATH="))
	}
	require.NotNil(t, spec.ContainerEdits.Hooks)
	require.Len(t, spec.ContainerEdits.Hooks, 1)
	hook := spec.ContainerEdits.Hooks[0]
	assert.Equal(t, "createContainer", hook.HookName)
	assert.Equal(t, "/usr/local/bin/rbln-cdi-hook", hook.Path)
	assert.Contains(t, hook.Args, "update-ldcache")
	assert.Contains(t, hook.Args, "--folder")
	assert.Contains(t, hook.Args, "/usr/lib64/rbln")
}

func TestGenerator_Generate_WithContainerPath_MountsUseContainerPath(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
			{
				Name:          "libbz2.so.1.0",
				Path:          "/lib/x86_64-linux-gnu/libbz2.so.1.0",
				ContainerPath: "/usr/lib64/rbln/libbz2.so.1.0",
				Type:          discover.LibraryTypeDependency,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	for _, mount := range spec.ContainerEdits.Mounts {
		if strings.Contains(mount.HostPath, "lib") {
			assert.True(t, strings.HasPrefix(mount.ContainerPath, "/usr/lib64/rbln/"))
		}
	}
}

func TestGenerator_Generate_WithContainerPath_BinaryUnchanged(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi", ContainerPath: "/usr/bin/rbln-smi"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	var toolMount *specs.Mount
	for _, mount := range spec.ContainerEdits.Mounts {
		if mount.HostPath == "/usr/bin/rbln-smi" {
			toolMount = mount
			break
		}
	}
	require.NotNil(t, toolMount)
	assert.Equal(t, "/usr/bin/rbln-smi", toolMount.ContainerPath)
}

func TestGenerator_Generate_WithContainerPath_SymlinkHookFromSONAME(t *testing.T) {
	// Given
	originalReadSONAME := discover.ReadSONAME
	originalLinkExists := discover.LinkExists
	defer func() {
		discover.ReadSONAME = originalReadSONAME
		discover.LinkExists = originalLinkExists
	}()

	discover.ReadSONAME = func(_ string) (string, error) {
		return "librbln-ml.so.1", nil
	}
	discover.LinkExists = func(path string) (bool, error) {
		if strings.HasSuffix(path, "librbln-ml.so.1") || strings.HasSuffix(path, "librbln-ml.so") {
			return true, nil
		}
		return false, nil
	}

	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so.1.0.0",
				Path:          "/usr/lib64/librbln-ml.so.1.0.0",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so.1.0.0",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)

	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/usr/lib64/librbln-ml.so.1.0.0", mount.HostPath)
	assert.Equal(t, "/usr/lib64/rbln/librbln-ml.so.1.0.0", mount.ContainerPath)

	var symlinkHook *specs.Hook
	for _, hook := range spec.ContainerEdits.Hooks {
		if strings.Contains(strings.Join(hook.Args, " "), "create-symlinks") {
			symlinkHook = hook
			break
		}
	}
	require.NotNil(t, symlinkHook, "Should have create-symlinks hook")
	hookArgs := strings.Join(symlinkHook.Args, " ")
	assert.Contains(t, hookArgs, "librbln-ml.so.1.0.0::/usr/lib64/rbln/librbln-ml.so.1")
	assert.Contains(t, hookArgs, "librbln-ml.so.1::/usr/lib64/rbln/librbln-ml.so")
}

// Hook generation tests
// Always use hooks for ldcache update (supports setuid binaries)

func TestGenerator_Generate_DefaultMode_HasHook(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	var hasLDPath bool
	for _, env := range spec.ContainerEdits.Env {
		if strings.HasPrefix(env, "LD_LIBRARY_PATH=") {
			hasLDPath = true
			break
		}
	}
	assert.False(t, hasLDPath)
	require.NotEmpty(t, spec.ContainerEdits.Hooks)
	hook := spec.ContainerEdits.Hooks[0]
	assert.Equal(t, "createContainer", hook.HookName)
	assert.Contains(t, hook.Args, "update-ldcache")
}

func TestGenerator_Generate_IsolationMode_HasHook(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	require.NotEmpty(t, spec.ContainerEdits.Hooks)
	hook := spec.ContainerEdits.Hooks[0]
	assert.Equal(t, "createContainer", hook.HookName)
	assert.Equal(t, cfg.Hooks.Path, hook.Path)
	assert.Contains(t, hook.Args, "rbln-cdi-hook")
	assert.Contains(t, hook.Args, "update-ldcache")
}

func TestGenerator_Generate_HookHasCorrectFolders(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
			{
				Name:          "libmlx5.so",
				Path:          "/usr/lib64/libibverbs/libmlx5.so",
				ContainerPath: "/usr/lib64/rbln/libibverbs/libmlx5.so",
				Type:          discover.LibraryTypePlugin,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Hooks)
	var ldcacheHook *specs.Hook
	for _, h := range spec.ContainerEdits.Hooks {
		for _, arg := range h.Args {
			if arg == "update-ldcache" {
				ldcacheHook = h
				break
			}
		}
	}
	require.NotNil(t, ldcacheHook)
	assert.Contains(t, ldcacheHook.Args, "--folder")
	assert.Contains(t, ldcacheHook.Args, "/usr/lib64/rbln")
}

func TestGenerator_Generate_HookHasLdconfigPath(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	cfg.Hooks.LdconfigPath = "/usr/sbin/ldconfig"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Hooks)
	hook := spec.ContainerEdits.Hooks[0]
	assert.Contains(t, hook.Env, "RBLN_CDI_HOOK_LDCONFIG_PATH=/usr/sbin/ldconfig")
}

func TestGenerator_Generate_EmptyLibraries_NoHook(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi", ContainerPath: "/usr/bin/rbln-smi"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	assert.Empty(t, spec.ContainerEdits.Hooks)
}

// Hook environment variables tests

func TestGenerator_Generate_HookEnvVariables_FolderAndDebug(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	cfg.Debug = true
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Hooks)
	hook := spec.ContainerEdits.Hooks[0]
	require.NotNil(t, hook.Env)
	require.Len(t, hook.Env, 2)
	var hasLdconfigPath bool
	for _, env := range hook.Env {
		if strings.HasPrefix(env, "RBLN_CDI_HOOK_LDCONFIG_PATH=") {
			hasLdconfigPath = true
			break
		}
	}
	assert.True(t, hasLdconfigPath, "RBLN_CDI_HOOK_LDCONFIG_PATH should be in env")
	assert.Contains(t, hook.Env, "RBLN_CDI_HOOK_DEBUG=true")
	assert.Contains(t, hook.Args, "--folder")
	assert.Contains(t, hook.Args, "/usr/lib64/rbln")
}

func TestGenerator_Generate_HookEnvVariables_DebugFalse(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	cfg.Debug = false
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Hooks)
	hook := spec.ContainerEdits.Hooks[0]
	require.NotNil(t, hook.Env)
	require.Len(t, hook.Env, 2)
	var hasLdconfigPath bool
	for _, env := range hook.Env {
		if strings.HasPrefix(env, "RBLN_CDI_HOOK_LDCONFIG_PATH=") {
			hasLdconfigPath = true
			break
		}
	}
	assert.True(t, hasLdconfigPath, "RBLN_CDI_HOOK_LDCONFIG_PATH should be in env")
	assert.Contains(t, hook.Env, "RBLN_CDI_HOOK_DEBUG=false")
}

// createEnvVars edge case tests (tested via Generate() method)
// Tests the private createEnvVars method indirectly through public Generate()
// Edge cases: empty libPaths, hasHooks true/false, LD_LIBRARY_PATH verification

func TestGenerator_Generate_CreateEnvVars_EmptyLibPaths_NoHooks(t *testing.T) {
	// Given - Empty libraries, no hooks expected
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{},
		Tools:     []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	// No libraries = no LD_LIBRARY_PATH expected
	for _, env := range spec.ContainerEdits.Env {
		assert.False(t, strings.HasPrefix(env, "LD_LIBRARY_PATH="),
			"LD_LIBRARY_PATH should not be set when no libraries present")
	}
	// No libraries = no hooks expected
	assert.Empty(t, spec.ContainerEdits.Hooks)
}

func TestGenerator_Generate_CreateEnvVars_PopulatedLibPaths_WithHooks(t *testing.T) {
	// Given - Libraries present, hooks enabled (default mode)
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
		Tools: []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	// When hooks are present, LD_LIBRARY_PATH should NOT be set (hooks handle it)
	for _, env := range spec.ContainerEdits.Env {
		assert.False(t, strings.HasPrefix(env, "LD_LIBRARY_PATH="),
			"LD_LIBRARY_PATH should not be set when hooks are present")
	}
	// Hooks should be present for library discovery
	assert.NotEmpty(t, spec.ContainerEdits.Hooks,
		"Hooks should be present when libraries exist")
}

func TestGenerator_Generate_CreateEnvVars_PopulatedLibPaths_IsolationMode_WithHooks(t *testing.T) {
	// Given - Libraries with isolation mode (container path set), hooks enabled
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
			{
				Name:          "libbz2.so.1.0",
				Path:          "/lib/x86_64-linux-gnu/libbz2.so.1.0",
				ContainerPath: "/usr/lib64/rbln/libbz2.so.1.0",
				Type:          discover.LibraryTypeDependency,
			},
		},
		Tools: []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	// Isolation mode with hooks: LD_LIBRARY_PATH should NOT be set
	for _, env := range spec.ContainerEdits.Env {
		assert.False(t, strings.HasPrefix(env, "LD_LIBRARY_PATH="),
			"LD_LIBRARY_PATH should not be set in isolation mode with hooks")
	}
	// Hooks should be present for ldcache update
	assert.NotEmpty(t, spec.ContainerEdits.Hooks,
		"Hooks should be present in isolation mode")
	var ldcacheHook *specs.Hook
	for _, h := range spec.ContainerEdits.Hooks {
		for _, arg := range h.Args {
			if arg == "update-ldcache" {
				ldcacheHook = h
				break
			}
		}
	}
	require.NotNil(t, ldcacheHook)
	assert.Contains(t, ldcacheHook.Args, "--folder")
	assert.Contains(t, ldcacheHook.Args, "/usr/lib64/rbln")
}

func TestGenerator_Generate_CreateEnvVars_MultipleLibraryPaths_WithHooks(t *testing.T) {
	// Given - Multiple libraries from different paths, hooks enabled
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
			{
				Name:          "librbln-thunk.so",
				Path:          "/usr/lib64/librbln-thunk.so",
				ContainerPath: "/usr/lib64/librbln-thunk.so",
				Type:          discover.LibraryTypeRBLN,
			},
			{
				Name:          "libmlx5.so",
				Path:          "/usr/lib64/libibverbs/libmlx5.so",
				ContainerPath: "/usr/lib64/libibverbs/libmlx5.so",
				Type:          discover.LibraryTypePlugin,
			},
		},
		Tools: []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)
	// Multiple libraries with hooks: LD_LIBRARY_PATH should NOT be set
	for _, env := range spec.ContainerEdits.Env {
		assert.False(t, strings.HasPrefix(env, "LD_LIBRARY_PATH="),
			"LD_LIBRARY_PATH should not be set when hooks are present")
	}
	assert.NotEmpty(t, spec.ContainerEdits.Hooks)
	var ldcacheHook *specs.Hook
	for _, h := range spec.ContainerEdits.Hooks {
		for _, arg := range h.Args {
			if arg == "update-ldcache" {
				ldcacheHook = h
				break
			}
		}
	}
	require.NotNil(t, ldcacheHook)
	assert.Contains(t, ldcacheHook.Args, "--folder")
	assert.Contains(t, ldcacheHook.Args, "/usr/lib64")
}

func TestGenerator_Generate_CreateEnvVars_NoEnvVarsWhenHooksPresent(t *testing.T) {
	// Given - Verify that LD_LIBRARY_PATH is never set when hooks are present
	// This is the core behavior of createEnvVars: hooks take precedence
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
		Tools: []discover.Tool{},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.Devices)

	// Verify hooks are present
	require.NotEmpty(t, spec.ContainerEdits.Hooks,
		"Hooks should be present for library discovery")

	// Verify LD_LIBRARY_PATH is NOT in env vars (hooks handle it)
	ldPathFound := false
	for _, env := range spec.ContainerEdits.Env {
		if strings.HasPrefix(env, "LD_LIBRARY_PATH=") {
			ldPathFound = true
			break
		}
	}
	assert.False(t, ldPathFound,
		"LD_LIBRARY_PATH must not be set when hooks are present (hooks handle library discovery)")
}

func TestGenerator_Generate_DriverRootSet_PathTransformation(t *testing.T) {
	// Given
	// driverRoot="/opt/driver" means host paths like "/opt/driver/usr/lib64/librbln.so"
	// should be mounted to "/usr/lib64/librbln.so" in container (prefix removed)
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/opt/driver/usr/lib64/librbln-ml.so",
				ContainerPath: "/opt/driver/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/opt/driver"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/opt/driver/usr/lib64/librbln-ml.so", mount.HostPath)
	// Container path should have driverRoot prefix removed
	assert.Equal(t, "/opt/driver/usr/lib64/librbln-ml.so", mount.ContainerPath)
}

func TestGenerator_Generate_DriverRootEmpty_DefaultBehavior(t *testing.T) {
	// Given - driverRoot is empty (default behavior)
	// Paths should be used as-is without transformation
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.DriverRoot = ""
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount.HostPath)
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount.ContainerPath)
}

func TestGenerator_Generate_DriverRootSlash_DefaultBehavior(t *testing.T) {
	// Given - driverRoot is "/" (root, same as empty)
	// Paths should be used as-is without transformation
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount.HostPath)
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount.ContainerPath)
}

func TestGenerator_Generate_MountOptions_Complete(t *testing.T) {
	// Given - Verify all mount options are present (bind, ro, nosuid, nodev)
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.SELinux.Enabled = false
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	// Verify all required mount options
	assert.Contains(t, mount.Options, "bind")
	assert.Contains(t, mount.Options, "ro")
	assert.Contains(t, mount.Options, "nosuid")
	assert.Contains(t, mount.Options, "nodev")
	// Verify no SELinux context when disabled
	assert.NotContains(t, mount.Options, "z")
	assert.NotContains(t, mount.Options, "Z")
}

func TestGenerator_Generate_SymlinkHookWithDriverRoot(t *testing.T) {
	// Given
	originalReadSONAME := discover.ReadSONAME
	originalLinkExists := discover.LinkExists
	defer func() {
		discover.ReadSONAME = originalReadSONAME
		discover.LinkExists = originalLinkExists
	}()

	discover.ReadSONAME = func(_ string) (string, error) {
		return "librbln-ml.so.1", nil
	}
	discover.LinkExists = func(path string) (bool, error) {
		if strings.HasSuffix(path, "librbln-ml.so.1") || strings.HasSuffix(path, "librbln-ml.so") {
			return true, nil
		}
		return false, nil
	}

	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so.1.0.0",
				Path:          "/opt/driver/usr/lib64/librbln-ml.so.1.0.0",
				ContainerPath: "/usr/lib64/librbln-ml.so.1.0.0",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/opt/driver"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)

	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/opt/driver/usr/lib64/librbln-ml.so.1.0.0", mount.HostPath)
	assert.Equal(t, "/usr/lib64/librbln-ml.so.1.0.0", mount.ContainerPath)

	var symlinkHook *specs.Hook
	for _, hook := range spec.ContainerEdits.Hooks {
		if strings.Contains(strings.Join(hook.Args, " "), "create-symlinks") {
			symlinkHook = hook
			break
		}
	}
	require.NotNil(t, symlinkHook, "Should have create-symlinks hook")
	hookArgs := strings.Join(symlinkHook.Args, " ")
	assert.Contains(t, hookArgs, "librbln-ml.so.1.0.0::/usr/lib64/librbln-ml.so.1")
	assert.Contains(t, hookArgs, "librbln-ml.so.1::/usr/lib64/librbln-ml.so")
}

func TestGenerator_Generate_DriverRootWithoutLeadingSlash_PathTransformation(t *testing.T) {
	// Given - driverRoot without leading slash (edge case)
	// Should still work correctly
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/opt/driver/usr/lib64/librbln-ml.so",
				ContainerPath: "/opt/driver/usr/lib64/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "opt/driver"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	require.NotEmpty(t, spec.ContainerEdits.Mounts)
	mount := spec.ContainerEdits.Mounts[0]
	assert.Equal(t, "/opt/driver/usr/lib64/librbln-ml.so", mount.HostPath)
}

// Integration test: Complete CDI spec output verification
// Tests end-to-end flow: Generator → Writer → YAML parsing → verification

func TestGenerator_Generate_CompleteSpecOutput(t *testing.T) {
	// Given - Create discovery result with libraries to generate mounts and hooks
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{
				Name:          "librbln-ml.so",
				Path:          "/usr/lib64/librbln-ml.so",
				ContainerPath: "/usr/lib64/rbln/librbln-ml.so",
				Type:          discover.LibraryTypeRBLN,
			},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Libraries.ContainerPath = "/usr/lib64/rbln"
	cfg.Debug = true
	gen := NewGenerator(cfg, nil)
	writer := NewWriter()

	// When - Generate spec and write to buffer
	spec, err := gen.Generate(result)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = writer.WriteToWriter(spec, &buf, "yaml")
	require.NoError(t, err)

	// Parse YAML output
	var parsed map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)

	// Then - Verify complete spec structure

	// 1. Verify flow style in raw YAML output (mount options inline array)
	output := buf.String()
	assert.Contains(t, output, "options: [ro,", "Mount options should be in flow style (inline array)")

	// 2. Verify devices exist — with no NPUs discovered we still emit the
	// `all` named handle plus its v0.1.x `runtime` compat alias (same content).
	devicesRaw, ok := parsed["devices"]
	require.True(t, ok, "devices field should exist")
	devices, ok := devicesRaw.([]interface{})
	require.True(t, ok, "devices should be a list")
	require.Len(t, devices, 2, "no NPU discovery → only `all` + `runtime` alias remain")

	// 3. Verify the named handles are "all" and its "runtime" alias.
	deviceNames := make([]string, 0, len(devices))
	for _, d := range devices {
		m, ok := d.(map[string]interface{})
		require.True(t, ok, "device should be a map")
		deviceNames = append(deviceNames, m["name"].(string))
	}
	assert.ElementsMatch(t, []string{"all", "runtime"}, deviceNames,
		"named handles should be 'all' and the v0.1.x 'runtime' alias")

	// 4. Verify top-level containerEdits (libs/tools/hooks/env now live there)
	editsRaw, ok := parsed["containerEdits"]
	require.True(t, ok, "top-level containerEdits should exist")
	edits, ok := editsRaw.(map[string]interface{})
	require.True(t, ok, "containerEdits should be a map")

	// 5. Verify mounts exist and have correct structure
	mountsRaw, ok := edits["mounts"]
	require.True(t, ok, "mounts should exist in containerEdits")
	mounts, ok := mountsRaw.([]interface{})
	require.True(t, ok, "mounts should be a list")
	require.NotEmpty(t, mounts, "should have at least one mount")

	mount, ok := mounts[0].(map[string]interface{})
	require.True(t, ok, "mount should be a map")
	assert.Equal(t, "/usr/lib64/librbln-ml.so", mount["hostPath"])
	assert.Equal(t, "/usr/lib64/rbln/librbln-ml.so", mount["containerPath"])

	// Verify mount options are present
	optionsRaw, ok := mount["options"]
	require.True(t, ok, "mount options should exist")
	options, ok := optionsRaw.([]interface{})
	require.True(t, ok, "options should be a list")
	require.NotEmpty(t, options, "options should not be empty")

	// Convert options to strings for assertion
	optionStrs := make([]string, len(options))
	for i, opt := range options {
		optionStrs[i] = opt.(string)
	}
	assert.Contains(t, optionStrs, "ro")
	assert.Contains(t, optionStrs, "nosuid")
	assert.Contains(t, optionStrs, "nodev")
	assert.Contains(t, optionStrs, "bind")

	// 6. Verify hooks exist and have correct structure
	hooksRaw, ok := edits["hooks"]
	require.True(t, ok, "hooks should exist in containerEdits")
	hooks, ok := hooksRaw.([]interface{})
	require.True(t, ok, "hooks should be a list")
	require.Len(t, hooks, 1, "should have exactly one hook (ldcache only, no symlinks in test data)")

	hook, ok := hooks[0].(map[string]interface{})
	require.True(t, ok, "hook should be a map")
	assert.Equal(t, "createContainer", hook["hookname"])
	assert.Equal(t, "/usr/local/bin/rbln-cdi-hook", hook["path"])

	// 7. Verify hook args contain --folder (folders are passed as CLI args, not env)
	argsRaw, ok := hook["args"]
	require.True(t, ok, "hook args should exist")
	args, ok := argsRaw.([]interface{})
	require.True(t, ok, "args should be a list")
	require.NotEmpty(t, args, "args should not be empty")

	argStrs := make([]string, len(args))
	for i, arg := range args {
		argStrs[i] = arg.(string)
	}
	assert.Contains(t, argStrs, "rbln-cdi-hook")
	assert.Contains(t, argStrs, "update-ldcache")
	assert.Contains(t, argStrs, "--folder")
	assert.Contains(t, argStrs, "/usr/lib64/rbln")

	// 8. Verify hook env variables (LDCONFIG_PATH and DEBUG only; FOLDER moved to args)
	envRaw, ok := hook["env"]
	require.True(t, ok, "hook env should exist")
	envList, ok := envRaw.([]interface{})
	require.True(t, ok, "env should be a list")
	require.Len(t, envList, 2, "should have exactly 2 env vars (LDCONFIG_PATH and DEBUG)")

	envStrs := make([]string, len(envList))
	for i, e := range envList {
		envStrs[i] = e.(string)
	}

	// FOLDER should NOT be in env (moved to args to avoid Viper comma bug)
	for _, env := range envStrs {
		assert.False(t, strings.HasPrefix(env, "RBLN_CDI_HOOK_FOLDER="),
			"RBLN_CDI_HOOK_FOLDER should not be in env (moved to --folder args)")
	}

	// Verify RBLN_CDI_HOOK_DEBUG env var
	var debugEnvFound bool
	for _, env := range envStrs {
		if strings.HasPrefix(env, "RBLN_CDI_HOOK_DEBUG=") {
			debugEnvFound = true
			assert.Contains(t, env, "true", "DEBUG env should be set to true")
			break
		}
	}
	assert.True(t, debugEnvFound, "RBLN_CDI_HOOK_DEBUG env var should exist")
}

func TestGenerator_Generate_WithDevices_PerNPUEntries(t *testing.T) {
	// Given: a typical discovery with two NPUs and one RSD group device.
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
			{Path: "/dev/rbln1", ContainerPath: "/dev/rbln1"},
			{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then: per-NPU "0"/"1", per-RSD "rsd0", "all", and the "runtime" alias.
	require.NoError(t, err)
	assertDeviceNames(t, spec, "0", "1", "rsd0", "all", "runtime")

	// Per-NPU entries carry exactly their own rbln node.
	npu0 := findDevice(t, spec, "0")
	require.Len(t, npu0.ContainerEdits.DeviceNodes, 1)
	assert.Equal(t, "/dev/rbln0", npu0.ContainerEdits.DeviceNodes[0].Path)
	assert.Equal(t, "rw", npu0.ContainerEdits.DeviceNodes[0].Permissions)

	npu1 := findDevice(t, spec, "1")
	require.Len(t, npu1.ContainerEdits.DeviceNodes, 1)
	assert.Equal(t, "/dev/rbln1", npu1.ContainerEdits.DeviceNodes[0].Path)

	// Per-RSD entry exposes the group device by its driver basename.
	rsd0 := findDevice(t, spec, "rsd0")
	require.Len(t, rsd0.ContainerEdits.DeviceNodes, 1)
	assert.Equal(t, "/dev/rsd0", rsd0.ContainerEdits.DeviceNodes[0].Path)

	// "all" entry mirrors the full device set, and the "runtime" alias
	// carries the same nodes for v0.1.x consumers.
	all := findDevice(t, spec, "all")
	require.Len(t, all.ContainerEdits.DeviceNodes, 3)
	runtime := findDevice(t, spec, "runtime")
	require.Len(t, runtime.ContainerEdits.DeviceNodes, 3)
}

func TestGenerator_Generate_ResolverAttachesRSDPerNPU(t *testing.T) {
	// Given: a resolver that knows NPU 0 → group 1 and NPU 1 → group 2.
	// This is the production path — librbln-ml will be queried in Phase 2 to
	// produce the same mapping, but here we drive it with a fake so the
	// generator can be exercised without cgo.
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
			{Path: "/dev/rbln1", ContainerPath: "/dev/rbln1"},
			{Path: "/dev/rsd1", ContainerPath: "/dev/rsd1"},
			{Path: "/dev/rsd2", ContainerPath: "/dev/rsd2"},
		},
	}
	cfg := config.DefaultConfig()
	resolver := &fakeResolver{mapping: map[uint32]uint32{0: 1, 1: 2}}
	gen := NewGenerator(cfg, resolver)

	// When
	spec, err := gen.Generate(result)

	// Then: each per-NPU entry carries its rbln node AND the resolved
	// /dev/rsdM, so `--device rebellions.ai/npu=0` is functional on its own.
	require.NoError(t, err)
	assertDevicePaths(t, findDevice(t, spec, "0"), "/dev/rbln0", "/dev/rsd1")
	assertDevicePaths(t, findDevice(t, spec, "1"), "/dev/rbln1", "/dev/rsd2")

	// Top-level is RSD-free now — the resolver handles attachment.
	assert.Empty(t, spec.ContainerEdits.DeviceNodes,
		"top-level must carry no device nodes; per-NPU entries own RSD attachment")
}

func TestGenerator_Generate_ResolverUnknownMapping_NpuOnly(t *testing.T) {
	// Given: a resolver that doesn't know NPU 0's group (driver unreachable,
	// pre-Phase-2 stub build, or K8s mode where device-plugin owns mapping).
	// The per-NPU entry must still be valid but limited to the rbln node.
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
			{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil) // nil → NoopResolver

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	assertDevicePaths(t, findDevice(t, spec, "0"), "/dev/rbln0")
	assert.Empty(t, spec.ContainerEdits.DeviceNodes,
		"no resolver mapping → no RSD anywhere except the explicit per-RSD entry and `all`")
}

func TestGenerator_Generate_ResolverPointsToMissingRSD_NpuOnly(t *testing.T) {
	// Given: a resolver claims NPU 0 belongs to group 5, but /dev/rsd5 isn't
	// present on the host (out-of-sync driver state, custom userland setup).
	// The generator must not invent device nodes that don't exist — drop the
	// RSD attachment for this NPU and emit a warning-worthy "rbln only" entry.
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
			{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
		},
	}
	cfg := config.DefaultConfig()
	resolver := &fakeResolver{mapping: map[uint32]uint32{0: 5}}
	gen := NewGenerator(cfg, resolver)

	// When
	spec, err := gen.Generate(result)

	// Then
	require.NoError(t, err)
	assertDevicePaths(t, findDevice(t, spec, "0"), "/dev/rbln0")
}

// fakeResolver is the test double used to exercise resolver-driven mapping
// without bringing librbln-ml into unit tests. Each entry is "NPU index → RSD
// group index"; unmapped NPUs trigger ok=false, matching production behavior
// when the driver doesn't know.
type fakeResolver struct {
	mapping map[uint32]uint32
}

func (f *fakeResolver) Resolve(npu uint32) (uint32, bool) {
	if f == nil {
		return 0, false
	}
	rsd, ok := f.mapping[npu]
	return rsd, ok
}

// assertDevicePaths checks that a CDI device entry carries exactly the given
// device-node paths in order. Used heavily by per-NPU tests where the
// expected node set is small and order-dependent (rbln first, then optional
// resolved rsd).
func assertDevicePaths(t *testing.T, dev specs.Device, paths ...string) {
	t.Helper()
	got := make([]string, 0, len(dev.ContainerEdits.DeviceNodes))
	for _, n := range dev.ContainerEdits.DeviceNodes {
		got = append(got, n.Path)
	}
	assert.Equal(t, paths, got, "device %q nodes", dev.Name)
}

func TestGenerator_Generate_WithDevices_DriverRoot(t *testing.T) {
	// Given: Devices with different host and container paths
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/run/rbln/driver/dev/rbln0", ContainerPath: "/dev/rbln0"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.DriverRoot = "/run/rbln/driver"
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then: per-NPU entry keeps host path mapping intact.
	require.NoError(t, err)
	npu0 := findDevice(t, spec, "0")
	require.Len(t, npu0.ContainerEdits.DeviceNodes, 1)
	assert.Equal(t, "/dev/rbln0", npu0.ContainerEdits.DeviceNodes[0].Path)
	assert.Equal(t, "/run/rbln/driver/dev/rbln0", npu0.ContainerEdits.DeviceNodes[0].HostPath)
}

func TestGenerator_Generate_DeviceNodes_YAMLOutput(t *testing.T) {
	// Given
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
		},
	}
	cfg := config.DefaultConfig()
	gen := NewGenerator(cfg, nil)
	writer := NewWriter()

	// When
	spec, err := gen.Generate(result)
	require.NoError(t, err)

	var buf bytes.Buffer
	err = writer.WriteToWriter(spec, &buf, "yaml")
	require.NoError(t, err)

	// Then: YAML output should contain the per-NPU entry's deviceNode block.
	output := buf.String()
	assert.Contains(t, output, "deviceNodes:")
	assert.Contains(t, output, "path: /dev/rbln0")
	assert.Contains(t, output, "hostPath: /dev/rbln0")
	assert.Contains(t, output, "permissions: rw")
	assert.Contains(t, output, "name: \"0\"")
	assert.Contains(t, output, "name: all")
	assert.Contains(t, output, "name: runtime")
}

func TestGenerator_Generate_DevicesDisabled_OmitsDeviceNodes(t *testing.T) {
	// Given: caller still hands us discovered devices, but Devices.Disabled is
	// set (Kubernetes path: device-plugin owns per-Pod device injection, so
	// CTK must not pin /dev/rsd0 et al. into the CDI spec).
	result := &discover.DiscoveryResult{
		Devices: []discover.Device{
			{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
			{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
		},
	}
	cfg := config.DefaultConfig()
	cfg.Devices.Disabled = true
	gen := NewGenerator(cfg, nil)

	// When
	spec, err := gen.Generate(result)

	// Then: only the "all" library/tool handle (and its "runtime" alias)
	// survive; no device nodes anywhere — neither top-level (default RSD
	// suppressed) nor per-device (no per-NPU/per-RSD entries emitted at all).
	require.NoError(t, err)
	assertDeviceNames(t, spec, "all", "runtime")
	assert.Empty(t, spec.ContainerEdits.DeviceNodes,
		"Devices.Disabled must suppress the default RSD in top-level edits")
	all := findDevice(t, spec, "all")
	assert.Empty(t, all.ContainerEdits.DeviceNodes,
		"Devices.Disabled must keep the `all` entry free of device nodes")
	runtime := findDevice(t, spec, "runtime")
	assert.Empty(t, runtime.ContainerEdits.DeviceNodes,
		"Devices.Disabled must keep the `runtime` alias free of device nodes")
}

// TestGenerator_Generate_RuntimeAliasMirrorsAll guards the v0.1.x compat
// contract end-to-end: whatever ContainerEdits the `all` entry carries, the
// `runtime` alias must carry the exact same content so downstream consumers
// pinning `rebellions.ai/npu=runtime` behave identically to `=all`. Covers
// both the populated (Docker) and empty (K8s / Devices.Disabled) cases.
func TestGenerator_Generate_RuntimeAliasMirrorsAll(t *testing.T) {
	cases := []struct {
		name           string
		devicesDisabled bool
		devices        []discover.Device
	}{
		{
			name: "docker_path_populated",
			devices: []discover.Device{
				{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
				{Path: "/dev/rbln1", ContainerPath: "/dev/rbln1"},
				{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
			},
		},
		{
			name:            "k8s_path_devices_disabled",
			devicesDisabled: true,
			devices: []discover.Device{
				{Path: "/dev/rbln0", ContainerPath: "/dev/rbln0"},
				{Path: "/dev/rsd0", ContainerPath: "/dev/rsd0"},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := &discover.DiscoveryResult{
				Libraries: []discover.Library{
					{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", ContainerPath: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
				},
				Devices: tc.devices,
			}
			cfg := config.DefaultConfig()
			cfg.Devices.Disabled = tc.devicesDisabled
			gen := NewGenerator(cfg, nil)

			spec, err := gen.Generate(result)
			require.NoError(t, err)

			all := findDevice(t, spec, "all")
			runtime := findDevice(t, spec, "runtime")
			assert.Equal(t, all.ContainerEdits, runtime.ContainerEdits,
				"`runtime` alias must mirror `all`'s ContainerEdits exactly")
		})
	}
}

// findDevice returns the CDI device entry with the given name or fails the test.
// Tests use this instead of indexing into spec.Devices because per-NPU,
// per-RSD, and "all" entries are interleaved in append order — searching by
// name keeps assertions readable as the entry layout evolves.
func findDevice(t *testing.T, spec *specs.Spec, name string) specs.Device {
	t.Helper()
	for _, d := range spec.Devices {
		if d.Name == name {
			return d
		}
	}
	t.Fatalf("device %q not found in spec (have: %v)", name, deviceNames(spec))
	return specs.Device{}
}

// assertDeviceNames checks that spec.Devices has exactly the given names in
// the given order. Tests use the ordering guarantee to catch accidental
// reshuffles that would surface to end-users as a churn in `rbln-ctk cdi list`.
func assertDeviceNames(t *testing.T, spec *specs.Spec, want ...string) {
	t.Helper()
	assert.Equal(t, want, deviceNames(spec))
}

func deviceNames(spec *specs.Spec) []string {
	out := make([]string, 0, len(spec.Devices))
	for _, d := range spec.Devices {
		out = append(out, d.Name)
	}
	return out
}
