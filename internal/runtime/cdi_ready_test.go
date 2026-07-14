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

package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubVersion swaps commandRunner to report a fixed version banner (or an error
// to simulate an undetectable version) for the duration of the test.
func stubVersion(t *testing.T, banner string, cmdErr error) {
	t.Helper()
	orig := commandRunner
	t.Cleanup(func() { commandRunner = orig })
	commandRunner = func(_ context.Context, _ string, _ ...string) ([]byte, error) {
		if cmdErr != nil {
			return nil, cmdErr
		}
		return []byte(banner), nil
	}
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestCDIReady_Containerd(t *testing.T) {
	t.Run("2.0+ with no config is ready (default on)", func(t *testing.T) {
		stubVersion(t, "containerd v2.0.0 x", nil)
		ready, err := CDIReady(RuntimeContainerd, filepath.Join(t.TempDir(), "missing.toml"), "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("2.0+ but explicitly disabled is not ready", func(t *testing.T) {
		stubVersion(t, "containerd v2.0.0 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"[plugins.\"io.containerd.grpc.v1.cri\"]\n    enable_cdi = false\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("version unknown, CDI already written is ready", func(t *testing.T) {
		stubVersion(t, "", errors.New("no binary"))
		content := `version = 2

[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
`
		cfg := writeFile(t, t.TempDir(), "config.toml", content)
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready, "config where enabling CDI is a no-op must be ready")
	})

	t.Run("version unknown, no config is not ready", func(t *testing.T) {
		stubVersion(t, "", errors.New("no binary"))
		ready, err := CDIReady(RuntimeContainerd, filepath.Join(t.TempDir(), "missing.toml"), "/")
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("2.0+ but cdi_spec_dirs excludes our dir is not ready", func(t *testing.T) {
		stubVersion(t, "containerd v2.0.0 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"[plugins.\"io.containerd.grpc.v1.cri\"]\n    enable_cdi = true\n    cdi_spec_dirs = [\"/opt/cdi\"]\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready, "custom cdi_spec_dirs omitting /var/run/cdi must not be ready")
	})

	t.Run("enable_cdi=true but cdi_spec_dirs excludes our dir is not ready", func(t *testing.T) {
		stubVersion(t, "", errors.New("no binary"))
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"[plugins.\"io.containerd.grpc.v1.cri\"]\n    enable_cdi = true\n    cdi_spec_dirs = [\"/opt/cdi\"]\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready, "spec-dir gate must apply on the fallback path too")
	})

	t.Run("1.x without CDI is not ready", func(t *testing.T) {
		stubVersion(t, "containerd 1.7.13 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml", "version = 2\n[plugins]\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("default dir outside cdi_spec_dirs does not spoof readiness", func(t *testing.T) {
		// cdi_spec_dirs omits /var/run/cdi, but the string appears elsewhere in
		// the config. Scoped parsing must still report not-ready.
		stubVersion(t, "containerd v2.0.0 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"root = \"/var/run/cdi/unrelated\"\n[plugins.\"io.containerd.grpc.v1.cri\"]\n    enable_cdi = true\n    cdi_spec_dirs = [\"/opt/cdi\"]\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready, "unrelated /var/run/cdi must not count as spec-dir coverage")
	})

	t.Run("multi-line cdi_spec_dirs array including our dir is ready", func(t *testing.T) {
		stubVersion(t, "containerd 1.7.13 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"[plugins.\"io.containerd.grpc.v1.cri\"]\n    enable_cdi = true\n    cdi_spec_dirs = [\n        \"/etc/cdi\",\n        \"/var/run/cdi\",\n    ]\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("commented-out enable_cdi does not count as enabled", func(t *testing.T) {
		// 1.x node where an operator disabled CDI by commenting the line: the
		// substring "enable_cdi = true" is present but inactive. Must not be ready.
		stubVersion(t, "containerd 1.7.13 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"[plugins.\"io.containerd.grpc.v1.cri\"]\n    # enable_cdi = true\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready, "commented-out enable_cdi must not skip the restart")
	})

	t.Run("enable_cdi=true without spaces is ready", func(t *testing.T) {
		// Formatting variance must not cause a false negative (needless restart).
		stubVersion(t, "containerd 1.7.13 x", nil)
		cfg := writeFile(t, t.TempDir(), "config.toml",
			"[plugins.\"io.containerd.grpc.v1.cri\"]\n    enable_cdi=true\n    cdi_spec_dirs=[\"/var/run/cdi\"]\n")
		ready, err := CDIReady(RuntimeContainerd, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})
}

func TestCDIReady_CRIO(t *testing.T) {
	// setupCrio lays out an /etc/crio-style tree: an optional main crio.conf and
	// a crio.conf.d drop-in dir with the given files, then returns the toolkit
	// drop-in path the daemon would pass to CDIReady.
	setupCrio := func(t *testing.T, mainConf string, dropIns map[string]string) string {
		t.Helper()
		root := t.TempDir()
		if mainConf != "" {
			require.NoError(t, os.WriteFile(filepath.Join(root, "crio.conf"), []byte(mainConf), 0o644))
		}
		dir := filepath.Join(root, "crio.conf.d")
		require.NoError(t, os.MkdirAll(dir, 0o755))
		for name, content := range dropIns {
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644))
		}
		return filepath.Join(dir, "99-rbln.conf")
	}
	rblnDropIn := (&crioConfigurator{}).generateConfig()

	t.Run("default node with no cdi_spec_dirs override is ready", func(t *testing.T) {
		// The core fix: a fresh CRI-O node where nothing overrides cdi_spec_dirs
		// relies on the built-in default (which includes /var/run/cdi) and must
		// NOT trigger a drop-in write or a full restart on first install.
		cfg := setupCrio(t, "", nil)
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("main crio.conf without cdi_spec_dirs is ready", func(t *testing.T) {
		cfg := setupCrio(t, "[crio.runtime]\ndefault_runtime = \"runc\"\n", nil)
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("main crio.conf already listing /var/run/cdi is ready without our drop-in", func(t *testing.T) {
		cfg := setupCrio(t, "[crio.runtime]\ncdi_spec_dirs = [\"/etc/cdi\", \"/var/run/cdi\"]\n", nil)
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("cdi_spec_dirs excluding /var/run/cdi is not ready", func(t *testing.T) {
		cfg := setupCrio(t, "[crio.runtime]\ncdi_spec_dirs = [\"/etc/cdi\"]\n", nil)
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready, "a config that omits /var/run/cdi must still be configured + restarted")
	})

	t.Run("our drop-in restores coverage over an excluding main config", func(t *testing.T) {
		// crio.conf drops /var/run/cdi, but the toolkit's higher-precedence
		// drop-in adds it back → effective config scans it → ready (idempotent
		// re-deploy skips the restart).
		cfg := setupCrio(t, "[crio.runtime]\ncdi_spec_dirs = [\"/etc/cdi\"]\n", map[string]string{
			"99-rbln.conf": rblnDropIn,
		})
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("lower-precedence drop-in excluding /var/run/cdi is overridden by ours", func(t *testing.T) {
		cfg := setupCrio(t, "", map[string]string{
			"10-operator.conf": "[crio.runtime]\ncdi_spec_dirs = [\"/etc/cdi\"]\n",
			"99-rbln.conf":     rblnDropIn,
		})
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("higher-precedence drop-in excluding /var/run/cdi is not ready", func(t *testing.T) {
		// An operator drop-in sorted after ours strips /var/run/cdi again;
		// last-writer-wins means the node is genuinely not scanning our dir.
		cfg := setupCrio(t, "", map[string]string{
			"99-rbln.conf":     rblnDropIn,
			"zz-operator.conf": "[crio.runtime]\ncdi_spec_dirs = [\"/etc/cdi\"]\n",
		})
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("unparseable drop-in is skipped and default coverage stands", func(t *testing.T) {
		// A running CRI-O would have rejected an invalid drop-in, so a file we
		// cannot parse must not spuriously force a restart.
		cfg := setupCrio(t, "", map[string]string{
			"garbage.conf": "this is not = valid = toml [[[",
		})
		ready, err := CDIReady(RuntimeCRIO, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})
}

func TestCDIReady_Docker(t *testing.T) {
	t.Run("28.2+ with no config is ready (default on)", func(t *testing.T) {
		stubVersion(t, "Docker version 28.2.0, build x", nil)
		ready, err := CDIReady(RuntimeDocker, filepath.Join(t.TempDir(), "daemon.json"), "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("28.2+ but features.cdi=false is not ready", func(t *testing.T) {
		stubVersion(t, "Docker version 28.2.0, build x", nil)
		cfg := writeFile(t, t.TempDir(), "daemon.json", `{"features":{"cdi":false}}`)
		ready, err := CDIReady(RuntimeDocker, cfg, "/")
		assert.NoError(t, err)
		assert.False(t, ready)
	})

	t.Run("version unknown, features.cdi=true is ready", func(t *testing.T) {
		stubVersion(t, "", errors.New("no binary"))
		cfg := writeFile(t, t.TempDir(), "daemon.json", `{"features":{"cdi":true}}`)
		ready, err := CDIReady(RuntimeDocker, cfg, "/")
		assert.NoError(t, err)
		assert.True(t, ready)
	})

	t.Run("version unknown, no config is not ready", func(t *testing.T) {
		stubVersion(t, "", errors.New("no binary"))
		ready, err := CDIReady(RuntimeDocker, filepath.Join(t.TempDir(), "daemon.json"), "/")
		assert.NoError(t, err)
		assert.False(t, ready)
	})
}

func TestCDIReady_UnknownRuntime(t *testing.T) {
	_, err := CDIReady(RuntimeType("podman"), "/tmp/x", "/")
	assert.Error(t, err)
}
