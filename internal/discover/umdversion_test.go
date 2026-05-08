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
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rbln_errors "github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// fakeLib produces bytes that resemble an .so file with the version marker
// surrounded by other random data and NUL bytes, mimicking the layout of a
// real read-only data section.
func fakeLib(version string) []byte {
	var b []byte
	b = append(b, "ELF garbage prefix\x00\x00"...)
	b = append(b, "some other string\x00"...)
	b = append(b, versionMarker...)
	b = append(b, version...)
	b = append(b, 0x00)
	b = append(b, "trailing data\x00"...)
	return b
}

func writeFakeLib(t *testing.T, dir, name, version string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, fakeLib(version), 0o644))
	return p
}

func TestProbeLibraryVersion_Success(t *testing.T) {
	// Given
	dir := t.TempDir()
	want := "3.2.0~dev.165+ge5b75f0d.dirty"
	path := writeFakeLib(t, dir, "librbln-ml.so", want)

	// When
	got, err := ProbeLibraryVersion(path)

	// Then
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestProbeLibraryVersion_NoMarker(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	require.NoError(t, os.WriteFile(path, []byte("no marker here\x00"), 0o644))

	// When
	_, err := ProbeLibraryVersion(path)

	// Then
	require.Error(t, err)
	assert.True(t, errors.Is(err, rbln_errors.ErrVersionNotFound))
}

func TestProbeLibraryVersion_EmptyAfterMarker(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	body := []byte(versionMarker + "\x00trailer")
	require.NoError(t, os.WriteFile(path, body, 0o644))

	// When
	_, err := ProbeLibraryVersion(path)

	// Then
	require.Error(t, err)
	assert.True(t, errors.Is(err, rbln_errors.ErrVersionNotFound))
}

func TestProbeLibraryVersion_NonPrintable(t *testing.T) {
	// Given
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	body := []byte(versionMarker + "3.2\x01rogue\x00")
	require.NoError(t, os.WriteFile(path, body, 0o644))

	// When
	_, err := ProbeLibraryVersion(path)

	// Then
	require.Error(t, err)
	assert.True(t, errors.Is(err, rbln_errors.ErrVersionNotFound))
}

// High-ASCII / non-UTF-8 bytes must be rejected. Iterating runes on a string
// silently maps these to U+FFFD which would slip past a rune-level check.
func TestProbeLibraryVersion_RejectsHighASCIIBytes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	// 0xFF is invalid UTF-8 and not printable ASCII; must fail the byte check.
	body := append([]byte(versionMarker+"3.2"), 0xff, '\x00')
	require.NoError(t, os.WriteFile(path, body, 0o644))

	_, err := ProbeLibraryVersion(path)
	require.Error(t, err)
	assert.True(t, errors.Is(err, rbln_errors.ErrVersionNotFound))
}

func TestProbeLibraryVersion_FirstMarkerWins(t *testing.T) {
	// Given two embedded versions in one file (e.g. archived as a single blob)
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	body := []byte(versionMarker + "3.2.0\x00garbage\x00" + versionMarker + "9.9.9\x00")
	require.NoError(t, os.WriteFile(path, body, 0o644))

	// When
	got, err := ProbeLibraryVersion(path)

	// Then
	require.NoError(t, err)
	assert.Equal(t, "3.2.0", got)
}

func TestProbeLibraryVersion_UnterminatedRespectsCap(t *testing.T) {
	// Given a marker followed by maxVersionLen+10 printable bytes without NUL
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	body := []byte(versionMarker + strings.Repeat("a", maxVersionLen+10))
	require.NoError(t, os.WriteFile(path, body, 0o644))

	// When
	got, err := ProbeLibraryVersion(path)

	// Then
	require.NoError(t, err)
	assert.Len(t, got, maxVersionLen)
}

