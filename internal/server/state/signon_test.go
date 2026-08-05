package state

import (
	"testing"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

func TestSignonWriterBuffersAndCurrent(t *testing.T) {
	var buffers []*srvtypes.MessageBuffer
	var current *srvtypes.MessageBuffer
	w := NewSignonWriter(&buffers, &current, 31500, func() uint32 { return 0 })
	if err := w.AddBuffer(); err != nil {
		t.Fatalf("AddBuffer: %v", err)
	}
	if len(buffers) != 1 || current == nil {
		t.Fatalf("buffers=%d current-nil=%v, want 1 / non-nil", len(buffers), current == nil)
	}
	if err := w.WriteByte(0x42); err != nil {
		t.Fatalf("WriteByte: %v", err)
	}
	if current.Len() != 1 || current.Data[0] != 0x42 {
		t.Fatalf("current len=%d data=%x, want 1 / 0x42", current.Len(), current.Data[0])
	}
}

func TestSignonWriterOverflowChainsBuffer(t *testing.T) {
	var buffers []*srvtypes.MessageBuffer
	var current *srvtypes.MessageBuffer
	w := NewSignonWriter(&buffers, &current, 8, func() uint32 { return 0 })
	// Chained writes: each 6-byte string fits, but the second must chain a
	// new segment because the first is full.
	if err := w.WriteString("aaaaaa"); err != nil { // 7 bytes + null
		t.Fatalf("WriteString[0]: %v", err)
	}
	if err := w.WriteString("bbbbbb"); err != nil {
		t.Fatalf("WriteString[1]: %v", err)
	}
	if len(buffers) != 2 {
		t.Fatalf("buffers = %d, want 2 (second string chained a new segment)", len(buffers))
	}
}

func TestSignonWriterResetClearsExternalStorage(t *testing.T) {
	var buffers []*srvtypes.MessageBuffer
	var current *srvtypes.MessageBuffer
	w := NewSignonWriter(&buffers, &current, 64, func() uint32 { return 0 })
	_ = w.AddBuffer()
	w.Reset()
	if len(buffers) != 0 || current != nil {
		t.Fatalf("after Reset: buffers=%d current-nil=%v, want 0 / nil", len(buffers), current == nil)
	}
}
