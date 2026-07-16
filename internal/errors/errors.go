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

// Package errors defines error types for the RBLN Container Toolkit.
package errors

import "errors"

var (
	// Config errors
	ErrConfigNotFound = errors.New("configuration file not found")
	ErrInvalidConfig  = errors.New("invalid configuration")

	// Discovery errors
	ErrNoLibrariesFound   = errors.New("no RBLN libraries found")
	ErrNoToolsFound       = errors.New("no tools found")
	ErrLdcacheParseFailed = errors.New("failed to parse ldcache")
	ErrLddFailed          = errors.New("ldd execution failed")

	// CDI errors
	ErrInvalidCDISpec = errors.New("invalid CDI specification")
	ErrFileNotFound   = errors.New("file not found")
	ErrWriteFailed    = errors.New("failed to write file")

	// Runtime errors
	ErrRuntimeNotFound  = errors.New("runtime not found")
	ErrConfigureFailed  = errors.New("failed to configure runtime")
	ErrPermissionDenied = errors.New("permission denied")
)
