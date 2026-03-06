//go:build !sdl3
// +build !sdl3

package audio

func NewSDL3AudioBackend() Backend {
	return nil
}
