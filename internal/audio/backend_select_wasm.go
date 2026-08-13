//go:build js && wasm

package audio

import "log/slog"

// newOtoBackend returns the Web Audio API backend on browsers. Oto needs
// native audio and is not available in wasm; the Web Audio backend also
// handles the browser autoplay restriction (the AudioContext starts
// suspended and is resumed on the first user gesture).
func newOtoBackend() Backend {
	slog.Debug("selecting WASM Web Audio backend")
	return NewWASMAudioBackend()
}
