// sv_send.go implements the server-to-client network message encoding
// for entity state, client data, and statistics.
//
// # Delta Encoding
//
// Quake uses delta encoding to minimize network bandwidth. Instead of
// sending the full state of every entity every frame, the server tracks
// the last-known state per client and only sends fields that changed.
//
// Entity updates use the U_* flag system:
//   - U_ORIGIN1/2/3: changed origin components (X, Y, Z)
//   - U_ANGLE1/2/3:  changed angle components (pitch, yaw, roll)
//   - U_MODEL:       changed model index
//   - U_FRAME:       changed animation frame
//   - U_SKIN:        changed skin
//   - U_EFFECTS:     changed entity effects (light, trail, etc.)
//   - U_SOLID:       changed solid type / bounding box
//
// Client data uses the SU_* flag system:
//   - SU_VIEWHEIGHT: view height changed
//   - SU_IDEALPITCH: ideal pitch changed
//   - SU_PUNCH:      punch angle changed (recoil)
//   - SU_VELOCITY1/2/3: velocity components changed
//   - SU_ITEMS:      item inventory changed
//   - SU_WEAPONFRAME: weapon animation frame changed
//
// FitzQuake protocol extensions add:
//   - PROTOCOL_FITZQUAKE (666): alpha, scale, glow, frame2 support
//   - PROTOCOL_NETQUAKE (15): original Quake protocol
//
// # C Lineage
//
// Mirrors SV_WriteEntitiesToClient, SV_WriteClientdataToMessage,
// WriteEntityUpdate, and WriteDelta in sv_send.c. The C version
// used direct pointer comparisons between the current and baseline
// edict state; the Go version compares struct fields.
//
// # Precision
//
// Origin coordinates are quantized to 1/8 unit (3 mantissa bits) for
// network transmission, matching C behavior. Angles are quantized to
// 256 steps per 360 degrees (8-bit). This matches the original
// Quake protocol exactly.

// This file belongs to the Network/Protocol subsystem: server-to-client message encoding, client management, and protocol types.
package server

import (
	"math"
	"sort"

	inet "github.com/darkliquid/ironwail-go/internal/net"
)

func (s *Server) StartParticle(org, dir [3]float32, color, count int) {
	if s.Datagram.Len() > MaxDatagram-18 {
		return
	}

	s.Datagram.PutByte(byte(inet.SVCParticle))
	flags := uint32(s.ProtocolFlags())
	s.Datagram.WriteCoord(org[0], flags)
	s.Datagram.WriteCoord(org[1], flags)
	s.Datagram.WriteCoord(org[2], flags)

	for i := 0; i < 3; i++ {
		v := int(dir[i] * 16)
		if v > 127 {
			v = 127
		} else if v < -128 {
			v = -128
		}
		s.Datagram.WriteChar(int8(v))
	}

	s.Datagram.PutByte(byte(count))
	s.Datagram.PutByte(byte(color))
}

