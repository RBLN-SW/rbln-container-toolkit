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

// Package output provides output formatting utilities.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"gopkg.in/yaml.v3"

	"github.com/RBLN-SW/rbln-container-toolkit/internal/discover"
)

// Formatter formats discovery results for output.
type Formatter struct {
	writer io.Writer
}

// NewFormatter creates a new output formatter.
func NewFormatter(w io.Writer) *Formatter {
	return &Formatter{writer: w}
}

// Format outputs the discovery result in the specified format.
func (f *Formatter) Format(result *discover.DiscoveryResult, format string) error {
	switch strings.ToLower(format) {
	case "table":
		return f.formatTable(result)
	case "json":
		return f.formatJSON(result)
	case "yaml":
		return f.formatYAML(result)
	default:
		return fmt.Errorf("unsupported format: %s (expected table, json, or yaml)", format)
	}
}

// formatTable outputs as human-readable table.
func (f *Formatter) formatTable(result *discover.DiscoveryResult) error {
	w := tabwriter.NewWriter(f.writer, 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "TYPE\tNAME\tPATH")

	if result != nil {
		for _, lib := range result.Libraries {
			fmt.Fprintf(w, "library\t%s\t%s\n", lib.Name, lib.Path)
		}
		for _, tool := range result.Tools {
			fmt.Fprintf(w, "tool\t%s\t%s\n", tool.Name, tool.Path)
		}
		for _, dev := range result.Devices {
			fmt.Fprintf(w, "device\t%s\t%s\n", filepath.Base(dev.Path), dev.Path)
		}
	}

	if result == nil || (len(result.Libraries) == 0 && len(result.Tools) == 0 && len(result.Devices) == 0) {
		fmt.Fprintln(w)
		fmt.Fprintln(f.writer, "No RBLN libraries or tools found. Ensure the RBLN driver is installed.")
	}

	return nil
}

// ListOutput is the JSON/YAML output structure.
type ListOutput struct {
	Libraries []LibraryOutput `json:"libraries" yaml:"libraries"`
	Tools     []ToolOutput    `json:"tools" yaml:"tools"`
	Devices   []DeviceOutput  `json:"devices" yaml:"devices"`
}

// LibraryOutput is the library output structure.
type LibraryOutput struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
	Type string `json:"type" yaml:"type"`
}

// ToolOutput is the tool output structure.
type ToolOutput struct {
	Name string `json:"name" yaml:"name"`
	Path string `json:"path" yaml:"path"`
}

// DeviceOutput is the device output structure.
type DeviceOutput struct {
	Path string `json:"path" yaml:"path"`
}

func (f *Formatter) toListOutput(result *discover.DiscoveryResult) ListOutput {
	output := ListOutput{
		Libraries: []LibraryOutput{},
		Tools:     []ToolOutput{},
		Devices:   []DeviceOutput{},
	}

	if result != nil {
		for _, lib := range result.Libraries {
			output.Libraries = append(output.Libraries, LibraryOutput{
				Name: lib.Name,
				Path: lib.Path,
				Type: lib.Type.String(),
			})
		}
		for _, tool := range result.Tools {
			output.Tools = append(output.Tools, ToolOutput{
				Name: tool.Name,
				Path: tool.Path,
			})
		}
		for _, dev := range result.Devices {
			output.Devices = append(output.Devices, DeviceOutput{
				Path: dev.Path,
			})
		}
	}

	return output
}

// formatJSON outputs as JSON.
func (f *Formatter) formatJSON(result *discover.DiscoveryResult) error {
	output := f.toListOutput(result)
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(f.writer, string(data))
	return err
}

// formatYAML outputs as YAML.
func (f *Formatter) formatYAML(result *discover.DiscoveryResult) error {
	output := f.toListOutput(result)
	data, err := yaml.Marshal(output)
	if err != nil {
		return err
	}
	_, err = f.writer.Write(data)
	return err
}
