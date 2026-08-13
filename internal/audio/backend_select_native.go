//go:build !(js && wasm)

package audio

import "log/slog"

// newOtoBackend returns the native Oto audio backend on non-browser targets.
func newOtoBackend() Backend {
	slog.Debug("selecting Oto audio backend")
	return NewOtoBackend()
}
