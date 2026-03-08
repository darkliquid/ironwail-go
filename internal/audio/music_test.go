package audio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os/exec"
	"testing"
)

func TestPlayCDTrackStreamsAndLoopsCurrentTrack(t *testing.T) {
	sys := newTestMusicSystem()
	trackData := testMusicWAV(t, 44100, 2, 2, 64)

	err := sys.PlayCDTrack(2, 2, func(name string) ([]byte, error) {
		if name != "music/track02.wav" {
			return nil, fmt.Errorf("unexpected path %q", name)
		}
		return trackData, nil
	})
	if err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}

	sys.updateMusic(64)
	if sys.rawSamples.End < 64 {
		t.Fatalf("rawSamples.End = %d, want at least 64", sys.rawSamples.End)
	}
	if sys.rawSamples.Samples[0].Left == 0 && sys.rawSamples.Samples[0].Right == 0 {
		t.Fatalf("expected streamed music samples to be queued")
	}

	sys.updateMusic(128)
	if got := sys.CurrentMusicTrack(); got != 2 {
		t.Fatalf("CurrentMusicTrack = %d, want 2", got)
	}
	if sys.rawSamples.End < 128 {
		t.Fatalf("rawSamples.End after loop = %d, want at least 128", sys.rawSamples.End)
	}
}

func TestLoadWAVParsesStandardPCMHeaders(t *testing.T) {
	mono := testMusicWAV(t, 22050, 1, 2, 16)
	sampleData, info, err := LoadWAV("sound/test.wav", mono)
	if err != nil {
		t.Fatalf("LoadWAV failed: %v", err)
	}
	if info.Rate != 22050 || info.Channels != 1 || info.Samples != 16 {
		t.Fatalf("LoadWAV info = %+v, want rate=22050 channels=1 samples=16", info)
	}
	if len(sampleData) != 16*2 {
		t.Fatalf("LoadWAV sample bytes = %d, want %d", len(sampleData), 16*2)
	}

	stereo := testMusicWAV(t, 44100, 2, 2, 8)
	sampleData, info, err = LoadMusicWAV("music/track02.wav", stereo)
	if err != nil {
		t.Fatalf("LoadMusicWAV failed: %v", err)
	}
	if info.Rate != 44100 || info.Channels != 2 || info.Samples != 8 {
		t.Fatalf("LoadMusicWAV info = %+v, want rate=44100 channels=2 samples=8", info)
	}
	if len(sampleData) != 8*2*2 {
		t.Fatalf("LoadMusicWAV sample bytes = %d, want %d", len(sampleData), 8*2*2)
	}
}

func TestPlayCDTrackTransitionsToLoopTrack(t *testing.T) {
	sys := newTestMusicSystem()
	loads := []string{}
	track2 := testMusicWAV(t, 44100, 2, 2, 64)
	track3 := testMusicWAV(t, 44100, 1, 2, 32)

	err := sys.PlayCDTrack(2, 3, func(name string) ([]byte, error) {
		loads = append(loads, name)
		switch name {
		case "music/track02.wav":
			return track2, nil
		case "music/track03.wav":
			return track3, nil
		default:
			return nil, fmt.Errorf("missing %s", name)
		}
	})
	if err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}

	sys.updateMusic(96)

	if got := sys.CurrentMusicTrack(); got != 3 {
		t.Fatalf("CurrentMusicTrack = %d, want 3 after loop transition", got)
	}
	if len(loads) != 2 {
		t.Fatalf("loader called %d times, want 2", len(loads))
	}
	if sys.music == nil || sys.music.position != 32 {
		t.Fatalf("music position after transition = %#v, want 32 frames into loop track", sys.music)
	}
}

func TestStopMusicClearsQueuedSamples(t *testing.T) {
	sys := newTestMusicSystem()
	trackData := testMusicWAV(t, 44100, 1, 2, 32)

	if err := sys.PlayCDTrack(2, 2, func(name string) ([]byte, error) { return trackData, nil }); err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}
	sys.updateMusic(32)
	if sys.rawSamples.End == 0 {
		t.Fatalf("expected queued raw samples before stopping music")
	}

	sys.StopMusic()

	if got := sys.CurrentMusicTrack(); got != 0 {
		t.Fatalf("CurrentMusicTrack = %d, want 0 after StopMusic", got)
	}
	if got := sys.rawSamples.End; got != sys.paintedTime {
		t.Fatalf("rawSamples.End = %d, want %d after StopMusic", got, sys.paintedTime)
	}
}

func TestPlayCDTrackLoadsOGGWhenWAVMissing(t *testing.T) {
	sys := newTestMusicSystem()
	oggData := testMusicOGG(t, 44100, 2, 2, 64)

	var loads []string
	err := sys.PlayCDTrack(2, 2, func(name string) ([]byte, error) {
		loads = append(loads, name)
		switch name {
		case "music/track02.wav":
			return nil, fmt.Errorf("missing %s", name)
		case "music/track02.ogg":
			return oggData, nil
		default:
			return nil, fmt.Errorf("unexpected path %q", name)
		}
	})
	if err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}
	if len(loads) != 2 {
		t.Fatalf("loader called %d times, want 2 (wav then ogg)", len(loads))
	}

	sys.updateMusic(128)
	if sys.rawSamples.End < 128 {
		t.Fatalf("rawSamples.End = %d, want at least 128", sys.rawSamples.End)
	}
	if got := sys.CurrentMusicTrack(); got != 2 {
		t.Fatalf("CurrentMusicTrack = %d, want 2", got)
	}
	if sys.music == nil || sys.music.track == nil {
		t.Fatalf("expected active OGG track")
	}
	if sys.music.track.width != 2 {
		t.Fatalf("music width = %d, want 2", sys.music.track.width)
	}
	if sys.music.track.channels != 2 {
		t.Fatalf("music channels = %d, want 2", sys.music.track.channels)
	}
}

