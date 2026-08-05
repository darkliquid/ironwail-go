package client

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/common"
	inet "github.com/darkliquid/ironwail-go/internal/net"
)

// benchEntityUpdateBits is the RMQ entity-update bits value used by the
// benchmarks: signal + model + frame (the most common per-field delta
// subset). It stays within one byte so no morebits byte is needed.
const benchEntityUpdateBits = inet.U_SIGNAL | inet.U_MODEL | inet.U_FRAME

// benchEntityUpdateMsg builds a minimal RMQ entity-update message: bits,
// an entity number, model/ and a frame.
func benchEntityUpdateMsg() *common.SizeBuf {
	payload := []byte{
		byte(benchEntityUpdateBits & 0x7f),
		5, // entnum
		9, // modelindex
		3, // frame
	}
	sb := common.NewSizeBuf(len(payload))
	_ = sb.Write(payload)
	sb.BeginReading()
	return sb
}

// BenchmarkParseEntityUpdate measures per-frame entity-update decoding. The
// manual byte readers must not allocate on the hot line (plan 20.3).
func BenchmarkParseEntityUpdate(b *testing.B) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.ProtocolFlags = 0 // 1-byte coords/angles (default path)
	p := NewParser(c)
	msg := benchEntityUpdateMsg()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg.BeginReading()
		if err := p.parseEntityUpdate(msg, byte(benchEntityUpdateBits&0x7f)); err != nil {
			b.Fatalf("parseEntityUpdate: %v", err)
		}
	}
}

// TestParseEntityUpdateNoAlloc pins zero allocations on the hot decode line.
func TestParseEntityUpdateNoAlloc(t *testing.T) {
	c := NewClient()
	c.Protocol = inet.PROTOCOL_FITZQUAKE
	c.ProtocolFlags = 0 // 1-byte coords/angles (default path)
	p := NewParser(c)
	// Warm up parser + baseline map so the Entities map stops growing.
	for i := 0; i < 200; i++ {
		msg := benchEntityUpdateMsg()
		_ = p.parseEntityUpdate(msg, byte(benchEntityUpdateBits&0x7f))
	}
	// Reuse one pre-built message so the measure isolates parseEntityUpdate
	// (NewSizeBuf allocates the message buffer once, outside the loop).
	msg := benchEntityUpdateMsg()
	allocs := testing.AllocsPerRun(200, func() {
		msg.BeginReading()
		_ = p.parseEntityUpdate(msg, byte(benchEntityUpdateBits&0x7f))
	})
	if allocs > 0 {
		t.Fatalf("parseEntityUpdate allocs/run = %.2f, want 0", allocs)
	}
}
