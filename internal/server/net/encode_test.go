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
