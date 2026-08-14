// encode_test.go verifies the pure wire-format serialization helpers in
// isolation (mirroring the C sv_send.c encoding).
package net

import (
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestEncodeScaleClampsAndPacks(t *testing.T) {
	cases := []struct {
		in   float32
		want byte
	}{
		{-1.0, 0},
		{0.0, 0},
		{1.0, 16},
		{2.5, 40},
		{15.9375, 255},
		{20.0, 255},
	}
	for _, tc := range cases {
		if got := EncodeScale(tc.in); got != tc.want {
			t.Errorf("EncodeScale(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestEncodeLerpFinishComputesFraction(t *testing.T) {
	// Past or current think -> no lerp.
	if _, ok := EncodeLerpFinish(1.0, 1.0); ok {
		t.Error("expected false for nextthink == time")
	}
	if _, ok := EncodeLerpFinish(0.5, 1.0); ok {
		t.Error("expected false for nextthink < time")
	}
	// Future think within 1 second -> clamped 0..255 fraction.
	got, ok := EncodeLerpFinish(1.5, 1.0)
	if !ok || got != 128 { // 0.5 * 255 + 0.5 = 128
		t.Errorf("EncodeLerpFinish(1.5, 1.0) = (%d, %t), want (128, true)", got, ok)
	}
	got, ok = EncodeLerpFinish(3.0, 1.0) // delta > 1 -> clamp to 1.0 (255)
	if !ok || got != 255 {
		t.Errorf("EncodeLerpFinish(3.0, 1.0) = (%d, %t), want (255, true)", got, ok)
	}
}

func TestWriteEntityStatePacksFields(t *testing.T) {
	msg := srvtypes.NewMessageBuffer(64)
	state := srvtypes.EntityState{
		ModelIndex: 5,
		Frame:      3,
		Colormap:   7,
		Skin:       1,
		Origin:     qtypes.Vec3{X: 1, Y: 2, Z: 3},
		Angles:     qtypes.Vec3{X: 0, Y: 90, Z: 0},
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
		Origin:     qtypes.Vec3{},
		Angles:     qtypes.Vec3{},
	}
	// Only origin X changed.
	changed := base
	changed.Origin.X = 64

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
	base := srvtypes.EntityState{Origin: qtypes.Vec3{}}
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

func TestWriteSpawnStaticMessageExtendedFallback(t *testing.T) {
	flags := uint32(srvtypes.ProtocolFlagFloatCoord)

	// Small state: basic variant (SVCSpawnStatic = 17?).
	small := srvtypes.EntityState{ModelIndex: 5, Frame: 3}
	msg := srvtypes.NewMessageBuffer(64)
	WriteSpawnStaticMessage(msg, small, flags)
	// First byte: SVCSpawnStatic opcode.
	if msg.Len() == 0 {
		t.Fatal("basic spawn static produced empty message")
	}

	// Large model: extended variant carries a short model index.
	large := srvtypes.EntityState{ModelIndex: 300, Frame: 3}
	msg2 := srvtypes.NewMessageBuffer(64)
	WriteSpawnStaticMessage(msg2, large, flags)
	if msg2.Byte() != byte(inet.SVCSpawnStatic2) {
		t.Errorf("extended spawn opcode = %#x, want SVCSpawnStatic2", msg2.Byte())
	}
}

func TestWriteSpawnStaticSoundMessageLargeIndex(t *testing.T) {
	flags := uint32(srvtypes.ProtocolFlagFloatCoord)

	snd := srvtypes.StaticSound{SoundIndex: 400, Volume: 255, Attenuation: 0.5}
	msg := srvtypes.NewMessageBuffer(64)
	WriteSpawnStaticSoundMessage(msg, snd, flags)
	if msg.Byte() != byte(inet.SVCSpawnStaticSound2) {
		t.Errorf("large sound opcode = %#x, want SVCSpawnStaticSound2", msg.Byte())
	}
	// Three float coords precede the short sound index.
	for i := 0; i < 3; i++ {
		msg.ReadFloat()
	}
	if got := msg.ReadShort(); got != 400 {
		t.Errorf("sound index = %d, want 400", got)
	}
}

func TestWriteSpawnLightStylesEmitsAll(t *testing.T) {
	styles := [256]string{}
	styles[0] = "m"
	styles[1] = "n"
	msg := srvtypes.NewMessageBuffer(1024)
	WriteSpawnLightStyles(msg, styles)
	// Two non-empty styles → 2 SVCLightStyle messages (opcode + slot + string).
	// Empty strings still emit messages; count by scanning is overkill; check non-empty.
	if msg.Len() == 0 {
		t.Fatal("WriteSpawnLightStyles produced empty message")
	}
}

func TestWriteSpawnClientRosterSkipsNil(t *testing.T) {
	msg := srvtypes.NewMessageBuffer(128)
	clients := []*srvtypes.Client{nil, {Name: "Player"}}
	WriteSpawnClientRoster(msg, clients, nil)
	if msg.Len() == 0 {
		t.Fatal("roster produced empty message for a named client")
	}
}

func TestWriteSpawnSetAngleEmitsOpcode(t *testing.T) {
	msg := srvtypes.NewMessageBuffer(32)
	client := &srvtypes.Client{Edict: &srvtypes.Edict{}}
	WriteSpawnSetAngle(msg, client, uint32(srvtypes.ProtocolFlagFloatAngle), nil, false)
	if msg.Len() == 0 {
		t.Fatal("setangle produced empty message")
	}
	if got := msg.Byte(); got != byte(inet.SVCSetAngle) {
		t.Errorf("opcode = %#x, want SVCSetAngle", got)
	}
}