// StartSound serializes a positional sound event from QC builtin sound() into network protocol fields.
func (s *Server) StartSound(ent *Edict, channel int, sample string, volume int, attenuation float32) {
	if volume < 0 || volume > 255 {
		return
	}
	if attenuation < 0 || attenuation > 4 {
		return
	}
	if channel < 0 || channel > 7 {
		return
	}
	if s.Datagram.Len() > MaxDatagram-21 {
		return
	}

	soundNum := s.FindSound(sample)
	if soundNum < 0 {
		return
	}

	entNum := s.NumForEdict(ent)

	fieldMask := 0
	if volume != DefaultSoundVolume {
		fieldMask |= 1
	}
	if attenuation != DefaultSoundAttenuation {
		fieldMask |= 2
	}
	// FitzQuake/RMQ extension: large entity/sound numbers.
	// NetQuake protocol can't support these — silently drop the sound.
	if s.Protocol == ProtocolNetQuake {
		if entNum >= 8192 || soundNum >= 256 || channel >= 8 {
			return
		}
	} else {
		if entNum >= 8192 {
			fieldMask |= inet.SND_LARGEENTITY
		}
		if soundNum >= 256 || channel >= 8 {
			fieldMask |= inet.SND_LARGESOUND
		}
	}

	if s.Datagram.Len() > MaxDatagram-21 {
		return
	}

	s.Datagram.PutByte(byte(inet.SVCSound))
	s.Datagram.PutByte(byte(fieldMask))

	if fieldMask&1 != 0 {
		s.Datagram.PutByte(byte(volume))
	}
	if fieldMask&2 != 0 {
		s.Datagram.PutByte(byte(attenuation * 64))
	}

	if fieldMask&inet.SND_LARGEENTITY != 0 {
		s.Datagram.WriteShort(int16(entNum))
		s.Datagram.PutByte(byte(channel))
	} else {
		s.Datagram.WriteShort(int16(entNum<<3 | channel))
	}
	if fieldMask&inet.SND_LARGESOUND != 0 {
		s.Datagram.WriteShort(int16(soundNum))
	} else {
		s.Datagram.PutByte(byte(soundNum))
	}

	flags := uint32(s.ProtocolFlags())
	org := ent.Origin(s)
	mins := ent.Mins(s)
	maxs := ent.Maxs(s)
	for i := 0; i < 3; i++ {
		s.Datagram.WriteCoord(org[i]+0.5*(mins[i]+maxs[i]), flags)
	}
}

// FindSound returns the precache index for a sound sample name used by network sound messages.
func (s *Server) FindSound(sample string) int {
	for i, name := range s.SoundPrecache {
		if name == sample {
			return i
		}
	}
	return -1
}

// LocalSound sends a non-positional local-only sound to one client's reliable message queue.
func (s *Server) LocalSound(client *Client, sample string) {
	soundNum := s.FindSound(sample)
	if soundNum < 0 {
		return
	}

	fieldMask := 0
	if soundNum >= 256 {
		if s.Protocol == ProtocolNetQuake {
			return
		}
		fieldMask = inet.SND_LARGESOUND
	}

	if client.Message.Len() > MaxDatagram-4 {
		return
	}

	client.Message.PutByte(byte(inet.SVCLocalSound))
	client.Message.PutByte(byte(fieldMask))
	if fieldMask&inet.SND_LARGESOUND != 0 {
		client.Message.WriteShort(int16(soundNum))
	} else {
		client.Message.PutByte(byte(soundNum))
	}
}

