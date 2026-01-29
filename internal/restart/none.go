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

// NoneRestarter is a no-op restarter that skips restart.
type NoneRestarter struct{}

// newNoneRestarter creates a new NoneRestarter.
func newNoneRestarter() *NoneRestarter {
	return &NoneRestarter{}
}

// Restart does nothing and returns nil.
// A warning should be logged by the caller.
func (r *NoneRestarter) Restart(_ string) error {
	// No-op: restart is intentionally skipped
	return nil
}

// DryRun returns a description of what would happen.
func (r *NoneRestarter) DryRun(runtime string) string {
	return fmt.Sprintf("Would skip restart for %s (restart-mode=none)", runtime)
}

// SkipMessage returns a warning message about restart being skipped.
func (r *NoneRestarter) SkipMessage(runtime string) string {
	defaults := GetRuntimeDefaults(runtime)
	return fmt.Sprintf("Restart skipped. To apply changes, manually restart %s:\n  sudo systemctl restart %s",
		runtime, defaults.Service)
}
