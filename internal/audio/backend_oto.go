//go:build audio_oto && cgo
// +build audio_oto,cgo

package audio

import (
	"io"
	"sync"
	"time"

	"github.com/ebitengine/oto/v3"
)

type OtoBackend struct {
	sampleRate int
	sampleBits int
	channels   int
	bufferSize int

	dma *DMAInfo

	ctx    *oto.Context
	player *oto.Player
	pipeR  *io.PipeReader
	pipeW  *io.PipeWriter

	quit chan struct{}
	wg   sync.WaitGroup

	mu      sync.Mutex
	blocked bool
	pos     int
}

func NewOtoBackend() Backend {
	return &OtoBackend{}
}

func (b *OtoBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	b.sampleRate = sampleRate
	b.sampleBits = sampleBits
	b.channels = channels
	b.bufferSize = bufferSize

	op := &oto.NewContextOptions{
		SampleRate:   sampleRate,
		ChannelCount: channels,
		Format:       oto.FormatSignedInt16LE,
	}
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}
	<-ready

	dma := &DMAInfo{
		Channels:        channels,
		Samples:         bufferSize,
		SubmissionChunk: 1,
		SamplePos:       0,
		SampleBits:      sampleBits,
		Speed:           sampleRate,
		Buffer:          make([]byte, bufferSize*channels*(sampleBits/8)),
	}

	pr, pw := io.Pipe()
	player := ctx.NewPlayer(pr)
	player.Play()

	b.ctx = ctx
	b.player = player
	b.pipeR = pr
	b.pipeW = pw
	b.dma = dma
	b.quit = make(chan struct{})

	b.wg.Add(1)
	go b.streamLoop()

	return dma, nil
}

func (b *OtoBackend) Shutdown() {
	b.mu.Lock()
	quit := b.quit
	b.quit = nil
	b.mu.Unlock()

	if quit != nil {
		close(quit)
	}
	b.wg.Wait()

	if b.player != nil {
		_ = b.player.Close()
		b.player = nil
	}
	if b.pipeW != nil {
		_ = b.pipeW.Close()
		b.pipeW = nil
	}
	if b.pipeR != nil {
		_ = b.pipeR.Close()
		b.pipeR = nil
	}
	b.ctx = nil
	b.dma = nil
}

func (b *OtoBackend) Lock() {
	if b.dma != nil {
		b.dma.mu.Lock()
	}
}

func (b *OtoBackend) Unlock() {
	if b.dma != nil {
		b.dma.mu.Unlock()
	}
}

func (b *OtoBackend) GetPosition() int {
	if b.dma == nil {
		return 0
	}
	b.dma.mu.Lock()
	pos := b.dma.SamplePos
	b.dma.mu.Unlock()
	return pos
}

func (b *OtoBackend) Block() {
	b.mu.Lock()
	b.blocked = true
	ctx := b.ctx
	b.mu.Unlock()
	if ctx != nil {
		_ = ctx.Suspend()
	}
}

func (b *OtoBackend) Unblock() {
	b.mu.Lock()
	b.blocked = false
	ctx := b.ctx
	b.mu.Unlock()
	if ctx != nil {
		_ = ctx.Resume()
	}
}

func (b *OtoBackend) streamLoop() {
	defer b.wg.Done()

	if b.dma == nil || b.pipeW == nil || b.sampleRate <= 0 {
		return
	}

	const chunkFrames = 256
	bytesPerFrame := b.channels * (b.sampleBits / 8)
	chunkBytes := chunkFrames * bytesPerFrame
	period := time.Second * time.Duration(chunkFrames) / time.Duration(b.sampleRate)
	if period <= 0 {
		period = time.Millisecond * 5
	}

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-b.quit:
			return
		case <-ticker.C:
			b.mu.Lock()
			blocked := b.blocked
			b.mu.Unlock()
			if blocked {
				continue
			}

			out := make([]byte, chunkBytes)
			b.dma.mu.Lock()
			if len(b.dma.Buffer) == 0 || b.bufferSize <= 0 {
				b.dma.mu.Unlock()
				continue
			}

			bytePos := (b.pos % b.bufferSize) * bytesPerFrame
			if bytePos+chunkBytes <= len(b.dma.Buffer) {
				copy(out, b.dma.Buffer[bytePos:bytePos+chunkBytes])
			} else {
				first := len(b.dma.Buffer) - bytePos
				copy(out[:first], b.dma.Buffer[bytePos:])
				copy(out[first:], b.dma.Buffer[:chunkBytes-first])
			}

			b.pos = (b.pos + chunkFrames) % b.bufferSize
			b.dma.SamplePos = b.pos
			b.dma.mu.Unlock()

			if _, err := b.pipeW.Write(out); err != nil {
				return
			}
		}
	}
}
