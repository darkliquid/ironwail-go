package audio

import (
	"testing"
)



func TestSelectAudioBackendReturnsPlatformBackend(t *testing.T) {
	if got := selectAudioBackend(); got == nil {
		t.Fatal("selectAudioBackend() returned nil")
	}
}