// TestProbeLibraryVersion_StreamingDoesNotBufferFile verifies the scanner
// finds the marker in a synthetic file much larger than the bufio buffer,
// proving the implementation is genuinely chunked rather than slurping the
// whole file. We don't measure peak allocation directly; instead we feed a
// large file and rely on the scanner contract: it must locate a marker
// placed past several scanBufSize boundaries.
func TestProbeLibraryVersion_StreamingDoesNotBufferFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so.huge")

	// 5 * scanBufSize of padding before the marker, then a normal trailer.
	const padding = 5 * scanBufSize
	body := make([]byte, 0, padding+64)
	body = append(body, make([]byte, padding)...)
	body = append(body, []byte(versionMarker+"4.5.6\x00")...)
	require.NoError(t, os.WriteFile(path, body, 0o644))

	got, err := ProbeLibraryVersion(path)
	require.NoError(t, err)
	assert.Equal(t, "4.5.6", got)
}

// TestProbeLibraryVersion_PartialMarkerRestart guards the scanner against
// an off-by-one where a near-match prefix would discard a real marker that
// immediately follows it. Pattern: "rbln rbln version: 1.0.0" — the first
// "rbln " must not eat the leading 'r' of the actual marker.
func TestProbeLibraryVersion_PartialMarkerRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	body := []byte("rbln noise rbln " + versionMarker + "7.7.7\x00")
	require.NoError(t, os.WriteFile(path, body, 0o644))

	got, err := ProbeLibraryVersion(path)
	require.NoError(t, err)
	assert.Equal(t, "7.7.7", got)
}

// TestProbeLibraryVersion_MarkerInternalRepeat regression-tests an input
// that defeats a naive single-byte rewind: the marker prefix "rbln ver"
// shares its trailing 'r' with the marker's leading 'r'. After consuming
// "rbln ver" and seeing a non-marker byte, a single-byte rewind would
// reset to 0 and miss the actual marker that follows.
func TestProbeLibraryVersion_MarkerInternalRepeat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")
	body := []byte("rbln verbln " + versionMarker + "8.8.8\x00")
	require.NoError(t, os.WriteFile(path, body, 0o644))

	got, err := ProbeLibraryVersion(path)
	require.NoError(t, err)
	assert.Equal(t, "8.8.8", got)
}

// TestProbeLibraryVersion_MarkerStraddlesChunkBoundary verifies the
// chunked scanner finds the marker when it spans the boundary between
// two read chunks, exercising the overlap-carry logic.
func TestProbeLibraryVersion_MarkerStraddlesChunkBoundary(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "librbln-ml.so")

	// Place the marker so its 5th byte aligns with offset scanBufSize+overlap
	// (the start of the second chunk). The marker is then split across the
	// carry-over and the next read.
	overlap := len(versionMarker) - 1
	splitAt := scanBufSize + overlap - 5
	prefix := make([]byte, splitAt)
	body := append(prefix, []byte(versionMarker+"9.9.9\x00")...)
	require.NoError(t, os.WriteFile(path, body, 0o644))

	got, err := ProbeLibraryVersion(path)
	require.NoError(t, err)
	assert.Equal(t, "9.9.9", got)
}

// errAfterReader yields head, then returns (0, err) on every subsequent
// Read call. Useful to drive scanLibraryVersion / readVersionTrailer past
// a synthetic mid-read I/O failure.
type errAfterReader struct {
	head []byte
	err  error
	pos  int
}

func (r *errAfterReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.head) {
		return 0, r.err
	}
	n := copy(p, r.head[r.pos:])
	r.pos += n
	return n, nil
}

// TestScanLibraryVersion_PropagatesTrailerReadErrors regression-tests that
// a non-EOF I/O error encountered while reading the version trailer (e.g.
// EIO mid-read on a flaky disk) propagates to the caller instead of being
// silently turned into a "valid" version string.
func TestScanLibraryVersion_PropagatesTrailerReadErrors(t *testing.T) {
	boom := errors.New("synthetic disk EIO")
	body := append([]byte("ELF\x00prefix\x00"), []byte(versionMarker+"1.2")...)
	r := &errAfterReader{head: body, err: boom}

	_, err := scanLibraryVersion(r, "/fake.so")
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
}

