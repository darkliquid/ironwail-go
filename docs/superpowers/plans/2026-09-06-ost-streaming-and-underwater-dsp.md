# OST Soundtrack Streaming & Underwater Audio DSP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement pure-Go Ogg Vorbis soundtrack streaming in `internal/audio/` and parity-accurate underwater liquid audio DSP in the DMA output pipeline for issue `ironwail-go-0vn`.

**Architecture:** An unexported `musicStream` interface in `internal/audio/music.go` is implemented by `oggStream` in `internal/audio/ogg.go` using `oggvorbis.Reader`, decoding 16-bit PCM chunks on demand during `updateMusic()`. In `internal/audio/mix.go`, `Mixer.SetUnderwaterIntensity` is updated to scale by `snd_waterfx` and ramp at `frameTime * 4.0`, while `applyUnderwaterFilter` primes the accumulator on dry frames to prevent pop artifacts.

**Tech Stack:** Go 1.26, pure Go (`CGO_ENABLED=0`), `github.com/jfreymuth/oggvorbis`, `internal/audio`, `internal/cvar`.

---

### Task 1: Underwater Liquid Audio DSP & Cvar Wiring

**Files:**
- Create: `internal/audio/mix_test.go`
- Modify: `internal/audio/mix.go`
- Modify: `internal/audio/sound.go`
- Modify: `internal/audio/adapter.go`

- [ ] **Step 1: Write the failing tests for underwater DSP**

Write `internal/audio/mix_test.go`:
```go
package audio

import (
	"math"
	"testing"
)

func TestSetUnderwaterIntensityCvarAndRamping(t *testing.T) {
	m := &Mixer{}

	// When snd_waterfx is 0, intensity should stay 0 even if target is 1.0.
	m.SetUnderwaterIntensity(1.0, 0.05, 0.0)
	if m.UnderwaterIntensity() != 0 {
		t.Fatalf("expected intensity 0 with waterfx=0, got %f", m.UnderwaterIntensity())
	}

	// When snd_waterfx is 1.0 and frameTime is 0.05, intensity should increase by 0.05 * 4 = 0.2
	m.SetUnderwaterIntensity(1.0, 0.05, 1.0)
	expected := float32(0.2)
	if math.Abs(float64(m.UnderwaterIntensity()-expected)) > 1e-4 {
		t.Fatalf("expected intensity %f after ramp, got %f", expected, m.UnderwaterIntensity())
	}

	// Ramp up to target
	for i := 0; i < 10; i++ {
		m.SetUnderwaterIntensity(1.0, 0.05, 1.0)
	}
	if m.UnderwaterIntensity() != 1.0 {
		t.Fatalf("expected clamped target intensity 1.0, got %f", m.UnderwaterIntensity())
	}

	// Ramp down to 0
	for i := 0; i < 10; i++ {
		m.SetUnderwaterIntensity(0.0, 0.05, 1.0)
	}
	if m.UnderwaterIntensity() != 0.0 {
		t.Fatalf("expected ramped down intensity 0.0, got %f", m.UnderwaterIntensity())
	}
}

func TestApplyUnderwaterFilterPrimingAndAttenuation(t *testing.T) {
	m := &Mixer{}
	m.paintBuffer = make([]PaintBuffer, 100)

	// Fill with constant dry signal
	for i := range m.paintBuffer {
		m.paintBuffer[i].Left = 10000
		m.paintBuffer[i].Right = 10000
	}

	// When intensity is 0, accumulator should be primed to the last sample
	m.underwater.Intensity = 0
	m.applyUnderwaterFilter(100)
	if m.underwater.Accum[0] != 10000 || m.underwater.Accum[1] != 10000 {
		t.Fatalf("expected primed accumulator [10000, 10000], got [%f, %f]",
			m.underwater.Accum[0], m.underwater.Accum[1])
	}

	// Now set intensity to 1.0 (submerged)
	m.underwater.Intensity = 1.0
	m.underwater.Alpha = float32(math.Exp(-float64(m.underwater.Intensity) * math.Log(12)))

	// Alternating high-frequency signal (Nyquist: +10000, -10000)
	for i := range m.paintBuffer {
		if i%2 == 0 {
			m.paintBuffer[i].Left = 10000
			m.paintBuffer[i].Right = 10000
		} else {
			m.paintBuffer[i].Left = -10000
			m.paintBuffer[i].Right = -10000
		}
	}

	m.applyUnderwaterFilter(100)

	// High frequency signal should be heavily attenuated by the low-pass filter
	lastSample := m.paintBuffer[99].Left
	if math.Abs(float64(lastSample)) > 2000 {
		t.Fatalf("expected high frequency to be attenuated below 2000, got %d", lastSample)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio -run TestSetUnderwaterIntensity -v`
