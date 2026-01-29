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
	"fmt"
	"sync"
)

// State represents the state of the daemon.
type State int

const (
	// StateStarting indicates initialization is in progress.
	StateStarting State = iota
	// StateRunning indicates setup is complete and daemon is waiting for signal.
	StateRunning
	// StateShuttingDown indicates cleanup is in progress.
	StateShuttingDown
	// StateStopped indicates daemon has exited.
	StateStopped
	// StateFailed indicates an error occurred.
	StateFailed
)

// stateNames maps states to their string representations.
var stateNames = map[State]string{
	StateStarting:     "starting",
	StateRunning:      "running",
	StateShuttingDown: "shutting_down",
	StateStopped:      "stopped",
	StateFailed:       "failed",
}

// validTransitions defines which state transitions are allowed.
var validTransitions = map[State][]State{
	StateStarting:     {StateRunning, StateFailed},
	StateRunning:      {StateShuttingDown},
	StateShuttingDown: {StateStopped},
	StateFailed:       {StateStopped},
	StateStopped:      {}, // terminal state, no transitions allowed
}

// String returns the string representation of the state.
func (s State) String() string {
	if name, ok := stateNames[s]; ok {
		return name
	}
	return fmt.Sprintf("unknown(%d)", s)
}

// CanTransitionTo returns true if transitioning to the given state is allowed.
func (s State) CanTransitionTo(to State) bool {
	allowed, ok := validTransitions[s]
	if !ok {
		return false
	}
	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	return false
}

// IsTerminal returns true if this is a terminal state.
func (s State) IsTerminal() bool {
	return s == StateStopped
}

// StateMachine manages daemon state transitions.
type StateMachine struct {
	mu      sync.RWMutex
	current State
}

// NewStateMachine creates a new state machine in the Starting state.
func NewStateMachine() *StateMachine {
	return &StateMachine{
		current: StateStarting,
	}
}

// Current returns the current state.
func (sm *StateMachine) Current() State {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.current
}

// TransitionTo attempts to transition to the given state.
func (sm *StateMachine) TransitionTo(to State) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.current.CanTransitionTo(to) {
		return fmt.Errorf("invalid state transition: %s -> %s", sm.current, to)
	}

	sm.current = to
	return nil
}
