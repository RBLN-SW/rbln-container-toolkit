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

package ldconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewRunner(t *testing.T) {
	tests := []struct {
		name          string
		ldconfigPath  string
		containerRoot string
		directories   []string
		wantErr       bool
		errContains   string
	}{
		{
			name:          "Given valid arguments / When NewRunner called / Then returns cmd",
			ldconfigPath:  "/sbin/ldconfig",
			containerRoot: "/var/lib/containers/rootfs",
			directories:   []string{"/usr/lib64/rbln"},
			wantErr:       false,
		},
		{
			name:          "Given multiple directories / When NewRunner called / Then returns cmd",
			ldconfigPath:  "/sbin/ldconfig",
			containerRoot: "/tmp/test-root",
			directories:   []string{"/usr/lib64/rbln", "/usr/lib64/rbln/libibverbs"},
			wantErr:       false,
		},
		{
			name:          "Given empty container root / When NewRunner called / Then returns error",
			ldconfigPath:  "/sbin/ldconfig",
			containerRoot: "",
			directories:   []string{"/usr/lib64/rbln"},
			wantErr:       true,
			errContains:   "container root must be specified",
		},
		{
			name:          "Given system root / When NewRunner called / Then returns error",
			ldconfigPath:  "/sbin/ldconfig",
			containerRoot: "/",
			directories:   []string{"/usr/lib64/rbln"},
			wantErr:       true,
			errContains:   "not be the system root",
		},
		{
			name:          "Given no directories / When NewRunner called / Then returns cmd",
			ldconfigPath:  "/sbin/ldconfig",
			containerRoot: "/tmp/test-root",
			directories:   nil,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			ldconfigPath := tt.ldconfigPath
			containerRoot := tt.containerRoot
			directories := tt.directories

			// When
			cmd, err := NewRunner(ldconfigPath, containerRoot, directories...)

			// Then
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if cmd == nil {
				t.Error("expected cmd but got nil")
			}
		})
	}
}

func TestWriteDirectories(t *testing.T) {
	tests := []struct {
		name string
		dirs []string
		want string
	}{
		{
			name: "single directory",
			dirs: []string{"/usr/lib64/rbln"},
			want: "/usr/lib64/rbln\n",
		},
		{
			name: "multiple directories",
			dirs: []string{"/usr/lib64/rbln", "/usr/lib64/rbln/libibverbs"},
			want: "/usr/lib64/rbln\n/usr/lib64/rbln/libibverbs\n",
		},
		{
			name: "duplicate directories",
			dirs: []string{"/usr/lib64/rbln", "/usr/lib64/rbln", "/usr/lib64/other"},
			want: "/usr/lib64/rbln\n/usr/lib64/other\n",
		},
		{
			name: "empty directories",
			dirs: []string{},
			want: "",
		},
		{
			name: "nil directories",
			dirs: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			var buf strings.Builder
			dirs := tt.dirs

			// When
			err := writeDirectories(&buf, dirs...)

			// Then
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if buf.String() != tt.want {
				t.Errorf("writeDirectories() = %q, want %q", buf.String(), tt.want)
			}
		})
	}
}

