// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package client

import (
	"errors"
)

// State represents client connection state
type State int

const (
	StateDisconnected State = iota
	StateConnected
)

// GetState returns the current client state
func GetState(s State) State {
	return s
}

// String returns a string representation of the state
func (s State) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnected:
		return "connected"
	default:
		return "unknown"
	}
}

// Error wraps an error with state information
type Error struct {
	Err   error
	State State
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return "unknown error"
}

func (e *Error) Unwrap() error {
	return e.Err
}

// NewError creates a new error with state
func NewError(err error, state State) *Error {
	return &Error{Err: err, State: state}
}

// IsTemporary returns whether the error is temporary (recoverable)
func (e *Error) IsTemporary() bool {
	// For now, treat all errors as non-temporary
	return false
}
