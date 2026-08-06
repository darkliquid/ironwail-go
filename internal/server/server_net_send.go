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
	"sort"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	srvnet "github.com/darkliquid/ironwail-go/internal/server/net"
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
	srvnet.WriteEntityState(msg, ent, extended, includeEntNum, entNum, uint32(s.ProtocolFlags()))
}

// WriteClientDataToMessage serializes player-centric data (damage, view, ammo, items) for one frame.
// The bit-packing logic lives in the net subpackage; this delegator wires the
// server's surfaces (precache lookup, telemetry, ideal-pitch trace, protocol).
func (s *Server) WriteClientDataToMessage(ent *Edict, msg *MessageBuffer) {
	srvnet.WriteClientData(srvnet.ClientDataDeps{
		Handle:                     s,
		Precacher:                  s,
		Logger:                     s.DebugTelemetry,
		SetIdealPitch:              s.SetIdealPitch,
		EdictNum:                   s.EdictNum,
		NumForEdict:                s.NumForEdict,
		Protocol:                   s.Protocol,
		StandardQuakeWeaponEncoding: s.standardQuakeWeaponEncoding(),
		Flags:                      uint32(s.ProtocolFlags()),
	}, ent, msg)
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
	return srvnet.EncodeScale(a)
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
	return srvnet.EncodeLerpFinish(nextThink, time)
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
	return srvnet.EntitySendSortKey(ent, origin, forward, s)
}

// writeEntityUpdate performs Quake's bitflag delta encoding between baseline and current entity states.
func (s *Server) writeEntityUpdate(msg *MessageBuffer, entNum int, state, baseline EntityState, force bool, moveType float32, lerpFinish byte, hasLerpFinish bool) bool {
	return srvnet.WriteEntityUpdate(msg, entNum, state, baseline, force, MoveType(moveType), s.Protocol, uint32(s.ProtocolFlags()), lerpFinish, hasLerpFinish)
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
