//go:build amd64 && (linux || windows)

package audio

import (
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/samborkent/miniaudio"
)

type MiniaudioBackend struct {
	sampleRate int
	sampleBits int
	channels   int
	bufferSize int

	dma    *DMAInfo
	device *miniaudio.Device

	mu      sync.Mutex
	blocked bool
	pos     int
}

func NewMiniaudioBackend() Backend {
	return &MiniaudioBackend{}
}

func (b *MiniaudioBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	if sampleBits != 16 {
		return nil, fmt.Errorf("miniaudio backend only supports 16-bit PCM, got %d", sampleBits)
	}
	if channels <= 0 {
		return nil, fmt.Errorf("miniaudio backend requires at least one channel")
	}
	if bufferSize <= 0 {
		return nil, fmt.Errorf("miniaudio backend requires a positive buffer size")
	}
	if err := miniaudio.Init(); err != nil {
		return nil, err
	}

	b.sampleRate = sampleRate
	b.sampleBits = sampleBits
	b.channels = channels
	b.bufferSize = bufferSize
	b.pos = 0
	b.blocked = false

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

	config := &miniaudio.DeviceConfig{
		DeviceType: miniaudio.DeviceTypePlayback,
		SampleRate: sampleRate,
		Playback: miniaudio.FormatConfig{
			Format:   miniaudio.FormatInt16,
			Channels: channels,
		},
	}
	if err := miniaudio.SetPlaybackCallback(config, b.playbackCallback); err != nil {
		return nil, err
	}

	device, err := miniaudio.NewDevice(config)
	if err != nil {
		return nil, err
	}
	if err := device.Init(); err != nil {
		return nil, err
	}
	if err := device.Start(); err != nil {
		device.Uninit()
		return nil, err
	}

	b.device = device
	return dma, nil
}

func (b *MiniaudioBackend) Shutdown() {
	b.mu.Lock()
	device := b.device
	b.device = nil
	b.blocked = false
	b.mu.Unlock()

	if device != nil {
		_ = device.Stop()
		device.Uninit()
	}

	b.pos = 0
	b.dma = nil
}

func (b *MiniaudioBackend) Lock() {
	if b.dma != nil {
		b.dma.mu.Lock()
	}
}

func (b *MiniaudioBackend) Unlock() {
	if b.dma != nil {
		b.dma.mu.Unlock()
	}
}

func (b *MiniaudioBackend) GetPosition() int {
	if b.dma == nil {
		return 0
	}
	b.dma.mu.Lock()
	pos := b.dma.SamplePos
	b.dma.mu.Unlock()
	return pos
}

func (b *MiniaudioBackend) Block() {
	b.mu.Lock()
	if b.blocked {
		b.mu.Unlock()
		return
	}
	b.blocked = true
	device := b.device
	b.mu.Unlock()

	if device != nil {
		_ = device.Stop()
	}
}

func (b *MiniaudioBackend) Unblock() {
	b.mu.Lock()
	if !b.blocked {
		b.mu.Unlock()
		return
	}
	b.blocked = false
	device := b.device
	b.mu.Unlock()

	if device != nil {
		_ = device.Start()
	}
}

func (b *MiniaudioBackend) playbackCallback(frameCount, channelCount int) [][]int16 {
	return b.copyFrames(frameCount, channelCount)
}

func (b *MiniaudioBackend) copyFrames(frameCount, channelCount int) [][]int16 {
	frames := make([][]int16, frameCount)
	if frameCount <= 0 || channelCount <= 0 {
		return frames
	}

	samples := make([]int16, frameCount*channelCount)
	for frame := range frameCount {
		start := frame * channelCount
		frames[frame] = samples[start : start+channelCount]
	}

	if b.dma == nil || len(b.dma.Buffer) == 0 || b.bufferSize <= 0 {
		return frames
	}

	b.dma.mu.Lock()
	defer b.dma.mu.Unlock()

	if b.blocked {
		return frames
	}

	bytesPerSample := b.sampleBits / 8
	bytesPerFrame := b.channels * bytesPerSample
	if bytesPerFrame <= 0 {
		return frames
	}

	copyChannels := channelCount
	if copyChannels > b.channels {
		copyChannels = b.channels
	}

	bytePos := (b.pos % b.bufferSize) * bytesPerFrame
	for frame := range frameCount {
		frameBase := frame * channelCount
		for channel := range copyChannels {
			offset := bytePos + channel*bytesPerSample
			samples[frameBase+channel] = int16(binary.LittleEndian.Uint16(b.dma.Buffer[offset : offset+bytesPerSample]))
		}
		bytePos += bytesPerFrame
		if bytePos >= len(b.dma.Buffer) {
			bytePos = 0
		}
	}

	b.pos = (b.pos + frameCount) % b.bufferSize
	b.dma.SamplePos = b.pos

	return frames
}