func TestPlayCDTrackUsesResolverSelection(t *testing.T) {
	sys := newTestMusicSystem()
	oggData := testMusicOGG(t, 44100, 2, 2, 64)
	loaderCalled := false
	resolverCalled := false

	err := sys.PlayCDTrack(2, 2, func(name string) ([]byte, error) {
		loaderCalled = true
		return nil, fmt.Errorf("loader should not be used, got %s", name)
	}, func(candidates []string) (string, []byte, error) {
		resolverCalled = true
		if len(candidates) != 2 {
			t.Fatalf("resolver candidate count = %d, want 2", len(candidates))
		}
		if got := candidates[0]; got != "music/track02.wav" {
			t.Fatalf("resolver first candidate = %q, want music/track02.wav", got)
		}
		if got := candidates[1]; got != "music/track02.ogg" {
			t.Fatalf("resolver second candidate = %q, want music/track02.ogg", got)
		}
		return "music/track02.ogg", oggData, nil
	})
	if err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}
	if loaderCalled {
		t.Fatalf("expected loader to be bypassed when resolver is provided")
	}
	if !resolverCalled {
		t.Fatalf("expected resolver to be called")
	}

	sys.updateMusic(96)
	if sys.rawSamples.End < 96 {
		t.Fatalf("rawSamples.End = %d, want at least 96", sys.rawSamples.End)
	}
	if sys.music == nil || sys.music.track == nil {
		t.Fatalf("expected active music track")
	}
	if got := sys.music.track.name; got != "music/track02.ogg" {
		t.Fatalf("resolved track name = %q, want music/track02.ogg", got)
	}
}

func newTestMusicSystem() *System {
	return &System{
		started: true,
		dma: &DMAInfo{
			Channels:   2,
			Samples:    4096,
			SampleBits: 16,
			Speed:      44100,
			Buffer:     make([]byte, 4096*2*2),
		},
		mixer: NewMixer(),
	}
}

func testMusicWAV(t *testing.T, sampleRate, channels, width, frames int) []byte {
	t.Helper()

	blockAlign := channels * width
	dataSize := frames * blockAlign
	var data bytes.Buffer
	for frame := 0; frame < frames; frame++ {
		for channel := 0; channel < channels; channel++ {
			if width != 2 {
				t.Fatalf("test helper only supports 16-bit PCM, got width %d", width)
			}
			sample := int16((frame + 1) * 256)
			if channel%2 == 1 {
				sample = -sample
			}
			if err := binary.Write(&data, binary.LittleEndian, sample); err != nil {
				t.Fatalf("binary.Write sample: %v", err)
			}
		}
	}

	var wav bytes.Buffer
	writeString := func(value string) {
		if _, err := wav.WriteString(value); err != nil {
			t.Fatalf("WriteString(%q): %v", value, err)
		}
	}

	writeString("RIFF")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(36+dataSize)); err != nil {
		t.Fatalf("binary.Write RIFF size: %v", err)
	}
	writeString("WAVE")
	writeString("fmt ")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(16)); err != nil {
		t.Fatalf("binary.Write fmt size: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(WAVFormatPCM)); err != nil {
		t.Fatalf("binary.Write format: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(channels)); err != nil {
		t.Fatalf("binary.Write channels: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint32(sampleRate)); err != nil {
		t.Fatalf("binary.Write sample rate: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint32(sampleRate*blockAlign)); err != nil {
		t.Fatalf("binary.Write byte rate: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(blockAlign)); err != nil {
		t.Fatalf("binary.Write block align: %v", err)
	}
	if err := binary.Write(&wav, binary.LittleEndian, uint16(width*8)); err != nil {
		t.Fatalf("binary.Write bits per sample: %v", err)
	}
	writeString("data")
	if err := binary.Write(&wav, binary.LittleEndian, uint32(dataSize)); err != nil {
		t.Fatalf("binary.Write data size: %v", err)
	}
	if _, err := wav.Write(data.Bytes()); err != nil {
		t.Fatalf("Write data: %v", err)
	}

	return wav.Bytes()
}

func testMusicOGG(t *testing.T, sampleRate, channels, width, frames int) []byte {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skipf("ffmpeg not available: %v", err)
	}

	wavData := testMusicWAV(t, sampleRate, channels, width, frames)
	cmd := exec.Command("ffmpeg", "-loglevel", "error", "-f", "wav", "-i", "pipe:0", "-c:a", "libvorbis", "-f", "ogg", "pipe:1")
	cmd.Stdin = bytes.NewReader(wavData)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		t.Fatalf("ffmpeg OGG encoding failed: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("ffmpeg OGG encoding returned empty output")
	}
	return out.Bytes()
}
