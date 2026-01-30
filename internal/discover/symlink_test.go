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
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLinkExists(t *testing.T) {
	t.Run("existing symlink returns true", func(t *testing.T) {
		// Given
		tempDir := t.TempDir()
		target := filepath.Join(tempDir, "target.txt")
		link := filepath.Join(tempDir, "link.txt")

		require.NoError(t, os.WriteFile(target, []byte("content"), 0644))
		require.NoError(t, os.Symlink(target, link))

		// When
		exists, err := LinkExists(link)

		// Then
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("non-existent path returns false", func(t *testing.T) {
		// Given
		tempDir := t.TempDir()
		nonExistent := filepath.Join(tempDir, "does-not-exist")

		// When
		exists, err := LinkExists(nonExistent)

		// Then
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("regular file returns false", func(t *testing.T) {
		// Given
		tempDir := t.TempDir()
		regularFile := filepath.Join(tempDir, "regular.txt")
		require.NoError(t, os.WriteFile(regularFile, []byte("content"), 0644))

		// When
		exists, err := LinkExists(regularFile)

		// Then
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("broken symlink returns true", func(t *testing.T) {
		// Given - symlink exists but target doesn't
		tempDir := t.TempDir()
		nonExistentTarget := filepath.Join(tempDir, "missing-target.txt")
		brokenLink := filepath.Join(tempDir, "broken-link.txt")
		require.NoError(t, os.Symlink(nonExistentTarget, brokenLink))

		// When
		exists, err := LinkExists(brokenLink)

		// Then - symlink itself exists, even though target doesn't
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("directory returns false", func(t *testing.T) {
		// Given
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		// When
		exists, err := LinkExists(subDir)

		// Then
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("symlink to directory returns true", func(t *testing.T) {
		// Given
		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		linkToDir := filepath.Join(tempDir, "link-to-dir")
		require.NoError(t, os.Mkdir(subDir, 0755))
		require.NoError(t, os.Symlink(subDir, linkToDir))

		// When
		exists, err := LinkExists(linkToDir)

		// Then
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}

func TestLinkExists_FunctionVariable(t *testing.T) {
	t.Run("can be overridden for testing", func(t *testing.T) {
		// Given
		original := LinkExists
		defer func() { LinkExists = original }()

		LinkExists = func(_ string) (bool, error) {
			return true, nil
		}

		// When
		exists, err := LinkExists("/any/path")

		// Then
		assert.NoError(t, err)
		assert.True(t, exists)
	})
}
