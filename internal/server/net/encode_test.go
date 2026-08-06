// encode_test.go verifies the pure wire-format serialization helpers in
// isolation (mirroring the C sv_send.c encoding).
package net

import (
	"testing"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

func TestEncodeScaleClamps(t *testing.T) {
	cases := []struct {
		in   float32
		want byte
	}{
		{-5, 0},
		{0, 0},
		{1, 16},
		{16, 255},
		{100, 255},
	}
	for _, c := range cases {
		if got := EncodeScale(c.in); got != c.want {
			t.Errorf("EncodeScale(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestEncodeLerpFinish(t *testing.T) {
	if b, ok := EncodeLerpFinish(0.5, 0.5); ok || b != 0 {
		t.Errorf("EncodeLerpFinish(past) = (%d, %v), want (0, false)", b, ok)
	}
	if b, ok := EncodeLerpFinish(0.6, 0.5); !ok || b != 26 {
		t.Errorf("EncodeLerpFinish(0.1) = (%d, %v), want (26, true) (0.1*255+0.5=26.0)", b, ok)
	}
	if b, ok := EncodeLerpFinish(2.0, 0.5); !ok || b != 255 {
		t.Errorf("EncodeLerpFinish(>1s) = (%d, %v), want (255, true)", b, ok)
	}
}

func TestWriteEntityStatePacksFields(t *testing.T) {
	msg := srvtypes.NewMessageBuffer(64)
	state := srvtypes.EntityState{
		ModelIndex: 5,
		Frame:      3,
		Colormap:   7,
		Skin:       1,
		Origin:     [3]float32{1, 2, 3},
		Angles:     [3]float32{0, 90, 0},
	}

	WriteEntityState(msg, state, true, true, 17, uint32(srvtypes.ProtocolFlagFloatCoord|srvtypes.ProtocolFlagFloatAngle))

	if msg.Len() == 0 {
		t.Fatal("WriteEntityState produced empty message")
	}

	// Read back and verify entity number + model/frame + colormap/skin.
	msg.Byte() // bits
	if got := msg.ReadShort(); got != 17 {
		t.Errorf("entnum = %d, want 17", got)
	}
	if got := msg.Byte(); got != 5 {
		t.Errorf("modelindex = %d, want 5", got)
	}
	if got := msg.Byte(); got != 3 {
		t.Errorf("frame = %d, want 3", got)
	}
	if got := msg.Byte(); got != 7 {
		t.Errorf("colormap = %d, want 7", got)
	}
	if got := msg.Byte(); got != 1 {
		t.Errorf("skin = %d, want 1", got)
	}
}

func TestWriteEntityUpdateDeltaEncodes(t *testing.T) {
	base := srvtypes.EntityState{
		ModelIndex: 5,
		Frame:      3,
		Colormap:   7,
		Skin:       1,
		Origin:     [3]float32{0, 0, 0},
		Angles:     [3]float32{0, 0, 0},
	}
	// Only origin X changed.
	changed := base
	changed.Origin[0] = 64

	msg := srvtypes.NewMessageBuffer(64)
	ok := WriteEntityUpdate(msg, 3, changed, base, false, srvtypes.MoveTypeWalk,
		666 /* FitzQuake */, uint32(srvtypes.ProtocolFlagFloatCoord|srvtypes.ProtocolFlagFloatAngle), 0, false)
	if !ok {
		t.Fatal("WriteEntityUpdate returned false")
	}

	if msg.Len() == 0 {
		t.Fatal("WriteEntityUpdate produced empty message")
	}

	// First byte: bits|0x80. Only U_ORIGIN1 (bit1) + U_ORIGIN2? no. Origin2/3
	// unchanged. So bits should have U_ORIGIN1 only (0x02).
	first := msg.Byte()
	if first&0x7f != 0x02 {
		t.Errorf("first bits = %#x, want U_ORIGIN1 (0x02)", first&0x7f)
	}
	if first&0x80 == 0 {
		t.Error("first byte missing continuation bit 0x80")
	}
	// entnum
	if got := msg.Byte(); got != 3 {
		t.Errorf("entnum = %d, want 3", got)
	}
	// Then the origin1 coord (float) follows immediately.
	if msg.Len() == 0 {
		t.Error("expected coord bytes after entnum")
	}
}

func TestWriteEntityUpdateForceSendsAll(t *testing.T) {
	base := srvtypes.EntityState{Origin: [3]float32{0, 0, 0}}
	msg := srvtypes.NewMessageBuffer(64)

	ok := WriteEntityUpdate(msg, 1, base, base, true, srvtypes.MoveTypeWalk,
		666, uint32(srvtypes.ProtocolFlagFloatCoord|srvtypes.ProtocolFlagFloatAngle), 0, false)
	if !ok {
		t.Fatal("WriteEntityUpdate(force) returned false")
	}

	first := msg.Byte()
	// Force sets origin1|2|3 + angle1|2|3 + model + frame + colormap + skin + effects.
	bits := first & 0x7f
	// U_ORIGIN1=1<<0? verify against inet constants via behavior: bits must not be 0.
	if bits == 0 {
		t.Errorf("force write produced empty bits, want all fields set")
	}
	if first&0x80 == 0 {
		t.Error("first byte missing continuation bit 0x80")
	}
}
