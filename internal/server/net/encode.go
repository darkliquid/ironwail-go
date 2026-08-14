// encode.go implements pure wire-format serialization helpers for the server
// network layer: entity state bit packing, scale/lerp encoding, and entity
// send sort keys. These were extracted from server_net_send.go so they can be
// unit-tested without a live server.
package net

import (
	"math"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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
//
// Matches C's `MSG_WriteByte(msg, (byte)Q_rint((nextthink - qcvm->time)*255))`
// in sv_main.c:952: Q_rint rounds-to-nearest, and `+0.5` with integer
// truncation is the same for the only deltas ever sent (0 < delta <= 1).
// Negative/past deltas are never sent (delta <= 0 -> false), so
// round-half-away-from-zero never differs. Verified by TestEncodeLerpFinish in
// this package and internal/server/sv_send_test.go.
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
	msg.WriteCoord(ent.Origin.X, flags)
	msg.WriteAngle(ent.Angles.X, flags)
	msg.WriteCoord(ent.Origin.Y, flags)
	msg.WriteAngle(ent.Angles.Y, flags)
	msg.WriteCoord(ent.Origin.Z, flags)
	msg.WriteAngle(ent.Angles.Z, flags)
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
func EntitySendSortKey(ent *srvtypes.Edict, origin, forward qtypes.Vec3, sh srvtypes.ServerHandle) int {
	if ent == nil {
		return 0
	}

	distSq := float32(0)
	sizeSq := float32(0)
	absMin := ent.AbsMin(sh)
	absMax := ent.AbsMax(sh)

	clampedX := origin.X
	if clampedX < absMin.X {
		clampedX = absMin.X
	} else if clampedX > absMax.X {
		clampedX = absMax.X
	}
	deltaX := clampedX - origin.X
	distSq += deltaX * deltaX
	sizeX := absMax.X - absMin.X
	sizeSq += sizeX * sizeX

	clampedY := origin.Y
	if clampedY < absMin.Y {
		clampedY = absMin.Y
	} else if clampedY > absMax.Y {
		clampedY = absMax.Y
	}
	deltaY := clampedY - origin.Y
	distSq += deltaY * deltaY
	sizeY := absMax.Y - absMin.Y
	sizeSq += sizeY * sizeY

	clampedZ := origin.Z
	if clampedZ < absMin.Z {
		clampedZ = absMin.Z
	} else if clampedZ > absMax.Z {
		clampedZ = absMax.Z
	}
	deltaZ := clampedZ - origin.Z
	distSq += deltaZ * deltaZ
	sizeZ := absMax.Z - absMin.Z
	sizeSq += sizeZ * sizeZ

	if sizeSq < 1 {
		sizeSq = 1
	}
	dist := int(math.Min(255, 8*math.Sqrt(math.Sqrt(float64(distSq/sizeSq)))))

	forwardDist := float32(0)
	edgeX := absMax.X
	if forward.X < 0 {
		edgeX = absMin.X
	}
	forwardDist += (edgeX - origin.X) * forward.X

	edgeY := absMax.Y
	if forward.Y < 0 {
		edgeY = absMin.Y
	}
	forwardDist += (edgeY - origin.Y) * forward.Y

	edgeZ := absMax.Z
	if forward.Z < 0 {
		edgeZ = absMin.Z
	}
	forwardDist += (edgeZ - origin.Z) * forward.Z

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
	if force || math.Abs(float64(state.Origin.X-baseline.Origin.X)) > 0.1 {
		bits |= inet.U_ORIGIN1
	}
	if force || math.Abs(float64(state.Origin.Y-baseline.Origin.Y)) > 0.1 {
		bits |= inet.U_ORIGIN2
	}
	if force || math.Abs(float64(state.Origin.Z-baseline.Origin.Z)) > 0.1 {
		bits |= inet.U_ORIGIN3
	}
	if force || state.Angles.X != baseline.Angles.X {
		bits |= inet.U_ANGLE1
	}
	if force || state.Angles.Y != baseline.Angles.Y {
		bits |= inet.U_ANGLE2
	}
	if force || state.Angles.Z != baseline.Angles.Z {
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
		msg.WriteCoord(state.Origin.X, flags)
	}
	if bits&inet.U_ANGLE1 != 0 {
		msg.WriteAngle(state.Angles.X, flags)
	}
	if bits&inet.U_ORIGIN2 != 0 {
		msg.WriteCoord(state.Origin.Y, flags)
	}
	if bits&inet.U_ANGLE2 != 0 {
		msg.WriteAngle(state.Angles.Y, flags)
	}
	if bits&inet.U_ORIGIN3 != 0 {
		msg.WriteCoord(state.Origin.Z, flags)
	}
	if bits&inet.U_ANGLE3 != 0 {
		msg.WriteAngle(state.Angles.Z, flags)
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
		msg.WriteCoord(snd.Origin.X, flags)
		msg.WriteCoord(snd.Origin.Y, flags)
		msg.WriteCoord(snd.Origin.Z, flags)
		msg.WriteShort(int16(snd.SoundIndex))
		msg.PutByte(byte(snd.Volume))
		msg.PutByte(byte(snd.Attenuation * 64))
		return
	}
	msg.PutByte(byte(inet.SVCSpawnStaticSound))
	msg.WriteCoord(snd.Origin.X, flags)
	msg.WriteCoord(snd.Origin.Y, flags)
	msg.WriteCoord(snd.Origin.Z, flags)
	msg.PutByte(byte(snd.SoundIndex))
	msg.PutByte(byte(snd.Volume))
	msg.PutByte(byte(snd.Attenuation * 64))
}

// WriteSpawnLightStyles emits SVCLightStyle messages for every lightstyle
// slot (matching the C spawn signon block).
func WriteSpawnLightStyles(msg *srvtypes.MessageBuffer, styles [256]string) {
	if msg == nil {
		return
	}
	for style, value := range styles {
		msg.PutByte(byte(inet.SVCLightStyle))
		msg.PutByte(byte(style))
		msg.WriteString(value)
	}
}

// WriteSpawnClientRoster emits SVCUpdateName/Frags/Colors for every client
// slot so the connecting client sees the current roster.
func WriteSpawnClientRoster(msg *srvtypes.MessageBuffer, clients []*srvtypes.Client, sh srvtypes.ServerHandle) {
	if msg == nil {
		return
	}
	for playerNum, rosterClient := range clients {
		name := ""
		frags := 0
		color := 0
		if rosterClient != nil {
			name = rosterClient.Name
			if rosterClient.Edict != nil {
				frags = int(rosterClient.Edict.Frags(sh))
			}
			color = rosterClient.Color
		}
		msg.PutByte(byte(inet.SVCUpdateName))
		msg.PutByte(byte(playerNum))
		msg.WriteString(name)
		msg.PutByte(byte(inet.SVCUpdateFrags))
		msg.PutByte(byte(playerNum))
		msg.WriteShort(int16(frags))
		msg.PutByte(byte(inet.SVCUpdateColors))
		msg.PutByte(byte(playerNum))
		msg.PutByte(byte(color))
	}
}

// WriteSpawnSetAngle emits SVCSetAngle with the client's spawn angles
// (vangles on load-game, angles otherwise), matching the C signon block.
func WriteSpawnSetAngle(msg *srvtypes.MessageBuffer, client *srvtypes.Client, flags uint32, sh srvtypes.ServerHandle, useVAngle bool) {
	if client == nil || client.Edict == nil || msg == nil {
		return
	}
	msg.PutByte(byte(inet.SVCSetAngle))
	angles := client.Edict.Angles(sh)
	if useVAngle {
		angles = client.Edict.VAngle(sh)
	}
	msg.WriteAngle(angles.X, flags)
	msg.WriteAngle(angles.Y, flags)
	msg.WriteAngle(0, flags)
}
