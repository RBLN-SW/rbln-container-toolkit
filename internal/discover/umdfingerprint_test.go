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
	"debug/elf"
	"encoding/binary"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeNote assembles a single ELF note (namesz|descsz|type|name|desc) with the
// name and descriptor padded to 4-byte boundaries, in the given byte order.
func makeNote(bo binary.ByteOrder, name string, typ uint32, desc []byte) []byte {
	b := &bytes.Buffer{}
	nameBytes := append([]byte(name), 0) // NUL-terminated owner name
	_ = binary.Write(b, bo, uint32(len(nameBytes)))
	_ = binary.Write(b, bo, uint32(len(desc)))
	_ = binary.Write(b, bo, typ)
	b.Write(nameBytes)
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
	b.Write(desc)
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
	return b.Bytes()
}

func TestParseBuildIDNote(t *testing.T) {
	id := []byte{0x13, 0xe7, 0x99, 0xb4, 0xac, 0x4a, 0xe6, 0x9c, 0xfa, 0xc0}

	t.Run("little endian GNU note", func(t *testing.T) {
		got, err := parseBuildIDNote(makeNote(binary.LittleEndian, "GNU", ntGNUBuildID, id), binary.LittleEndian)
		require.NoError(t, err)
		assert.Equal(t, hex.EncodeToString(id), got)
	})

	t.Run("big endian GNU note", func(t *testing.T) {
		got, err := parseBuildIDNote(makeNote(binary.BigEndian, "GNU", ntGNUBuildID, id), binary.BigEndian)
		require.NoError(t, err)
		assert.Equal(t, hex.EncodeToString(id), got)
	})

	t.Run("skips non-GNU note then finds build-id", func(t *testing.T) {
		data := append(
			makeNote(binary.LittleEndian, "stapsdt", 42, []byte{0x01, 0x02}),
			makeNote(binary.LittleEndian, "GNU", ntGNUBuildID, id)...,
		)
		got, err := parseBuildIDNote(data, binary.LittleEndian)
		require.NoError(t, err)
		assert.Equal(t, hex.EncodeToString(id), got)
	})

	t.Run("wrong note type is not a build-id", func(t *testing.T) {
		_, err := parseBuildIDNote(makeNote(binary.LittleEndian, "GNU", 1, id), binary.LittleEndian)
		assert.ErrorIs(t, err, errNoBuildID)
	})

	t.Run("truncated header", func(t *testing.T) {
		_, err := parseBuildIDNote([]byte{0x04, 0x00, 0x00}, binary.LittleEndian)
		assert.ErrorIs(t, err, errNoBuildID)
	})

	t.Run("oversized descsz fails the length guard, no panic", func(t *testing.T) {
		// namesz=4 ("GNU\0"), descsz claims 0xFFFFFFFF but no bytes follow.
		note := make([]byte, 16)
		binary.LittleEndian.PutUint32(note[0:], 4)
		binary.LittleEndian.PutUint32(note[4:], 0xFFFFFFFF)
		binary.LittleEndian.PutUint32(note[8:], ntGNUBuildID)
		copy(note[12:], "GNU\x00")
		_, err := parseBuildIDNote(note, binary.LittleEndian)
		assert.ErrorIs(t, err, errNoBuildID)
	})
}

// buildELFWithBuildID assembles a minimal but valid little-endian ELF64 image
// carrying a single .note.gnu.build-id section, enough for debug/elf to open
// and for Section(...).Data() to return the note. This exercises the real
// readELFBuildID path (elf.Open + section lookup) end-to-end.
func buildELFWithBuildID(buildID []byte) []byte {
	return buildELFWithNote(buildID, 0, 0)
}

