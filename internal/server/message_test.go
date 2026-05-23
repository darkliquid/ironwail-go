package server

import "testing"

func TestMessageBufferWriteCoordUsesQuakeRounding(t *testing.T) {
	tests := []struct {
		name  string
		value float32
		flags uint32
		want  int32
	}{
		{name: "default positive half", value: 0.0625, want: 1},
		{name: "default negative half", value: -0.0625, want: -1},
		{name: "int32 positive half", value: 0.03125, flags: uint32(ProtocolFlagInt32Coord), want: 1},
		{name: "int32 negative half", value: -0.03125, flags: uint32(ProtocolFlagInt32Coord), want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessageBuffer(8)
			msg.WriteCoord(tt.value, tt.flags)
			msg.ReadPos = 0
			var got int32
			if tt.flags&uint32(ProtocolFlagInt32Coord) != 0 {
				got = msg.ReadLong()
			} else {
				got = int32(msg.ReadShort())
			}
			if got != tt.want {
				t.Fatalf("encoded coord = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestMessageBufferWriteReadAngleMatchesQuakeRoundingAndSignedRead(t *testing.T) {
	tests := []struct {
		name      string
		angle     float32
		flags     uint32
		wantWire  int32
		wantAngle float32
	}{
		{name: "byte rounds to nearest", angle: 1.4, wantWire: 1, wantAngle: 360.0 / 256.0},
		{name: "byte wraps rounded full turn", angle: 359.9, wantWire: 0, wantAngle: 0},
		{name: "byte reads signed char", angle: -1.4, wantWire: 255, wantAngle: -360.0 / 256.0},
		{name: "short rounds to nearest", angle: 0.003, flags: uint32(ProtocolFlagShortAngle), wantWire: 1, wantAngle: 360.0 / 65536.0},
		{name: "short negative reads signed short", angle: -0.003, flags: uint32(ProtocolFlagShortAngle), wantWire: -1, wantAngle: -360.0 / 65536.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewMessageBuffer(8)
			msg.WriteAngle(tt.angle, tt.flags)
			msg.ReadPos = 0
			var gotWire int32
			if tt.flags&uint32(ProtocolFlagShortAngle) != 0 {
				gotWire = int32(msg.ReadShort())
			} else {
				gotWire = int32(msg.Byte())
			}
			if gotWire != tt.wantWire {
				t.Fatalf("wire angle = %d, want %d", gotWire, tt.wantWire)
			}
			msg.ReadPos = 0
			if gotAngle := msg.ReadAngle(tt.flags); gotAngle != tt.wantAngle {
				t.Fatalf("decoded angle = %f, want %f", gotAngle, tt.wantAngle)
			}
		})
	}
}