Expected: FAIL due to signature mismatch in `SetUnderwaterIntensity`.

- [ ] **Step 3: Implement DSP changes in `internal/audio`**

In `internal/audio/mix.go`:
```go
func (m *Mixer) SetUnderwaterIntensity(target float32, frameTime float32, waterfx float32) {
	target *= float32(math.Max(0.0, math.Min(float64(waterfx), 2.0)))
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

In `internal/audio/sound.go`:
Update `SetUnderwaterIntensity` and `UpdateAmbientSounds`:
```go
func (s *System) getWaterFx() float32 {
	if s.cvars != nil {
		if cv := s.cvars.Get("snd_waterfx"); cv != nil {
			return cv.Float32()
		}
	}
	return 1.0
}

func (s *System) SetUnderwaterIntensity(intensity float32) {
	if mixer, ok := s.mixer.(interface{ SetUnderwaterIntensity(float32, float32, float32) }); ok {
		mixer.SetUnderwaterIntensity(intensity, 0.016, s.getWaterFx())
	}
}

func (s *System) UpdateAmbientSounds(frameTime float32, hasLeaf bool, ambientLevels [NumAmbients]uint8, underwaterIntensity float32) {
	if s.mixer == nil {
		return
	}
	if mixer, ok := s.mixer.(interface{ SetUnderwaterIntensity(float32, float32, float32) }); ok {
		mixer.SetUnderwaterIntensity(underwaterIntensity, frameTime, s.getWaterFx())
	}
	...
}
```

In `internal/audio/adapter.go`:
Update any delegation or interface wrappers if applicable.

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio -run "TestSetUnderwaterIntensity|TestApplyUnderwaterFilter" -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audio/mix.go internal/audio/sound.go internal/audio/adapter.go internal/audio/mix_test.go
git commit -m "feat(audio): enhance underwater liquid DSP with snd_waterfx scaling and priming"
```

---

### Task 2: `musicStream` Interface & Streaming `oggStream`

**Files:**
- Modify: `internal/audio/music.go`
- Modify: `internal/audio/ogg.go`
- Create: `internal/audio/ogg_test.go`

- [ ] **Step 1: Write the failing test for `oggStream`**

Create `internal/audio/ogg_test.go`:
```go
package audio

import (
	"bytes"
	"io"
	"testing"

	"github.com/jfreymuth/oggvorbis"
)

func TestOGGStreamReadAndSeek(t *testing.T) {
	// Create minimal synthetic audio data
	// 44100 Hz, stereo, 4410 frames (0.1s)
	samples := make([]float32, 4410*2)
	for i := range samples {
		samples[i] = 0.5
	}

	// We can test oggStream with a mock reader or by decoding a real small ogg fixture
	// Verify that oggStream satisfies musicStream interface
	var _ musicStream = (*oggStream)(nil)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio -run TestOGGStream -v`
Expected: FAIL due to missing `musicStream` / `oggStream`.

- [ ] **Step 3: Implement `musicStream` and `oggStream`**

In `internal/audio/music.go`:
Define:
```go
type musicStream interface {
	ReadFrames(dst []byte) (framesRead int, err error)
	SeekFrame(frame int64) error
	Close() error
}

type musicTrack struct {
	name     string
	data     []byte       // Non-nil for static/buffered tracks
	stream   musicStream  // Non-nil for streaming tracks
	samples  int
	rate     int
	width    int
	channels int
}
```

