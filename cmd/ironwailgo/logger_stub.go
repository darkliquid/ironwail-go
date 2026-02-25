//go:build !gogpu
// +build !gogpu

package main

import (
	"log/slog"
)

func init() {
	// No gogpu logger setup needed for OpenGL or stub backends
	_ = slog.Default() // Use slog for other backends if needed
}
