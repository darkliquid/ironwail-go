package audio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
)

// TestPlayCDTrackStreamsAndLoopsCurrentTrack tests CD music streaming and looping.
// It replicating the original Quake's ability to play and loop background music tracks.
// Where in C: CDAudio_Play and CDAudio_Update in cd_sdl.c (or similar)
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

// TestLoadWAVParsesStandardPCMHeaders tests WAV file loading.
// It supporting standard uncompressed audio for sounds and music.
// Where in C: S_LoadSound in snd_dma.c
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

// TestPlayCDTrackTransitionsToLoopTrack tests the transition between a \"start\" track and a \"loop\" track.
// It supporting advanced music logic where a track has an intro followed by a repeating section.
// Where in C: N/A (Common engine extension for digital music)
func TestPlayCDTrackTransitionsToLoopTrack(t *testing.T) {
	sys := newTestMusicSystem()
	track2 := testMusicWAV(t, 44100, 2, 2, 64)
	track3 := testMusicWAV(t, 44100, 1, 2, 32)

	err := sys.PlayCDTrack(2, 3, func(name string) ([]byte, error) {
		if strings.HasSuffix(name, ".wav") && strings.Contains(name, "track02") {
			return track2, nil
		}
		if strings.HasSuffix(name, ".wav") && strings.Contains(name, "track03") {
			return track3, nil
		}
		return nil, fmt.Errorf("missing %s", name)
	})
	if err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}

	sys.updateMusic(96)

	if got := sys.CurrentMusicTrack(); got != 3 {
		t.Fatalf("CurrentMusicTrack = %d, want 3 after loop transition", got)
	}
	if sys.music == nil || sys.music.position != 32 {
		t.Fatalf("music position after transition = %#v, want 32 frames into loop track", sys.music)
	}
}

// TestStopMusicClearsQueuedSamples tests music stoppage.
// It ensuring all audio buffers are cleared when music is stopped to prevent \"hanging\" notes or sounds.
// Where in C: S_StopAllSounds or similar in snd_dma.c
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

