//go:build js && wasm

package audio

import (
	"log/slog"
	"sync"
	"syscall/js"
)

// WASMAudioBackend implements audio.Backend for WebAssembly browser DOM environments
// using the Web Audio API.
type WASMAudioBackend struct {
	mu         sync.Mutex
	sampleRate int
	sampleBits int
	channels   int
	bufferSize int
	dma        *DMAInfo
	pos        int
	blocked    bool
	audioCtx   js.Value
	scriptNode js.Value
	jsCallback js.Func
}

// NewWASMAudioBackend creates a new Web Audio API backend.
func NewWASMAudioBackend() *WASMAudioBackend {
	return &WASMAudioBackend{}
}

func (b *WASMAudioBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

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

	window := js.Global().Get("window")
	if window.IsUndefined() || window.IsNull() {
		slog.Warn("WASM audio: window object unavailable")
		return dma, nil
	}

	audioCtxClass := window.Get("AudioContext")
	if audioCtxClass.IsUndefined() || audioCtxClass.IsNull() {
		audioCtxClass = window.Get("webkitAudioContext")
	}

	if audioCtxClass.IsUndefined() || audioCtxClass.IsNull() {
		slog.Warn("WASM audio: Web Audio API unavailable")
		return dma, nil
	}

	opts := js.Global().Get("Object").New()
	opts.Set("sampleRate", sampleRate)
	audioCtx := audioCtxClass.New(opts)
	b.audioCtx = audioCtx

	const bufferFrames = 1024
	scriptNode := audioCtx.Call("createScriptProcessor", bufferFrames, 0, channels)
	b.scriptNode = scriptNode

	bytesPerFrame := channels * (sampleBits / 8)

	processFunc := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) == 0 || b.blocked || b.dma == nil {
			return nil
		}

		b.dma.mu.Lock()
		defer b.dma.mu.Unlock()

		if len(b.dma.Buffer) == 0 {
			return nil
		}

		event := args[0]
		outputBuffer := event.Get("outputBuffer")

		var leftChannel, rightChannel js.Value
		if channels >= 1 {
			leftChannel = outputBuffer.Call("getChannelData", 0)
		}
		if channels >= 2 {
			rightChannel = outputBuffer.Call("getChannelData", 1)
		}

		currBytePos := (b.pos % b.bufferSize) * bytesPerFrame
		for i := 0; i < bufferFrames; i++ {
			byteIdx := (currBytePos + i*bytesPerFrame) % len(b.dma.Buffer)

			// Convert 16-bit PCM signed integer sample to float32 [-1.0, 1.0]
			sample16Left := int16(uint16(b.dma.Buffer[byteIdx]) | (uint16(b.dma.Buffer[byteIdx+1]) << 8))
			sampleFloatLeft := float32(sample16Left) / 32768.0

			if !leftChannel.IsUndefined() {
				leftChannel.SetIndex(i, sampleFloatLeft)
			}

			if channels >= 2 && !rightChannel.IsUndefined() {
				sample16Right := int16(uint16(b.dma.Buffer[byteIdx+2]) | (uint16(b.dma.Buffer[byteIdx+3]) << 8))
				sampleFloatRight := float32(sample16Right) / 32768.0
				rightChannel.SetIndex(i, sampleFloatRight)
			}
		}

		b.pos = (b.pos + bufferFrames) % b.bufferSize
		b.dma.SamplePos = b.pos
		return nil
	})

	b.jsCallback = processFunc
	scriptNode.Set("onaudioprocess", processFunc)
	scriptNode.Call("connect", audioCtx.Get("destination"))

	// Browser autoplay policy starts an AudioContext suspended; expose a
	// global resume hook that the DOM input backend invokes on the first
	// user gesture (keydown/click). The hook is idempotent: calling resume()
	// on a running context is a no-op.
	js.Global().Set("__ironwailAudioResume", js.FuncOf(func(this js.Value, args []js.Value) any {
		_ = audioCtx.Call("resume")
		return nil
	}))

	slog.Info("Web Audio API audio backend initialized", "sampleRate", sampleRate, "channels", channels)
	return dma, nil
}

func (b *WASMAudioBackend) Shutdown() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.scriptNode.IsUndefined() && !b.scriptNode.IsNull() {
		b.scriptNode.Call("disconnect")
		b.scriptNode.Set("onaudioprocess", js.Null())
	}
	if b.jsCallback.Value.Truthy() {
		b.jsCallback.Release()
		b.jsCallback = js.Func{}
	}
	if !b.audioCtx.IsUndefined() && !b.audioCtx.IsNull() {
		_ = b.audioCtx.Call("close")
	}
	b.audioCtx = js.Null()
	b.scriptNode = js.Null()
}

func (b *WASMAudioBackend) Lock() {
	if b.dma != nil {
		b.dma.mu.Lock()
	}
}

func (b *WASMAudioBackend) Unlock() {
	if b.dma != nil {
		b.dma.mu.Unlock()
	}
}

func (b *WASMAudioBackend) Position() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.pos
}

func (b *WASMAudioBackend) Block() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked = true
	if !b.audioCtx.IsUndefined() && !b.audioCtx.IsNull() {
		_ = b.audioCtx.Call("suspend")
	}
}

func (b *WASMAudioBackend) Unblock() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.blocked = false
	if !b.audioCtx.IsUndefined() && !b.audioCtx.IsNull() {
		_ = b.audioCtx.Call("resume")
	}
}
