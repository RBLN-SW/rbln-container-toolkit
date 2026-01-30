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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrRestartFailed_Error(t *testing.T) {
	// Given
	cause := errors.New("connection refused")
	err := &ErrRestartFailed{
		Runtime: "docker",
		Service: "docker",
		Cause:   cause,
	}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "docker")
	assert.Contains(t, msg, "connection refused")
	assert.Contains(t, msg, "systemctl restart docker")
}

func TestErrRestartFailed_Unwrap(t *testing.T) {
	// Given
	cause := errors.New("original error")
	err := &ErrRestartFailed{
		Runtime: "containerd",
		Service: "containerd",
		Cause:   cause,
	}

	// When
	unwrapped := err.Unwrap()

	// Then
	assert.Equal(t, cause, unwrapped)
}

func TestErrRestartFailed_ErrorsIs(t *testing.T) {
	// Given
	cause := errors.New("timeout")
	err := &ErrRestartFailed{
		Runtime: "crio",
		Service: "crio",
		Cause:   cause,
	}

	// When
	// Then
	assert.True(t, errors.Is(err, cause))
}

func TestErrConfigFailed_Error(t *testing.T) {
	// Given
	cause := errors.New("parse error")
	err := &ErrConfigFailed{
		Runtime: "docker",
		Path:    "/etc/docker/daemon.json",
		Cause:   cause,
	}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "docker")
	assert.Contains(t, msg, "/etc/docker/daemon.json")
	assert.Contains(t, msg, "parse error")
}

func TestErrConfigFailed_Unwrap(t *testing.T) {
	// Given
	cause := errors.New("original error")
	err := &ErrConfigFailed{
		Runtime: "containerd",
		Path:    "/etc/containerd/config.toml",
		Cause:   cause,
	}

	// When
	unwrapped := err.Unwrap()

	// Then
	assert.Equal(t, cause, unwrapped)
}

func TestErrCDIGenerateFailed_Error(t *testing.T) {
	// Given
	cause := errors.New("no devices found")
	err := &ErrCDIGenerateFailed{
		Path:  "/var/run/cdi/rbln.yaml",
		Cause: cause,
	}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "/var/run/cdi/rbln.yaml")
	assert.Contains(t, msg, "no devices found")
}

func TestErrCDIGenerateFailed_Unwrap(t *testing.T) {
	// Given
	cause := errors.New("original error")
	err := &ErrCDIGenerateFailed{
		Path:  "/var/run/cdi/rbln.yaml",
		Cause: cause,
	}

	// When
	unwrapped := err.Unwrap()

	// Then
	assert.Equal(t, cause, unwrapped)
}

func TestErrPermissionDenied_Error(t *testing.T) {
	// Given
	cause := errors.New("operation not permitted")
	err := &ErrPermissionDenied{
		Operation: "write",
		Path:      "/etc/docker/daemon.json",
		Cause:     cause,
	}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "permission denied")
	assert.Contains(t, msg, "write")
	assert.Contains(t, msg, "/etc/docker/daemon.json")
	assert.Contains(t, msg, "sudo")
}

func TestErrPermissionDenied_Unwrap(t *testing.T) {
	// Given
	cause := errors.New("EPERM")
	err := &ErrPermissionDenied{
		Operation: "read",
		Path:      "/etc/config",
		Cause:     cause,
	}

	// When
	unwrapped := err.Unwrap()

	// Then
	assert.Equal(t, cause, unwrapped)
}

func TestErrSocketNotFound_Error(t *testing.T) {
	// Given
	err := &ErrSocketNotFound{
		Socket:  "/var/run/docker.sock",
		Runtime: "docker",
	}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "/var/run/docker.sock")
	assert.Contains(t, msg, "docker")
	assert.Contains(t, msg, "running")
}

func TestErrUnsupportedMode_Error(t *testing.T) {
	// Given
	err := &ErrUnsupportedMode{
		Runtime: "crio",
		Mode:    "signal",
	}

	// When
	msg := err.Error()

	// Then
	assert.Contains(t, msg, "signal")
	assert.Contains(t, msg, "crio")
	assert.Contains(t, msg, "not supported")
}

func TestErrorTypes_AreDistinct(t *testing.T) {
	// Given
	cause := errors.New("test")
	errs := []error{
		&ErrRestartFailed{Runtime: "docker", Service: "docker", Cause: cause},
		&ErrConfigFailed{Runtime: "docker", Path: "/test", Cause: cause},
		&ErrCDIGenerateFailed{Path: "/test", Cause: cause},
		&ErrPermissionDenied{Operation: "write", Path: "/test", Cause: cause},
		&ErrSocketNotFound{Socket: "/test.sock", Runtime: "docker"},
		&ErrUnsupportedMode{Runtime: "crio", Mode: "signal"},
	}

	// When
	// Then
	for i, err := range errs {
		_, isRestart := err.(*ErrRestartFailed)
		_, isConfig := err.(*ErrConfigFailed)
		_, isCDI := err.(*ErrCDIGenerateFailed)
		_, isPermission := err.(*ErrPermissionDenied)
		_, isSocket := err.(*ErrSocketNotFound)
		_, isUnsupported := err.(*ErrUnsupportedMode)

		count := 0
		if isRestart {
			count++
		}
		if isConfig {
			count++
		}
		if isCDI {
			count++
		}
		if isPermission {
			count++
		}
		if isSocket {
			count++
		}
		if isUnsupported {
			count++
		}

		assert.Equal(t, 1, count, "Error at index %d should match exactly 1 type", i)
	}
}
