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
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

// Version is a parsed container-runtime version (major.minor.patch). Patch is
// captured for completeness but the CDI-default gates only compare major.minor.
type Version struct {
	Major int
	Minor int
	Patch int
}

// AtLeast reports whether v >= major.minor.
func (v Version) AtLeast(major, minor int) bool {
	if v.Major != major {
		return v.Major > major
	}
	return v.Minor >= minor
}

// versionPattern extracts the first N.N[.N] token from a `--version` string.
// Runtime version banners embed the semver as e.g. "v2.0.0" (containerd) or
// "Docker version 28.2.0, build ..." — the leading digit.digit is always the
// version, so a left-to-right match is safe.
var versionPattern = regexp.MustCompile(`(\d+)\.(\d+)(?:\.(\d+))?`)

// parseVersion pulls a Version out of a runtime `--version` banner. ok is false
// when no version token is present.
func parseVersion(s string) (Version, bool) {
	m := versionPattern.FindStringSubmatch(s)
	if m == nil {
		return Version{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	patch := 0
	if m[3] != "" {
		patch, _ = strconv.Atoi(m[3])
	}
	return Version{Major: major, Minor: minor, Patch: patch}, true
}

// versionArgs returns the argv that prints the runtime version banner, or nil
// for runtimes whose CDI readiness does not depend on version (crio relies on
// drop-in content equivalence, so no version probe is needed).
func versionArgs(rt RuntimeType) []string {
	switch rt {
	case RuntimeContainerd:
		return []string{"containerd", "--version"}
	case RuntimeDocker:
		return []string{"dockerd", "--version"}
	default:
		return nil
	}
}

// commandRunner executes a command and returns combined stdout+stderr. It is a
// package var so tests can stub version detection without exec-ing real host
// binaries.
var commandRunner = func(ctx context.Context, name string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}

// DetectVersion queries the runtime binary for its version. When hostRoot is a
// real mount ("" and "/" mean "run directly"), it chroots into it so the daemon
// container observes the host's runtime binary — mirroring the restart package.
//
// ok is false whenever the version can't be determined (binary absent, probe
// error, unparseable output, or a runtime with no version probe). Callers must
// fall back to config-content comparison in that case rather than assuming a
// default-on runtime.
func DetectVersion(rt RuntimeType, hostRoot string) (Version, bool) {
	argv := versionArgs(rt)
	if argv == nil {
		return Version{}, false
	}

	name, args := argv[0], argv[1:]
	if hostRoot != "" && hostRoot != "/" {
		args = append([]string{hostRoot, name}, args...)
		name = "chroot"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := commandRunner(ctx, name, args...)
	if err != nil {
		return Version{}, false
	}
	return parseVersion(string(out))
}
