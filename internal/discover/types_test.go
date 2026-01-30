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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLibraryType_String(t *testing.T) {
	tests := []struct {
		name     string
		libType  LibraryType
		expected string
	}{
		{"RBLN type", LibraryTypeRBLN, "rbln"},
		{"Dependency type", LibraryTypeDependency, "dependency"},
		{"Plugin type", LibraryTypePlugin, "plugin"},
		{"Unknown type", LibraryType(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When
			result := tt.libType.String()

			// Then
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLibraryType_Constants(t *testing.T) {
	// Given: LibraryType constants defined

	// When: Checking their values
	// Then: They should be distinct iota values
	assert.Equal(t, LibraryType(0), LibraryTypeRBLN)
	assert.Equal(t, LibraryType(1), LibraryTypeDependency)
	assert.Equal(t, LibraryType(2), LibraryTypePlugin)
}

func TestLibrary_Struct(t *testing.T) {
	// Given: A Library struct
	lib := Library{
		Name:          "librbln-ml.so",
		Path:          "/usr/lib64/librbln-ml.so",
		ContainerPath: "/rbln/lib64/librbln-ml.so",
		RealPath:      "/usr/lib64/librbln-ml.so.1.0.0",
		Type:          LibraryTypeRBLN,
	}

	// When: Accessing fields
	// Then: All fields should be correct
	assert.Equal(t, "librbln-ml.so", lib.Name)
	assert.Equal(t, "/usr/lib64/librbln-ml.so", lib.Path)
	assert.Equal(t, "/rbln/lib64/librbln-ml.so", lib.ContainerPath)
	assert.Equal(t, "/usr/lib64/librbln-ml.so.1.0.0", lib.RealPath)
	assert.Equal(t, LibraryTypeRBLN, lib.Type)
}

func TestTool_Struct(t *testing.T) {
	// Given: A Tool struct
	tool := Tool{
		Name:          "rbln-smi",
		Path:          "/usr/bin/rbln-smi",
		ContainerPath: "/usr/bin/rbln-smi",
	}

	// When: Accessing fields
	// Then: All fields should be correct
	assert.Equal(t, "rbln-smi", tool.Name)
	assert.Equal(t, "/usr/bin/rbln-smi", tool.Path)
	assert.Equal(t, "/usr/bin/rbln-smi", tool.ContainerPath)
}

func TestDiscoveryResult_Struct(t *testing.T) {
	// Given: A DiscoveryResult with libraries and tools
	result := DiscoveryResult{
		Libraries: []Library{
			{Name: "librbln-ml.so", Type: LibraryTypeRBLN},
			{Name: "libdep.so", Type: LibraryTypeDependency},
		},
		Tools: []Tool{
			{Name: "rbln-smi"},
		},
	}

	// When: Accessing fields
	// Then: Should contain correct counts
	assert.Len(t, result.Libraries, 2)
	assert.Len(t, result.Tools, 1)
}
