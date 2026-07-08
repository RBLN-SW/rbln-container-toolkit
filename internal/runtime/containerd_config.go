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
	"fmt"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// criPluginKey is the containerd config table that holds the CRI plugin's CDI
// settings (enable_cdi, cdi_spec_dirs). CTK reads and writes this table.
const criPluginKey = "io.containerd.grpc.v1.cri"

// containerdConfig is a parsed containerd config.toml, held as a generic tree so
// that round-tripping preserves every unrelated setting by value; only the CDI
// keys are inspected or mutated.
//
// Parsing (rather than the substring/regex matching this replaced) is what makes
// the CDI-readiness decision correct: it naturally handles comments, whitespace
// variance, inline vs multi-line arrays, and nested tables — the edge cases that
// a text scan kept getting wrong, and that now directly gate whether the daemon
// restarts the runtime.
//
// Tradeoff on write: re-serializing a generic tree does NOT preserve comments or
// key ordering in the operator's config.toml (it does preserve all values). This
// matches the approach taken by nvidia-container-toolkit for the same file, and
// is accepted here because a correct, unambiguous edit is worth more than byte
// stability of a machine-managed runtime config.
type containerdConfig struct {
	root map[string]interface{}
}

// parseContainerdConfig parses containerd config.toml text. Empty content yields
// an empty config (all CDI settings absent). A syntax error is returned so
// callers treat the config as undeterminable rather than silently wrong.
func parseContainerdConfig(content string) (*containerdConfig, error) {
	root := map[string]interface{}{}
	if strings.TrimSpace(content) != "" {
		if err := toml.Unmarshal([]byte(content), &root); err != nil {
			return nil, fmt.Errorf("parse containerd config: %w", err)
		}
	}
	return &containerdConfig{root: root}, nil
}

// criSection returns the CRI plugin table, creating the [plugins] and CRI
// subtables when create is true. Returns nil when absent and create is false.
func (c *containerdConfig) criSection(create bool) map[string]interface{} {
	plugins, ok := c.root["plugins"].(map[string]interface{})
	if !ok {
		if !create {
			return nil
		}
		plugins = map[string]interface{}{}
		c.root["plugins"] = plugins
	}
	cri, ok := plugins[criPluginKey].(map[string]interface{})
	if !ok {
		if !create {
			return nil
		}
		cri = map[string]interface{}{}
		plugins[criPluginKey] = cri
	}
	return cri
}

// enableCDI reports the enable_cdi setting. present is false when the key is
// absent, so callers fall back to the runtime version default.
func (c *containerdConfig) enableCDI() (enabled, present bool) {
	cri := c.criSection(false)
	if cri == nil {
		return false, false
	}
	v, ok := cri["enable_cdi"].(bool)
	if !ok {
		return false, false
	}
	return v, true
}

// specDirs returns the cdi_spec_dirs entries and whether the key is present.
func (c *containerdConfig) specDirs() (dirs []string, present bool) {
	cri := c.criSection(false)
	if cri == nil {
		return nil, false
	}
	raw, ok := cri["cdi_spec_dirs"].([]interface{})
	if !ok {
		return nil, false
	}
	for _, e := range raw {
		if s, ok := e.(string); ok {
			dirs = append(dirs, s)
		}
	}
	return dirs, true
}

// scansDefaultSpecDir reports whether containerd would scan defaultCDISpecDir
// (the dir the configurator writes RBLN specs into). An absent cdi_spec_dirs
// means the built-in default applies (which includes /var/run/cdi); an explicit
// list must contain it.
func (c *containerdConfig) scansDefaultSpecDir() bool {
	dirs, present := c.specDirs()
	if !present {
		return true
	}
	return containsCleanPath(dirs, defaultCDISpecDir)
}

// render serializes the config back to TOML text.
func (c *containerdConfig) render() (string, error) {
	out, err := toml.Marshal(c.root)
	if err != nil {
		return "", fmt.Errorf("render containerd config: %w", err)
	}
	return string(out), nil
}

// containsCleanPath reports whether target (already clean) equals any entry in
// dirs after path cleaning, so "/var/run/cdi/" matches "/var/run/cdi".
func containsCleanPath(dirs []string, target string) bool {
	for _, d := range dirs {
		if filepath.Clean(d) == target {
			return true
		}
	}
	return false
}