// buildELFWithNote is buildELFWithBuildID with two knobs for exercising
// readELFBuildID's defensive guards: extraFlags is OR'd into the note
// section's sh_flags (e.g. elf.SHF_COMPRESSED), and a non-zero sizeOverride
// replaces the note section's sh_size — letting a test fake a compressed or
// implausibly large section without allocating one.
func buildELFWithNote(buildID []byte, extraFlags, sizeOverride uint64) []byte {
	le := binary.LittleEndian
	note := makeNote(le, "GNU", ntGNUBuildID, buildID)

	// Section header string table: "\0.note.gnu.build-id\0.shstrtab\0".
	shstr := &bytes.Buffer{}
	shstr.WriteByte(0)
	nameNote := shstr.Len()
	shstr.WriteString(".note.gnu.build-id")
	shstr.WriteByte(0)
	nameStr := shstr.Len()
	shstr.WriteString(".shstrtab")
	shstr.WriteByte(0)

	const ehSize = 64
	const shEntSize = 64
	offNote := ehSize
	offStr := offNote + len(note)
	offSh := offStr + shstr.Len()
	if r := offSh % 8; r != 0 {
		offSh += 8 - r
	}

	buf := make([]byte, offSh+3*shEntSize)

	// ELF identification + header.
	copy(buf[0:], []byte{0x7f, 'E', 'L', 'F', 2 /*ELFCLASS64*/, 1 /*little-endian*/, 1 /*version*/, 0})
	le.PutUint16(buf[16:], 3)  // e_type = ET_DYN
	le.PutUint16(buf[18:], 62) // e_machine = EM_X86_64
	le.PutUint32(buf[20:], 1)  // e_version
	le.PutUint64(buf[40:], uint64(offSh))
	le.PutUint16(buf[52:], ehSize)
	le.PutUint16(buf[58:], shEntSize)
	le.PutUint16(buf[60:], 3) // e_shnum
	le.PutUint16(buf[62:], 2) // e_shstrndx

	copy(buf[offNote:], note)
	copy(buf[offStr:], shstr.Bytes())

	// Section header [1]: .note.gnu.build-id (SHT_NOTE).
	shSize := uint64(len(note))
	if sizeOverride != 0 {
		shSize = sizeOverride
	}
	sh1 := offSh + shEntSize
	le.PutUint32(buf[sh1+0:], uint32(nameNote))
	le.PutUint32(buf[sh1+4:], 7)            // SHT_NOTE
	le.PutUint64(buf[sh1+8:], 2|extraFlags) // SHF_ALLOC (| extra, e.g. SHF_COMPRESSED)
	le.PutUint64(buf[sh1+24:], uint64(offNote))
	le.PutUint64(buf[sh1+32:], shSize)
	le.PutUint64(buf[sh1+48:], 4) // sh_addralign

	// Section header [2]: .shstrtab (SHT_STRTAB).
	sh2 := offSh + 2*shEntSize
	le.PutUint32(buf[sh2+0:], uint32(nameStr))
	le.PutUint32(buf[sh2+4:], 3) // SHT_STRTAB
	le.PutUint64(buf[sh2+24:], uint64(offStr))
	le.PutUint64(buf[sh2+32:], uint64(shstr.Len()))
	le.PutUint64(buf[sh2+48:], 1) // sh_addralign

	return buf
}

// buildELFNoBuildID assembles a minimal valid little-endian ELF64 with only a
// .shstrtab section — a parseable ELF that carries no build-id note, so it
// exercises libraryFingerprint's fallback for a real (not corrupt) library.
func buildELFNoBuildID() []byte {
	le := binary.LittleEndian

	shstr := &bytes.Buffer{}
	shstr.WriteByte(0)
	nameStr := shstr.Len()
	shstr.WriteString(".shstrtab")
	shstr.WriteByte(0)

	const ehSize = 64
	const shEntSize = 64
	offStr := ehSize
	offSh := offStr + shstr.Len()
	if r := offSh % 8; r != 0 {
		offSh += 8 - r
	}

	buf := make([]byte, offSh+2*shEntSize)

	copy(buf[0:], []byte{0x7f, 'E', 'L', 'F', 2, 1, 1, 0})
	le.PutUint16(buf[16:], 3)
	le.PutUint16(buf[18:], 62)
	le.PutUint32(buf[20:], 1)
	le.PutUint64(buf[40:], uint64(offSh))
	le.PutUint16(buf[52:], ehSize)
	le.PutUint16(buf[58:], shEntSize)
	le.PutUint16(buf[60:], 2) // e_shnum
	le.PutUint16(buf[62:], 1) // e_shstrndx

	copy(buf[offStr:], shstr.Bytes())

	sh1 := offSh + shEntSize
	le.PutUint32(buf[sh1+0:], uint32(nameStr))
	le.PutUint32(buf[sh1+4:], 3) // SHT_STRTAB
	le.PutUint64(buf[sh1+24:], uint64(offStr))
	le.PutUint64(buf[sh1+32:], uint64(shstr.Len()))
	le.PutUint64(buf[sh1+48:], 1)

	return buf
}

func TestReadELFBuildID(t *testing.T) {
	id := []byte{0x13, 0xe7, 0x99, 0xb4, 0xac, 0x4a, 0xe6, 0x9c, 0xfa, 0xc0, 0xb9, 0x20, 0xe7, 0xbb, 0xc7, 0x78, 0xd2, 0x8d, 0xe8, 0x61}
	path := filepath.Join(t.TempDir(), "lib.so")
	require.NoError(t, os.WriteFile(path, buildELFWithBuildID(id), 0o644))

	got, err := readELFBuildID(path)
	require.NoError(t, err)
	assert.Equal(t, hex.EncodeToString(id), got)
}

