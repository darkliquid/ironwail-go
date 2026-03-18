package server

import (
	"bytes"
	"testing"

	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestEncodeAlpha(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float32
		want byte
	}{
		{name: "zero", in: 0.0, want: 1},
		{name: "half", in: 0.5, want: 128},
		{name: "one", in: 1.0, want: 255},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := encodeAlpha(tc.in); got != tc.want {
				t.Fatalf("encodeAlpha(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEncodeScale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   float32
		want byte
	}{
		{name: "one", in: 1.0, want: 16},
		{name: "two", in: 2.0, want: 32},
		{name: "zero", in: 0.0, want: 0},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := encodeScale(tc.in); got != tc.want {
				t.Fatalf("encodeScale(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestEntityStateForClient_AlphaScaleDefaultsWhenFieldsMissing(t *testing.T) {
	t.Parallel()

	s := &Server{
		QCVM:         newTestQCVM(),
		QCFieldAlpha: -1,
		QCFieldScale: -1,
	}
	ent := &Edict{
		Vars:  &EntVars{},
		Alpha: 77,
		Scale: 99,
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 0 {
		t.Fatalf("state.Alpha = %d, want 0", state.Alpha)
	}
	if state.Scale != 16 {
		t.Fatalf("state.Scale = %d, want 16", state.Scale)
	}
}

func TestEntityStateForClient_ReadsQCAlphaScale(t *testing.T) {
	t.Parallel()

	vm := newTestQCVM()
	vm.SetEFloat(1, 0, 0.5) // alpha
	vm.SetEFloat(1, 1, 2.0) // scale

	s := &Server{
		QCVM:         vm,
		QCFieldAlpha: 0,
		QCFieldScale: 1,
	}
	ent := &Edict{
		Vars: &EntVars{},
	}

	state, ok := s.entityStateForClient(1, ent)
	if !ok {
		t.Fatal("entityStateForClient returned ok=false")
	}
	if state.Alpha != 128 {
		t.Fatalf("state.Alpha = %d, want 128", state.Alpha)
	}
	if state.Scale != 32 {
		t.Fatalf("state.Scale = %d, want 32", state.Scale)
	}
}

func newTestQCVM() *qc.VM {
	vm := &qc.VM{
		NumEdicts: 2,
		EdictSize: 28 + 8, // prefix + 2 float fields
	}
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)
	return vm
}

func TestEncodeLerpFinish(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		nextThink float32
		time      float32
		want      byte
		ok        bool
	}{
		{name: "zero delta omitted", nextThink: 10.0, time: 10.0, want: 0, ok: false},
		{name: "half second", nextThink: 10.5, time: 10.0, want: 128, ok: true},
		{name: "clamped to one second", nextThink: 12.0, time: 10.0, want: 255, ok: true},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := encodeLerpFinish(tc.nextThink, tc.time)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("encodeLerpFinish(%v, %v) = (%d, %v), want (%d, %v)", tc.nextThink, tc.time, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestWriteEntityUpdate_FieldOrderMatchesCProtocol(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	state := EntityState{
		Origin:     [3]float32{1.25, 2.5, 3.75},
		Angles:     [3]float32{10, 20, 30},
		ModelIndex: 0x123,
		Frame:      0x234,
		Colormap:   4,
		Skin:       5,
		Effects:    6,
		Alpha:      7,
		Scale:      8,
	}

	msg := NewMessageBuffer(512)
	if !s.writeEntityUpdate(msg, 1, state, EntityState{}, true, 200, true) {
		t.Fatal("writeEntityUpdate returned false")
	}

	_, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	want := NewMessageBuffer(512)
	flags := uint32(s.ProtocolFlags())
	want.WriteByte(byte(state.ModelIndex))
	want.WriteByte(byte(state.Frame))
	want.WriteByte(byte(state.Colormap))
	want.WriteByte(byte(state.Skin))
	want.WriteByte(byte(state.Effects))
	want.WriteCoord(state.Origin[0], flags)
	want.WriteAngle(state.Angles[0], flags)
	want.WriteCoord(state.Origin[1], flags)
	want.WriteAngle(state.Angles[1], flags)
	want.WriteCoord(state.Origin[2], flags)
	want.WriteAngle(state.Angles[2], flags)
	want.WriteByte(state.Alpha)
	want.WriteByte(state.Scale)
	want.WriteByte(byte(state.Frame >> 8))
	want.WriteByte(byte(state.ModelIndex >> 8))
	want.WriteByte(200)

	if !bytes.Equal(payload, want.Data[:want.Len()]) {
		t.Fatalf("payload order mismatch:\n got: %v\nwant: %v", payload, want.Data[:want.Len()])
	}
}

func TestWriteEntityUpdate_OriginsAnglesInterleaved(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	state := EntityState{
		Origin: [3]float32{10, 20, 30},
		Angles: [3]float32{40, 50, 60},
	}
	prev := state
	prev.Origin = [3]float32{}
	prev.Angles = [3]float32{}

	msg := NewMessageBuffer(256)
	if !s.writeEntityUpdate(msg, 1, state, prev, false, 0, false) {
		t.Fatal("writeEntityUpdate returned false")
	}

	_, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	want := NewMessageBuffer(256)
	flags := uint32(s.ProtocolFlags())
	want.WriteCoord(state.Origin[0], flags)
	want.WriteAngle(state.Angles[0], flags)
	want.WriteCoord(state.Origin[1], flags)
	want.WriteAngle(state.Angles[1], flags)
	want.WriteCoord(state.Origin[2], flags)
	want.WriteAngle(state.Angles[2], flags)

	if !bytes.Equal(payload, want.Data[:want.Len()]) {
		t.Fatalf("origin/angle interleave mismatch:\n got: %v\nwant: %v", payload, want.Data[:want.Len()])
	}
}

func TestWriteEntityUpdate_Frame2Model2AfterAlphaScale(t *testing.T) {
	t.Parallel()

	s := &Server{Protocol: ProtocolFitzQuake}
	state := EntityState{
		ModelIndex: 0x345,
		Frame:      0x267,
		Alpha:      0x89,
		Scale:      0x9a,
	}
	prev := EntityState{
		ModelIndex: 1,
		Frame:      1,
		Alpha:      0,
		Scale:      16,
	}

	msg := NewMessageBuffer(256)
	if !s.writeEntityUpdate(msg, 1, state, prev, false, 0, false) {
		t.Fatal("writeEntityUpdate returned false")
	}

	_, payload := decodeEntityUpdateBitsAndPayload(t, msg.Data[:msg.Len()])

	// Byte fields only in this test: MODEL, FRAME, ALPHA, SCALE, FRAME2, MODEL2
	want := []byte{
		byte(state.ModelIndex),
		byte(state.Frame),
		state.Alpha,
		state.Scale,
		byte(state.Frame >> 8),
		byte(state.ModelIndex >> 8),
	}

	if !bytes.Equal(payload, want) {
		t.Fatalf("FRAME2/MODEL2 order mismatch:\n got: %v\nwant: %v", payload, want)
	}
}

func decodeEntityUpdateBitsAndPayload(t *testing.T, data []byte) (uint32, []byte) {
	t.Helper()
	if len(data) < 2 {
		t.Fatalf("short entity update: %v", data)
	}
	i := 0
	first := data[i]
	i++
	bits := uint32(first & 0x7f)
	if bits&inet.U_MOREBITS != 0 {
		if i >= len(data) {
			t.Fatalf("missing morebits byte in %v", data)
		}
		bits |= uint32(data[i]) << 8
		i++
	}
	if bits&inet.U_EXTEND1 != 0 {
		if i >= len(data) {
			t.Fatalf("missing extend1 byte in %v", data)
		}
		bits |= uint32(data[i]) << 16
		i++
	}
	if bits&inet.U_EXTEND2 != 0 {
		if i >= len(data) {
			t.Fatalf("missing extend2 byte in %v", data)
		}
		bits |= uint32(data[i]) << 24
		i++
	}
	if bits&inet.U_LONGENTITY != 0 {
		i += 2
	} else {
		i++
	}
	if i > len(data) {
		t.Fatalf("invalid entity header in %v", data)
	}
	return bits, data[i:]
}
