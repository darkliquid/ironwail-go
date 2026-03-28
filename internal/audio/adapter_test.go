package audio

import (
	"fmt"
	"testing"
)

type adapterTestBackend struct {
	name        string
	failRates   map[int]error
	initCalls   []int
	shutdowns   int
	returnedDMA *DMAInfo
}

func (b *adapterTestBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	b.initCalls = append(b.initCalls, sampleRate)
	if err := b.failRates[sampleRate]; err != nil {
		return nil, err
	}
	if b.returnedDMA != nil {
		return b.returnedDMA, nil
	}
	return &DMAInfo{
		Channels:   channels,
		Samples:    bufferSize,
		SampleBits: sampleBits,
		Speed:      sampleRate,
		Buffer:     make([]byte, bufferSize*channels*(sampleBits/8)),
	}, nil
}

func (b *adapterTestBackend) Shutdown() { b.shutdowns++ }
func (b *adapterTestBackend) Lock()     {}
func (b *adapterTestBackend) Unlock()   {}
func (b *adapterTestBackend) GetPosition() int {
	return 0
}
func (b *adapterTestBackend) Block()   {}
func (b *adapterTestBackend) Unblock() {}

func TestAudioAdapterInitFallsBackAcrossBackends(t *testing.T) {
	oldNewSDL3 := newSDL3AudioBackend
	oldNewOto := newOtoBackend
	oldNewMiniaudio := newMiniaudioBackend
	t.Cleanup(func() {
		newSDL3AudioBackend = oldNewSDL3
		newOtoBackend = oldNewOto
		newMiniaudioBackend = oldNewMiniaudio
	})

	sdl3 := &adapterTestBackend{name: "sdl3", failRates: map[int]error{44100: fmt.Errorf("no device"), 48000: fmt.Errorf("no device")}}
	oto := &adapterTestBackend{name: "oto", failRates: map[int]error{}}
	miniaudio := &adapterTestBackend{name: "miniaudio", failRates: map[int]error{}}

	newSDL3AudioBackend = func() Backend { return sdl3 }
	newOtoBackend = func() Backend { return oto }
	newMiniaudioBackend = func() Backend { return miniaudio }

	sys := NewSystem()
	adapter := NewAudioAdapter(sys)
	if err := adapter.Init(); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if sys.backend != oto {
		t.Fatalf("selected backend = %T, want oto backend after SDL3 failure", sys.backend)
	}
	if len(sdl3.initCalls) != 2 || sdl3.initCalls[0] != 44100 || sdl3.initCalls[1] != 48000 {
		t.Fatalf("SDL3 init calls = %v, want [44100 48000]", sdl3.initCalls)
	}
	if len(oto.initCalls) != 1 || oto.initCalls[0] != 44100 {
		t.Fatalf("oto init calls = %v, want [44100]", oto.initCalls)
	}
	if len(miniaudio.initCalls) != 0 {
		t.Fatalf("miniaudio should not be tried when oto succeeds, got %v", miniaudio.initCalls)
	}
}

func TestAudioAdapterInitUsesMiniaudioWhenOtoUnavailable(t *testing.T) {
	oldNewSDL3 := newSDL3AudioBackend
	oldNewOto := newOtoBackend
	oldNewMiniaudio := newMiniaudioBackend
	t.Cleanup(func() {
		newSDL3AudioBackend = oldNewSDL3
		newOtoBackend = oldNewOto
		newMiniaudioBackend = oldNewMiniaudio
	})

	miniaudio := &adapterTestBackend{name: "miniaudio", failRates: map[int]error{}}

	newSDL3AudioBackend = func() Backend { return nil }
	newOtoBackend = func() Backend { return nil }
	newMiniaudioBackend = func() Backend { return miniaudio }

	sys := NewSystem()
	adapter := NewAudioAdapter(sys)
	if err := adapter.Init(); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	if sys.backend != miniaudio {
		t.Fatalf("selected backend = %T, want miniaudio backend", sys.backend)
	}
	if len(miniaudio.initCalls) != 1 || miniaudio.initCalls[0] != 44100 {
		t.Fatalf("miniaudio init calls = %v, want [44100]", miniaudio.initCalls)
	}
}
