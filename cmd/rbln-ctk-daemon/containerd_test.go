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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewContainerdCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newContainerdCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "containerd", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
	})

	t.Run("has setup and cleanup subcommands", func(t *testing.T) {
		// When
		cmd := newContainerdCmd()

		// Then
		subcommands := make(map[string]bool)
		for _, sub := range cmd.Commands() {
			subcommands[sub.Use] = true
		}
		assert.True(t, subcommands["setup"], "should have 'setup' subcommand")
		assert.True(t, subcommands["cleanup"], "should have 'cleanup' subcommand")
	})
}

func TestNewContainerdSetupCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newContainerdSetupCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "setup", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
	})

	t.Run("has RunE function", func(t *testing.T) {
		// When
		cmd := newContainerdSetupCmd()

		// Then
		assert.NotNil(t, cmd.RunE)
	})
}

func TestNewContainerdCleanupCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newContainerdCleanupCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "cleanup", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
	})

	t.Run("has RunE function", func(t *testing.T) {
		// When
		cmd := newContainerdCleanupCmd()

		// Then
		assert.NotNil(t, cmd.RunE)
	})
}

func TestContainerdCmdHierarchy(t *testing.T) {
	// Given
	runtimeCmd := newRuntimeCmd()

	// When
	var containerdCmd *struct{ setup, cleanup bool }
	for _, sub := range runtimeCmd.Commands() {
		if sub.Use == "containerd" {
			containerdCmd = &struct{ setup, cleanup bool }{}
			for _, subsub := range sub.Commands() {
				switch subsub.Use {
				case "setup":
					containerdCmd.setup = true
				case "cleanup":
					containerdCmd.cleanup = true
				}
			}
			break
		}
	}

	// Then
	assert.NotNil(t, containerdCmd, "containerd command should exist")
	assert.True(t, containerdCmd.setup, "containerd should have setup")
	assert.True(t, containerdCmd.cleanup, "containerd should have cleanup")
}
