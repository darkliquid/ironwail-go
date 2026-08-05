// Package state implements the server-side connection/signon state helpers
// that are pure over wire primitives, extracted from the monolithic server
// package (plan 16b §10.2 item 8, server/state).
//
// SignonWriter owns the shared signon buffer construction that populates the
// initial game state sent to every connecting client. It is deliberately
// dependency-injected: the parent server supplies the protocol-flag predicate
// and the signon buffer size so this package imports neither the parent nor
// any engine-global state.
package state

import (
	"fmt"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// ProtocolFlagsFunc reports the server's wire-protocol flags (PRFL_*), which
// determine coordinate/angle wire sizes during signon writing.
type ProtocolFlagsFunc func() uint32

// SignonWriter accumulates the server's shared signon buffers. Each buffer
// holds a segment of the initial game state (precaches, baselines, statics);
// buffers are appended when one fills, up to MaxSignonBuffers segments.
type SignonWriter struct {
	// buffers/current point at the parent server's SignonBuffers/Signon
	// fields, which tests and the msg_init builtin treat as authoritative
	// storage. The writer operates on them so there is a single source of
	// truth.
	buffers *[]*srvtypes.MessageBuffer
	current **srvtypes.MessageBuffer
	// signonSize is the per-segment capacity (parent's SignonSize).
	signonSize int
	// flags reports wire-protocol flags for coord/angle sizing.
	flags ProtocolFlagsFunc
}

// NewSignonWriter creates a signon writer bound to the caller's buffers and
// current-pointer storage. signonSize is the per-segment capacity; flags
// reports PRFL_* for coord/angle wire sizing.
func NewSignonWriter(buffers *[]*srvtypes.MessageBuffer, current **srvtypes.MessageBuffer, signonSize int, flags ProtocolFlagsFunc) *SignonWriter {
	return &SignonWriter{buffers: buffers, current: current, signonSize: signonSize, flags: flags}
}

// Buffers returns the accumulated signon segments in order.
func (w *SignonWriter) Buffers() []*srvtypes.MessageBuffer {
	if w.buffers == nil {
		return nil
	}
	return *w.buffers
}

// Current returns the segment currently being written, or nil.
func (w *SignonWriter) Current() *srvtypes.MessageBuffer {
	if w.current == nil {
		return nil
	}
	return *w.current
}

// Reset clears all signon segments.
func (w *SignonWriter) Reset() {
	if w.buffers != nil {
		*w.buffers = nil
	}
	if w.current != nil {
		*w.current = nil
	}
}

// AddBuffer appends a fresh signon segment and makes it current.
func (w *SignonWriter) AddBuffer() error {
	if w.buffers == nil {
		return fmt.Errorf("SV_AddSignonBuffer: unbounded writer")
	}
	if len(*w.buffers) >= srvtypes.MaxSignonBuffers {
		return fmt.Errorf("SV_AddSignonBuffer: overflow (%d buffers)", srvtypes.MaxSignonBuffers)
	}
	buf := srvtypes.NewMessageBuffer(w.signonSize)
	*w.buffers = append(*w.buffers, buf)
	if w.current != nil {
		*w.current = buf
	}
	return nil
}

// ReserveSpace ensures the current segment has room for size bytes,
// allocating a new segment if it would overflow. Mirrors
// SV_ReserveSignonSpace in C Ironwail (sv_main.c:1503).
func (w *SignonWriter) ReserveSpace(size int) error {
	cur := w.Current()
	if cur == nil {
		return w.AddBuffer()
	}
	if cur.Len()+size > len(cur.Data) {
		return w.AddBuffer()
	}
	return nil
}

// WriteByte writes a single byte to the current signon segment.
func (w *SignonWriter) WriteByte(b byte) error {
	if err := w.ReserveSpace(1); err != nil {
		return err
	}
	w.Current().PutByte(b)
	return nil
}

// WriteShort writes a 16-bit integer to the current signon segment.
func (w *SignonWriter) WriteShort(v int16) error {
	if err := w.ReserveSpace(2); err != nil {
		return err
	}
	w.Current().WriteShort(v)
	return nil
}

// WriteLong writes a 32-bit integer to the current signon segment.
func (w *SignonWriter) WriteLong(v int32) error {
	if err := w.ReserveSpace(4); err != nil {
		return err
	}
	w.Current().WriteLong(v)
	return nil
}

// WriteFloat writes a 32-bit float to the current signon segment.
func (w *SignonWriter) WriteFloat(f float32) error {
	if err := w.ReserveSpace(4); err != nil {
		return err
	}
	w.Current().WriteFloat(f)
	return nil
}

// WriteString writes a null-terminated string to the current signon segment.
func (w *SignonWriter) WriteString(str string) error {
	if err := w.ReserveSpace(len(str) + 1); err != nil {
		return err
	}
	w.Current().WriteString(str)
	return nil
}

// WriteCoord writes a coordinate to the current signon segment, sized per the
// protocol flags.
func (w *SignonWriter) WriteCoord(c float32) error {
	flags := uint32(0)
	if w.flags != nil {
		flags = w.flags()
	}
	if err := w.ReserveSpace(srvtypes.CoordWireSize(flags)); err != nil {
		return err
	}
	w.Current().WriteCoord(c, flags)
	return nil
}

// WriteAngle writes an angle to the current signon segment, sized per the
// protocol flags.
func (w *SignonWriter) WriteAngle(a float32) error {
	flags := uint32(0)
	if w.flags != nil {
		flags = w.flags()
	}
	if err := w.ReserveSpace(srvtypes.AngleWireSize(flags)); err != nil {
		return err
	}
	w.Current().WriteAngle(a, flags)
	return nil
}

// WriteData writes raw bytes to the current signon segment.
func (w *SignonWriter) WriteData(data []byte) error {
	if err := w.ReserveSpace(len(data)); err != nil {
		return err
	}
	w.Current().Write(data)
	return nil
}
