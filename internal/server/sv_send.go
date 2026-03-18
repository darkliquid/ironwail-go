package server

import (
	"github.com/ironwail/ironwail-go/internal/bsp"
	inet "github.com/ironwail/ironwail-go/internal/net"
)

// CalcStats derives HUD/stat slots from player entvars for SVCUpdateStat style networking.
func (s *Server) CalcStats(client *Client, statsi []int, statsf []float32, statss []string) {
	ent := client.Edict
	if ent == nil {
		return
	}

	for i := range statsi {
		statsi[i] = 0
	}
	for i := range statsf {
		statsf[i] = 0
	}
	for i := range statss {
		statss[i] = ""
	}

	const (
		StatHealth       = 0
		StatWeapon       = 2
		StatAmmo         = 3
		StatArmor        = 4
		StatWeaponFrame  = 5
		StatShells       = 6
		StatNails        = 7
		StatRockets      = 8
		StatCells        = 9
		StatActiveWeapon = 10
	)

	statsf[StatHealth] = ent.Vars.Health
	statsi[StatWeapon] = int(ent.Vars.WeaponModel)
	statsf[StatAmmo] = ent.Vars.CurrentAmmo
	statsf[StatArmor] = ent.Vars.ArmorValue
	statsf[StatWeaponFrame] = ent.Vars.WeaponFrame
	statsf[StatShells] = ent.Vars.AmmoShells
	statsf[StatNails] = ent.Vars.AmmoNails
	statsf[StatRockets] = ent.Vars.AmmoRockets
	statsf[StatCells] = ent.Vars.AmmoCells
	statsf[StatActiveWeapon] = ent.Vars.Weapon
}

// StartParticle emits a transient particle event into the unreliable datagram broadcast stream.
func (s *Server) StartParticle(org, dir [3]float32, color, count int) {
	if s.Datagram.Len() > MaxDatagram-18 {
		return
	}

	s.Datagram.WriteByte(byte(inet.SVCParticle))
	s.Datagram.WriteCoord(org[0])
	s.Datagram.WriteCoord(org[1])
	s.Datagram.WriteCoord(org[2])

	for i := 0; i < 3; i++ {
		v := int(dir[i] * 16)
		if v > 127 {
			v = 127
		} else if v < -128 {
			v = -128
		}
		s.Datagram.WriteChar(int8(v))
	}

	s.Datagram.WriteByte(byte(count))
	s.Datagram.WriteByte(byte(color))
}

// StartSound serializes a positional sound event from QC builtin sound() into network protocol fields.
func (s *Server) StartSound(ent *Edict, channel int, sample string, volume int, attenuation float32) {
	if volume < 0 || volume > 255 {
		return
	}
	if attenuation < 0 || attenuation > 4 {
		return
	}
	if channel < 0 || channel > 255 {
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
	if entNum >= 8192 || channel >= 8 {
		fieldMask |= inet.SND_LARGEENTITY
	}
	if soundNum >= 256 {
		fieldMask |= inet.SND_LARGESOUND
	}

	if s.Datagram.Len() > MaxDatagram-21 {
		return
	}

	s.Datagram.WriteByte(byte(inet.SVCSound))
	s.Datagram.WriteByte(byte(fieldMask))

	if fieldMask&1 != 0 {
		s.Datagram.WriteByte(byte(volume))
	}
	if fieldMask&2 != 0 {
		s.Datagram.WriteByte(byte(attenuation * 64))
	}

	if fieldMask&inet.SND_LARGEENTITY != 0 {
		s.Datagram.WriteShort(int16(entNum))
		s.Datagram.WriteByte(byte(channel))
	} else {
		s.Datagram.WriteShort(int16(entNum<<3 | channel))
	}
	if fieldMask&inet.SND_LARGESOUND != 0 {
		s.Datagram.WriteShort(int16(soundNum))
	} else {
		s.Datagram.WriteByte(byte(soundNum))
	}

	for i := 0; i < 3; i++ {
		s.Datagram.WriteCoord(ent.Vars.Origin[i] + 0.5*(ent.Vars.Mins[i]+ent.Vars.Maxs[i]))
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
		fieldMask = 1
	}

	if client.Message.Len() > MaxDatagram-4 {
		return
	}

	client.Message.WriteByte(byte(inet.SVCLocalSound))
	client.Message.WriteByte(byte(fieldMask))
	if fieldMask != 0 {
		client.Message.WriteShort(int16(soundNum))
	} else {
		client.Message.WriteByte(byte(soundNum))
	}
}

// writeEntityState encodes baseline/static entity payloads, including optional extended fields.
func (s *Server) writeEntityState(msg *MessageBuffer, ent EntityState, extended bool, includeEntNum bool, entNum int) {
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
		msg.WriteByte(bits)
	}
	if includeEntNum {
		msg.WriteShort(int16(entNum))
	}
	if extended && bits&(1<<0) != 0 {
		msg.WriteShort(int16(ent.ModelIndex))
	} else {
		msg.WriteByte(byte(ent.ModelIndex))
	}
	if extended && bits&(1<<1) != 0 {
		msg.WriteShort(int16(ent.Frame))
	} else {
		msg.WriteByte(byte(ent.Frame))
	}
	msg.WriteByte(byte(ent.Colormap))
	msg.WriteByte(byte(ent.Skin))
	// Origins and angles must be interleaved: O1, A1, O2, A2, O3, A3
	for i := 0; i < 3; i++ {
		msg.WriteCoord(ent.Origin[i])
		msg.WriteAngle(ent.Angles[i])
	}
	if extended && bits&(1<<2) != 0 {
		msg.WriteByte(ent.Alpha)
	}
	if extended && bits&(1<<3) != 0 {
		msg.WriteByte(ent.Scale)
	}
}

