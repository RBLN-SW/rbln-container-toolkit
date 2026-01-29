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

func TestNewDockerCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newDockerCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "docker", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
	})

	t.Run("has setup and cleanup subcommands", func(t *testing.T) {
		// When
		cmd := newDockerCmd()

		// Then
		subcommands := make(map[string]bool)
		for _, sub := range cmd.Commands() {
			subcommands[sub.Use] = true
		}
		assert.True(t, subcommands["setup"], "should have 'setup' subcommand")
		assert.True(t, subcommands["cleanup"], "should have 'cleanup' subcommand")
	})
}

func TestNewDockerSetupCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newDockerSetupCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "setup", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
	})

	t.Run("has RunE function", func(t *testing.T) {
		// When
		cmd := newDockerSetupCmd()

		// Then
		assert.NotNil(t, cmd.RunE)
	})
}

func TestNewDockerCleanupCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newDockerCleanupCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "cleanup", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
	})

	t.Run("has RunE function", func(t *testing.T) {
		// When
		cmd := newDockerCleanupCmd()

		// Then
		assert.NotNil(t, cmd.RunE)
	})
}

func TestDockerCmdHierarchy(t *testing.T) {
	// Given
	runtimeCmd := newRuntimeCmd()

	// When
	var dockerCmd *struct{ setup, cleanup bool }
	for _, sub := range runtimeCmd.Commands() {
		if sub.Use == "docker" {
			dockerCmd = &struct{ setup, cleanup bool }{}
			for _, subsub := range sub.Commands() {
				switch subsub.Use {
				case "setup":
					dockerCmd.setup = true
				case "cleanup":
					dockerCmd.cleanup = true
				}
			}
			break
		}
	}

	// Then
	assert.NotNil(t, dockerCmd, "docker command should exist")
	assert.True(t, dockerCmd.setup, "docker should have setup")
	assert.True(t, dockerCmd.cleanup, "docker should have cleanup")
}

func TestCliLogger(t *testing.T) {
	t.Run("Info does not panic", func(t *testing.T) {
		// Given
		logger := &cliLogger{}

		// When / Then
		assert.NotPanics(t, func() {
			logger.Info("test %s", "info")
		})
	})

	t.Run("Debug does not panic", func(t *testing.T) {
		// Given
		logger := &cliLogger{}

		// When / Then
		assert.NotPanics(t, func() {
			logger.Debug("test %s", "debug")
		})
	})

	t.Run("Warning does not panic", func(t *testing.T) {
		// Given
		logger := &cliLogger{}

		// When / Then
		assert.NotPanics(t, func() {
			logger.Warning("test %s", "warning")
		})
	})
}

func TestCliLoggerInterface(t *testing.T) {
	// Given
	// When
	var logger interface {
		Info(format string, args ...interface{})
		Debug(format string, args ...interface{})
		Warning(format string, args ...interface{})
	} = &cliLogger{}

	// Then
	assert.NotNil(t, logger)
}
