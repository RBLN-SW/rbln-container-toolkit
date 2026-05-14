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
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	rbln_errors "github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// rblnLibraryGlobs lists the filename patterns for RBLN UMD shared libraries
// that embed the `rbln version: ` marker. The driver currently bakes the
// marker into librbln-ccl and librbln-thunk only; librbln-ml is shipped
// without it, so a broad `librbln-*.so*` glob would produce a perpetual
// ErrVersionNotFound warning for ml on every probe tick without contributing
// any signal. Probing the two libraries that do carry the marker is still
// sufficient for change detection: they ship as one package, so a driver
// upgrade flips both versions in lockstep. Re-add entries here when a new
// library starts embedding the marker.
var rblnLibraryGlobs = []string{
	"librbln-ccl.so*",
	"librbln-thunk.so*",
}

// versionMarker is the byte sequence the RBLN UMD libraries embed immediately
// before their version string in the read-only data section. Driver team
// confirmed the contract via `strings -f librbln*.so | grep "rbln version"`.
const versionMarker = "rbln version: "

// maxVersionLen caps how far past the marker we scan for a NUL terminator,
// guarding against a marker followed by an unterminated buffer.
const maxVersionLen = 256

// scanBufSize is the per-chunk read size used while scanning shared
// libraries for the version marker. Sized to amortize syscall cost while
// keeping per-probe peak allocation flat (scanBufSize + len(marker)-1)
// regardless of library file size.
const scanBufSize = 64 * 1024

// ProbeLibraryVersion extracts the embedded `rbln version: <ver>` string from
// an RBLN UMD shared library by reading the file in fixed-size chunks via
// io.ReadFull and searching each chunk with bytes.Index. The whole library
// is never held in memory: peak allocation is one chunk plus a small slice
// for the version trailer, so the watcher's memory footprint stays flat as
// driver packages grow.
//
// Returns rbln_errors.ErrVersionNotFound (wrapped) when the file exists but
// no usable marker is present. I/O errors are returned as-is so callers can
// distinguish "library not installed" (errors.Is(err, fs.ErrNotExist)) from
// "library installed but unparseable".
func ProbeLibraryVersion(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return scanLibraryVersion(f, path)
}

// scanLibraryVersion is split out from ProbeLibraryVersion so tests can
// drive it with an arbitrary io.Reader without touching the filesystem.
//
// We read in fixed-size chunks and search each chunk with bytes.Index
// (Boyer-Moore-Horspool internally), carrying len(marker)-1 bytes between
// chunks so a marker straddling the boundary is still found. bytes.Index
// is provably correct for any pattern — including ones with internal
// repeats like "rbln version: " where 'r' appears at offset 6 — so we
// don't have to reason about KMP-style failure functions ourselves.
func scanLibraryVersion(r io.Reader, path string) (string, error) {
	marker := []byte(versionMarker)
	overlap := len(marker) - 1

	buf := make([]byte, overlap+scanBufSize)
	carry := 0

	for {
		n, readErr := io.ReadFull(r, buf[carry:])
		valid := carry + n

		if idx := bytes.Index(buf[:valid], marker); idx >= 0 {
			return readVersionTrailer(r, buf[idx+len(marker):valid], path)
		}

		if readErr == io.EOF || readErr == io.ErrUnexpectedEOF {
			return "", fmt.Errorf("library %s: %w", path, rbln_errors.ErrVersionNotFound)
		}
		if readErr != nil {
			return "", fmt.Errorf("read %s: %w", path, readErr)
		}

		// Carry the trailing `overlap` bytes so a marker that straddles
		// this chunk and the next is still detected.
		if valid > overlap {
			copy(buf, buf[valid-overlap:valid])
			carry = overlap
		} else {
			carry = valid
		}
	}
}