// WriteClientDataToMessage serializes player-centric data (damage, view, ammo, items) for one frame.
func (s *Server) WriteClientDataToMessage(ent *Edict, msg *MessageBuffer) {
	if ent.Vars.DmgTake != 0 || ent.Vars.DmgSave != 0 {
		other := s.EdictNum(int(ent.Vars.DmgInflictor))
		msg.WriteByte(byte(inet.SVCDamage))
		msg.WriteByte(byte(ent.Vars.DmgSave))
		msg.WriteByte(byte(ent.Vars.DmgTake))
		if other != nil {
			for i := 0; i < 3; i++ {
				msg.WriteCoord(other.Vars.Origin[i] + 0.5*(other.Vars.Mins[i]+other.Vars.Maxs[i]))
			}
		} else {
			for i := 0; i < 3; i++ {
				msg.WriteCoord(0)
			}
		}
		ent.Vars.DmgTake = 0
		ent.Vars.DmgSave = 0
	}

	s.SetIdealPitch(ent)

	if ent.Vars.FixAngle != 0 {
		msg.WriteByte(byte(inet.SVCSetAngle))
		for i := 0; i < 3; i++ {
			msg.WriteAngle(ent.Vars.Angles[i])
		}
		ent.Vars.FixAngle = 0
	}

	bits := uint32(0)

	if ent.Vars.ViewOfs[2] != ViewHeight {
		bits |= inet.SU_VIEWHEIGHT
	}
	if ent.Vars.IdealPitch != 0 {
		bits |= inet.SU_IDEALPITCH
	}
	bits |= inet.SU_ITEMS

	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		bits |= inet.SU_ONGROUND
	}
	if ent.Vars.WaterLevel >= 2 {
		bits |= inet.SU_INWATER
	}
	for i := 0; i < 3; i++ {
		if ent.Vars.PunchAngle[i] != 0 {
			bits |= inet.SU_PUNCH1 << i
		}
		if ent.Vars.Velocity[i] != 0 {
			bits |= inet.SU_VELOCITY1 << i
		}
	}
	if ent.Vars.WeaponFrame != 0 {
		bits |= inet.SU_WEAPONFRAME
	}
	if ent.Vars.ArmorValue != 0 {
		bits |= inet.SU_ARMOR
	}
	bits |= inet.SU_WEAPON

	msg.WriteByte(byte(inet.SVCClientData))
	msg.WriteShort(int16(bits))

	if bits&inet.SU_VIEWHEIGHT != 0 {
		msg.WriteChar(int8(ent.Vars.ViewOfs[2]))
	}
	if bits&inet.SU_IDEALPITCH != 0 {
		msg.WriteChar(int8(ent.Vars.IdealPitch))
	}
	for i := 0; i < 3; i++ {
		if bits&(inet.SU_PUNCH1<<i) != 0 {
			msg.WriteChar(int8(ent.Vars.PunchAngle[i]))
		}
		if bits&(inet.SU_VELOCITY1<<i) != 0 {
			msg.WriteChar(int8(ent.Vars.Velocity[i] / 16))
		}
	}

	items := uint32(ent.Vars.Items)
	msg.WriteLong(int32(items))

	if bits&inet.SU_WEAPONFRAME != 0 {
		msg.WriteByte(byte(ent.Vars.WeaponFrame))
	}
	if bits&inet.SU_ARMOR != 0 {
		msg.WriteByte(byte(ent.Vars.ArmorValue))
	}
	if bits&inet.SU_WEAPON != 0 {
		msg.WriteByte(byte(s.FindModel(s.GetString(ent.Vars.WeaponModel))))
	}

	msg.WriteShort(int16(ent.Vars.Health))
	msg.WriteByte(byte(ent.Vars.CurrentAmmo))
	msg.WriteByte(byte(ent.Vars.AmmoShells))
	msg.WriteByte(byte(ent.Vars.AmmoNails))
	msg.WriteByte(byte(ent.Vars.AmmoRockets))
	msg.WriteByte(byte(ent.Vars.AmmoCells))

	activeWeapon := byte(0)
	for i := 0; i < 32; i++ {
		if uint32(ent.Vars.Weapon)&(1<<i) != 0 {
			activeWeapon = byte(i)
			break
		}
	}
	msg.WriteByte(activeWeapon)
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

