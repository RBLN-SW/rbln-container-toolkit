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

package installer

import (
	"fmt"
)

// ErrRestartFailed indicates the runtime restart failed.
type ErrRestartFailed struct {
	Runtime string
	Service string
	Cause   error
}

func (e *ErrRestartFailed) Error() string {
	return fmt.Sprintf("failed to restart %s: %v\n\nTo restart manually:\n  sudo systemctl restart %s",
		e.Runtime, e.Cause, e.Service)
}

func (e *ErrRestartFailed) Unwrap() error {
	return e.Cause
}

// ErrConfigFailed indicates runtime configuration failed.
type ErrConfigFailed struct {
	Runtime string
	Path    string
	Cause   error
}

func (e *ErrConfigFailed) Error() string {
	return fmt.Sprintf("failed to configure %s at %s: %v", e.Runtime, e.Path, e.Cause)
}

func (e *ErrConfigFailed) Unwrap() error {
	return e.Cause
}

// ErrCDIGenerateFailed indicates CDI spec generation failed.
type ErrCDIGenerateFailed struct {
	Path  string
	Cause error
}

func (e *ErrCDIGenerateFailed) Error() string {
	return fmt.Sprintf("failed to generate CDI spec at %s: %v", e.Path, e.Cause)
}

func (e *ErrCDIGenerateFailed) Unwrap() error {
	return e.Cause
}

// ErrPermissionDenied indicates insufficient permissions.
type ErrPermissionDenied struct {
	Operation string
	Path      string
	Cause     error
}

func (e *ErrPermissionDenied) Error() string {
	return fmt.Sprintf("permission denied: %s at %s (try sudo)\n%v", e.Operation, e.Path, e.Cause)
}

func (e *ErrPermissionDenied) Unwrap() error {
	return e.Cause
}

// ErrSocketNotFound indicates the runtime socket was not found.
type ErrSocketNotFound struct {
	Socket  string
	Runtime string
}

func (e *ErrSocketNotFound) Error() string {
	return fmt.Sprintf("socket not found: %s\n\nIs %s running?", e.Socket, e.Runtime)
}

// ErrUnsupportedMode indicates an unsupported restart mode for a runtime.
type ErrUnsupportedMode struct {
	Runtime string
	Mode    string
}

func (e *ErrUnsupportedMode) Error() string {
	return fmt.Sprintf("restart mode '%s' is not supported for %s", e.Mode, e.Runtime)
}
