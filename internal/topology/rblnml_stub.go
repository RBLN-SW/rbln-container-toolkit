//go:build !with_rblnml

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

package topology

import "errors"

// ErrRblnmlUnavailable is returned by LoadRblnmlResolver in builds compiled
// without the with_rblnml tag. It lets callers branch on the cause and pick
// the right fallback (typically NoopResolver{} with a logged warning).
var ErrRblnmlUnavailable = errors.New(
	"rblnml resolver unavailable: rebuild with -tags with_rblnml and librbln-ml installed")

// LoadRblnmlResolver is the no-op stub used by pure-Go builds (default).
// It always returns ErrRblnmlUnavailable so production callers fall back to
// NoopResolver{} and emit a warning instead of silently dropping RSD
// auto-attachment.
func LoadRblnmlResolver() (RsdResolver, error) {
	return nil, ErrRblnmlUnavailable
}
