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

package cdi

//go:generate moq -rm -fmt=goimports -stub -out writer_mock.go . Writer

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
	"tags.cncf.io/container-device-interface/specs-go"

	rbln_errors "github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// yamlMount is a wrapper for specs.Mount with proper yaml omitempty tags.
type yamlMount struct {
	HostPath      string   `yaml:"hostPath"`
	ContainerPath string   `yaml:"containerPath"`
	Options       []string `yaml:"options,omitempty,flow"`
	Type          string   `yaml:"type,omitempty"`
}

// yamlContainerEdits is a wrapper for specs.ContainerEdits with proper yaml omitempty tags.
type yamlContainerEdits struct {
	Env            []string     `yaml:"env,omitempty"`
	DeviceNodes    []any        `yaml:"deviceNodes,omitempty"`
	Hooks          []any        `yaml:"hooks,omitempty"`
	Mounts         []*yamlMount `yaml:"mounts,omitempty"`
	IntelRdt       any          `yaml:"intelRdt,omitempty"`
	AdditionalGIDs []uint32     `yaml:"additionalGids,omitempty"`
}

// yamlDevice is a wrapper for specs.Device with proper yaml omitempty tags.
type yamlDevice struct {
	Name           string             `yaml:"name"`
	Annotations    map[string]string  `yaml:"annotations,omitempty"`
	ContainerEdits yamlContainerEdits `yaml:"containerEdits"`
}

// yamlSpec is a wrapper for specs.Spec with proper yaml omitempty tags.
type yamlSpec struct {
	CDIVersion     string             `yaml:"cdiVersion"`
	Kind           string             `yaml:"kind"`
	Annotations    map[string]string  `yaml:"annotations,omitempty"`
	Devices        []yamlDevice       `yaml:"devices,omitempty"`
	ContainerEdits yamlContainerEdits `yaml:"containerEdits,omitempty"`
}

// toYAMLSpec converts a specs.Spec to yamlSpec for clean YAML output.
func toYAMLSpec(spec *specs.Spec) yamlSpec {
	ys := yamlSpec{
		CDIVersion:  spec.Version,
		Kind:        spec.Kind,
		Annotations: spec.Annotations,
	}

	// Convert devices
	for i := range spec.Devices {
		d := &spec.Devices[i]
		yd := yamlDevice{
			Name:        d.Name,
			Annotations: d.Annotations,
			ContainerEdits: yamlContainerEdits{
				Env: d.ContainerEdits.Env,
			},
		}

		// Convert mounts
		for _, m := range d.ContainerEdits.Mounts {
			ym := &yamlMount{
				HostPath:      m.HostPath,
				ContainerPath: m.ContainerPath,
				Options:       m.Options,
			}
			// Only set Type if non-empty
			if m.Type != "" {
				ym.Type = m.Type
			}
			yd.ContainerEdits.Mounts = append(yd.ContainerEdits.Mounts, ym)
		}

		// Convert hooks
		for _, h := range d.ContainerEdits.Hooks {
			yd.ContainerEdits.Hooks = append(yd.ContainerEdits.Hooks, h)
		}

		ys.Devices = append(ys.Devices, yd)
	}

	return ys
}

// Writer writes CDI specifications to files or stdout.
type Writer interface {
	// Write writes the spec to a file.
	Write(spec *specs.Spec, path string, format string) error

	// WriteToWriter writes the spec to an io.Writer.
	WriteToWriter(spec *specs.Spec, w io.Writer, format string) error
}

// writer implements Writer interface.
type writer struct{}

// NewWriter creates a new CDI writer.
func NewWriter() Writer {
	return &writer{}
}

// Write writes the spec to a file.
func (w *writer) Write(spec *specs.Spec, path, format string) error {
	if spec == nil {
		return fmt.Errorf("spec is nil: %w", rbln_errors.ErrInvalidCDISpec)
	}

	// Validate format
	if format != "yaml" && format != "json" {
		return fmt.Errorf("unsupported format: %s (expected yaml or json): %w", format, rbln_errors.ErrWriteFailed)
	}

	// Create directory if needed
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create directory %s: %v", dir, err)
	}

	// Create file
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("create file %s: %v", path, err)
	}
	defer f.Close()

	return w.WriteToWriter(spec, f, format)
}

// WriteToWriter writes the spec to an io.Writer.
func (w *writer) WriteToWriter(spec *specs.Spec, out io.Writer, format string) error {
	if spec == nil {
		return fmt.Errorf("spec is nil: %w", rbln_errors.ErrInvalidCDISpec)
	}

	var data []byte
	var err error

	switch format {
	case "yaml":
		// Use yamlSpec wrapper for clean output (omits empty fields like type: "")
		yamlSpec := toYAMLSpec(spec)
		data, err = yaml.Marshal(yamlSpec)
	case "json":
		data, err = json.MarshalIndent(spec, "", "  ")
	default:
		return fmt.Errorf("unsupported format: %s (expected yaml or json): %w", format, rbln_errors.ErrWriteFailed)
	}

	if err != nil {
		return fmt.Errorf("marshal spec: %w", err)
	}

	if _, err := out.Write(data); err != nil {
		return fmt.Errorf("write spec: %w", err)
	}

	return nil
}
