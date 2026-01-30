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
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectOS(t *testing.T) {
	// When: Detecting the OS
	osType := DetectOS()

	// Then: Should return a valid OS type
	assert.NotEmpty(t, osType)

	// On the test machine, it should be one of the supported types
	validTypes := []OSType{OSUbuntu, OSRHEL, OSCoreOS, OSUnknown}
	assert.Contains(t, validTypes, osType)
}

func TestOSType_String(t *testing.T) {
	tests := []struct {
		os       OSType
		expected string
	}{
		{OSUbuntu, "ubuntu"},
		{OSRHEL, "rhel"},
		{OSCoreOS, "coreos"},
		{OSUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.os.String())
		})
	}
}

func TestDefaultLibraryPaths_Ubuntu(t *testing.T) {
	// Given: Ubuntu OS
	// When: Getting default library paths
	paths := DefaultLibraryPaths(OSUbuntu)

	// Then: Should include Ubuntu paths (same as Debian) based on current architecture
	expectedPath := LibraryPathForArch(OSUbuntu, GetArchitecture())
	assert.Contains(t, paths, expectedPath)
	assert.Contains(t, paths, "/usr/local/lib")
}

func TestDefaultLibraryPaths_RHEL(t *testing.T) {
	// Given: RHEL-based OS
	// When: Getting default library paths
	paths := DefaultLibraryPaths(OSRHEL)

	// Then: Should include RHEL-style paths
	assert.Contains(t, paths, "/usr/lib64")
	assert.Contains(t, paths, "/usr/local/lib64")
}

func TestDefaultLibraryPaths_CoreOS(t *testing.T) {
	// Given: CoreOS
	// When: Getting default library paths
	paths := DefaultLibraryPaths(OSCoreOS)

	// Then: Should include RHEL-style paths (CoreOS is RHEL-based)
	assert.Contains(t, paths, "/usr/lib64")
}

func TestDefaultBinaryPaths(t *testing.T) {
	// Given: Any OS
	// When: Getting default binary paths
	paths := DefaultBinaryPaths(OSUbuntu)

	// Then: Should include standard binary paths
	assert.Contains(t, paths, "/usr/bin")
	assert.Contains(t, paths, "/usr/local/bin")
}

func TestIsDebian(t *testing.T) {
	assert.True(t, OSUbuntu.IsDebian())
	assert.False(t, OSRHEL.IsDebian())
	assert.False(t, OSCoreOS.IsDebian())
}

func TestIsRHEL(t *testing.T) {
	assert.True(t, OSRHEL.IsRHEL())
	assert.True(t, OSCoreOS.IsRHEL())
	assert.False(t, OSUbuntu.IsRHEL())
}

