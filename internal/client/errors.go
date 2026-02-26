// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package client

// String returns a string representation of the state
func (s ClientState) String() string {
	switch s {
	case StateDisconnected:
		return "disconnected"
	case StateConnected:
		return "connected"
	case StateActive:
		return "active"
	default:
		return "unknown"
	}
}

// Error wraps an error with state information
type Error struct {
	Err   error
	State ClientState
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
func NewError(err error, state ClientState) *Error {
	return &Error{Err: err, State: state}
}

// IsTemporary returns whether the error is temporary (recoverable)
func (e *Error) IsTemporary() bool {
	// For now, treat all errors as non-temporary
	return false
}
