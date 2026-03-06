//go:build !audio_oto || !cgo
// +build !audio_oto !cgo

package audio

func NewOtoBackend() Backend {
	return nil
}