func TestLdconfig_createLdsoconfdFile(t *testing.T) {
	t.Run("creates config file with directories", func(t *testing.T) {
		// Given
		tmpDir, err := os.MkdirTemp("", "ldconfig-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		ldsoconfdDir := filepath.Join(tmpDir, "etc", "ld.so.conf.d")
		if mkdirErr := os.MkdirAll(ldsoconfdDir, 0755); mkdirErr != nil {
			t.Fatalf("failed to create ldsoconfd dir: %v", mkdirErr)
		}

		l := &Ldconfig{
			ContainerRoot: tmpDir,
			Directories:   []string{"/usr/lib64/rbln", "/usr/lib64/rbln/libibverbs"},
		}

		// When
		err = l.createLdsoconfdFile(ldsoconfdDir)

		// Then
		if err != nil {
			t.Errorf("createLdsoconfdFile() error = %v", err)
			return
		}

		files, err := filepath.Glob(filepath.Join(ldsoconfdDir, "00-rbln-*.conf"))
		if err != nil {
			t.Errorf("failed to glob files: %v", err)
			return
		}

		if len(files) == 0 {
			t.Error("expected config file to be created")
		}
	})

	t.Run("no file created with empty directories", func(t *testing.T) {
		// Given
		tmpDir, err := os.MkdirTemp("", "ldconfig-test-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		ldsoconfdDir := filepath.Join(tmpDir, "etc", "ld.so.conf.d")
		if mkdirErr := os.MkdirAll(ldsoconfdDir, 0755); mkdirErr != nil {
			t.Fatalf("failed to create ldsoconfd dir: %v", mkdirErr)
		}

		l := &Ldconfig{
			ContainerRoot: tmpDir,
			Directories:   []string{},
		}

		// When
		err = l.createLdsoconfdFile(ldsoconfdDir)

		// Then
		if err != nil {
			t.Errorf("createLdsoconfdFile() error = %v", err)
			return
		}

		files, _ := filepath.Glob(filepath.Join(ldsoconfdDir, "00-rbln-*.conf"))
		if len(files) > 0 {
			t.Error("expected no config file with empty directories")
		}
	})
}

func TestLdconfig_runLdconfig_BinaryNotFound(t *testing.T) {
	t.Run("returns error for non-existent ldconfig", func(t *testing.T) {
		// Given
		tmpDir, err := os.MkdirTemp("", "ldconfig-notfound-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		l := &Ldconfig{
			LdconfigPath:  "/nonexistent/ldconfig",
			ContainerRoot: tmpDir,
			Directories:   []string{"/usr/lib64/rbln"},
		}

		// When
		err = l.runLdconfig()

		// Then
		if err == nil {
			t.Error("expected error for non-existent ldconfig binary")
			return
		}

		if !strings.Contains(err.Error(), "ldconfig binary not found") {
			t.Errorf("error = %q, want to contain 'ldconfig binary not found'", err.Error())
		}
	})
}

func TestLdconfig_runLdconfig_ExecutionFailure(t *testing.T) {
	t.Run("returns error for ldconfig execution failure", func(t *testing.T) {
		// Given
		tmpDir, err := os.MkdirTemp("", "ldconfig-execfail-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		fakeLdconfig := "/bin/false"
		if _, statErr := os.Stat(fakeLdconfig); os.IsNotExist(statErr) {
			t.Skip("skipping test: /bin/false not available")
		}

		l := &Ldconfig{
			LdconfigPath:  fakeLdconfig,
			ContainerRoot: tmpDir,
			Directories:   []string{"/usr/lib64/rbln"},
		}

		// When
		err = l.runLdconfig()

		// Then
		if err == nil {
			t.Error("expected error for ldconfig execution failure")
			return
		}

		if !strings.Contains(err.Error(), "ldconfig failed") {
			t.Errorf("error = %q, want to contain 'ldconfig failed'", err.Error())
		}
	})
}

func TestHasPrefix(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		prefix string
		want   bool
	}{
		{
			name:   "string with prefix",
			s:      "--flag",
			prefix: "--",
			want:   true,
		},
		{
			name:   "string with single dash for double dash",
			s:      "-flag",
			prefix: "--",
			want:   false,
		},
		{
			name:   "string without prefix",
			s:      "flag",
			prefix: "--",
			want:   false,
		},
		{
			name:   "empty string",
			s:      "",
			prefix: "--",
			want:   false,
		},
		{
			name:   "string equal to prefix",
			s:      "--",
			prefix: "--",
			want:   true,
		},
		{
			name:   "string longer than prefix",
			s:      "---",
			prefix: "--",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given
			s := tt.s
			prefix := tt.prefix

			// When
			got := hasPrefix(s, prefix)

			// Then
			if got != tt.want {
				t.Errorf("hasPrefix(%q, %q) = %v, want %v", s, prefix, got, tt.want)
			}
		})
	}
}

func TestRunLdconfigUpdate(t *testing.T) {
	t.Run("returns error for insufficient arguments", func(t *testing.T) {
		// Given
		args := []string{"cmd", "--ldconfig-path", "/sbin/ldconfig"}

		// When
		err := runLdconfigUpdate(args)

		// Then
		if err == nil {
			t.Error("expected error for insufficient arguments")
			return
		}

		if !strings.Contains(err.Error(), "insufficient arguments") {
			t.Errorf("error = %q, want to contain 'insufficient arguments'", err.Error())
		}
	})

	t.Run("returns error for missing ldconfig path value", func(t *testing.T) {
		// Given
		args := []string{"cmd", "--ldconfig-path", "--container-root", "/tmp/root", "/dir1"}

		// When
		err := runLdconfigUpdate(args)

		// Then
		if err == nil {
			t.Error("expected error for missing ldconfig path value")
		}
	})

	t.Run("returns error for missing container root value", func(t *testing.T) {
		// Given
		args := []string{"cmd", "--ldconfig-path", "/sbin/ldconfig", "--container-root"}

		// When
		err := runLdconfigUpdate(args)

		// Then
		if err == nil {
			t.Error("expected error for missing container root value")
		}
	})

	t.Run("returns error for empty ldconfig path", func(t *testing.T) {
		// Given
		args := []string{"cmd", "--container-root", "/tmp/root", "/dir1", "/dir2"}

		// When
		err := runLdconfigUpdate(args)

		// Then
		if err == nil {
			t.Error("expected error for empty ldconfig path")
			return
		}

		if !strings.Contains(err.Error(), "ldconfig path must be specified") {
			t.Errorf("error = %q, want to contain 'ldconfig path must be specified'", err.Error())
		}
	})

	t.Run("returns error for system root as container root", func(t *testing.T) {
		// Given
		args := []string{"cmd", "--ldconfig-path", "/sbin/ldconfig", "--container-root", "/", "/dir1"}

		// When
		err := runLdconfigUpdate(args)

		// Then
		if err == nil {
			t.Error("expected error for system root")
			return
		}

		if !strings.Contains(err.Error(), "not be the system root") {
			t.Errorf("error = %q, want to contain 'not be the system root'", err.Error())
		}
	})
}

func TestLdconfig_UpdateLDCache(t *testing.T) {
	t.Run("returns error for non-existent ldconfig", func(t *testing.T) {
		// Given
		tmpDir, err := os.MkdirTemp("", "ldconfig-update-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		l := &Ldconfig{
			LdconfigPath:  "/nonexistent/ldconfig",
			ContainerRoot: tmpDir,
			Directories:   []string{"/usr/lib64/rbln"},
		}

		// When
		err = l.UpdateLDCache()

		// Then
		if err == nil {
			t.Error("expected error for non-existent ldconfig")
			return
		}

		if !strings.Contains(err.Error(), "ldconfig binary not found") {
			t.Errorf("error = %q, want to contain 'ldconfig binary not found'", err.Error())
		}
	})

	t.Run("creates ld.so.conf.d directory", func(t *testing.T) {
		// Given
		tmpDir, err := os.MkdirTemp("", "ldconfig-update-*")
		if err != nil {
			t.Fatalf("failed to create temp dir: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		l := &Ldconfig{
			LdconfigPath:  "/nonexistent/ldconfig",
			ContainerRoot: tmpDir,
			Directories:   []string{"/usr/lib64/rbln"},
		}

		// When
		_ = l.UpdateLDCache()

		// Then
		ldsoconfdDir := filepath.Join(tmpDir, "etc", "ld.so.conf.d")
		if _, err := os.Stat(ldsoconfdDir); os.IsNotExist(err) {
			t.Error("expected ld.so.conf.d directory to be created")
		}
	})
}

func TestLdconfigStruct(t *testing.T) {
	t.Run("returns expected values for struct fields", func(t *testing.T) {
		// Given
		l := &Ldconfig{
			LdconfigPath:  "/sbin/ldconfig",
			ContainerRoot: "/var/lib/containers/rootfs",
			Directories:   []string{"/usr/lib64/rbln", "/usr/lib64/other"},
		}

		// When
		ldconfigPath := l.LdconfigPath
		containerRoot := l.ContainerRoot
		directoriesLen := len(l.Directories)

		// Then
		if ldconfigPath != "/sbin/ldconfig" {
			t.Errorf("LdconfigPath = %q, want /sbin/ldconfig", ldconfigPath)
		}

		if containerRoot != "/var/lib/containers/rootfs" {
			t.Errorf("ContainerRoot = %q, want /var/lib/containers/rootfs", containerRoot)
		}

		if directoriesLen != 2 {
			t.Errorf("Directories length = %d, want 2", directoriesLen)
		}
	})
}
