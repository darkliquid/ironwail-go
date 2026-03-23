package compiler

import (
	"fmt"
	"go/token"
	"strings"
)

// CompileError represents a single compilation error with source position.
type CompileError struct {
	Pos token.Position
	Msg string
}

func (e *CompileError) Error() string {
	if e.Pos.IsValid() {
		return fmt.Sprintf("%s: %s", e.Pos, e.Msg)
	}
	return e.Msg
}

// ErrorList accumulates compilation errors.
type ErrorList struct {
	Errors []*CompileError
}

// Add appends a new error at the given position.
func (el *ErrorList) Add(pos token.Position, msg string) {
	el.Errors = append(el.Errors, &CompileError{Pos: pos, Msg: msg})
}

// Addf appends a new formatted error at the given position.
func (el *ErrorList) Addf(pos token.Position, format string, args ...any) {
	el.Errors = append(el.Errors, &CompileError{Pos: pos, Msg: fmt.Sprintf(format, args...)})
}

// Len returns the number of errors.
func (el *ErrorList) Len() int {
	return len(el.Errors)
}

// Err returns nil if no errors, otherwise returns the list as an error.
func (el *ErrorList) Err() error {
	if len(el.Errors) == 0 {
		return nil
	}
	return el
}

// Error implements the error interface.
func (el *ErrorList) Error() string {
	switch len(el.Errors) {
	case 0:
		return "no errors"
	case 1:
		return el.Errors[0].Error()
	default:
		var b strings.Builder
		for i, e := range el.Errors {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(e.Error())
		}
		return b.String()
	}
}
