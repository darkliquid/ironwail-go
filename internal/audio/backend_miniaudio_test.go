//go:build amd64 && (linux || windows)

package audio

import (
	"encoding/binary"
	"testing"
)

func TestMiniaudioBackendCopyFramesAdvancesCursor(t *testing.T) {
	backend := &MiniaudioBackend{
		sampleBits: 16,
		channels:   2,
		bufferSize: 4,
		dma: &DMAInfo{
			Channels:   2,
			Samples:    4,
			SampleBits: 16,
			Buffer:     make([]byte, 4*2*2),
		},
	}

	values := []int16{
		100, 200,
		300, 400,
		500, 600,
		700, 800,
	}
	for i, sample := range values {
		binary.LittleEndian.PutUint16(backend.dma.Buffer[i*2:], uint16(sample))
	}

	frames := backend.copyFrames(2, 2)
	if got, want := frames[0][0], int16(100); got != want {
		t.Fatalf("frame 0 left = %d, want %d", got, want)
	}
	if got, want := frames[1][1], int16(400); got != want {
		t.Fatalf("frame 1 right = %d, want %d", got, want)
	}
	if got, want := backend.GetPosition(), 2; got != want {
		t.Fatalf("position = %d, want %d", got, want)
	}

	frames = backend.copyFrames(3, 2)
	if got, want := frames[0][0], int16(500); got != want {
		t.Fatalf("wrapped frame 0 left = %d, want %d", got, want)
	}
	if got, want := frames[2][1], int16(200); got != want {
		t.Fatalf("wrapped frame 2 right = %d, want %d", got, want)
	}
	if got, want := backend.GetPosition(), 1; got != want {
		t.Fatalf("position after wrap = %d, want %d", got, want)
	}
}

func TestMiniaudioBackendCopyFramesLeavesExtraChannelsSilent(t *testing.T) {
	backend := &MiniaudioBackend{
		sampleBits: 16,
		channels:   1,
		bufferSize: 2,
		dma: &DMAInfo{
			Channels:   1,
			Samples:    2,
			SampleBits: 16,
			Buffer:     make([]byte, 2*2),
		},
	}
	binary.LittleEndian.PutUint16(backend.dma.Buffer[0:], uint16(int16(123)))
	binary.LittleEndian.PutUint16(backend.dma.Buffer[2:], uint16(int16(456)))

	frames := backend.copyFrames(2, 2)
	if got, want := frames[0][0], int16(123); got != want {
		t.Fatalf("frame 0 channel 0 = %d, want %d", got, want)
	}
	if got := frames[0][1]; got != 0 {
		t.Fatalf("frame 0 extra channel = %d, want 0", got)
	}
	if got, want := frames[1][0], int16(456); got != want {
		t.Fatalf("frame 1 channel 0 = %d, want %d", got, want)
	}
	if got := frames[1][1]; got != 0 {
		t.Fatalf("frame 1 extra channel = %d, want 0", got)
	}
}
