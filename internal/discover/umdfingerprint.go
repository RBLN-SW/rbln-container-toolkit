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
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// rblnLibraryGlobs lists the filename patterns for the RBLN UMD shared
// libraries the watcher fingerprints for change detection. ccl and thunk ship
// together in one driver package and bump in lockstep, so fingerprinting the
// two is a sufficient upgrade signal; librbln-ml is intentionally excluded to
// avoid per-tick work it would only duplicate. Add entries here if a new
// library must independently drive CDI regeneration.
var rblnLibraryGlobs = []string{
	"librbln-ccl.so*",
	"librbln-thunk.so*",
}

// errNoBuildID signals that an ELF file carries no usable GNU build-id note.
// It is an internal sentinel: callers fall back to file metadata rather than
// surfacing it, so it never reaches the watcher's error stream.
var errNoBuildID = errors.New("no usable .note.gnu.build-id note")

// ntGNUBuildID is the ELF note type (NT_GNU_BUILD_ID) that tags the build-id
// descriptor inside the .note.gnu.build-id section.
const ntGNUBuildID = 3

// ProbeRBLNLibraries scans libDirs for RBLN UMD libraries (librbln-*.so*) and
// returns a per-library change-detection fingerprint. Symlinks resolving to
// the same target are deduplicated so each underlying library file contributes
// a single entry, keyed by its resolved path.
//
// The fingerprint (see libraryFingerprint) is what the watcher diffs between
// ticks to decide whether the driver changed. It deliberately does NOT depend
// on the embedded `rbln version:` marker: that marker lives in the ELF
// .comment section, which RPM's brp-strip-comment-note removes on RHEL driver
// builds, so a marker-based snapshot is permanently empty there (baseline
// `<none>`, auto-refresh never fires, and a warning is logged every tick).
// A build-id / size+mtime fingerprint survives stripping and needs no marker.
//
// libDirs that don't exist or can't be read are skipped without error. A
// library that resolves but can't be fingerprinted (a genuine I/O error, not a
// missing marker) contributes an entry to errs but does not abort the scan.
func ProbeRBLNLibraries(libDirs []string) (fingerprints map[string]string, errs map[string]error) {
	fingerprints = make(map[string]string)
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

			fp, err := libraryFingerprint(resolved)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}
				errs[resolved] = err
				continue
			}
			fingerprints[resolved] = fp
		}
	}
	return fingerprints, errs
}

// libraryFingerprint returns an opaque value that changes whenever the library
// binary changes. It is the watcher's change-detection key: a driver upgrade
// flips the fingerprint, which triggers CDI regeneration.
//
// The GNU build-id (a linker-computed content hash in .note.gnu.build-id) is
// preferred because it survives stripping — including the RPM
// `brp-strip-comment-note` pass that deletes the `rbln version:` .comment
// marker on RHEL driver builds, which is exactly why the marker-based probe
// went blind there. When the build-id is absent (e.g. linked with
// --build-id=none) or the file is not parseable ELF, we fall back to size and
// mtime, which still flip on any real library replacement.
//
// A build-id that cannot be read — absent, unparseable, carried in a
// compressed or implausibly large section, or hit by a transient I/O error —
// is never itself an error: the function falls back to size+mtime. The only
// error it returns is a failing os.Stat on that fallback path, so the watcher
// no longer logs a per-tick warning for marker-less libraries.
func libraryFingerprint(path string) (string, error) {
	if id, err := readELFBuildID(path); err == nil && id != "" {
		return "build-id:" + id, nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("size:%d,mtime:%d", fi.Size(), fi.ModTime().UnixNano()), nil
}

// maxBuildIDNoteBytes caps how many bytes readELFBuildID will accept for the
// .note.gnu.build-id section. A genuine note is a 4-byte "GNU\0" owner plus a
// 16- or 20-byte digest — a few dozen bytes — so 64 KiB is orders of magnitude
// of headroom while still bounding the work a corrupt or hostile section
// header can demand on every watcher tick.
const maxBuildIDNoteBytes = 64 * 1024

// readELFBuildID opens path as an ELF image and extracts the hex-encoded GNU
// build-id. Only the ELF header, the section header table, and the small
// note section are read — never the whole (tens-of-MB) library — so this stays
// cheap enough to run against every discovered library on each watcher tick.
func readELFBuildID(path string) (string, error) {
	f, err := elf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sec := f.Section(".note.gnu.build-id")
	if sec == nil {
		return "", errNoBuildID
	}
	// A genuine GNU build-id note is never compressed and is only a few dozen
	// bytes. Refuse a SHF_COMPRESSED or oversized section rather than hand it to
	// sec.Data(): for a compressed section sec.Size is the attacker-controlled
	// *uncompressed* size from the ELF compression header, so a crafted note
	// section could balloon allocation on every tick. Falling back to size+mtime
	// keeps this function's "reads only the small note, never the whole library"
	// contract true even for hostile or corrupt input.
	if sec.Flags&elf.SHF_COMPRESSED != 0 || sec.Size > maxBuildIDNoteBytes {
		return "", errNoBuildID
	}
	data, err := sec.Data()
	if err != nil {
		return "", err
	}
	return parseBuildIDNote(data, f.ByteOrder)
}

// parseBuildIDNote walks the ELF notes in a .note.gnu.build-id section and
// returns the hex-encoded descriptor of the first NT_GNU_BUILD_ID note owned
// by "GNU". Each note is [namesz|descsz|type|name|desc] with name and desc
// individually padded to 4-byte boundaries; byte order follows the ELF file.
//
// Sizes are read as uint32 and kept in uint64 for all length math so a
// corrupt/hostile note (huge namesz/descsz) can neither overflow nor index
// out of bounds — it simply fails the length guard and yields errNoBuildID.
func parseBuildIDNote(data []byte, bo binary.ByteOrder) (string, error) {
	for len(data) >= 12 {
		namesz := uint64(bo.Uint32(data[0:4]))
		descsz := uint64(bo.Uint32(data[4:8]))
		typ := bo.Uint32(data[8:12])
		rest := data[12:]

		nameAligned := align4(namesz)
		if nameAligned > uint64(len(rest)) {
			break
		}
		name := rest[:namesz]
		rest = rest[nameAligned:]

		descAligned := align4(descsz)
		if descAligned > uint64(len(rest)) {
			break
		}
		desc := rest[:descsz]

		if typ == ntGNUBuildID && strings.TrimRight(string(name), "\x00") == "GNU" && descsz > 0 {
			return hex.EncodeToString(desc), nil
		}
		data = rest[descAligned:]
	}
	return "", errNoBuildID
}

// align4 rounds n up to the next multiple of 4, the alignment ELF notes use
// for their name and descriptor fields. Operating in uint64 keeps the rounding
// overflow-free for any uint32-derived size.
func align4(n uint64) uint64 { return (n + 3) &^ 3 }
