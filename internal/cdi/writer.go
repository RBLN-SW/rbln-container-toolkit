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

//go:generate moq -rm -fmt=goimports -stub -out writer_mock.go . Writer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"tags.cncf.io/container-device-interface/specs-go"

	rbln_errors "github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// toCDIFileSpec converts a specs.Spec into cdiFileSpec, the CDI key
// definitions parseSpec decodes with, so the YAML writer and the strict
// reader share one set of keys. specs-go v0.7.0 tags its types for JSON only;
// marshaling them with yaml.v3 directly falls back to lowercased field names
// (`hookname`), which runtimes built on CDI v1.1.1 or later reject. Both sides
// carry the same JSON key names, so the conversion goes through JSON.
//
// Unset fields stay out of the rendered spec: every optional cdiFileSpec field
// is `omitempty`, and optional structs such as IntelRdt are typed pointers,
// which yaml.v3 elides when nil. Routing them through `any` instead once leaked
// `intelRdt: null`, which a strict containerd CDI parser rejected.
func toCDIFileSpec(spec *specs.Spec) (*cdiFileSpec, error) {
	b, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}
	var out cdiFileSpec
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Writer writes CDI specifications to files or stdout.
type Writer interface {
	// Write writes the spec to a file.
	Write(spec *specs.Spec, path string, format string) error

	// WriteToWriter writes the spec to an io.Writer.
	WriteToWriter(spec *specs.Spec, w io.Writer, format string) error
}

// writer implements Writer interface.
type writer struct{}

// NewWriter creates a new CDI writer.
func NewWriter() Writer {
	return &writer{}
}

// Write writes the spec to a file atomically, with a best-effort flush
// for crash durability.
//
// The flow is: marshal → temp file in same dir → chmod → fsync → close →
// rename → fsync(parent dir). Concurrent readers (container runtimes
// loading the spec) always observe either the previous full content or
// the new full content, never a partial write. chmod runs before the
// file's fsync so permission metadata is part of the same flush as the
// contents — otherwise a crash between fsync and rename could land a
// 0o600 spec from os.CreateTemp on disk.
//
// The trailing parent-directory fsync increases the chance that the
// rename survives a power loss on filesystems like ext4 without
// data=journal, but it is best-effort: a failure here is logged and
// swallowed because the rename has already published the new spec to
// readers and unwinding would only obscure a correct on-disk state.
func (w *writer) Write(spec *specs.Spec, path, format string) error {
	if spec == nil {
		return fmt.Errorf("spec is nil: %w", rbln_errors.ErrInvalidCDISpec)
	}

	if format != "yaml" && format != "json" {
		return fmt.Errorf("unsupported format: %s (expected yaml or json): %w", format, rbln_errors.ErrWriteFailed)
	}

	var buf bytes.Buffer
	if err := w.WriteToWriter(spec, &buf, format); err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %v", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %v", dir, err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = tmp.Close() // idempotent close after explicit close in happy path
			_ = os.Remove(tmpPath)
		}
	}()

	// buf.WriteTo loops internally until the full buffer is written, so a
	// short write from the underlying io.Writer cannot leave a truncated
	// file behind — important for crash safety and to satisfy reviewers
	// who have been bitten by short writes on non-os.File destinations.
	if _, err := buf.WriteTo(tmp); err != nil {
		return fmt.Errorf("write temp file %s: %v", tmpPath, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		return fmt.Errorf("chmod temp file %s: %v", tmpPath, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("sync temp file %s: %v", tmpPath, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file %s: %v", tmpPath, err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s to %s: %v", tmpPath, path, err)
	}
	// Once the rename returns, the new spec is the live one for any reader
	// opening the file from now on. Marking committed here keeps the
	// deferred cleanup out of an already-published file, and ensures a
	// failure of the trailing directory fsync — which only affects crash
	// durability, not correctness for live readers — does not unwind the
	// successful rename.
	committed = true

	// Best-effort durability flush of the directory entry. Filesystems
	// where directory fsync isn't supported (some FUSE backends) would
	// otherwise turn a perfectly correct on-disk spec into a Write error.
	// We log and move on; the next regeneration (or a daemon restart that
	// re-runs this path) gets another chance to persist the dentry.
	if err := syncDir(dir); err != nil {
		log.Printf("WARNING: cdi-writer: fsync directory %s: %v (rename succeeded; spec is correct, durability across power loss may be reduced)", dir, err)
	}
	return nil
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := d.Sync()
	closeErr := d.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// WriteToWriter writes the spec to an io.Writer.
func (w *writer) WriteToWriter(spec *specs.Spec, out io.Writer, format string) error {
	if spec == nil {
		return fmt.Errorf("spec is nil: %w", rbln_errors.ErrInvalidCDISpec)
	}

	var data []byte
	var err error

	switch format {
	case "yaml":
		var fileSpec *cdiFileSpec
		fileSpec, err = toCDIFileSpec(spec)
		if err == nil {
			data, err = yaml.Marshal(fileSpec)
		}
	case "json":
		data, err = json.MarshalIndent(spec, "", "  ")
	default:
		return fmt.Errorf("unsupported format: %s (expected yaml or json): %w", format, rbln_errors.ErrWriteFailed)
	}

	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	if _, err := out.Write(data); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}

	return nil
}