func TestDefaultPluginPaths(t *testing.T) {
	tests := []struct {
		name     string
		osType   OSType
		validate func(t *testing.T, paths []string)
	}{
		{
			name:   "Ubuntu",
			osType: OSUbuntu,
			validate: func(t *testing.T, paths []string) {
				// Given: Ubuntu OS
				// When: Getting default plugin paths
				// Then: Should include Ubuntu-style plugin paths with architecture-specific library path
				expectedPath := LibraryPathForArch(OSUbuntu, GetArchitecture()) + "/libibverbs"
				assert.Contains(t, paths, expectedPath)
				assert.Len(t, paths, 1)
			},
		},
		{
			name:   "RHEL",
			osType: OSRHEL,
			validate: func(t *testing.T, paths []string) {
				// Given: RHEL-based OS
				// When: Getting default plugin paths
				// Then: Should include RHEL-style plugin paths
				assert.Contains(t, paths, "/usr/lib64/libibverbs")
				assert.Len(t, paths, 1)
			},
		},
		{
			name:   "CoreOS",
			osType: OSCoreOS,
			validate: func(t *testing.T, paths []string) {
				// Given: CoreOS (RHEL-based)
				// When: Getting default plugin paths
				// Then: Should include RHEL-style plugin paths (CoreOS is RHEL-based)
				assert.Contains(t, paths, "/usr/lib64/libibverbs")
				assert.Len(t, paths, 1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := DefaultPluginPaths(tt.osType)
			tt.validate(t, paths)
		})
	}
}

func TestPluginPaths_RHEL(t *testing.T) {
	// Given: RHEL-based OS
	// When: Getting plugin paths
	paths := DefaultPluginPaths(OSRHEL)

	// Then: Should include RHEL-style plugin paths
	assert.Contains(t, paths, "/usr/lib64/libibverbs")
}

func TestArchitecture(t *testing.T) {
	// When: Getting architecture
	arch := GetArchitecture()

	// Then: Should return valid architecture
	assert.NotEmpty(t, arch)

	// Should be the Go runtime's arch
	assert.Equal(t, runtime.GOARCH, arch)
}

func TestLibraryPathForArch_AMD64(t *testing.T) {
	// Given: AMD64 architecture on Ubuntu
	// When: Getting library path
	path := LibraryPathForArch(OSUbuntu, "amd64")

	// Then: Should return x86_64-linux-gnu path
	assert.Equal(t, "/usr/lib/x86_64-linux-gnu", path)
}

func TestLibraryPathForArch_ARM64(t *testing.T) {
	// Given: ARM64 architecture on Ubuntu
	// When: Getting library path
	path := LibraryPathForArch(OSUbuntu, "arm64")

	// Then: Should return aarch64-linux-gnu path
	assert.Equal(t, "/usr/lib/aarch64-linux-gnu", path)
}

func TestLibraryPathForArch_RHEL(t *testing.T) {
	// Given: AMD64 architecture on RHEL
	// When: Getting library path
	path := LibraryPathForArch(OSRHEL, "amd64")

	// Then: Should return lib64 path (RHEL doesn't use multiarch)
	assert.Equal(t, "/usr/lib64", path)
}

func TestParseOSRelease(t *testing.T) {
	// Given: A sample /etc/os-release content
	content := `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
VERSION_ID="22.04"
HOME_URL="https://www.ubuntu.com/"
`

	// When: Parsing the content
	osType := parseOSRelease(content)

	// Then: Should detect Ubuntu
	assert.Equal(t, OSUbuntu, osType)
}

func TestParseOSRelease_RHEL(t *testing.T) {
	// Given: A RHEL /etc/os-release content
	content := `NAME="Red Hat Enterprise Linux"
VERSION="9.3 (Plow)"
ID="rhel"
VERSION_ID="9.3"
`

	// When: Parsing the content
	osType := parseOSRelease(content)

	// Then: Should detect RHEL
	assert.Equal(t, OSRHEL, osType)
}

func TestParseOSRelease_CoreOS(t *testing.T) {
	// Given: A CoreOS /etc/os-release content
	content := `NAME="Red Hat Enterprise Linux CoreOS"
ID="rhcos"
VERSION="4.14"
VARIANT="CoreOS"
`

	// When: Parsing the content
	osType := parseOSRelease(content)

	// Then: Should detect CoreOS
	assert.Equal(t, OSCoreOS, osType)
}

func TestParseOSRelease_Unknown(t *testing.T) {
	// Given: An unknown OS
	content := `NAME="Custom Linux"
ID=custom
`

	// When: Parsing the content
	osType := parseOSRelease(content)

	// Then: Should return unknown
	assert.Equal(t, OSUnknown, osType)
}

func TestSELinuxEnabled(t *testing.T) {
	// When: Checking SELinux status
	enabled := IsSELinuxEnabled()

	// Then: Should return boolean (may be true or false depending on system)
	// Just verify it doesn't panic and returns a valid boolean
	t.Logf("SELinux enabled: %v", enabled)
}

func TestSELinuxEnabledForOS(t *testing.T) {
	// RHEL-based systems typically have SELinux
	assert.True(t, IsSELinuxEnabledByDefault(OSRHEL))
	assert.True(t, IsSELinuxEnabledByDefault(OSCoreOS))

	// Debian-based systems typically don't
	assert.False(t, IsSELinuxEnabledByDefault(OSUbuntu))
}

func TestLibraryPathForArch(t *testing.T) {
	tests := []struct {
		name     string
		osType   OSType
		arch     string
		expected string
	}{
		// Debian/Ubuntu tests
		{
			name:     "Debian x86_64 (amd64)",
			osType:   OSUbuntu,
			arch:     "amd64",
			expected: "/usr/lib/x86_64-linux-gnu",
		},
		{
			name:     "Debian aarch64 (arm64)",
			osType:   OSUbuntu,
			arch:     "arm64",
			expected: "/usr/lib/aarch64-linux-gnu",
		},
		{
			name:     "Debian i386",
			osType:   OSUbuntu,
			arch:     "386",
			expected: "/usr/lib/i386-linux-gnu",
		},
		{
			name:     "Debian unknown architecture",
			osType:   OSUbuntu,
			arch:     "unknown",
			expected: "/usr/lib",
		},
		{
			name:     "Debian empty architecture",
			osType:   OSUbuntu,
			arch:     "",
			expected: "/usr/lib",
		},
		// RHEL tests
		{
			name:     "RHEL x86_64 (amd64)",
			osType:   OSRHEL,
			arch:     "amd64",
			expected: "/usr/lib64",
		},
		{
			name:     "RHEL aarch64 (arm64)",
			osType:   OSRHEL,
			arch:     "arm64",
			expected: "/usr/lib64",
		},
		{
			name:     "RHEL unknown architecture",
			osType:   OSRHEL,
			arch:     "unknown",
			expected: "/usr/lib",
		},
		{
			name:     "RHEL empty architecture",
			osType:   OSRHEL,
			arch:     "",
			expected: "/usr/lib",
		},
		// CoreOS tests (RHEL-based)
		{
			name:     "CoreOS x86_64 (amd64)",
			osType:   OSCoreOS,
			arch:     "amd64",
			expected: "/usr/lib64",
		},
		{
			name:     "CoreOS aarch64 (arm64)",
			osType:   OSCoreOS,
			arch:     "arm64",
			expected: "/usr/lib64",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given: OS type and architecture
			// When: Getting library path for the architecture
			result := LibraryPathForArch(tt.osType, tt.arch)

			// Then: Should return expected path
			assert.Equal(t, tt.expected, result)
		})
	}
}
