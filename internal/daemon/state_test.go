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

package daemon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDaemonState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateShuttingDown, "shutting_down"},
		{StateStopped, "stopped"},
		{StateFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestDaemonState_Transitions(t *testing.T) {
	tests := []struct {
		name     string
		from     State
		to       State
		canTrans bool
	}{
		// Starting transitions
		{"starting to running", StateStarting, StateRunning, true},
		{"starting to failed", StateStarting, StateFailed, true},
		{"starting to stopped", StateStarting, StateStopped, false},
		{"starting to shutting_down", StateStarting, StateShuttingDown, false},

		// Running transitions
		{"running to shutting_down", StateRunning, StateShuttingDown, true},
		{"running to starting", StateRunning, StateStarting, false},
		{"running to stopped", StateRunning, StateStopped, false},
		{"running to failed", StateRunning, StateFailed, false},

		// ShuttingDown transitions
		{"shutting_down to stopped", StateShuttingDown, StateStopped, true},
		{"shutting_down to running", StateShuttingDown, StateRunning, false},
		{"shutting_down to starting", StateShuttingDown, StateStarting, false},

		// Failed transitions
		{"failed to stopped", StateFailed, StateStopped, true},
		{"failed to running", StateFailed, StateRunning, false},
		{"failed to starting", StateFailed, StateStarting, false},

		// Stopped transitions (terminal state)
		{"stopped to starting", StateStopped, StateStarting, false},
		{"stopped to running", StateStopped, StateRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.canTrans, tt.from.CanTransitionTo(tt.to))
		})
	}
}

func TestStateMachine_Transitions(t *testing.T) {
	// Given
	sm := NewStateMachine()

	// When
	err := sm.TransitionTo(StateRunning)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, StateRunning, sm.Current())
}

func TestStateMachine_InvalidTransition(t *testing.T) {
	// Given
	sm := NewStateMachine()

	// When
	err := sm.TransitionTo(StateStopped)

	// Then
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state transition")
	assert.Equal(t, StateStarting, sm.Current())
}

func TestStateMachine_FailurePath(t *testing.T) {
	// Given
	sm := NewStateMachine()

	// When
	err := sm.TransitionTo(StateFailed)

	// Then
	assert.NoError(t, err)
	assert.Equal(t, StateFailed, sm.Current())
}

func TestStateMachine_IsTerminal(t *testing.T) {
	tests := []struct {
		state      State
		isTerminal bool
	}{
		{StateStarting, false},
		{StateRunning, false},
		{StateShuttingDown, false},
		{StateFailed, false},
		{StateStopped, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			assert.Equal(t, tt.isTerminal, tt.state.IsTerminal())
		})
	}
}
