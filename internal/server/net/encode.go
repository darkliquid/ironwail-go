// encode.go implements pure wire-format serialization helpers for the server
// network layer: entity state bit packing, scale/lerp encoding, and entity
// send sort keys. These were extracted from server_net_send.go so they can be
// unit-tested without a live server.
package net

import (
	"math"

	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// EncodeScale packs a 0..15.9375 scale into a 4-bit byte.
func EncodeScale(a float32) byte {
	if a < 0 {
		a = 0
	}
	if a > 15.9375 {
		a = 15.9375
	}
	return byte(a * 16)
}

// EncodeLerpFinish encodes the interval to an entity's next think as a 0..255
// byte. Returns (0, false) when the think is not in the future.
func EncodeLerpFinish(nextThink, time float32) (byte, bool) {
	delta := nextThink - time
	if delta <= 0 {
		return 0, false
	}
	if delta > 1 {
		delta = 1
	}
	return byte(delta*255.0 + 0.5), true
}

// WriteEntityState serializes one entity state into the message buffer using
// the protocol's wire flags, packing model/frame/alpha/scale extension bits.
func WriteEntityState(msg *srvtypes.MessageBuffer, ent srvtypes.EntityState, extended, includeEntNum bool, entNum int, flags uint32) {
	var bits byte
	if ent.ModelIndex > 255 {
		bits |= 1
	}
	if ent.Frame > 255 {
		bits |= 1 << 1
	}
	if ent.Alpha != 0 {
		bits |= 1 << 2
	}
	if ent.Scale != 0 && ent.Scale != 16 {
		bits |= 1 << 3
	}

	if extended {
		msg.PutByte(bits)
	}
	if includeEntNum {
		msg.WriteShort(int16(entNum))
	}
	if extended && bits&(1<<0) != 0 {
		msg.WriteShort(int16(ent.ModelIndex))
	} else {
		msg.PutByte(byte(ent.ModelIndex))
	}
	if extended && bits&(1<<1) != 0 {
		msg.WriteShort(int16(ent.Frame))
	} else {
		msg.PutByte(byte(ent.Frame))
	}
	msg.PutByte(byte(ent.Colormap))
	msg.PutByte(byte(ent.Skin))
	// Origins and angles must be interleaved: O1, A1, O2, A2, O3, A3
	for i := 0; i < 3; i++ {
		msg.WriteCoord(ent.Origin[i], flags)
		msg.WriteAngle(ent.Angles[i], flags)
	}
	if extended && bits&(1<<2) != 0 {
		msg.PutByte(ent.Alpha)
	}
	if extended && bits&(1<<3) != 0 {
		msg.PutByte(ent.Scale)
	}
}

// EntitySendSortKey computes the distance-based sort key used for PVS entity
// send ordering: nearer/larger entities send first, with the far bit set when
// the entity is behind the view origin.
func EntitySendSortKey(ent *srvtypes.Edict, origin, forward [3]float32, sh srvtypes.ServerHandle) int {
	if ent == nil {
		return 0
	}

	distSq := float32(0)
	sizeSq := float32(0)
	absMin := ent.AbsMin(sh)
	absMax := ent.AbsMax(sh)
	for i := 0; i < 3; i++ {
		clamped := origin[i]
		if clamped < absMin[i] {
			clamped = absMin[i]
		} else if clamped > absMax[i] {
			clamped = absMax[i]
		}
		delta := clamped - origin[i]
		distSq += delta * delta
		size := absMax[i] - absMin[i]
		sizeSq += size * size
	}
	if sizeSq < 1 {
		sizeSq = 1
	}
	dist := int(math.Min(255, 8*math.Sqrt(math.Sqrt(float64(distSq/sizeSq)))))

	forwardDist := float32(0)
	for i := 0; i < 3; i++ {
		edge := absMax[i]
		if forward[i] < 0 {
			edge = absMin[i]
		}
		forwardDist += (edge - origin[i]) * forward[i]
	}
	if forwardDist < 0 {
		dist |= 128
	}
	return dist
}
