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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseContainerdConfig_Reading(t *testing.T) {
	t.Run("empty content has nothing present", func(t *testing.T) {
		cfg, err := parseContainerdConfig("")
		require.NoError(t, err)
		_, present := cfg.enableCDI()
		assert.False(t, present)
		_, spresent := cfg.specDirs()
		assert.False(t, spresent)
		assert.True(t, cfg.scansDefaultSpecDir(), "absent spec dirs => default applies")
	})

	t.Run("reads enable_cdi and spec dirs from the CRI table", func(t *testing.T) {
		cfg, err := parseContainerdConfig(`version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
`)
		require.NoError(t, err)
		enabled, present := cfg.enableCDI()
		assert.True(t, present)
		assert.True(t, enabled)
		dirs, spresent := cfg.specDirs()
		assert.True(t, spresent)
		assert.Equal(t, []string{"/etc/cdi", "/var/run/cdi"}, dirs)
		assert.True(t, cfg.scansDefaultSpecDir())
	})

	t.Run("enable_cdi outside the CRI table is not read", func(t *testing.T) {
		// A stray top-level enable_cdi must not be mistaken for the CRI setting.
		cfg, err := parseContainerdConfig("enable_cdi = true\n[plugins]\n")
		require.NoError(t, err)
		_, present := cfg.enableCDI()
		assert.False(t, present)
	})

	t.Run("hash inside a quoted string is not a comment", func(t *testing.T) {
		// A '#' inside a value must not be treated as a comment (the old text
		// scanner truncated at the first '#').
		cfg, err := parseContainerdConfig(`[plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi", "/data/#weird"]
`)
		require.NoError(t, err)
		dirs, present := cfg.specDirs()
		require.True(t, present)
		assert.Contains(t, dirs, "/data/#weird")
		assert.True(t, cfg.scansDefaultSpecDir())
	})

	t.Run("multi-line spec dirs array is parsed", func(t *testing.T) {
		cfg, err := parseContainerdConfig(`[plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = [
        "/etc/cdi",
        "/var/run/cdi",
    ]
`)
		require.NoError(t, err)
		assert.True(t, cfg.scansDefaultSpecDir())
	})

	t.Run("syntax error is surfaced", func(t *testing.T) {
		_, err := parseContainerdConfig("this is = = not toml [[[")
		assert.Error(t, err)
	})
}

func TestParseContainerdConfig_RoundTripPreservesUnrelatedSettings(t *testing.T) {
	cfg, err := parseContainerdConfig(`version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    sandbox_image = "registry.k8s.io/pause:3.9"
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
`)
	require.NoError(t, err)
	cri := cfg.criSection(true)
	cri["enable_cdi"] = true

	out, err := cfg.render()
	require.NoError(t, err)

	// Re-parse and confirm unrelated values survived the round trip.
	back, err := parseContainerdConfig(out)
	require.NoError(t, err)
	enabled, present := back.enableCDI()
	assert.True(t, present)
	assert.True(t, enabled)
	assert.Contains(t, out, "sandbox_image")
	assert.Contains(t, out, "default_runtime_name")
}
