package audio

import (
	"testing"
)

type fakeBackend struct{}

func (fakeBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	return nil, nil
}
func (fakeBackend) Shutdown()     {}
func (fakeBackend) Lock()         {}
func (fakeBackend) Unlock()       {}
func (fakeBackend) Position() int { return 0 }
func (fakeBackend) Block()        {}
func (fakeBackend) Unblock()      {}

func TestSelectAudioBackendReturnsPlatformBackend(t *testing.T) {
	if got := selectAudioBackend(); got == nil {
		t.Fatal("selectAudioBackend() returned nil")
	}
}