// entityStateForClient builds render/network state for an edict as seen by a specific client.
func (s *Server) entityStateForClient(entNum int, ent *Edict) (EntityState, bool) {
	if ent == nil || ent.Free || ent.Vars == nil {
		return EntityState{}, false
	}

	state := EntityState{
		Origin:     ent.Vars.Origin,
		Angles:     ent.Vars.Angles,
		ModelIndex: int(ent.Vars.ModelIndex),
		Frame:      int(ent.Vars.Frame),
		Colormap:   int(ent.Vars.Colormap),
		Skin:       int(ent.Vars.Skin),
		Effects:    int(ent.Vars.Effects),
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

// writeEntityUpdate performs Quake's bitflag delta encoding between previous and current entity states.
func (s *Server) writeEntityUpdate(msg *MessageBuffer, entNum int, state, prev EntityState, force bool) bool {
	bits := uint32(0)

	if entNum > 255 {
		bits |= inet.U_LONGENTITY
	}
	if force || state.Origin[0] != prev.Origin[0] {
		bits |= inet.U_ORIGIN1
	}
	if force || state.Origin[1] != prev.Origin[1] {
		bits |= inet.U_ORIGIN2
	}
	if force || state.Origin[2] != prev.Origin[2] {
		bits |= inet.U_ORIGIN3
	}
	if force || state.Angles[0] != prev.Angles[0] {
		bits |= inet.U_ANGLE1
	}
	if force || state.Angles[1] != prev.Angles[1] {
		bits |= inet.U_ANGLE2
	}
	if force || state.Angles[2] != prev.Angles[2] {
		bits |= inet.U_ANGLE3
	}
	if force || state.ModelIndex != prev.ModelIndex {
		bits |= inet.U_MODEL
	}
	if force || state.Frame != prev.Frame {
		bits |= inet.U_FRAME
	}
	if force || state.Colormap != prev.Colormap {
		bits |= inet.U_COLORMAP
	}
	if force || state.Skin != prev.Skin {
		bits |= inet.U_SKIN
	}
	if force || state.Effects != prev.Effects {
		bits |= inet.U_EFFECTS
	}

	// FitzQuake/RMQ extension bits — only for non-NetQuake protocols
	if s.Protocol != ProtocolNetQuake {
		if state.Alpha != prev.Alpha {
			if state.Alpha != 0 || prev.Alpha != 0 || force {
				bits |= inet.U_ALPHA
			}
		}
		if state.Scale != prev.Scale {
			if state.Scale != 16 || prev.Scale != 16 || force {
				bits |= inet.U_SCALE
			}
		}
		if bits&inet.U_FRAME != 0 && state.Frame > 255 {
			bits |= inet.U_FRAME2
		}
		if bits&inet.U_MODEL != 0 && state.ModelIndex > 255 {
			bits |= inet.U_MODEL2
		}
		if bits >= 65536 {
			bits |= inet.U_EXTEND1
		}
		if bits >= 16777216 {
			bits |= inet.U_EXTEND2
		}
	}

	if bits == 0 {
		return false
	}
	if bits&0x0000ff00 != 0 {
		bits |= inet.U_MOREBITS
	}

	first := byte(bits&0x7f) | 0x80
	msg.WriteByte(first)
	if bits&inet.U_MOREBITS != 0 {
		msg.WriteByte(byte(bits >> 8))
	}
	if bits&inet.U_EXTEND1 != 0 {
		msg.WriteByte(byte(bits >> 16))
	}
	if bits&inet.U_EXTEND2 != 0 {
		msg.WriteByte(byte(bits >> 24))
	}

	if bits&inet.U_LONGENTITY != 0 {
		msg.WriteShort(int16(entNum))
	} else {
		msg.WriteByte(byte(entNum))
	}
	// Field write order must match C exactly (sv_main.c:920-954):
	// MODEL, FRAME, COLORMAP, SKIN, EFFECTS,
	// ORIGIN1, ANGLE1, ORIGIN2, ANGLE2, ORIGIN3, ANGLE3,
	// ALPHA, SCALE, FRAME2, MODEL2, LERPFINISH
	if bits&inet.U_MODEL != 0 {
		msg.WriteByte(byte(state.ModelIndex))
	}
	if bits&inet.U_FRAME != 0 {
		msg.WriteByte(byte(state.Frame))
	}
	if bits&inet.U_COLORMAP != 0 {
		msg.WriteByte(byte(state.Colormap))
	}
	if bits&inet.U_SKIN != 0 {
		msg.WriteByte(byte(state.Skin))
	}
	if bits&inet.U_EFFECTS != 0 {
		msg.WriteByte(byte(state.Effects))
	}
	// Origins and angles are INTERLEAVED: O1, A1, O2, A2, O3, A3
	if bits&inet.U_ORIGIN1 != 0 {
		msg.WriteCoord(state.Origin[0])
	}
	if bits&inet.U_ANGLE1 != 0 {
		msg.WriteAngle(state.Angles[0])
	}
	if bits&inet.U_ORIGIN2 != 0 {
		msg.WriteCoord(state.Origin[1])
	}
	if bits&inet.U_ANGLE2 != 0 {
		msg.WriteAngle(state.Angles[1])
	}
	if bits&inet.U_ORIGIN3 != 0 {
		msg.WriteCoord(state.Origin[2])
	}
	if bits&inet.U_ANGLE3 != 0 {
		msg.WriteAngle(state.Angles[2])
	}
	// FitzQuake extensions come AFTER origins/angles
	if bits&inet.U_ALPHA != 0 {
		msg.WriteByte(state.Alpha)
	}
	if bits&inet.U_SCALE != 0 {
		msg.WriteByte(state.Scale)
	}
	if bits&inet.U_FRAME2 != 0 {
		msg.WriteByte(byte(state.Frame >> 8))
	}
	if bits&inet.U_MODEL2 != 0 {
		msg.WriteByte(byte(state.ModelIndex >> 8))
	}
	// TODO: U_LERPFINISH support (task #26)

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

	current := make(map[int]EntityState)
	for entNum := 1; entNum < s.NumEdicts; entNum++ {
		ent := s.Edicts[entNum]
		state, ok := s.entityStateForClient(entNum, ent)
		if !ok {
			continue
		}

		// PVS culling
		// Always send client's own entity, other players, and entities with NoPVS flag if it existed
		// In Quake, we usually always send players.
		isPlayer := entNum <= s.Static.MaxClients
		if !isPlayer && ent != client.Edict {
			if !s.SV_VisibleToClient(ent, client) {
				continue
			}
		}

		current[entNum] = state
		prev, seen := client.EntityStates[entNum]
		force := !seen
		if !s.writeEntityUpdate(msg, entNum, state, prev, force) {
			continue
		}
		client.EntityStates[entNum] = state
	}

	for entNum, prev := range client.EntityStates {
		if _, ok := current[entNum]; ok {
			continue
		}
		zero := prev
		zero.ModelIndex = 0
		if s.writeEntityUpdate(msg, entNum, zero, prev, false) {
			delete(client.EntityStates, entNum)
		}
	}
}

// SV_WriteStats compares stat cache and emits reliable SVCUpdateStat messages for changed HUD values.
func (s *Server) SV_WriteStats(client *Client, msg *MessageBuffer) {
	ent := client.Edict
	if ent != nil {
		client.Stats[inet.StatHealth] = int32(ent.Vars.Health)
		client.Stats[inet.StatItems] = int32(ent.Vars.Items)
		client.Stats[inet.StatArmor] = int32(ent.Vars.ArmorValue)
		client.Stats[inet.StatWeapon] = int32(s.FindModel(s.GetString(ent.Vars.WeaponModel)))
		client.Stats[inet.StatAmmo] = int32(ent.Vars.CurrentAmmo)
		client.Stats[inet.StatShells] = int32(ent.Vars.AmmoShells)
		client.Stats[inet.StatNails] = int32(ent.Vars.AmmoNails)
		client.Stats[inet.StatRockets] = int32(ent.Vars.AmmoRockets)
		client.Stats[inet.StatCells] = int32(ent.Vars.AmmoCells)
		client.Stats[inet.StatActiveWeapon] = int32(ent.Vars.Weapon)
	}

	for i := 0; i < 32; i++ {
		if client.Stats[i] != client.OldStats[i] {
			msg.WriteByte(byte(inet.SVCUpdateStat))
			msg.WriteByte(byte(i))
			msg.WriteLong(client.Stats[i])
			client.OldStats[i] = client.Stats[i]
		}
	}
}

// buildClientDatagram assembles one full per-frame packet: time, clientdata, stats, entities, events.
func (s *Server) buildClientDatagram(client *Client, msg *MessageBuffer) {
	msg.WriteByte(byte(inet.SVCTime))
	msg.WriteFloat(s.Time)

	// Build PVS for this client
	client.FatPVS = nil
	if client.Edict != nil {
		s.SV_AddToFatPVS(client.Edict.Vars.Origin, client)
	}

	s.WriteClientDataToMessage(client.Edict, msg)
	s.SV_WriteStats(client, msg)
	s.writeEntitiesToClient(client, msg)

	if s.Datagram.Len() > 0 && msg.Len()+s.Datagram.Len()+1 < MaxDatagram {
		msg.Write(s.Datagram.Data[:s.Datagram.Len()])
	}
	msg.WriteByte(0xff)
}

// SV_AddToFatPVS builds an expanded visibility set around a point to reduce pop-in during movement.
func (s *Server) SV_AddToFatPVS(org [3]float32, client *Client) {
	if s.WorldTree == nil || len(s.WorldTree.Nodes) == 0 {
		return
	}
	s.sv_AddToFatPVSRecursive(org, bsp.TreeChild{Index: 0, IsLeaf: false}, client)
}

// sv_AddToFatPVSRecursive walks BSP recursively and ORs visible leaves into the client's FatPVS mask.
func (s *Server) sv_AddToFatPVSRecursive(org [3]float32, child bsp.TreeChild, client *Client) {
	for {
		if child.IsLeaf {
			leaf := &s.WorldTree.Leafs[child.Index]
			if leaf.Contents != bsp.ContentsSolid {
				pvs := s.WorldTree.LeafPVS(leaf)
				if client.FatPVS == nil || len(client.FatPVS) != len(pvs) {
					client.FatPVS = make([]byte, len(pvs))
					copy(client.FatPVS, pvs)
				} else {
					for i := range pvs {
						client.FatPVS[i] |= pvs[i]
					}
				}
			}
			return
		}

		node := &s.WorldTree.Nodes[child.Index]
		plane := &s.WorldTree.Planes[node.PlaneNum]
		var d float32
		if plane.Type < 3 {
			d = org[plane.Type] - plane.Dist
		} else {
			d = VecDot(org, plane.Normal) - plane.Dist
		}

		if d > 8 {
			child = node.Children[0]
		} else if d < -8 {
			child = node.Children[1]
		} else {
			// go down both
			s.sv_AddToFatPVSRecursive(org, node.Children[0], client)
			child = node.Children[1]
		}
	}
}

// SV_VisibleToClient checks whether any entity leaf intersects the client's precomputed FatPVS.
func (s *Server) SV_VisibleToClient(ent *Edict, client *Client) bool {
	if client.FatPVS == nil || ent.NumLeafs == 0 {
		return true
	}

	for i := 0; i < ent.NumLeafs; i++ {
		leafIdx := ent.LeafNums[i]
		if leafIdx <= 0 {
			continue
		}
		if (client.FatPVS[(leafIdx-1)>>3] & (1 << (uint(leafIdx-1) & 7))) != 0 {
			return true
		}
	}

	return false
}

// SV_EdictInPVS checks whether any of the edict's leaf numbers are visible
// in the given PVS byte array. Returns true if any leaf is set.
func (s *Server) SV_EdictInPVS(test *Edict, pvs []byte) bool {
	if test == nil || len(pvs) == 0 || test.NumLeafs == 0 {
		return true
	}
	for i := 0; i < test.NumLeafs; i++ {
		leafIdx := test.LeafNums[i]
		if leafIdx <= 0 {
			continue
		}
		byteIdx := (leafIdx - 1) >> 3
		if byteIdx < 0 || byteIdx >= len(pvs) {
			continue
		}
		if (pvs[byteIdx] & (1 << (uint(leafIdx-1) & 7))) != 0 {
			return true
		}
	}
	return false
}