func TestReadELFBuildID_SkipsCompressedNoteSection(t *testing.T) {
	// A genuine GNU build-id note is never compressed. A section flagged
	// SHF_COMPRESSED must be refused — its sh_size is the attacker-controlled
	// *uncompressed* size — and fall back to size+mtime rather than be handed to
	// sec.Data(), which would decompress it on every watcher tick.
	id := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	path := filepath.Join(t.TempDir(), "librbln-ccl.so")
	require.NoError(t, os.WriteFile(path, buildELFWithNote(id, uint64(elf.SHF_COMPRESSED), 0), 0o644))

	_, err := readELFBuildID(path)
	require.ErrorIs(t, err, errNoBuildID)

	fp, err := libraryFingerprint(path)
	require.NoError(t, err)
	assert.Truef(t, strings.HasPrefix(fp, "size:"), "compressed build-id section should fall back to size+mtime, got %q", fp)
}

func TestReadELFBuildID_RefusesOversizedNoteSection(t *testing.T) {
	// A section header claiming a huge .note.gnu.build-id size must be refused
	// before sec.Data() allocates for it, bounding per-tick work against a
	// corrupt or hostile library.
	id := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	path := filepath.Join(t.TempDir(), "librbln-ccl.so")
	require.NoError(t, os.WriteFile(path, buildELFWithNote(id, 0, maxBuildIDNoteBytes+1), 0o644))

	_, err := readELFBuildID(path)
	require.ErrorIs(t, err, errNoBuildID)

	fp, err := libraryFingerprint(path)
	require.NoError(t, err)
	assert.Truef(t, strings.HasPrefix(fp, "size:"), "oversized build-id section should fall back to size+mtime, got %q", fp)
}

func TestLibraryFingerprint_PrefersBuildID(t *testing.T) {
	id := []byte{0xde, 0xad, 0xbe, 0xef, 0x00, 0x11, 0x22, 0x33}
	path := filepath.Join(t.TempDir(), "librbln-ccl.so.3.2.2")
	require.NoError(t, os.WriteFile(path, buildELFWithBuildID(id), 0o644))

	fp, err := libraryFingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, "build-id:"+hex.EncodeToString(id), fp)
}

func TestLibraryFingerprint_StatFallbackForELFWithoutBuildID(t *testing.T) {
	// A valid, parseable ELF that simply has no .note.gnu.build-id (the
	// --build-id=none case the fingerprint doc calls out) must fall back to
	// size+mtime — a different code path from a non-ELF file.
	path := filepath.Join(t.TempDir(), "librbln-ccl.so.3.2.2")
	require.NoError(t, os.WriteFile(path, buildELFNoBuildID(), 0o644))

	_, err := readELFBuildID(path)
	require.ErrorIs(t, err, errNoBuildID, "sanity: the ELF parses but has no build-id note")

	fp, err := libraryFingerprint(path)
	require.NoError(t, err)
	assert.Truef(t, strings.HasPrefix(fp, "size:"), "valid ELF without build-id should fall back to size+mtime, got %q", fp)
}

func TestLibraryFingerprint_StatFallbackForNonELF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "librbln-ccl.so.3.2.2")
	require.NoError(t, os.WriteFile(path, []byte("not an elf file, and no marker"), 0o644))

	fp, err := libraryFingerprint(path)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(fp, "size:"), "non-ELF file should use the size+mtime fallback, got %q", fp)

	// Deterministic for an unchanged file.
	again, err := libraryFingerprint(path)
	require.NoError(t, err)
	assert.Equal(t, fp, again)
}

func TestLibraryFingerprint_ChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.so")
	b := filepath.Join(dir, "b.so")
	require.NoError(t, os.WriteFile(a, []byte("short"), 0o644))
	require.NoError(t, os.WriteFile(b, []byte("a considerably longer body"), 0o644))

	fpA, err := libraryFingerprint(a)
	require.NoError(t, err)
	fpB, err := libraryFingerprint(b)
	require.NoError(t, err)
	assert.NotEqual(t, fpA, fpB, "different-sized libraries must fingerprint differently")
}

func TestLibraryFingerprint_MissingFileErrors(t *testing.T) {
	_, err := libraryFingerprint(filepath.Join(t.TempDir(), "nope.so"))
	assert.ErrorIs(t, err, fs.ErrNotExist)
}

// fakeLib produces arbitrary non-ELF bytes for a library file; `tag` varies the
// content so distinct libraries produce distinct fingerprints. The watcher's
// fingerprint no longer parses these bytes for a marker (a non-ELF file falls
// back to size+mtime), so the content only needs to exist and differ.
func fakeLib(tag string) []byte {
	var b []byte
	b = append(b, "ELF garbage prefix\x00\x00"...)
	b = append(b, tag...)
	b = append(b, "\x00trailing data\x00"...)
	return b
}

