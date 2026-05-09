package audio

import (
	"fmt"
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
		BufferSize:   50 * time.Millisecond, // Low-latency ALSA/system buffer
	}
	ctx, ready, err := oto.NewContext(op)
	if err != nil {
		return nil, err
	}

	select {
	case <-ready:
		// OK
	case <-time.After(2 * time.Second):
		return nil, fmt.Errorf("oto context readiness timeout")
	}

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
	// Oto default player buffer is 0.5s which adds ~500ms latency.
	// Reduce to ~50ms for responsive game audio without underrunning.
	bytesPerFrame := channels * (sampleBits / 8)
	player.SetBufferSize(sampleRate / 20 * bytesPerFrame) // ~50ms
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
	player := b.player
	pipeW := b.pipeW
	pipeR := b.pipeR
	b.player = nil
	b.pipeW = nil
	b.pipeR = nil
	b.ctx = nil
	b.dma = nil
	b.pos = 0
	b.mu.Unlock()

	if player != nil {
		player.Pause()
	}
	if quit != nil {
		close(quit)
	}
	if pipeW != nil {
		_ = pipeW.Close()
	}
	if pipeR != nil {
		_ = pipeR.Close()
	}
	b.wg.Wait()
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

func (b *OtoBackend) Position() int {
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
	player := b.player
	b.mu.Unlock()
	if ctx != nil {
		_ = ctx.Resume()
	}
	if player != nil {
		player.Play()
	}
}

func (b *OtoBackend) ResetQueuedAudio() {
	b.mu.Lock()
	player := b.player
	blocked := b.blocked
	b.mu.Unlock()
	if player == nil || blocked {
		return
	}
	player.Reset()
	go player.Play()
}

func (b *OtoBackend) streamLoop() {
	defer b.wg.Done()

	dma := b.dma
	if dma == nil || b.sampleRate <= 0 {
		return
	}

	const chunkFrames = 256
	bytesPerFrame := b.channels * (b.sampleBits / 8)
	chunkBytes := chunkFrames * bytesPerFrame
	out := make([]byte, chunkBytes)
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
			pipeW := b.pipeW
			b.mu.Unlock()
			if blocked {
				continue
			}
			if pipeW == nil {
				return
			}

			dma.mu.Lock()
			if len(dma.Buffer) == 0 || b.bufferSize <= 0 {
				dma.mu.Unlock()
				continue
			}

			bytePos := (b.pos % b.bufferSize) * bytesPerFrame
			if bytePos+chunkBytes <= len(dma.Buffer) {
				copy(out, dma.Buffer[bytePos:bytePos+chunkBytes])
			} else {
				first := len(dma.Buffer) - bytePos
				copy(out[:first], dma.Buffer[bytePos:])
				copy(out[first:], dma.Buffer[:chunkBytes-first])
			}

			b.pos = (b.pos + chunkFrames) % b.bufferSize
			dma.SamplePos = b.pos
			dma.mu.Unlock()

			if _, err := pipeW.Write(out); err != nil {
				return
			}
		}
	}
}
