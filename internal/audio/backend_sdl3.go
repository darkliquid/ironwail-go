//go:build sdl3
// +build sdl3

package audio

import (
	"fmt"

	sdl "github.com/Zyko0/go-sdl3/sdl"
)

type SDL3Backend struct {
	sampleRate int
	sampleBits int
	channels   int
	bufferSize int

	dma      *DMAInfo
	stream   *sdl.AudioStream
	callback sdl.AudioStreamCallback

	pos int
}

func NewSDL3AudioBackend() Backend {
	return &SDL3Backend{}
}

func (b *SDL3Backend) Init(sampleRate, sampleBits, channels, bufferSize int) (_ *DMAInfo, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("sdl init audio panic: %v", recovered)
		}
	}()

	if err := sdl.InitSubSystem(sdl.INIT_AUDIO); err != nil {
		return nil, fmt.Errorf("sdl init audio: %w", err)
	}

	b.sampleRate = sampleRate
	b.sampleBits = sampleBits
	b.channels = channels
	b.bufferSize = bufferSize

	dma := &DMAInfo{
		Channels:        channels,
		Samples:         bufferSize,
		SubmissionChunk: 1,
		SamplePos:       0,
		SampleBits:      sampleBits,
		Speed:           sampleRate,
		Buffer:          make([]byte, bufferSize*channels*(sampleBits/8)),
	}
	b.dma = dma

	spec := &sdl.AudioSpec{
		Format:   sdl.AUDIO_S16,
		Channels: int32(channels),
		Freq:     int32(sampleRate),
	}

	b.callback = sdl.NewAudioStreamCallback(func(stream *sdl.AudioStream, additionalAmount, totalAmount int32) {
		_ = totalAmount
		b.feedStream(stream, additionalAmount)
	})

	stream := sdl.AudioDeviceID(0).OpenAudioDeviceStream(spec, b.callback)
	if stream == nil {
		sdl.QuitSubSystem(sdl.INIT_AUDIO)
		return nil, fmt.Errorf("failed to open SDL audio stream")
	}
	b.stream = stream

	if err := b.stream.ResumeDevice(); err != nil {
		b.stream.Destroy()
		b.stream = nil
		sdl.QuitSubSystem(sdl.INIT_AUDIO)
		return nil, fmt.Errorf("failed to resume SDL audio stream: %w", err)
	}

	return dma, nil
}

func (b *SDL3Backend) Shutdown() {
	if b.stream != nil {
		b.stream.Destroy()
		b.stream = nil
	}
	b.dma = nil
	sdl.QuitSubSystem(sdl.INIT_AUDIO)
}

func (b *SDL3Backend) Lock() {
	if b.dma != nil {
		b.dma.mu.Lock()
	}
}

func (b *SDL3Backend) Unlock() {
	if b.dma != nil {
		b.dma.mu.Unlock()
	}
}

func (b *SDL3Backend) GetPosition() int {
	if b.dma == nil {
		return 0
	}
	b.dma.mu.Lock()
	pos := b.dma.SamplePos
	b.dma.mu.Unlock()
	return pos
}

func (b *SDL3Backend) Block() {
	if b.stream != nil {
		_ = b.stream.PauseDevice()
	}
}

func (b *SDL3Backend) Unblock() {
	if b.stream != nil {
		_ = b.stream.ResumeDevice()
	}
}

func (b *SDL3Backend) feedStream(stream *sdl.AudioStream, additionalAmount int32) {
	if b.dma == nil || additionalAmount <= 0 {
		return
	}

	bytesPerFrame := b.channels * (b.sampleBits / 8)
	if bytesPerFrame <= 0 || b.bufferSize <= 0 {
		return
	}

	request := int(additionalAmount)
	if request <= 0 {
		return
	}

	out := make([]byte, request)

	b.dma.mu.Lock()
	defer b.dma.mu.Unlock()

	if len(b.dma.Buffer) == 0 {
		_ = stream.PutData(out)
		return
	}

	bytePos := (b.pos % b.bufferSize) * bytesPerFrame
	if bytePos+request <= len(b.dma.Buffer) {
		copy(out, b.dma.Buffer[bytePos:bytePos+request])
	} else {
		first := len(b.dma.Buffer) - bytePos
		copy(out[:first], b.dma.Buffer[bytePos:])
		copy(out[first:], b.dma.Buffer[:request-first])
	}

	advancedFrames := request / bytesPerFrame
	b.pos = (b.pos + advancedFrames) % b.bufferSize
	b.dma.SamplePos = b.pos

	_ = stream.PutData(out)
}
