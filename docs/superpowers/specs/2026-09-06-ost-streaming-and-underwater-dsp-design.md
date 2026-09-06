# OST Soundtrack Streaming & Underwater Audio DSP Design Specification

## Overview

This specification defines the design and implementation for issue **`ironwail-go-0vn`** (P2):
1. Pure-Go Ogg Vorbis soundtrack streaming in `internal/audio/` for ambient music packs and CD soundtracks without upfront full-track PCM allocations.
2. Parity-accurate real-time audio DSP low-pass filter in the DMA output pipeline (`CameraInLiquid`, `snd_waterfx`, click-free accumulator priming).
3. Seamless track looping and cvar controls (`bgmvolume`, `snd_waterfx`).

---

## Architectural Analysis & Parity Baseline

### 1. Music Architecture
- In Quake and C Ironwail, background music (`music/track*.ogg` or custom music packs) can run for several minutes per track. At 44.1kHz 16-bit stereo, a 4-minute track is ~42 MB of raw PCM.
- Currently, `ironwail-go`'s [`internal/audio/ogg.go`](file:///home/darkliquid/Projects/ironwail-go/internal/audio/ogg.go) calls `oggvorbis.ReadAll(bytes.NewReader(data))`, decompressing the entire file into an in-memory `[]byte` slice upfront.
- To achieve streaming playback, `internal/audio` must decode audio frames incrementally on demand during `updateMusic(endTime)`, decoding only the exact number of frames required to fill the next DMA mix window.

### 2. Underwater Liquid Audio DSP
- In C Ironwail (`ironwail/Quake/snd_mix.c`):
  - `S_SetUnderwaterIntensity(float target)`:
    - Target intensity scales by `snd_waterfx` cvar (`target *= CLAMP(0.f, snd_waterfx.value, 2.f)`).
    - Intensity ramps towards target at `host_frametime * 4.f`.
    - Filter cutoff coefficient: $\alpha = \exp(-\text{intensity} \times \ln(12))$.
  - `S_UnderwaterFilter(int endtime)`:
    - If `underwater.intensity == 0`, it primes the accumulator from the last dry sample in `paintbuffer[endtime-1]`.
    - If `underwater.intensity > 0`, it filters each stereo sample using the single-pole IIR filter:
      $$\text{accum} \mathrel{+}= \alpha \times (\text{paintbuffer}[i] - \text{accum})$$
      $$\text{paintbuffer}[i] = \text{accum}$$