// writeEntityState encodes baseline/static entity payloads, including optional extended fields.
func (s *Server) writeEntityState(msg *MessageBuffer, ent EntityState, extended bool, includeEntNum bool, entNum int) {
	flags := uint32(s.ProtocolFlags())
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

// WriteClientDataToMessage serializes player-centric data (damage, view, ammo, items) for one frame.
func (s *Server) WriteClientDataToMessage(ent *Edict, msg *MessageBuffer) {
	flags := uint32(s.ProtocolFlags())
	fixAngleSent := ent.FixAngle(s) != 0
	dmgTake := ent.DmgTake(s)
	dmgSave := ent.DmgSave(s)
	if dmgTake != 0 || dmgSave != 0 {
		other := s.EdictNum(int(ent.DmgInflictor(s)))
		msg.PutByte(byte(inet.SVCDamage))
		msg.PutByte(byte(dmgSave))
		msg.PutByte(byte(dmgTake))
		if other != nil {
			oOrg := other.Origin(s)
			oMins := other.Mins(s)
			oMaxs := other.Maxs(s)
			for i := 0; i < 3; i++ {
				msg.WriteCoord(oOrg[i]+0.5*(oMins[i]+oMaxs[i]), flags)
			}
		} else {
			for i := 0; i < 3; i++ {
				msg.WriteCoord(0, flags)
			}
		}
		ent.SetDmgTake(s, 0)
		ent.SetDmgSave(s, 0)
	}

	s.SetIdealPitch(ent)

	if ent.FixAngle(s) != 0 {
		msg.PutByte(byte(inet.SVCSetAngle))
		vAng := ent.VAngle(s)
		for i := 0; i < 3; i++ {
			msg.WriteAngle(vAng[i], flags)
		}
		ent.SetFixAngle(s, 0)
	}

	bits := uint32(0)

	viewOfs := ent.ViewOfs(s)
	idealPitch := ent.IdealPitch(s)
	entFlags := ent.Flags(s)
	waterLevel := ent.WaterLevel(s)
	punchAngle := ent.PunchAngle(s)
	velocity := ent.Velocity(s)
	weaponFrame := ent.WeaponFrame(s)
	armorValue := ent.ArmorValue(s)
	weaponModel := int32(ent.WeaponModel(s))
	currentAmmo := ent.CurrentAmmo(s)
	ammoShells := ent.AmmoShells(s)
	ammoNails := ent.AmmoNails(s)
	ammoRockets := ent.AmmoRockets(s)
	ammoCells := ent.AmmoCells(s)
	itemsVal := ent.Items(s)
	weaponVal := ent.Weapon(s)
	healthVal := ent.Health(s)

	if viewOfs[2] != ViewHeight {
		bits |= inet.SU_VIEWHEIGHT
	}
	if idealPitch != 0 {
		bits |= inet.SU_IDEALPITCH
	}
	bits |= inet.SU_ITEMS

	if uint32(entFlags)&FlagOnGround != 0 {
		bits |= inet.SU_ONGROUND
	}
	if waterLevel >= 2 {
		bits |= inet.SU_INWATER
	}
	for i := 0; i < 3; i++ {
		if punchAngle[i] != 0 {
			bits |= inet.SU_PUNCH1 << i
		}
		if velocity[i] != 0 {
			bits |= inet.SU_VELOCITY1 << i
		}
	}
	if weaponFrame != 0 {
		bits |= inet.SU_WEAPONFRAME
	}
	if armorValue != 0 {
		bits |= inet.SU_ARMOR
	}
	bits |= inet.SU_WEAPON

	// FitzQuake/RMQ extension bits — only for non-NetQuake protocols
	weaponModelIdx := s.FindModel(s.String(weaponModel))
	if s.Protocol != ProtocolNetQuake {
		if bits&inet.SU_WEAPON != 0 && weaponModelIdx&0xFF00 != 0 {
			bits |= inet.SU_WEAPON2
		}
		if int(armorValue)&0xFF00 != 0 {
			bits |= inet.SU_ARMOR2
		}
		if int(currentAmmo)&0xFF00 != 0 {
			bits |= inet.SU_AMMO2
		}
		if int(ammoShells)&0xFF00 != 0 {
			bits |= inet.SU_SHELLS2
		}
		if int(ammoNails)&0xFF00 != 0 {
			bits |= inet.SU_NAILS2
		}
		if int(ammoRockets)&0xFF00 != 0 {
			bits |= inet.SU_ROCKETS2
		}
		if int(ammoCells)&0xFF00 != 0 {
			bits |= inet.SU_CELLS2
		}
		if bits&inet.SU_WEAPONFRAME != 0 && int(weaponFrame)&0xFF00 != 0 {
			bits |= inet.SU_WEAPONFRAME2
		}
		if bits&inet.SU_WEAPON != 0 && ent.Alpha != 0 { // weaponalpha = client entity alpha
			bits |= inet.SU_WEAPONALPHA
		}
		if bits >= 65536 {
			bits |= inet.SU_EXTEND1
		}
		if bits >= 16777216 {
			bits |= inet.SU_EXTEND2
		}
	}

	if entNum := s.NumForEdict(ent); s.DebugTelemetry != nil &&
		s.DebugTelemetry.ShouldLogEvent(DebugEventPhysics, s.QCVM, entNum, ent) {
		weaponModelName := s.String(weaponModel)
		s.DebugTelemetry.LogEventf(DebugEventPhysics, s.QCVM, entNum, ent,
			"clientdata serialize bits=%#x onground=%t waterlevel=%d viewofs=(%.1f %.1f %.1f) idealpitch=%.1f vel=(%.1f %.1f %.1f) punch=(%.1f %.1f %.1f) fixangle_sent=%t ground=%d teleport=%.3f items=%#x weapon=%#x weaponmodel=%q weaponmodelidx=%d ammo=%d shells=%d",
			bits, uint32(entFlags)&FlagOnGround != 0, int(waterLevel),
			viewOfs[0], viewOfs[1], viewOfs[2],
			idealPitch,
			velocity[0], velocity[1], velocity[2],
			punchAngle[0], punchAngle[1], punchAngle[2],
			fixAngleSent, int(ent.GroundEntity(s)), ent.TeleportTime(s),
			uint32(itemsVal), uint32(weaponVal), weaponModelName, weaponModelIdx,
			int(currentAmmo), int(ammoShells))
	}

	msg.PutByte(byte(inet.SVCClientData))
	msg.WriteShort(int16(bits))

	if bits&inet.SU_EXTEND1 != 0 {
		msg.PutByte(byte(bits >> 16))
	}
	if bits&inet.SU_EXTEND2 != 0 {
		msg.PutByte(byte(bits >> 24))
	}

	if bits&inet.SU_VIEWHEIGHT != 0 {
		msg.WriteChar(int8(viewOfs[2]))
	}
	if bits&inet.SU_IDEALPITCH != 0 {
		msg.WriteChar(int8(idealPitch))
	}
	for i := 0; i < 3; i++ {
		if bits&(inet.SU_PUNCH1<<i) != 0 {
			msg.WriteChar(int8(punchAngle[i]))
		}
		if bits&(inet.SU_VELOCITY1<<i) != 0 {
			msg.WriteChar(int8(velocity[i] / 16))
		}
	}

	items := uint32(itemsVal)
	msg.WriteLong(int32(items))

	if bits&inet.SU_WEAPONFRAME != 0 {
		msg.PutByte(byte(weaponFrame))
	}
	if bits&inet.SU_ARMOR != 0 {
		msg.PutByte(byte(armorValue))
	}
	if bits&inet.SU_WEAPON != 0 {
		msg.PutByte(byte(weaponModelIdx))
	}

	msg.WriteShort(int16(healthVal))
	msg.PutByte(byte(currentAmmo))
	msg.PutByte(byte(ammoShells))
	msg.PutByte(byte(ammoNails))
	msg.PutByte(byte(ammoRockets))
	msg.PutByte(byte(ammoCells))

	weaponValue := int32(weaponVal)
	if s.standardQuakeWeaponEncoding() {
		msg.PutByte(byte(weaponValue))
	} else {
		activeWeapon := byte(0)
		for i := 0; i < 32; i++ {
			if weaponValue&(1<<i) != 0 {
				activeWeapon = byte(i)
				break
			}
		}
		msg.PutByte(activeWeapon)
	}

	// FitzQuake extension data
	if bits&inet.SU_WEAPON2 != 0 {
		msg.PutByte(byte(weaponModelIdx >> 8))
	}
	if bits&inet.SU_ARMOR2 != 0 {
		msg.PutByte(byte(int(armorValue) >> 8))
	}
	if bits&inet.SU_AMMO2 != 0 {
		msg.PutByte(byte(int(currentAmmo) >> 8))
	}
	if bits&inet.SU_SHELLS2 != 0 {
		msg.PutByte(byte(int(ammoShells) >> 8))
	}
	if bits&inet.SU_NAILS2 != 0 {
		msg.PutByte(byte(int(ammoNails) >> 8))
	}
	if bits&inet.SU_ROCKETS2 != 0 {
		msg.PutByte(byte(int(ammoRockets) >> 8))
	}
	if bits&inet.SU_CELLS2 != 0 {
		msg.PutByte(byte(int(ammoCells) >> 8))
	}
	if bits&inet.SU_WEAPONFRAME2 != 0 {
		msg.PutByte(byte(int(weaponFrame) >> 8))
	}
	if bits&inet.SU_WEAPONALPHA != 0 {
		msg.PutByte(ent.Alpha) // weaponalpha = client entity alpha
	}

	// Compatibility hack from C Ironwail for Alkaline: the clientdata payload only
	// carries a byte for STAT_ACTIVEWEAPON, so resend the full 32-bit stat when the
	// QuakeC weapon bitmask does not fit in that byte.
	if uint32(byte(weaponValue)) != uint32(weaponValue) && msg.Len()+6 <= msg.Limit() {
		msg.PutByte(byte(inet.SVCUpdateStat))
		msg.PutByte(byte(inet.StatActiveWeapon))
		msg.WriteLong(weaponValue)
	}
}

// FindModel returns a model precache slot index used by entity baselines and delta updates.
func (s *Server) FindModel(name string) int {
	if name == "" {
		return 0
	}
	for i, n := range s.ModelPrecache {
		if n == name {
			return i
		}
	}
	return 0
}

// encodeScale converts a QC scale float to byte encoding.
// Matches C's ENTSCALE_ENCODE: (byte)(CLAMP(0, s, 15.9375) * 16).
func encodeScale(a float32) byte {
	if a < 0 {
		a = 0
	}
	if a > 15.9375 {
		a = 15.9375
	}
	return byte(a * 16)
}

// entityStateForClient builds render/network state for an edict as seen by a specific client.
func (s *Server) entityStateForClient(entNum int, ent *Edict) (EntityState, bool) {
	if ent == nil || ent.Free {
		return EntityState{}, false
	}

	// Read alpha and scale from QC edict fields (matching C's GetEdictFieldValueByName).
	// Field offsets are cached on server init to avoid per-frame string lookups.
	if s.QCVM != nil {
		if s.QCFieldAlpha >= 0 {
			ent.Alpha = inet.ENTALPHA_ENCODE(s.QCVM.EFloat(entNum, s.QCFieldAlpha))
		}
		if s.QCFieldScale >= 0 {
			ent.Scale = encodeScale(s.QCVM.EFloat(entNum, s.QCFieldScale))
		} else {
			ent.Scale = 16 // ENTSCALE_DEFAULT
		}
	}

	state := EntityState{
		Origin:     ent.Origin(s),
		Angles:     ent.Angles(s),
		ModelIndex: int(ent.ModelIndex(s)),
		Frame:      int(ent.Frame(s)),
		Colormap:   int(ent.Colormap(s)),
		Skin:       int(ent.Skin(s)),
		Effects:    int(ent.Effects(s)) & s.effectsMask(),
		Alpha:      ent.Alpha,
		Scale:      ent.Scale,
	}
	if state.Scale == 0 {
		state.Scale = 16
	}

	if s.Static != nil && entNum > 0 && entNum <= s.Static.MaxClients {
		state.Colormap = entNum
		if playerModel := s.FindModel("progs/player.mdl"); playerModel != 0 {
			state.ModelIndex = playerModel
		}
	}

	if entNum > 0 && s.Static != nil && entNum > s.Static.MaxClients && state.ModelIndex == 0 {
		return EntityState{}, false
	}

	return state, true
}

func encodeLerpFinish(nextThink, time float32) (byte, bool) {
	delta := nextThink - time
	if delta <= 0 {
		return 0, false
	}
	if delta > 1 {
		delta = 1
	}
	return byte(delta*255.0 + 0.5), true
}

type entitySendCandidate struct {
	entNum        int
	ent           *Edict
	state         EntityState
	moveType      float32
	lerpFinish    byte
	hasLerpFinish bool
	sortKey       int
}

func (s *Server) entitySendSortBasis(client *Client) (origin, forward [3]float32, ok bool) {
	if client == nil || client.Edict == nil {
		return origin, forward, false
	}
	origin = client.Edict.Origin(s)
	vOfs := client.Edict.ViewOfs(s)
	origin[0] += vOfs[0]
	origin[1] += vOfs[1]
	origin[2] += vOfs[2]
	var right, up [3]float32
	vAng := client.Edict.VAngle(s)
	AngleVectors(vAng, &forward, &right, &up)
	return origin, forward, true
}

func (s *Server) entitySendSortKey(ent *Edict, origin, forward [3]float32) int {
	if ent == nil {
		return 0
	}

	distSq := float32(0)
	sizeSq := float32(0)
	absMin := ent.AbsMin(s)
	absMax := ent.AbsMax(s)
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

// writeEntityUpdate performs Quake's bitflag delta encoding between baseline and current entity states.
func (s *Server) writeEntityUpdate(msg *MessageBuffer, entNum int, state, baseline EntityState, force bool, moveType float32, lerpFinish byte, hasLerpFinish bool) bool {
	flags := uint32(s.ProtocolFlags())
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
	if MoveType(moveType) == MoveTypeStep {
		bits |= inet.U_STEP
	}

	// FitzQuake/RMQ extension bits — only for non-NetQuake protocols
	if s.Protocol != ProtocolNetQuake {
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

// writeEntitiesToClient applies PVS culling then emits per-entity deltas for the target client.
func (s *Server) writeEntitiesToClient(client *Client, msg *MessageBuffer) {
	if client == nil {
		return
	}
	if client.EntityStates == nil {
		client.EntityStates = make(map[int]EntityState)
	}
	sortOrigin, sortForward, haveSortBasis := s.entitySendSortBasis(client)
	candidates := make([]entitySendCandidate, 0, s.NumEdicts)

	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.Edicts[entNum]
		state, ok := s.entityStateForClient(entNum, ent)
		if !ok {
			continue
		}
		if state.Alpha == inet.ENTALPHA_ZERO && state.Effects == 0 {
			continue
		}

		if ent != client.Edict && !s.SV_VisibleToClient(ent, client) {
			continue
		}

		var lerpFinish byte
		hasLerpFinish := ent.SendInterval
		if hasLerpFinish {
			lerpFinish, hasLerpFinish = encodeLerpFinish(ent.NextThink(s), s.Time)
		}
		candidate := entitySendCandidate{
			entNum:        entNum,
			ent:           ent,
			state:         state,
			moveType:      ent.MoveType(s),
			lerpFinish:    lerpFinish,
			hasLerpFinish: hasLerpFinish,
		}
		if ent == client.Edict {
			candidate.sortKey = -1
		} else if haveSortBasis {
			candidate.sortKey = s.entitySendSortKey(ent, sortOrigin, sortForward)
		} else {
			candidate.sortKey = entNum
		}
		candidates = append(candidates, candidate)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].sortKey != candidates[j].sortKey {
			return candidates[i].sortKey < candidates[j].sortKey
		}
		return candidates[i].entNum < candidates[j].entNum
	})

	for _, candidate := range candidates {
		if msg.Len()+40 > msg.Limit() {
			break
		}
		if !s.writeEntityUpdate(msg, candidate.entNum, candidate.state, candidate.ent.Baseline, false, candidate.moveType, candidate.lerpFinish, candidate.hasLerpFinish) {
			continue
		}
		client.EntityStates[candidate.entNum] = candidate.state
	}
}

func (s *Server) updateClientStats(client *Client) {
	if client == nil {
		return
	}
	ent := client.Edict
	if ent != nil {
		client.Stats[inet.StatHealth] = int32(ent.Health(s))
		client.Stats[inet.StatItems] = int32(ent.Items(s))
		client.Stats[inet.StatArmor] = int32(ent.ArmorValue(s))
		client.Stats[inet.StatWeapon] = int32(s.FindModel(s.String(int32(ent.WeaponModel(s)))))
		client.Stats[inet.StatAmmo] = int32(ent.CurrentAmmo(s))
		client.Stats[inet.StatShells] = int32(ent.AmmoShells(s))
		client.Stats[inet.StatNails] = int32(ent.AmmoNails(s))
		client.Stats[inet.StatRockets] = int32(ent.AmmoRockets(s))
		client.Stats[inet.StatCells] = int32(ent.AmmoCells(s))
		client.Stats[inet.StatActiveWeapon] = int32(ent.Weapon(s))
	}
	s.updateClientGlobalStats(client)
}

func (s *Server) updateClientGlobalStats(client *Client) {
	if s == nil || client == nil || s.QCVM == nil {
		return
	}
	s.updateClientGlobalStat(client, inet.StatTotalSecrets, "total_secrets")
	s.updateClientGlobalStat(client, inet.StatTotalMonsters, "total_monsters")
	s.updateClientGlobalStat(client, inet.StatSecrets, "found_secrets")
	s.updateClientGlobalStat(client, inet.StatMonsters, "killed_monsters")
}

func (s *Server) updateClientGlobalStat(client *Client, stat int, global string) {
	if client == nil || s == nil || s.QCVM == nil || s.QCVM.FindGlobal(global) < 0 {
		return
	}
	client.Stats[stat] = int32(s.QCVM.GlobalFloat(global))
}

// SV_WriteStats compares stat cache and emits reliable SVCUpdateStat messages for changed non-client HUD values.
func (s *Server) SV_WriteStats(client *Client) {
	if client == nil || client.Message == nil {
		return
	}

	s.updateClientStats(client)

	for i := statNonClient; i < len(client.Stats); i++ {
		if client.Stats[i] != client.OldStats[i] {
			client.Message.PutByte(byte(inet.SVCUpdateStat))
			client.Message.PutByte(byte(i))
			client.Message.WriteLong(client.Stats[i])
			client.OldStats[i] = client.Stats[i]
		}
	}
}

func (s *Server) writeUnderwaterOverride(client *Client) {
	if client == nil || client.Message == nil || client.Edict == nil || !client.Edict.SendForceWater {
		return
	}
	client.Edict.SendForceWater = false
	client.Message.PutByte(byte(inet.SVCStuffText))
	if client.Edict.ForceWater {
		client.Message.WriteString("//v_water 1\n")
		return
	}
	client.Message.WriteString("//v_water 0\n")
}

// buildClientDatagram assembles one full per-frame packet: time, clientdata, stats, entities, events.
func (s *Server) buildClientDatagram(client *Client, msg *MessageBuffer) {
	msg.PutByte(byte(inet.SVCTime))
	msg.WriteFloat(s.Time)

	// Build PVS for this client
	client.FatPVS = nil
	if client.Edict != nil {
		org := client.Edict.Origin(s)
		vOfs := client.Edict.ViewOfs(s)
		org[0] += vOfs[0]
		org[1] += vOfs[1]
		org[2] += vOfs[2]
		s.SV_AddToFatPVS(org, client)
	}

	s.WriteClientDataToMessage(client.Edict, msg)
	s.writeEntitiesToClient(client, msg)

	if s.Datagram != nil && s.Datagram.Len() > 0 && msg.Len()+s.Datagram.Len()+1 < msg.Limit() {
		msg.Write(s.Datagram.Data[:s.Datagram.Len()])
	}
	msg.PutByte(0xff)
	s.recordDevStatsPacketSize(msg.Len())
}
