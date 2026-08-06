// encode.go implements pure wire-format serialization helpers for the server
// network layer: entity state bit packing, scale/lerp encoding, and entity
// send sort keys. These were extracted from server_net_send.go so they can be
// unit-tested without a live server.
package net

import (
	"math"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

// Protocol identifiers (mirrored from server root for pure function params).
const (
	ProtocolNetQuake = 15
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

// WriteEntityUpdate delta-encodes one entity state against its baseline into
// the message buffer, matching the C sv_main.c field write order (MODEL,
// FRAME, COLORMAP, SKIN, EFFECTS, then interleaved ORIGIN/ANGLE, then Fitz
// extensions ALPHA, SCALE, FRAME2, MODEL2, LERPFINISH). protocol is the
// negotiated protocol number (15 = NetQuake); bits beyond NetQuake's set are
// only emitted for non-NetQuake protocols.
func WriteEntityUpdate(msg *srvtypes.MessageBuffer, entNum int, state, baseline srvtypes.EntityState, force bool, moveType srvtypes.MoveType, protocol int, flags uint32, lerpFinish byte, hasLerpFinish bool) bool {
	bits := uint32(0)

	if entNum > 255 {
		bits |= inet.U_LONGENTITY
	}
	if force || math.Abs(float64(state.Origin[0]-baseline.Origin[0])) > 0.1 {
		bits |= inet.U_ORIGIN1
	}
	if force || math.Abs(float64(state.Origin[1]-baseline.Origin[1])) > 0.1 {
		bits |= inet.U_ORIGIN2
	}
	if force || math.Abs(float64(state.Origin[2]-baseline.Origin[2])) > 0.1 {
		bits |= inet.U_ORIGIN3
	}
	if force || state.Angles[0] != baseline.Angles[0] {
		bits |= inet.U_ANGLE1
	}
	if force || state.Angles[1] != baseline.Angles[1] {
		bits |= inet.U_ANGLE2
	}
	if force || state.Angles[2] != baseline.Angles[2] {
		bits |= inet.U_ANGLE3
	}
	if force || state.ModelIndex != baseline.ModelIndex {
		bits |= inet.U_MODEL
	}
	if force || state.Frame != baseline.Frame {
		bits |= inet.U_FRAME
	}
	if force || state.Colormap != baseline.Colormap {
		bits |= inet.U_COLORMAP
	}
	if force || state.Skin != baseline.Skin {
		bits |= inet.U_SKIN
	}
	if force || state.Effects != baseline.Effects {
		bits |= inet.U_EFFECTS
	}
	if moveType == srvtypes.MoveTypeStep {
		bits |= inet.U_STEP
	}

	// FitzQuake/RMQ extension bits — only for non-NetQuake protocols
	if protocol != ProtocolNetQuake {
		if state.Alpha != baseline.Alpha {
			if state.Alpha != 0 || baseline.Alpha != 0 || force {
				bits |= inet.U_ALPHA
			}
		}
		if state.Scale != baseline.Scale {
			if state.Scale != 16 || baseline.Scale != 16 || force {
				bits |= inet.U_SCALE
			}
		}
		if bits&inet.U_FRAME != 0 && state.Frame > 255 {
			bits |= inet.U_FRAME2
		}
		if bits&inet.U_MODEL != 0 && state.ModelIndex > 255 {
			bits |= inet.U_MODEL2
		}
		if hasLerpFinish {
			bits |= inet.U_LERPFINISH
		}
		if bits >= 65536 {
			bits |= inet.U_EXTEND1
		}
		if bits >= 16777216 {
			bits |= inet.U_EXTEND2
		}
	}

	if bits&0x0000ff00 != 0 {
		bits |= inet.U_MOREBITS
	}

	first := byte(bits&0x7f) | 0x80
	msg.PutByte(first)
	if bits&inet.U_MOREBITS != 0 {
		msg.PutByte(byte(bits >> 8))
	}
	if bits&inet.U_EXTEND1 != 0 {
		msg.PutByte(byte(bits >> 16))
	}
	if bits&inet.U_EXTEND2 != 0 {
		msg.PutByte(byte(bits >> 24))
	}

	if bits&inet.U_LONGENTITY != 0 {
		msg.WriteShort(int16(entNum))
	} else {
		msg.PutByte(byte(entNum))
	}
	// Field write order must match C exactly (sv_main.c:920-954):
	// MODEL, FRAME, COLORMAP, SKIN, EFFECTS,
	// ORIGIN1, ANGLE1, ORIGIN2, ANGLE2, ORIGIN3, ANGLE3,
	// ALPHA, SCALE, FRAME2, MODEL2, LERPFINISH
	if bits&inet.U_MODEL != 0 {
		msg.PutByte(byte(state.ModelIndex))
	}
	if bits&inet.U_FRAME != 0 {
		msg.PutByte(byte(state.Frame))
	}
	if bits&inet.U_COLORMAP != 0 {
		msg.PutByte(byte(state.Colormap))
	}
	if bits&inet.U_SKIN != 0 {
		msg.PutByte(byte(state.Skin))
	}
	if bits&inet.U_EFFECTS != 0 {
		msg.PutByte(byte(state.Effects))
	}
	// Origins and angles are INTERLEAVED: O1, A1, O2, A2, O3, A3
	if bits&inet.U_ORIGIN1 != 0 {
		msg.WriteCoord(state.Origin[0], flags)
	}
	if bits&inet.U_ANGLE1 != 0 {
		msg.WriteAngle(state.Angles[0], flags)
	}
	if bits&inet.U_ORIGIN2 != 0 {
		msg.WriteCoord(state.Origin[1], flags)
	}
	if bits&inet.U_ANGLE2 != 0 {
		msg.WriteAngle(state.Angles[1], flags)
	}
	if bits&inet.U_ORIGIN3 != 0 {
		msg.WriteCoord(state.Origin[2], flags)
	}
	if bits&inet.U_ANGLE3 != 0 {
		msg.WriteAngle(state.Angles[2], flags)
	}
	// FitzQuake extensions come AFTER origins/angles
	if bits&inet.U_ALPHA != 0 {
		msg.PutByte(state.Alpha)
	}
	if bits&inet.U_SCALE != 0 {
		msg.PutByte(state.Scale)
	}
	if bits&inet.U_FRAME2 != 0 {
		msg.PutByte(byte(state.Frame >> 8))
	}
	if bits&inet.U_MODEL2 != 0 {
		msg.PutByte(byte(state.ModelIndex >> 8))
	}
	if bits&inet.U_LERPFINISH != 0 {
		msg.PutByte(lerpFinish)
	}

	return true
}
// WriteSpawnStaticMessage emits a static entity signon message, using the
// extended variant when the model/frame/alpha/scale do not fit in a byte.
func WriteSpawnStaticMessage(msg *srvtypes.MessageBuffer, ent srvtypes.EntityState, flags uint32) {
	extended := ent.ModelIndex > 255 || ent.Frame > 255 || ent.Alpha != 0 || (ent.Scale != 0 && ent.Scale != 16)
	if extended {
		msg.PutByte(byte(inet.SVCSpawnStatic2))
		WriteEntityState(msg, ent, true, false, 0, flags)
		return
	}
	msg.PutByte(byte(inet.SVCSpawnStatic))
	WriteEntityState(msg, ent, false, false, 0, flags)
}

// WriteSpawnStaticSoundMessage emits ambient/static sound signon messages with
// large-index fallback.
func WriteSpawnStaticSoundMessage(msg *srvtypes.MessageBuffer, snd srvtypes.StaticSound, flags uint32) {
	if snd.SoundIndex > 255 {
		msg.PutByte(byte(inet.SVCSpawnStaticSound2))
		for i := 0; i < 3; i++ {
			msg.WriteCoord(snd.Origin[i], flags)
		}
		msg.WriteShort(int16(snd.SoundIndex))
		msg.PutByte(byte(snd.Volume))
		msg.PutByte(byte(snd.Attenuation * 64))
		return
	}
	msg.PutByte(byte(inet.SVCSpawnStaticSound))
	for i := 0; i < 3; i++ {
		msg.WriteCoord(snd.Origin[i], flags)
	}
	msg.PutByte(byte(snd.SoundIndex))
	msg.PutByte(byte(snd.Volume))
	msg.PutByte(byte(snd.Attenuation * 64))
}
