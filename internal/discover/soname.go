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

package discover

import (
	"debug/elf"
	"fmt"
	"path/filepath"
	"strings"
)

// ReadSONAME reads the DT_SONAME entry from an ELF binary.
// Returns the SONAME string (e.g., "librbln-ccl.so.3").
// If no SONAME is found, returns the basename of the file path as fallback.
//
// We use a function variable to allow overriding for testing.
var ReadSONAME = func(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open ELF: %w", err)
	}
	defer f.Close()

	sonames, err := f.DynString(elf.DT_SONAME)
	if err != nil {
		return "", fmt.Errorf("failed to read DT_SONAME: %w", err)
	}

	if len(sonames) == 0 {
		return filepath.Base(path), nil
	}

	return sonames[0], nil
}

// GetSoLink returns the unversioned .so filename for a given soname.
// It recursively strips version suffixes (e.g., "libfoo.so.3.0.0" -> "libfoo.so").
// Returns empty string if the name doesn't contain ".so".
func GetSoLink(soname string) string {
	ext := filepath.Ext(soname)
	if ext == "" {
		return ""
	}
	if ext == ".so" {
		return soname
	}
	return GetSoLink(strings.TrimSuffix(soname, ext))
}
