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
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/pelletier/go-toml/v2"
)

// crioScansSpecDir reports whether CRI-O's effective configuration would scan
// defaultCDISpecDir (the dir the toolkit writes RBLN CDI specs into).
//
// CRI-O has no enable_cdi toggle: CDI injection is always on and is driven
// solely by cdi_spec_dirs (built-in default: /etc/cdi, /var/run/cdi). So
// readiness reduces to a single question — does the merged configuration scan
// our spec dir? CRI-O layers config lowest-to-highest as the main crio.conf,
// then every file in crio.conf.d sorted lexically, with the last file that sets
// a key winning (arrays replace, they do not merge). When no file sets
// cdi_spec_dirs the built-in default applies, which already includes
// /var/run/cdi — so a default CRI-O node is ready and needs no drop-in or
// restart.
//
// dropInPath is the toolkit's drop-in path (already host-root-prefixed by the
// caller, e.g. /host/etc/crio/crio.conf.d/99-rbln.conf); the main crio.conf and
// sibling drop-ins are derived from it. This assumes the standard CRI-O layout
// (main config at ../crio.conf relative to the drop-in dir, which is CTK's
// default target). A non-standard main config location (crio --config=...) is
// not discovered, so on such a host a cdi_spec_dirs override set only in that
// file would be missed.
func crioScansSpecDir(dropInPath string) (bool, error) {
	dropInDir := filepath.Dir(dropInPath)
	mainConf := filepath.Join(filepath.Dir(dropInDir), "crio.conf")

	files, err := crioConfigFiles(mainConf, dropInDir)
	if err != nil {
		return false, err
	}

	dirs, present := crioEffectiveSpecDirs(files)
	if !present {
		// No config file overrides cdi_spec_dirs → CRI-O's built-in default
		// applies, and that default includes /var/run/cdi.
		return true, nil
	}
	return containsCleanPath(dirs, defaultCDISpecDir), nil
}

// crioConfigFiles returns CRI-O config files in ascending precedence order: the
// main crio.conf first (lowest precedence), then every regular file in the
// drop-in dir sorted lexically. CRI-O reads all files in crio.conf.d, not only
// *.conf, so this does not filter by extension. A missing crio.conf or a missing
// drop-in dir is not an error — it simply contributes nothing.
func crioConfigFiles(mainConf, dropInDir string) ([]string, error) {
	var files []string

	if fi, err := os.Stat(mainConf); err == nil {
		if !fi.IsDir() {
			files = append(files, mainConf)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat %s: %w", mainConf, err)
	}

	entries, err := os.ReadDir(dropInDir)
	if err != nil {
		if os.IsNotExist(err) {
			return files, nil
		}
		return nil, fmt.Errorf("read CRI-O drop-in dir %s: %w", dropInDir, err)
	}

	dropIns := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dropIns = append(dropIns, filepath.Join(dropInDir, e.Name()))
	}
	sort.Strings(dropIns)

	return append(files, dropIns...), nil
}

// crioEffectiveSpecDirs walks files in ascending precedence order and returns
// the cdi_spec_dirs set by the highest-precedence file that specifies it.
// present is false when no file sets the key (caller then applies CRI-O's
// built-in default).
//
// A file that cannot be read or parsed is skipped (it does not contribute a
// value): a running CRI-O would have rejected an invalid drop-in, so a file we
// cannot parse is generally not one CRI-O honors either. The skip is logged so
// the decision is not silent — a high-precedence file that CRI-O parses but we
// do not could otherwise flip readiness the wrong way without a trace.
func crioEffectiveSpecDirs(files []string) (dirs []string, present bool) {
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			log.Printf("WARNING: CRI-O CDI readiness: skipping unreadable config %s: %v", f, err)
			continue
		}
		d, ok, err := crioSpecDirs(data)
		if err != nil {
			log.Printf("WARNING: CRI-O CDI readiness: skipping unparseable config %s: %v", f, err)
			continue
		}
		if ok {
			dirs, present = d, true // a later (higher-precedence) file wins
		}
	}
	return dirs, present
}

// crioSpecDirs extracts crio.runtime.cdi_spec_dirs from one CRI-O config file's
// TOML. present is false when the key is absent; err is non-nil only when the
// file is not valid TOML.
func crioSpecDirs(data []byte) (dirs []string, present bool, err error) {
	var root map[string]interface{}
	if err := toml.Unmarshal(data, &root); err != nil {
		return nil, false, err
	}
	crio, ok := root["crio"].(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	rt, ok := crio["runtime"].(map[string]interface{})
	if !ok {
		return nil, false, nil
	}
	raw, ok := rt["cdi_spec_dirs"].([]interface{})
	if !ok {
		return nil, false, nil
	}
	for _, e := range raw {
		if s, ok := e.(string); ok {
			dirs = append(dirs, s)
		}
	}
	return dirs, true, nil
}
