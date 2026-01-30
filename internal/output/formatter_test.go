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

package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

func TestNewFormatter(t *testing.T) {
	// Given: A buffer
	var buf bytes.Buffer

	// When: Creating a formatter
	formatter := NewFormatter(&buf)

	// Then: Should create formatter
	assert.NotNil(t, formatter)
}

func TestFormatter_Format_Table(t *testing.T) {
	// Given: A discovery result
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
			{Name: "libibverbs.so.1", Path: "/usr/lib64/libibverbs.so.1", Type: discover.LibraryTypePlugin},
		},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi"},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting as table
	err := formatter.Format(result, "table")

	// Then: Should format successfully
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "PATH")
	assert.Contains(t, output, "librbln-ml.so")
	assert.Contains(t, output, "library")
	assert.Contains(t, output, "tool")
	assert.Contains(t, output, "rbln-smi")
}

func TestFormatter_Format_JSON(t *testing.T) {
	// Given: A discovery result
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi"},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting as JSON
	err := formatter.Format(result, "json")

	// Then: Should format as valid JSON
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Contains(t, parsed, "libraries")
	assert.Contains(t, parsed, "tools")
}

func TestFormatter_Format_YAML(t *testing.T) {
	// Given: A discovery result
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
		},
		Tools: []discover.Tool{
			{Name: "rbln-smi", Path: "/usr/bin/rbln-smi"},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting as YAML
	err := formatter.Format(result, "yaml")

	// Then: Should format as valid YAML
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Contains(t, parsed, "libraries")
	assert.Contains(t, parsed, "tools")
}

func TestFormatter_Format_EmptyResult(t *testing.T) {
	// Given: An empty discovery result
	result := &discover.DiscoveryResult{}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting as table
	err := formatter.Format(result, "table")

	// Then: Should handle empty result (just show header)
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "NAME")
	assert.Contains(t, output, "PATH")
}

func TestFormatter_Format_InvalidFormat(t *testing.T) {
	// Given: A discovery result
	result := &discover.DiscoveryResult{}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting with invalid format
	err := formatter.Format(result, "invalid")

	// Then: Should return error
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
}

func TestFormatter_Format_NilResult(t *testing.T) {
	// Given
	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When
	err := formatter.Format(nil, "table")

	// Then
	require.NoError(t, err)
}

func TestFormatter_Format_JSON_LibraryTypes(t *testing.T) {
	// Given: Libraries of different types
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
			{Name: "libdep.so", Path: "/usr/lib64/libdep.so", Type: discover.LibraryTypeDependency},
			{Name: "libibverbs.so", Path: "/usr/lib64/libibverbs.so", Type: discover.LibraryTypePlugin},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting as JSON
	err := formatter.Format(result, "json")

	// Then: Should show library types in JSON
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, `"type": "rbln"`)
	assert.Contains(t, output, `"type": "dependency"`)
	assert.Contains(t, output, `"type": "plugin"`)
}

func TestFormatter_Format_YAML_LibraryTypes(t *testing.T) {
	// Given: Libraries of different types
	result := &discover.DiscoveryResult{
		Libraries: []discover.Library{
			{Name: "librbln-ml.so", Path: "/usr/lib64/librbln-ml.so", Type: discover.LibraryTypeRBLN},
			{Name: "libdep.so", Path: "/usr/lib64/libdep.so", Type: discover.LibraryTypeDependency},
		},
	}

	var buf bytes.Buffer
	formatter := NewFormatter(&buf)

	// When: Formatting as YAML
	err := formatter.Format(result, "yaml")

	// Then: Should show library types
	require.NoError(t, err)
	output := buf.String()
	assert.Contains(t, output, "type: rbln")
	assert.Contains(t, output, "type: dependency")
}
