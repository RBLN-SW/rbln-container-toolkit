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

//go:generate moq -rm -fmt=goimports -stub -out validator_mock.go . Validator

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"tags.cncf.io/container-device-interface/specs-go"

	rbln_errors "github.com/RBLN-SW/rbln-container-toolkit/internal/errors"
)

// Validator validates CDI specifications.
type Validator interface {
	// Validate validates a CDI spec in memory.
	Validate(spec *specs.Spec) error

	// ValidateFile validates a CDI spec from a file.
	ValidateFile(path string) error
}

// validator implements Validator interface.
type validator struct{}

// NewValidator creates a new CDI validator.
func NewValidator() Validator {
	return &validator{}
}

// Validate validates a CDI spec in memory.
func (v *validator) Validate(spec *specs.Spec) error {
	if spec == nil {
		return fmt.Errorf("spec is nil: %w", rbln_errors.ErrInvalidCDISpec)
	}

	// Check version
	if spec.Version == "" {
		return fmt.Errorf("missing cdiVersion: %w", rbln_errors.ErrInvalidCDISpec)
	}

	// Check kind format (vendor/class)
	if !strings.Contains(spec.Kind, "/") {
		return fmt.Errorf("invalid kind format (expected vendor/class): %w", rbln_errors.ErrInvalidCDISpec)
	}

	// Check devices have names
	for i := range spec.Devices {
		if spec.Devices[i].Name == "" {
			return fmt.Errorf("device %d has empty name: %w", i, rbln_errors.ErrInvalidCDISpec)
		}
	}

	return nil
}

// ValidateFile validates a CDI spec from a file.
func (v *validator) ValidateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("spec file not found: %s: %w", path, rbln_errors.ErrFileNotFound)
		}
		return fmt.Errorf("read spec file: %w", err)
	}

	var spec specs.Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return fmt.Errorf("parse spec file: %w", rbln_errors.ErrInvalidCDISpec)
	}

	return v.Validate(&spec)
}