// TestPlayCDTrackLoadsOGGWhenWAVMissing tests OGG music support.
// It providing modern compressed audio support for background music, a key Ironwail feature.
// Where in C: Ironwail's OGG loader in snd_ogg.c
func TestPlayCDTrackLoadsOGGWhenWAVMissing(t *testing.T) {
	sys := newTestMusicSystem()
	oggData := testMusicOGG(t, 44100, 2, 2, 64)

	var loads []string
	err := sys.PlayCDTrack(2, 2, func(name string) ([]byte, error) {
		loads = append(loads, name)
		if strings.HasSuffix(name, ".ogg") {
			return oggData, nil
		}
		return nil, fmt.Errorf("missing %s", name)
	})
	if err != nil {
		t.Fatalf("PlayCDTrack failed: %v", err)
	}
	// OGG is now first in priority order (matching C Ironwail), so it's found on the first try.
	if len(loads) != 1 {
		t.Fatalf("loader called %d times, want 1 (ogg found first)", len(loads))
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

// TestPlayCDTrackUsesResolverSelection tests the music file resolution logic.
// It ensuring the engine searches for music in the correct priority order (OGG -> Opus -> MP3 -> etc.).
// Where in C: Ironwail's S_FindMusic or similar.
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
		if len(candidates) != 9 {
			t.Fatalf("resolver candidate count = %d, want 9", len(candidates))
		}
		if got := candidates[0]; got != "music/track02.ogg" {
			t.Fatalf("resolver first candidate = %q, want music/track02.ogg", got)
		}
		if got := candidates[1]; got != "music/track02.opus" {
			t.Fatalf("resolver second candidate = %q, want music/track02.opus", got)
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

// TestPlayMusicResolvesExtensionlessNameViaResolver tests music resolution without explicit extensions.
// It allowing flexible music filenames in QuakeC and console commands.
// Where in C: Ironwail's music resolution logic.
func TestPlayMusicResolvesExtensionlessNameViaResolver(t *testing.T) {
	sys := newTestMusicSystem()
	oggData := testMusicOGG(t, 44100, 2, 2, 64)

	var gotCandidates []string
	if err := sys.PlayMusic("track02", nil, func(candidates []string) (string, []byte, error) {
		gotCandidates = append([]string(nil), candidates...)
		return "music/track02.ogg", oggData, nil
	}); err != nil {
		t.Fatalf("PlayMusic failed: %v", err)
	}

	if len(gotCandidates) != 9 {
		t.Fatalf("resolver candidate count = %d, want 9", len(gotCandidates))
	}
	if got := gotCandidates[0]; got != "music/track02.ogg" {
		t.Fatalf("first candidate = %q, want music/track02.ogg", got)
	}
	if got := sys.CurrentMusic(); got != "music/track02.ogg" {
		t.Fatalf("CurrentMusic = %q, want music/track02.ogg", got)
	}
}

// TestPauseMusicStopsQueueingUntilResume tests music pausing and resuming.
// It correctly handling game pauses by stopping music playback and resuming from the same point.
// Where in C: S_PauseSound in snd_dma.c
func TestPauseMusicStopsQueueingUntilResume(t *testing.T) {
	sys := newTestMusicSystem()
	trackData := testMusicWAV(t, 44100, 2, 2, 64)

	if err := sys.PlayMusic("track02.wav", func(name string) ([]byte, error) {
		if name != "music/track02.wav" {
			return nil, fmt.Errorf("unexpected path %q", name)
		}
		return trackData, nil
	}); err != nil {
		t.Fatalf("PlayMusic failed: %v", err)
	}

	sys.PauseMusic()
	sys.updateMusic(32)
	if got := sys.rawSamples.End; got != 0 {
		t.Fatalf("rawSamples.End while paused = %d, want 0", got)
	}

	sys.ResumeMusic()
	sys.updateMusic(32)
	if got := sys.rawSamples.End; got < 32 {
		t.Fatalf("rawSamples.End after resume = %d, want at least 32", got)
	}
}

func newTestMusicSystem() *System {
	return &System{
		started:   true,
		musicLoop: true,
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

type mockMusicStream struct {
	readFramesFunc func(dst []byte) (int, error)
	seekFrameFunc  func(frame int64) error
	closeFunc      func() error
	seekCalls      []int64
	closeCalls     int
}

func (m *mockMusicStream) ReadFrames(dst []byte) (int, error) {
	if m.readFramesFunc != nil {
		return m.readFramesFunc(dst)
	}
	return 0, io.EOF
}

func (m *mockMusicStream) SeekFrame(frame int64) error {
	m.seekCalls = append(m.seekCalls, frame)
	if m.seekFrameFunc != nil {
		return m.seekFrameFunc(frame)
	}
	return nil
}

func (m *mockMusicStream) Close() error {
	m.closeCalls++
	if m.closeFunc != nil {
		return m.closeFunc()
	}
	return nil
}

// TestStreamingMusicPlaybackInMixer verifies that streaming music playback decodes PCM
// audio into raw samples in the mixer.
func TestStreamingMusicPlaybackInMixer(t *testing.T) {
	sys := newTestMusicSystem()
	oggData := testMusicOGG(t, 44100, 2, 2, 128)

	err := sys.PlayMusic("track02.ogg", func(name string) ([]byte, error) {
		if name != "music/track02.ogg" {
			return nil, fmt.Errorf("unexpected path %q", name)
		}
		return oggData, nil
	})
	if err != nil {
		t.Fatalf("PlayMusic failed: %v", err)
	}

	if sys.music == nil || sys.music.track == nil || sys.music.track.stream == nil {
		t.Fatalf("expected active streaming track")
	}

	sys.updateMusic(64)

	if sys.rawSamples.End < 64 {
		t.Fatalf("rawSamples.End = %d, want at least 64", sys.rawSamples.End)
	}
	hasNonZero := false
	for i := 0; i < sys.rawSamples.End; i++ {
		if sys.rawSamples.Samples[i].Left != 0 || sys.rawSamples.Samples[i].Right != 0 {
			hasNonZero = true
			break
		}
	}
	if !hasNonZero {
		t.Fatalf("expected raw samples to be populated with decoded PCM from streaming music")
	}
}

// TestStreamingMusicLooping tests that looping streaming tracks resets position to 0 via
// SeekFrame(0) and continues playback seamlessly across loop boundaries.
func TestStreamingMusicLooping(t *testing.T) {
	sys := newTestMusicSystem()
	sys.musicLoop = true

	totalFrames := 64
	streamPos := 0
	mock := &mockMusicStream{
		readFramesFunc: func(dst []byte) (int, error) {
			frameSize := 4
			maxFrames := len(dst) / frameSize
			avail := totalFrames - streamPos
			if avail <= 0 {
				return 0, io.EOF
			}
			toRead := maxFrames
			if toRead > avail {
				toRead = avail
			}
			for f := 0; f < toRead; f++ {
				val := int16((streamPos + f + 1) * 100)
				binary.LittleEndian.PutUint16(dst[f*frameSize:], uint16(val))
				binary.LittleEndian.PutUint16(dst[f*frameSize+2:], uint16(-val))
			}
			streamPos += toRead
			return toRead, nil
		},
		seekFrameFunc: func(frame int64) error {
			streamPos = int(frame)
			return nil
		},
	}

	sys.music = &musicState{
		loop: true,
		track: &musicTrack{
			name:     "music/mock_loop.ogg",
			stream:   mock,
			samples:  totalFrames,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	// Update past the 64-frame track length to 100 frames
	sys.updateMusic(100)

	if len(mock.seekCalls) == 0 {
		t.Fatalf("expected SeekFrame to be called upon looping, got 0 calls")
	}
	if mock.seekCalls[0] != 0 {
		t.Fatalf("expected SeekFrame(0), got SeekFrame(%d)", mock.seekCalls[0])
	}
	if sys.music == nil {
		t.Fatalf("expected music to remain active after loop")
	}
	if sys.music.position != 36 {
		t.Fatalf("music position after loop = %d, want 36", sys.music.position)
	}
	if sys.rawSamples.End < 100 {
		t.Fatalf("rawSamples.End = %d, want at least 100", sys.rawSamples.End)
	}

	sampleBeforeLoop := sys.rawSamples.Samples[63]
	sampleAfterLoop := sys.rawSamples.Samples[64]
	if sampleBeforeLoop.Left == 0 && sampleBeforeLoop.Right == 0 {
		t.Fatalf("expected non-zero samples before loop boundary")
	}
	if sampleAfterLoop.Left == 0 && sampleAfterLoop.Right == 0 {
		t.Fatalf("expected non-zero samples after loop boundary")
	}
}

// TestStreamingMusicSeekErrorHandling verifies that when SeekFrame(0) fails, advanceMusicTrack
// returns an error and updateMusic cleanly stops music without entering an infinite loop.
func TestStreamingMusicSeekErrorHandling(t *testing.T) {
	sys := newTestMusicSystem()
	sys.musicLoop = true
	mock := &mockMusicStream{
		seekFrameFunc: func(frame int64) error {
			return errors.New("seek failed: disk I/O error")
		},
		readFramesFunc: func(dst []byte) (int, error) {
			return 0, io.EOF
		},
	}

	// 1. Verify advanceMusicTrack returns an error on SeekFrame failure for regular music
	sys.music = &musicState{
		loop:     true,
		position: 32,
		track: &musicTrack{
			name:     "music/seek_fail.ogg",
			stream:   mock,
			samples:  32,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	err := sys.advanceMusicTrack()
	if err == nil {
		t.Fatalf("advanceMusicTrack() expected error when SeekFrame fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to seek music stream") {
		t.Fatalf("advanceMusicTrack() error = %q, want containing 'failed to seek music stream'", err.Error())
	}

	// 2. Verify updateMusic stops music cleanly without an infinite loop
	sys.music = &musicState{
		loop:     true,
		position: 32,
		track: &musicTrack{
			name:     "music/seek_fail.ogg",
			stream:   mock,
			samples:  32,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	sys.updateMusic(64)

	if sys.music != nil {
		t.Fatalf("expected StopMusic() to be called after seek failure, got sys.music != nil")
	}
	if mock.closeCalls == 0 {
		t.Fatalf("expected stream to be closed after StopMusic()")
	}

	// 3. Verify advanceMusicTrack returns an error on SeekFrame failure for same-track CD loop
	sys.music = &musicState{
		requestTrack: 2,
		loopTrack:    2,
		activeTrack:  2,
		loop:         true,
		position:     32,
		track: &musicTrack{
			name:     "music/track02.ogg",
			stream:   mock,
			samples:  32,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	err = sys.advanceMusicTrack()
	if err == nil {
		t.Fatalf("advanceMusicTrack() for CD track expected error when SeekFrame fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to seek music stream") {
		t.Fatalf("advanceMusicTrack() CD loop error = %q, want containing 'failed to seek music stream'", err.Error())
	}
}

// TestStreamingMusicCorruptDecodeHandling verifies that non-EOF decode errors from ReadFrames
// cause updateMusic to cleanly stop playback.
func TestStreamingMusicCorruptDecodeHandling(t *testing.T) {
	sys := newTestMusicSystem()
	mock := &mockMusicStream{
		readFramesFunc: func(dst []byte) (int, error) {
			return 0, errors.New("corrupted ogg stream data")
		},
	}
	sys.music = &musicState{
		loop:     true,
		position: 16,
		track: &musicTrack{
			name:     "music/corrupt.ogg",
			stream:   mock,
			samples:  64,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	sys.updateMusic(64)

	if sys.music != nil {
		t.Fatalf("expected StopMusic() on corrupt decode error, got sys.music != nil")
	}
	if mock.closeCalls == 0 {
		t.Fatalf("expected stream to be closed when corrupt music stopped")
	}
}

// TestStreamingMusicUnplayableStreamStops verifies that an unplayable stream (0 frames at pos 0)
// stops music immediately to avoid infinite loops.
func TestStreamingMusicUnplayableStreamStops(t *testing.T) {
	sys := newTestMusicSystem()
	mock := &mockMusicStream{
		readFramesFunc: func(dst []byte) (int, error) {
			return 0, io.EOF
		},
	}
	sys.music = &musicState{
		loop:     true,
		position: 0,
		track: &musicTrack{
			name:     "music/unplayable.ogg",
			stream:   mock,
			samples:  64,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	sys.updateMusic(64)

	if sys.music != nil {
		t.Fatalf("expected StopMusic() on unplayable stream (0 frames at pos 0), got sys.music != nil")
	}
}

// TestStreamingMusicStopClosesStream verifies that StopMusic calls stream.Close().
func TestStreamingMusicStopClosesStream(t *testing.T) {
	sys := newTestMusicSystem()
	mock := &mockMusicStream{}
	sys.music = &musicState{
		track: &musicTrack{
			name:   "music/test.ogg",
			stream: mock,
		},
	}

	sys.StopMusic()

	if mock.closeCalls != 1 {
		t.Fatalf("expected stream.Close() to be called once, got %d calls", mock.closeCalls)
	}
	if sys.music != nil {
		t.Fatalf("expected sys.music == nil after StopMusic()")
	}
}

// TestOGGStreamSeekFrameBounds verifies that oggStream.SeekFrame rejects negative frame
// offsets and offsets past the stream length.
func TestOGGStreamSeekFrameBounds(t *testing.T) {
	oggData := testMusicOGG(t, 44100, 2, 2, 64)
	track, err := decodeMusicOGG("music/bounds.ogg", oggData)
	if err != nil {
		t.Fatalf("decodeMusicOGG failed: %v", err)
	}

	if err := track.stream.SeekFrame(-1); err == nil {
		t.Fatalf("expected SeekFrame(-1) to return error, got nil")
	}
	if err := track.stream.SeekFrame(int64(track.samples) + 1); err == nil {
		t.Fatalf("expected SeekFrame(past end) to return error, got nil")
	}
	if err := track.stream.SeekFrame(0); err != nil {
		t.Fatalf("SeekFrame(0) should succeed, got %v", err)
	}
}

// TestStreamingMusicZeroSamplesStopsPlayback asserts that a track with 0 samples
// immediately stops playback in updateMusic rather than hanging in an infinite loop.
func TestStreamingMusicZeroSamplesStopsPlayback(t *testing.T) {
	sys := NewSystem()
	sys.started = true
	sys.dma = &DMAInfo{Speed: 44100, Channels: 2, SampleBits: 16}

	mock := &mockMusicStream{}
	sys.music = &musicState{
		activeTrack: 1,
		loop:        true,
		track: &musicTrack{
			name:     "music/empty.ogg",
			stream:   mock,
			samples:  0,
			rate:     44100,
			width:    2,
			channels: 2,
		},
	}

	sys.updateMusic(1024)

	if sys.music != nil {
		t.Fatalf("expected playback to stop for track with 0 samples, got non-nil music state")
	}
}
