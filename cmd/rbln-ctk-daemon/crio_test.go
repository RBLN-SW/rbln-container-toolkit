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

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewCrioCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newCrioCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "crio", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
	})

	t.Run("has setup and cleanup subcommands", func(t *testing.T) {
		// When
		cmd := newCrioCmd()

		// Then
		subcommands := make(map[string]bool)
		for _, sub := range cmd.Commands() {
			subcommands[sub.Use] = true
		}
		assert.True(t, subcommands["setup"], "should have 'setup' subcommand")
		assert.True(t, subcommands["cleanup"], "should have 'cleanup' subcommand")
	})
}

func TestNewCrioSetupCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newCrioSetupCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "setup", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
	})

	t.Run("has RunE function", func(t *testing.T) {
		// When
		cmd := newCrioSetupCmd()

		// Then
		assert.NotNil(t, cmd.RunE)
	})
}

func TestNewCrioCleanupCmd(t *testing.T) {
	t.Run("returns valid cobra command", func(t *testing.T) {
		// When
		cmd := newCrioCleanupCmd()

		// Then
		assert.NotNil(t, cmd)
		assert.Equal(t, "cleanup", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
	})

	t.Run("has RunE function", func(t *testing.T) {
		// When
		cmd := newCrioCleanupCmd()

		// Then
		assert.NotNil(t, cmd.RunE)
	})
}

func TestCrioSignalRestartModeValidation(t *testing.T) {
	t.Run("runCrioSetup returns error with signal restart mode", func(t *testing.T) {
		// Given
		originalRestartMode := restartMode
		originalViperValue := viper.GetString("restart_mode")
		defer func() {
			restartMode = originalRestartMode
			viper.Set("restart_mode", originalViperValue)
		}()
		restartMode = "signal"
		viper.Set("restart_mode", "")

		// When
		err := runCrioSetup()

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signal restart mode is not supported for CRI-O")
	})

	t.Run("runCrioCleanup returns error with signal restart mode", func(t *testing.T) {
		// Given
		originalRestartMode := restartMode
		originalViperValue := viper.GetString("restart_mode")
		defer func() {
			restartMode = originalRestartMode
			viper.Set("restart_mode", originalViperValue)
		}()
		restartMode = "signal"
		viper.Set("restart_mode", "")

		// When
		err := runCrioCleanup()

		// Then
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signal restart mode is not supported for CRI-O")
	})
}

func TestCrioCmdHierarchy(t *testing.T) {
	// Given
	runtimeCmd := newRuntimeCmd()

	// When
	var crioCmd *struct{ setup, cleanup bool }
	for _, sub := range runtimeCmd.Commands() {
		if sub.Use == "crio" {
			crioCmd = &struct{ setup, cleanup bool }{}
			for _, subsub := range sub.Commands() {
				switch subsub.Use {
				case "setup":
					crioCmd.setup = true
				case "cleanup":
					crioCmd.cleanup = true
				}
			}
			break
		}
	}

	// Then
	assert.NotNil(t, crioCmd, "crio command should exist")
	assert.True(t, crioCmd.setup, "crio should have setup")
	assert.True(t, crioCmd.cleanup, "crio should have cleanup")
}