// readVersionTrailer collects up to maxVersionLen bytes of version data
// after the marker, drawing first from the buffer slice already in hand
// and topping up from the reader if necessary, then truncating at the
// first NUL. EOF / ErrUnexpectedEOF while reading the trailer are
// expected (a marker near the end of the file): we keep what we got.
// Any other read error is propagated so a real I/O failure (EIO, etc.)
// is not silently turned into a "valid" version string.
func readVersionTrailer(r io.Reader, head []byte, path string) (string, error) {
	if len(head) > maxVersionLen {
		head = head[:maxVersionLen]
	}
	vbuf := make([]byte, 0, maxVersionLen)
	vbuf = append(vbuf, head...)
	if len(vbuf) < maxVersionLen {
		extra := make([]byte, maxVersionLen-len(vbuf))
		n, err := io.ReadFull(r, extra)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			return "", fmt.Errorf("read %s version trailer: %w", path, err)
		}
		vbuf = append(vbuf, extra[:n]...)
	}
	if i := bytes.IndexByte(vbuf, 0); i >= 0 {
		vbuf = vbuf[:i]
	}
	return validateVersion(vbuf, path)
}

// ProbeLibraryVersions returns a snapshot of versions for the given paths.
// Missing libraries are silently omitted from the result so the snapshot only
// reflects libraries that are actually installed at probe time. Per-path
// parse failures are aggregated into errs without aborting the scan, letting
// the caller decide whether to treat them as a real change signal.
func ProbeLibraryVersions(paths []string) (versions map[string]string, errs map[string]error) {
	versions = make(map[string]string, len(paths))
	errs = make(map[string]error)
	for _, p := range paths {
		v, err := ProbeLibraryVersion(p)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			errs[p] = err
			continue
		}
		versions[p] = v
	}
	return versions, errs
}

// ProbeRBLNLibraries scans libDirs for RBLN UMD libraries (librbln-*.so*)
// and returns a snapshot of the version embedded in each. Symlinks resolving
// to the same target are deduplicated so each underlying library file
// contributes a single entry, keyed by its resolved path.
//
// libDirs that don't exist or can't be read are skipped without error;
// libraries lacking the version marker contribute an entry to errs but do
// not abort the scan. This matches the watcher's tolerance for partially
// populated host filesystems and packaging quirks.
func ProbeRBLNLibraries(libDirs []string) (versions map[string]string, errs map[string]error) {
	versions = make(map[string]string)
	errs = make(map[string]error)

	seen := make(map[string]struct{})
	for _, dir := range libDirs {
		var matches []string
		for _, pat := range rblnLibraryGlobs {
			m, err := filepath.Glob(filepath.Join(dir, pat))
			if err != nil {
				// filepath.Glob only returns ErrBadPattern, which our literal
				// patterns cannot trigger; surface defensively rather than panic.
				errs[dir] = err
				continue
			}
			matches = append(matches, m...)
		}
		for _, m := range matches {
			resolved, err := filepath.EvalSymlinks(m)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				errs[m] = err
				continue
			}
			if _, dup := seen[resolved]; dup {
				continue
			}
			seen[resolved] = struct{}{}

			v, err := ProbeLibraryVersion(resolved)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				errs[resolved] = err
				continue
			}
			versions[resolved] = v
		}
	}
	return versions, errs
}

// validateVersion enforces the printable-ASCII contract on the raw bytes
// extracted after the marker. Iterating runes on a string would silently
// fold invalid UTF-8 into U+FFFD and let high-ASCII garbage past, so the
// check operates on bytes directly — including the leading/trailing
// whitespace trim, which must run before the printable-byte loop.
func validateVersion(slice []byte, path string) (string, error) {
	for len(slice) > 0 && asciiWhitespace(slice[0]) {
		slice = slice[1:]
	}
	for len(slice) > 0 && asciiWhitespace(slice[len(slice)-1]) {
		slice = slice[:len(slice)-1]
	}
	if len(slice) == 0 {
		return "", fmt.Errorf("library %s: empty version after marker: %w", path, rbln_errors.ErrVersionNotFound)
	}
	for _, b := range slice {
		if b < 0x20 || b >= 0x7f {
			return "", fmt.Errorf("library %s: non-printable byte 0x%02x in version: %w", path, b, rbln_errors.ErrVersionNotFound)
		}
	}
	return string(slice), nil
}

func asciiWhitespace(b byte) bool {
	switch b {
	case ' ', '\t', '\n', '\v', '\f', '\r':
		return true
	}
	return false
}
