package audio

import "testing"

type fakeBackend struct{}

func (fakeBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	return nil, nil
}
func (fakeBackend) Shutdown()        {}
func (fakeBackend) Lock()            {}
func (fakeBackend) Unlock()          {}
func (fakeBackend) GetPosition() int { return 0 }
func (fakeBackend) Block()           {}
func (fakeBackend) Unblock()         {}

func TestSelectAudioBackendPrefersProvidedBackend(t *testing.T) {
	backend := fakeBackend{}
	if got := selectAudioBackend(backend); got != backend {
		t.Fatalf("selectAudioBackend(fakeBackend) = %T, want fakeBackend passthrough", got)
	}
}

func TestSelectAudioBackendFallsBackToNull(t *testing.T) {
	if got := selectAudioBackend(nil); got == nil {
		t.Fatal("selectAudioBackend(nil) returned nil, want fallback backend")
	} else if _, ok := got.(*NullBackend); !ok {
		t.Fatalf("selectAudioBackend(nil) = %T, want *NullBackend", got)
	}
}