- In `ironwail-go`:
  - [`internal/audio/mix.go`](file:///home/darkliquid/Projects/ironwail-go/internal/audio/mix.go) has the IIR filter formula, but:
    - Ramps at a fixed `0.016` step ignoring actual frame duration.
    - Does not factor in `snd_waterfx` cvar.
    - Does not prime accumulator on dry frames, resulting in potential waveform pops on liquid transitions.

---

## Component Architecture

```
+-------------------------------------------------------------+
| internal/game (Coordinator)                                 |
|  - syncRuntimeAmbientAudio(): detects camera liquid leaf    |
|  - Audio.UpdateAmbientSounds(frameTime, leaf, underwater)   |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
| internal/audio.System                                       |
|  - Cvars: bgmvolume, snd_waterfx                            |
|  - updateMusic(): calls stream.ReadFrames(scratch)          |
|  - mixer.SetUnderwaterIntensity(target, frameTime, waterfx) |
+-------------------------------------------------------------+
            |                                     |
            v                                     v
+------------------------+             +----------------------+
| musicStream Interface  |             | Mixer (DSP Pipeline) |
|  - ReadFrames(dst)     |             |  - S_LowpassFilter   |
|  - SeekFrame(pos)      |             |  - S_UnderwaterFilter|
|  - Close()             |             |  - S_UpdateLevels    |
+------------------------+             +----------------------+
            |                                     |
            v                                     v
+------------------------+             +----------------------+
| oggStream (oggvorbis)  |             | DMA Paint Buffer     |
+------------------------+             +----------------------+
```

---

## Detailed Design

### 1. `musicStream` and `oggStream` in `internal/audio`

#### Interface Definition (`internal/audio/music.go`)
```go
// musicStream represents a streaming audio source that decodes PCM on demand.
type musicStream interface {
	// ReadFrames decodes up to len(dst)/(channels*width) frames into dst as 16-bit PCM.
	// Returns the number of frames read and any error (io.EOF when finished).
	ReadFrames(dst []byte) (framesRead int, err error)
	// SeekFrame positions the stream at the given sample frame offset.
	SeekFrame(frame int64) error
	// Close releases any resources associated with the stream.
	Close() error
}
```

#### Struct Adjustments (`internal/audio/music.go`)
```go
type musicTrack struct {
	name     string
	data     []byte       // Non-nil for static/buffered tracks (WAV, tracker)
	stream   musicStream  // Non-nil for streaming tracks (OGG Vorbis)
	samples  int          // Total sample frames
	rate     int          // Sample rate (Hz)
	width    int          // Sample width (always 2 bytes)
	channels int          // 1 (mono) or 2 (stereo)
}

type musicState struct {
	requestTrack int
	loopTrack    int
	activeTrack  int
	position     int
	loop         bool
	paused       bool
	loader       func(string) ([]byte, error)
	resolver     musicResolveFunc
	track        *musicTrack
	streamBuf    []byte   // Reusable scratch buffer for decoding chunks
}
```

#### OGG Streaming Implementation (`internal/audio/ogg.go`)
```go
type oggStream struct {
	reader   *oggvorbis.Reader
	channels int
	rate     int
	length   int64
	fltBuf   []float32 // Scratch buffer for float32 decoding
}

func (s *oggStream) ReadFrames(dst []byte) (int, error) {
	frameSize := s.channels * 2
	maxFrames := len(dst) / frameSize
	if maxFrames == 0 {
		return 0, nil
	}
	neededSamples := maxFrames * s.channels
	if len(s.fltBuf) < neededSamples {
		s.fltBuf = make([]float32, neededSamples)
	}

	n, err := s.reader.Read(s.fltBuf[:neededSamples])
	frames := n / s.channels
	for i := 0; i < n; i++ {
		scaled := int32(s.fltBuf[i] * 32768.0)
		if scaled > 32767 {
			scaled = 32767
		} else if scaled < -32768 {
			scaled = -32768
		}
		val := uint16(int16(scaled))
		idx := i * 2
		dst[idx] = byte(val)
		dst[idx+1] = byte(val >> 8)
	}
	return frames, err
}

func (s *oggStream) SeekFrame(frame int64) error {
	return s.reader.SetPosition(frame)
}

func (s *oggStream) Close() error {
	return nil
}
```

#### Streaming Mix Loop (`updateMusic`)
- In `updateMusic(endTime int)`:
  - When `s.music.track.stream != nil`:
    - Determine `inputFrames := resampleInputFrames(neededOut, track.rate, dma.Speed)`.
    - Decode up to `inputFrames` into `s.music.streamBuf`.
    - If `io.EOF` or frames read == 0, trigger `advanceMusicTrack()`.
    - Call `s.AddRawSamples(framesRead, track.rate, track.width, track.channels, pcm, 1)`.
    - Update `s.music.position += framesRead`.
  - When `s.music.track.data != nil`:
    - Preserves existing slice copy from `s.music.track.data[start:stop]`.
- In `advanceMusicTrack()`:
  - If looping same track with stream: calls `track.stream.SeekFrame(0)` and sets `position = 0`.
  - If switching track: closes old stream, opens new stream.
- In `StopMusic()`:
  - Closes active stream if any, sets `s.music = nil`.

---

### 2. Underwater Liquid Audio DSP & Cvar Wiring

#### Mixer Interface & Method Updates (`internal/audio/mix.go`)
```go
func (m *Mixer) SetUnderwaterIntensity(target float32, frameTime float32, waterfx float32) {
	// Factor in snd_waterfx cvar (0=off, 1=on, up to 2=extra intensity)
	target *= float32(math.Max(0.0, math.Min(float64(waterfx), 2.0)))

	// Ramp towards target with host_frametime * 4.0
	step := frameTime * 4.0
	if step <= 0 {
		step = 0.016
	}

	if m.underwater.Intensity < target {
		m.underwater.Intensity += step
		if m.underwater.Intensity > target {
			m.underwater.Intensity = target
		}
	} else if m.underwater.Intensity > target {
		m.underwater.Intensity -= step
		if m.underwater.Intensity < target {
			m.underwater.Intensity = target
		}
	}

	m.underwater.Alpha = float32(math.Exp(-float64(m.underwater.Intensity) * math.Log(12)))
}

func (m *Mixer) applyUnderwaterFilter(count int) {
	if m.underwater.Intensity <= 0 {
		if count > 0 {
			m.underwater.Accum[0] = float32(m.paintBuffer[count-1].Left)
			m.underwater.Accum[1] = float32(m.paintBuffer[count-1].Right)
		}
		return
	}

	for i := 0; i < count; i++ {
		m.underwater.Accum[0] += m.underwater.Alpha * (float32(m.paintBuffer[i].Left) - m.underwater.Accum[0])
		m.underwater.Accum[1] += m.underwater.Alpha * (float32(m.paintBuffer[i].Right) - m.underwater.Accum[1])
		m.paintBuffer[i].Left = int32(m.underwater.Accum[0])
		m.paintBuffer[i].Right = int32(m.underwater.Accum[1])
	}
}
```

#### System Wiring (`internal/audio/sound.go`)
In `UpdateAmbientSounds`:
- Lookup `snd_waterfx` cvar value (`1.0` if cvar subsystem is unconfigured).
- Pass `underwaterIntensity`, `frameTime`, and `waterfx` to `mixer.SetUnderwaterIntensity(...)`.

---

## Verification & Testing Plan

1. **Streaming Decoder Unit Tests (`internal/audio/ogg_test.go`):**
   - Encode a synthetic Vorbis audio stream (e.g. sine wave with header).
   - Test `decodeMusicOGG`: verify `stream` is non-nil and `data` is nil.
   - Verify `ReadFrames` returns correct PCM samples and advances position.
   - Verify `SeekFrame(0)` correctly resets stream to beginning for looping.
   - Verify `updateMusic` with streaming OGG feeds `rawSamples` identical to full-decode.
2. **DSP Underwater Unit Tests (`internal/audio/mix_test.go`):**
   - Verify `SetUnderwaterIntensity` scaling with `snd_waterfx = 0` (remains 0 intensity even when in liquid).
   - Verify `SetUnderwaterIntensity` ramping rate proportional to `frameTime`.
   - Verify accumulator priming: when transitioning from dry to wet, no discontinuous jump occurs.
   - Verify frequency attenuation: high-frequency signals (> 1 kHz) are attenuated when submerged, while DC / low-frequency signals pass through with near-unity gain.
3. **Full Regression Gates:**
   - Run `CGO_ENABLED=0 go test ./internal/audio/...`.
   - Run `mise run verify` and `mise run lint`.
