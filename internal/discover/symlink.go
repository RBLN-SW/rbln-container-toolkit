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

package discover

import (
	"os"
)

// LinkExists checks if the specified path is a symlink.
// Returns (true, nil) if path is a symlink (even if broken/dangling).
// Returns (false, nil) if path doesn't exist or is not a symlink.
// Returns (false, error) for other filesystem errors.
//
// We use a function variable to allow overriding for testing.
var LinkExists = func(linkPath string) (bool, error) {
	info, err := os.Lstat(linkPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// Check if the path is a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		return true, nil
	}
	return false, nil
}