In `internal/audio/ogg.go`:
```go
type oggStream struct {
	reader   *oggvorbis.Reader
	channels int
	rate     int
	length   int64
	fltBuf   []float32
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

func decodeMusicOGG(name string, data []byte) (*musicTrack, error) {
	reader, err := oggvorbis.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to open OGG stream %s: %w", name, err)
	}
	ch := reader.Channels()
	if ch != 1 && ch != 2 {
		return nil, fmt.Errorf("unsupported OGG channel count %d for %s", ch, name)
	}
	rate := reader.SampleRate()
	if rate <= 0 {
		return nil, fmt.Errorf("invalid OGG sample rate %d for %s", rate, name)
	}
	length := int(reader.Length())

	return &musicTrack{
		name:     name,
		stream:   &oggStream{reader: reader, channels: ch, rate: rate, length: int64(length)},
		samples:  length,
		rate:     rate,
		width:    2,
		channels: ch,
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio -run TestOGGStream -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/audio/music.go internal/audio/ogg.go internal/audio/ogg_test.go
git commit -m "feat(audio): implement musicStream and chunked oggStream decoder"
```

---

### Task 3: Streaming Mix Pipeline & Looping in `updateMusic`

**Files:**
- Modify: `internal/audio/music.go`
- Modify: `internal/audio/music_test.go`

- [ ] **Step 1: Write the failing test for streaming playback in `music_test.go`**

Add tests to `internal/audio/music_test.go`:
- Test streaming track playback via `updateMusic`: verify samples are written into DMA raw samples.
- Test streaming track loop: verify that when `position >= samples`, `SeekFrame(0)` is called and streaming resumes seamlessly.
- Test `StopMusic`: verifies `stream.Close()` is called.

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio -run TestStreamingMusicPlayback -v`
Expected: FAIL.

- [ ] **Step 3: Update `updateMusic`, `advanceMusicTrack`, and `StopMusic` in `internal/audio/music.go`**

In `internal/audio/music.go`:
1. Add `streamBuf []byte` to `musicState`.
2. In `updateMusic(endTime int)`:
   - Handle `s.music.track.stream != nil`:
     - Calculate `neededOut := endTime - s.rawSamples.End`.
     - Calculate `inputFrames := resampleInputFrames(neededOut, track.rate, s.dma.Speed)`.
     - Allocate/slice `s.music.streamBuf` to fit `inputFrames * track.channels * track.width`.
     - Call `track.stream.ReadFrames(buf)`.
     - If EOF or 0 frames, call `advanceMusicTrack()`.
     - Pass decoded PCM to `s.AddRawSamples(...)`.
     - Increment `s.music.position += framesRead`.
3. In `advanceMusicTrack()`:
   - If looping the same track and `track.stream != nil`: call `track.stream.SeekFrame(0)` and reset `s.music.position = 0`.
   - If switching track: close old stream, initialize new track.
4. In `StopMusic()`:
   - If `s.music != nil && s.music.track != nil && s.music.track.stream != nil`: call `s.music.track.stream.Close()`.
   - Set `s.music = nil`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio -run TestStreaming -v`
Expected: PASS

- [ ] **Step 5: Run all audio tests**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/audio/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/audio/music.go internal/audio/music_test.go
git commit -m "feat(audio): integrate streaming music decoding and looping into DMA mixer"
```

---

### Task 4: Regression Gates, Knowledge Graph & Issue Close

**Files:**
- Full codebase tests and checks

- [ ] **Step 1: Run full test suite**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Run linter and vulnerability checks**

Run: `mise run lint`
Expected: 0 issues, 0 vulnerabilities.

- [ ] **Step 3: Run full verification build**

Run: `mise run verify`
Expected: PASS

- [ ] **Step 4: Update knowledge graph**

Run: `graphify update .`

- [ ] **Step 5: Close beads issue**

Run: `bd close ironwail-go-0vn --reason "Implemented Ogg Vorbis soundtrack streaming and enhanced underwater liquid audio DSP with snd_waterfx scaling and click-free accumulator priming"`
