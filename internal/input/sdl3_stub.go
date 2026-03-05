//go:build !sdl3
// +build !sdl3

package input

// NewSDL3Backend is a build-tag stub when sdl3 is not enabled.
func NewSDL3Backend(sys *System) Backend { return nil }