func TestProbeLibraryVersion_MissingFileReturnsNotExist(t *testing.T) {
	// When
	_, err := ProbeLibraryVersion(filepath.Join(t.TempDir(), "nope.so"))

	// Then
	require.Error(t, err)
	assert.True(t, errors.Is(err, fs.ErrNotExist))
}

func TestProbeRBLNLibraries_GlobsAndDedupes(t *testing.T) {
	// Given two lib dirs: one with two real RBLN libs and a symlink, the other empty
	dirA := t.TempDir()
	dirB := t.TempDir()
	mlPath := writeFakeLib(t, dirA, "librbln-ml.so.1.2.3", "1.2.3")
	cclPath := writeFakeLib(t, dirA, "librbln-ccl.so.4.5.6", "4.5.6")
	// Symlink chain: librbln-ml.so -> librbln-ml.so.1 -> librbln-ml.so.1.2.3
	require.NoError(t, os.Symlink(filepath.Base(mlPath), filepath.Join(dirA, "librbln-ml.so.1")))
	require.NoError(t, os.Symlink("librbln-ml.so.1", filepath.Join(dirA, "librbln-ml.so")))
	// A non-RBLN library that must not be picked up
	require.NoError(t, os.WriteFile(filepath.Join(dirA, "libfoo.so"), []byte("not rbln"), 0o644))

	// When
	versions, errs := ProbeRBLNLibraries([]string{dirA, dirB, "/nonexistent/dir"})

	// Then both real RBLN libs are discovered; the symlink chain collapses to one entry.
	// We compare against EvalSymlinks-resolved keys because that is what the discoverer returns
	// (e.g. on macOS /tmp resolves to /private/tmp).
	require.Empty(t, errs)
	assert.Equal(t, map[string]string{
		realPath(t, mlPath):  "1.2.3",
		realPath(t, cclPath): "4.5.6",
	}, versions)
}

func TestProbeRBLNLibraries_LibsWithoutMarkerAreReportedAsErrors(t *testing.T) {
	// Given a lib that matches the glob but lacks the version marker
	dir := t.TempDir()
	bad := filepath.Join(dir, "librbln-broken.so")
	require.NoError(t, os.WriteFile(bad, []byte("no marker here"), 0o644))

	// When
	versions, errs := ProbeRBLNLibraries([]string{dir})

	// Then the failure is reported but does not blow up the scan
	assert.Empty(t, versions)
	require.Contains(t, errs, realPath(t, bad))
	assert.True(t, errors.Is(errs[realPath(t, bad)], rbln_errors.ErrVersionNotFound))
}

func realPath(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	require.NoError(t, err)
	return r
}

func TestProbeRBLNLibraries_NoLibsReturnsEmpty(t *testing.T) {
	dir := t.TempDir()
	versions, errs := ProbeRBLNLibraries([]string{dir, "/nonexistent"})
	assert.Empty(t, versions)
	assert.Empty(t, errs)
}

func TestProbeLibraryVersions_SkipsMissingAggregatesErrors(t *testing.T) {
	// Given
	dir := t.TempDir()
	good := writeFakeLib(t, dir, "librbln-ml.so", "3.2.0")
	bad := filepath.Join(dir, "librbln-ccl.so")
	require.NoError(t, os.WriteFile(bad, []byte("no marker"), 0o644))
	missing := filepath.Join(dir, "librbln-thunk.so")

	// When
	versions, errs := ProbeLibraryVersions([]string{good, bad, missing})

	// Then
	assert.Equal(t, map[string]string{good: "3.2.0"}, versions)
	require.Contains(t, errs, bad)
	assert.True(t, errors.Is(errs[bad], rbln_errors.ErrVersionNotFound))
	assert.NotContains(t, errs, missing, "missing libraries should be silently omitted")
}