func writeFakeLib(t *testing.T, dir, name, tag string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, fakeLib(tag), 0o644))
	return p
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestProbeRBLNLibraries_GlobsAndDedupes(t *testing.T) {
	// Given two lib dirs: one with two real RBLN libs and a symlink chain,
	// the other empty
	dirA := t.TempDir()
	dirB := t.TempDir()
	thunkPath := writeFakeLib(t, dirA, "librbln-thunk.so.1.2.3", "thunk-1.2.3")
	cclPath := writeFakeLib(t, dirA, "librbln-ccl.so.4.5.6", "ccl-4.5.6")
	// Symlink chain: librbln-thunk.so -> librbln-thunk.so.1 -> librbln-thunk.so.1.2.3
	require.NoError(t, os.Symlink(filepath.Base(thunkPath), filepath.Join(dirA, "librbln-thunk.so.1")))
	require.NoError(t, os.Symlink("librbln-thunk.so.1", filepath.Join(dirA, "librbln-thunk.so")))
	// A non-RBLN library that must not be picked up
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "libfoo.so"), []byte("not rbln"), 0o644))
	// librbln-ml is intentionally excluded from the probe globs; a file matching
	// the old broad glob must NOT contribute to the probe result.
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "librbln-ml.so"), []byte("ml"), 0o644))

	// When
	versions, errs := ProbeRBLNLibraries([]string{dirA, dirB, "/nonexistent/dir"})

	// Then both real RBLN libs are discovered; the symlink chain collapses to one entry.
	// We compare against EvalSymlinks-resolved keys because that is what the discoverer returns
	// (e.g. on macOS /tmp resolves to /private/tmp).
	require.Empty(t, errs)
	require.Len(t, versions, 2)
	assert.Contains(t, versions, realPath(t, thunkPath))
	assert.Contains(t, versions, realPath(t, cclPath))
	// These fake libs are not valid ELF, so the fingerprint falls back to
	// size+mtime rather than a build-id.
	for k, v := range versions {
		assert.Truef(t, strings.HasPrefix(v, "size:"), "non-ELF lib %s should fall back to size+mtime, got %q", k, v)
	}
}

func TestProbeRBLNLibraries_UsesBuildIDForRealELF(t *testing.T) {
	// A real (ELF) driver library is fingerprinted by its build-id, which
	// survives the RPM strip that removes the version marker.
	dir := t.TempDir()
	id := []byte{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89}
	p := filepath.Join(dir, "librbln-thunk.so.3.2.2")
	require.NoError(t, os.WriteFile(p, buildELFWithBuildID(id), 0o644))

	versions, errs := ProbeRBLNLibraries([]string{dir})

	assert.Empty(t, errs)
	assert.Equal(t, "build-id:"+hex.EncodeToString(id), versions[realPath(t, p)])
}

func TestProbeRBLNLibraries_MarkerlessLibIsFingerprinted(t *testing.T) {
	// A RHEL driver library carries no `rbln version:` marker (RPM's
	// brp-strip-comment-note deletes the .comment section that holds it). It
	// must still be fingerprinted so the watcher can detect changes, and it
	// must NOT surface as an error — the per-tick ErrVersionNotFound warning
	// spam is exactly the regression this fingerprint switch removes.
	dir := t.TempDir()
	lib := filepath.Join(dir, "librbln-ccl.so")
	require.NoError(t, os.WriteFile(lib, []byte("no marker here"), 0o644))

	// When
	versions, errs := ProbeRBLNLibraries([]string{dir})

	// Then the lib is fingerprinted and no error is reported.
	assert.Empty(t, errs)
	require.Contains(t, versions, realPath(t, lib))
	assert.NotEmpty(t, versions[realPath(t, lib)])
}

func TestProbeRBLNLibraries_SkipsLibrariesWithoutMarkerContract(t *testing.T) {
	// The probe scopes itself to the opted-in UMD libraries (ccl, thunk).
	// librbln-ml and any other librbln-*.so must be ignored at the glob layer
	// so they neither contribute a fingerprint nor surface as an error.
	dir := t.TempDir()
	writeFakeLib(t, dir, "librbln-ccl.so.3.0.0", "ccl-3.0.0")
	writeFakeLib(t, dir, "librbln-thunk.so.3.0.0", "thunk-3.0.0")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "librbln-ml.so"), []byte("ml"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "librbln-future.so"), []byte("future"), 0o644))

	versions, errs := ProbeRBLNLibraries([]string{dir})

	assert.Empty(t, errs, "non-probed librbln-*.so files must not surface as errors")
	assert.Len(t, versions, 2, "only the two opted-in libraries should be probed")
}

func TestProbeRBLNLibraries_NoLibsReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	versions, errs := ProbeRBLNLibraries([]string{dir, "/nonexistent"})
	assert.Empty(t, versions)
	assert.Empty(t, errs)
}
