//go:build !linux

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

// Package restart provides runtime restart functionality.
package restart

import (
	"fmt"
)

// SignalRestarter is a stub for non-Linux platforms.
type SignalRestarter struct{}

// newSignalRestarter returns an error on non-Linux platforms.
func newSignalRestarter(_ Options) (*SignalRestarter, error) {
	return nil, fmt.Errorf("signal restart mode is not supported on this platform")
}

// Restart returns an error on non-Linux platforms.
func (r *SignalRestarter) Restart(_ string) error {
	return fmt.Errorf("signal restart mode is not supported on this platform")
}

// DryRun returns a description of what would happen.
func (r *SignalRestarter) DryRun(_ string) string {
	return "Signal restart mode is not supported on this platform"
}
