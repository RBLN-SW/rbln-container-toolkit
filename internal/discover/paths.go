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
	"os"
	"runtime"
	"strings"
)

// OSType represents the detected operating system.
type OSType string

const (
	OSUbuntu  OSType = "ubuntu"
	OSRHEL    OSType = "rhel"
	OSCoreOS  OSType = "coreos"
	OSUnknown OSType = "unknown"
)

// String returns the string representation of OSType.
func (o OSType) String() string {
	return string(o)
}

// IsDebian returns true if the OS is Debian-based (Ubuntu).
func (o OSType) IsDebian() bool {
	return o == OSUbuntu
}

// IsRHEL returns true if the OS is RHEL-based (RHEL, CoreOS).
func (o OSType) IsRHEL() bool {
	return o == OSRHEL || o == OSCoreOS
}

// DetectOS detects the current operating system from /etc/os-release.
func DetectOS() OSType {
	content, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return OSUnknown
	}
	return parseOSRelease(string(content))
}

// parseOSRelease parses the content of /etc/os-release and returns the OS type.
func parseOSRelease(content string) OSType {
	lines := strings.Split(content, "\n")
	idMap := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), "\"")
		idMap[key] = value
	}

	// Check ID first
	id := strings.ToLower(idMap["ID"])

	switch {
	case id == "ubuntu":
		return OSUbuntu
	case id == "rhel":
		return OSRHEL
	case id == "rhcos" || strings.Contains(strings.ToLower(idMap["NAME"]), "coreos"):
		return OSCoreOS
	}

	// Check ID_LIKE as fallback
	idLike := strings.ToLower(idMap["ID_LIKE"])
	switch {
	case strings.Contains(idLike, "debian") || strings.Contains(idLike, "ubuntu"):
		return OSUbuntu
	case strings.Contains(idLike, "rhel") || strings.Contains(idLike, "fedora") || strings.Contains(idLike, "centos"):
		return OSRHEL
	}

	return OSUnknown
}

// DefaultLibraryPaths returns the default library search paths for the given OS.
func DefaultLibraryPaths(osType OSType) []string {
	if osType.IsDebian() {
		return []string{
			LibraryPathForArch(osType, GetArchitecture()),
			"/usr/local/lib",
			"/usr/lib",
		}
	}

	// RHEL-based systems use lib64 for 64-bit libraries
	return []string{
		"/usr/lib64",
		"/usr/local/lib64",
		"/usr/lib",
		"/usr/local/lib",
	}
}

// DefaultBinaryPaths returns the default binary search paths.
func DefaultBinaryPaths(_ OSType) []string {
	return []string{
		"/usr/bin",
		"/usr/local/bin",
		"/usr/sbin",
		"/usr/local/sbin",
	}
}

// DefaultPluginPaths returns the default plugin library paths for the given OS.
func DefaultPluginPaths(osType OSType) []string {
	if osType.IsDebian() {
		arch := GetArchitecture()
		libPath := LibraryPathForArch(osType, arch)
		return []string{
			libPath + "/libibverbs",
		}
	}

	// RHEL-based
	return []string{
		"/usr/lib64/libibverbs",
	}
}

// GetArchitecture returns the current system architecture.
func GetArchitecture() string {
	return runtime.GOARCH
}

// LibraryPathForArch returns the library path for a specific architecture and OS.
func LibraryPathForArch(osType OSType, arch string) string {
	if osType.IsDebian() {
		// Debian uses multiarch directory structure
		switch arch {
		case "amd64":
			return "/usr/lib/x86_64-linux-gnu"
		case "arm64":
			return "/usr/lib/aarch64-linux-gnu"
		case "386":
			return "/usr/lib/i386-linux-gnu"
		default:
			return "/usr/lib"
		}
	}

	// RHEL-based systems
	switch arch {
	case "amd64":
		return "/usr/lib64"
	case "arm64":
		return "/usr/lib64"
	default:
		return "/usr/lib"
	}
}

// IsSELinuxEnabled checks if SELinux is currently enabled on the system.
func IsSELinuxEnabled() bool {
	// Check if SELinux filesystem is mounted
	_, err := os.Stat("/sys/fs/selinux")
	if err != nil {
		return false
	}

	// Check enforcing status
	content, err := os.ReadFile("/sys/fs/selinux/enforce")
	if err != nil {
		return false
	}

	return strings.TrimSpace(string(content)) == "1"
}

// IsSELinuxEnabledByDefault returns true if SELinux is typically enabled by default
// for the given OS type.
func IsSELinuxEnabledByDefault(osType OSType) bool {
	// RHEL-based systems have SELinux enabled by default
	return osType.IsRHEL()
}
