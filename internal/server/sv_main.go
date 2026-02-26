package server

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
)

const (
	ProtocolNetQuake  = 15
	ProtocolFitzQuake = 666
	ProtocolRMQ       = 999
	// Note: MaxDatagram and MaxSignonBuffers are defined in types.go
	DatagramMTU   = 1400
	SignonSize    = 31500
	NumSpawnParms = 16
	NumPingTimes  = 16
)

var LocalModels [MaxModels][8]byte

func init() {
	for i := 0; i < MaxModels; i++ {
		copy(LocalModels[i][:], fmt.Sprintf("*%d", i))
	}
}

func (s *Server) Init(maxClients int) error {
	if maxClients <= 0 {
		return fmt.Errorf("maxClients must be > 0")
	}
	if maxClients > MaxClients {
		return fmt.Errorf("maxClients %d exceeds limit %d", maxClients, MaxClients)
	}

	s.Active = false
	s.Paused = false
	s.LoadGame = false
	s.State = ServerStateLoading
	s.Name = ""
	s.ModelName = ""
	s.WorldModel = nil
	s.Time = 1
	s.FrameTime = 0.1

	if s.MaxEdicts <= 0 {
		s.MaxEdicts = MaxEdicts
	}

	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.SoundPrecache = make([]string, MaxSounds)
	s.ModelPrecache = make([]string, MaxModels)

	s.Static = &ServerStatic{
		MaxClients:      maxClients,
		MaxClientsLimit: maxClients,
		Clients:         make([]*Client, maxClients),
	}

	s.Edicts = make([]*Edict, maxClients+1)
	for i := range s.Edicts {
		s.Edicts[i] = &Edict{Vars: &EntVars{}, Scale: 16}
	}
	s.NumEdicts = len(s.Edicts)

	for i := 0; i < maxClients; i++ {
		clientEdict := s.Edicts[i+1]
		s.Static.Clients[i] = &Client{
			Edict:   clientEdict,
			Message: NewMessageBuffer(MaxDatagram),
			Name:    "unconnected",
		}
	}

	return nil
}

func (s *Server) Shutdown() {
	s.Active = false
	s.Paused = false
	s.State = ServerStateLoading
	s.WorldModel = nil
	s.Edicts = nil
	s.NumEdicts = 0
	s.Static = nil
	s.SoundPrecache = nil
	s.ModelPrecache = nil
	if s.Datagram != nil {
		s.Datagram.Clear()
	}
}

func (s *Server) SpawnServer(mapName string, vfs *fs.FileSystem) error {
	if s.Static == nil {
		return errors.New("server not initialized")
	}
	if vfs == nil {
		return errors.New("filesystem is nil")
	}
	if mapName == "" {
		return errors.New("map name is empty")
	}

	s.Active = false
	s.Paused = false
	s.State = ServerStateLoading
	s.Time = 1

	s.Name = filepath.Base(mapName)
	s.ModelName = fmt.Sprintf("maps/%s.bsp", s.Name)

	bspData, err := vfs.LoadFile(s.ModelName)
	if err != nil {
		return fmt.Errorf("load map %q: %w", s.ModelName, err)
	}

	tree, err := bsp.LoadTree(bytes.NewReader(bspData))
	if err != nil {
		return fmt.Errorf("parse map %q: %w", s.ModelName, err)
	}

	s.WorldModel = worldModelFromBSPTree(s.ModelName, tree)

	if s.Edicts[0] == nil {
		s.Edicts[0] = &Edict{Vars: &EntVars{}, Scale: 16}
	}
	world := s.Edicts[0]
	world.Vars.ModelIndex = 1
	world.Vars.Solid = float32(SolidBSP)
	world.Vars.MoveType = float32(MoveTypePush)

	s.ModelPrecache[0] = ""
	s.ModelPrecache[1] = s.ModelName

	s.ClearWorld()
	s.LinkEdict(world, false)

	s.Active = true
	s.State = ServerStateActive

	slog.Info("server spawned map start", "map", mapName)

	return nil
}

func worldModelFromBSPTree(modelName string, tree *bsp.Tree) *model.Model {
	m := &model.Model{
		Name:      modelName,
		Type:      model.ModBrush,
		NumLeafs:  len(tree.Leafs),
		NumNodes:  len(tree.Nodes),
		Entities:  string(tree.Entities),
		NumPlanes: len(tree.Planes),
	}

	if len(tree.Models) > 0 {
		m.Mins = tree.Models[0].BoundsMin
		m.Maxs = tree.Models[0].BoundsMax
	}

	m.Planes = make([]model.MPlane, len(tree.Planes))
	for i, p := range tree.Planes {
		m.Planes[i] = model.MPlane{
			Normal: p.Normal,
			Dist:   p.Dist,
			Type:   uint8(p.Type),
		}
	}

	m.Nodes = make([]model.MNode, len(tree.Nodes))
	for i, n := range tree.Nodes {
		m.Nodes[i] = model.MNode{
			Contents: int(bsp.ContentsEmpty),
			MinMaxs: [6]float32{
				n.BoundsMin[0], n.BoundsMin[1], n.BoundsMin[2],
				n.BoundsMax[0], n.BoundsMax[1], n.BoundsMax[2],
			},
			FirstSurface: n.FirstFace,
			NumSurfaces:  n.NumFaces,
		}
		if int(n.PlaneNum) >= 0 && int(n.PlaneNum) < len(m.Planes) {
			m.Nodes[i].Plane = &m.Planes[n.PlaneNum]
		}
	}

	for i, n := range tree.Nodes {
		for side := 0; side < 2; side++ {
			child := n.Children[side]
			if !child.IsLeaf && child.Index >= 0 && child.Index < len(m.Nodes) {
				m.Nodes[i].Children[side] = &m.Nodes[child.Index]
			}
		}
	}

	for i := range m.Hulls {
		m.Hulls[i].FirstClipNode = -1
		m.Hulls[i].LastClipNode = -1
	}

	return m
}

type ProtocolFlags int

const (
	ProtocolFlagFloatCoords ProtocolFlags = 1 << iota
	ProtocolFlagFloatAngles
)

func (s *Server) ProtocolFlags() ProtocolFlags {
	return ProtocolFlagFloatCoords | ProtocolFlagFloatAngles
}

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

func (s *Server) StartParticle(org, dir [3]float32, color, count int) {
	if s.Datagram.Len() > MaxDatagram-18 {
		return
	}

	s.Datagram.WriteByte(byte(SVCParticle))
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

	if s.Datagram.Len() > MaxDatagram-21 {
		return
	}

	s.Datagram.WriteByte(byte(SVCSound))
	s.Datagram.WriteByte(byte(fieldMask))

	if fieldMask&1 != 0 {
		s.Datagram.WriteByte(byte(volume))
	}
	if fieldMask&2 != 0 {
		s.Datagram.WriteByte(byte(attenuation * 64))
	}

	s.Datagram.WriteShort(int16(entNum<<3 | channel))
	s.Datagram.WriteByte(byte(soundNum))

	for i := 0; i < 3; i++ {
		s.Datagram.WriteCoord(ent.Vars.Origin[i] + 0.5*(ent.Vars.Mins[i]+ent.Vars.Maxs[i]))
	}
}

func (s *Server) FindSound(sample string) int {
	for i, name := range s.SoundPrecache {
		if name == sample {
			return i
		}
	}
	return -1
}

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

	client.Message.WriteByte(byte(SVCLocalSound))
	client.Message.WriteByte(byte(fieldMask))
	if fieldMask != 0 {
		client.Message.WriteShort(int16(soundNum))
	} else {
		client.Message.WriteByte(byte(soundNum))
	}
}

func (s *Server) SendServerInfo(client *Client) {
	client.Message.WriteByte(byte(SVCPrint))
	client.Message.WriteString(fmt.Sprintf("\nFITZQUAKE GO SERVER\n"))

	client.Message.WriteByte(byte(SVCServerInfo))
	client.Message.WriteLong(ProtocolFitzQuake)
	client.Message.WriteByte(byte(s.Static.MaxClients))

	if !s.Coop && s.Deathmatch {
		client.Message.WriteByte(1)
	} else {
		client.Message.WriteByte(0)
	}

	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		client.Message.WriteString(s.GetString(int32(s.Edicts[0].Vars.Message)))
	} else {
		client.Message.WriteString("")
	}

	for i := 1; i < len(s.ModelPrecache); i++ {
		if s.ModelPrecache[i] == "" {
			break
		}
		client.Message.WriteString(s.ModelPrecache[i])
	}
	client.Message.WriteByte(0)

	for i := 1; i < len(s.SoundPrecache); i++ {
		if s.SoundPrecache[i] == "" {
			break
		}
		client.Message.WriteString(s.SoundPrecache[i])
	}
	client.Message.WriteByte(0)

	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		client.Message.WriteByte(byte(SVCCDTrack))
		client.Message.WriteByte(byte(s.Edicts[0].Vars.Sounds))
		client.Message.WriteByte(byte(s.Edicts[0].Vars.Sounds))
	}

	client.Message.WriteByte(byte(SVCSetView))
	client.Message.WriteShort(int16(s.NumForEdict(client.Edict)))

	client.Message.WriteByte(byte(SVCSignOnNum))
	client.Message.WriteByte(1)

	client.SendSignon = SignonFlush
	client.Spawned = false
}

func (s *Server) GetString(idx int32) string {
	if idx <= 0 {
		return ""
	}
	if s.QCVM == nil {
		return ""
	}
	return s.QCVM.GetString(idx)
}

func (s *Server) ConnectClient(clientNum int) {
	client := s.Static.Clients[clientNum]
	if client == nil {
		return
	}

	edictNum := clientNum + 1
	ent := s.EdictNum(edictNum)
	if ent == nil {
		ent = s.AllocEdict()
		if ent == nil {
			return
		}
	}

	client.Active = true
	client.Spawned = false
	client.Edict = ent
	client.Name = "unconnected"

	if s.LoadGame {
	} else {
		if s.QCVM != nil {
			if setNewParms := s.QCVM.FindFunction("SetNewParms"); setNewParms >= 0 {
				s.QCVM.Time = float64(s.Time)
				s.QCVM.ExecuteFunction(setNewParms)
				for i := 0; i < NumSpawnParms; i++ {
					client.SpawnParms[i] = float32(s.QCVM.GetGlobalFloat(fmt.Sprintf("parm%d", i+1)))
				}
			}
		}
	}

	s.SendServerInfo(client)
}

func (s *Server) ClearDatagram() {
	s.Datagram.Clear()
}

func (s *Server) WriteClientDataToMessage(ent *Edict, msg *MessageBuffer) {
	if ent.Vars.DmgTake != 0 || ent.Vars.DmgSave != 0 {
		other := s.EdictNum(int(ent.Vars.DmgInflictor))
		msg.WriteByte(byte(SVCDamage))
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
		msg.WriteByte(byte(SVCSetAngle))
		for i := 0; i < 3; i++ {
			msg.WriteAngle(ent.Vars.Angles[i])
		}
		ent.Vars.FixAngle = 0
	}

	bits := 0

	if ent.Vars.ViewOfs[2] != ViewHeight {
		bits |= 1
	}
	if ent.Vars.IdealPitch != 0 {
		bits |= 2
	}
	bits |= 4

	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		bits |= 8
	}
	if ent.Vars.WaterLevel >= 2 {
		bits |= 16
	}
	for i := 0; i < 3; i++ {
		if ent.Vars.PunchAngle[i] != 0 {
			bits |= 32 << i
		}
		if ent.Vars.Velocity[i] != 0 {
			bits |= 256 << i
		}
	}
	if ent.Vars.WeaponFrame != 0 {
		bits |= 2048
	}
	if ent.Vars.ArmorValue != 0 {
		bits |= 4096
	}
	bits |= 8192

	msg.WriteByte(byte(SVCClientData))
	msg.WriteShort(int16(bits))

	if bits&1 != 0 {
		msg.WriteChar(int8(ent.Vars.ViewOfs[2]))
	}
	if bits&2 != 0 {
		msg.WriteChar(int8(ent.Vars.IdealPitch))
	}
	for i := 0; i < 3; i++ {
		if bits&(32<<i) != 0 {
			msg.WriteChar(int8(ent.Vars.PunchAngle[i]))
		}
		if bits&(256<<i) != 0 {
			msg.WriteChar(int8(ent.Vars.Velocity[i] / 16))
		}
	}

	items := uint32(ent.Vars.Items)
	msg.WriteLong(int32(items))

	if bits&2048 != 0 {
		msg.WriteByte(byte(ent.Vars.WeaponFrame))
	}
	if bits&4096 != 0 {
		msg.WriteByte(byte(ent.Vars.ArmorValue))
	}
	if bits&8192 != 0 {
		msg.WriteByte(byte(s.FindModel(s.GetString(ent.Vars.WeaponModel))))
	}

	msg.WriteShort(int16(ent.Vars.Health))
	msg.WriteByte(byte(ent.Vars.CurrentAmmo))
	msg.WriteByte(byte(ent.Vars.AmmoShells))
	msg.WriteByte(byte(ent.Vars.AmmoNails))
	msg.WriteByte(byte(ent.Vars.AmmoRockets))
	msg.WriteByte(byte(ent.Vars.AmmoCells))

	for i := 0; i < 32; i++ {
		if uint32(ent.Vars.Weapon)&(1<<i) != 0 {
			msg.WriteByte(byte(i))
			break
		}
	}
}

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

func (s *Server) SendClientDatagram(client *Client) bool {
	var msg MessageBuffer
	msg.Data = make([]byte, MaxDatagram)

	msg.WriteByte(byte(SVCTime))
	msg.WriteFloat(s.Time)

	s.WriteClientDataToMessage(client.Edict, &msg)

	if msg.Len()+s.Datagram.Len() < MaxDatagram {
		msg.Write(s.Datagram.Data[:s.Datagram.Len()])
	}

	return true
}

func (s *Server) SendNop(client *Client) {
	var msg MessageBuffer
	msg.Data = make([]byte, 4)
	msg.WriteByte(byte(SVCNop))
}

func (s *Server) SendClientMessages() {
	s.UpdateToReliableMessages()

	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		if client.Spawned {
			s.SendClientDatagram(client)
		} else {
			if client.SendSignon == SignonNone {
				continue
			}
			if client.Message.Len() > 0 || client.DropASAP {
				if client.DropASAP {
					s.DropClient(client, false)
				} else {
					client.Message.Clear()
				}
			}
		}
	}

	s.CleanupEnts()
}

func (s *Server) UpdateToReliableMessages() {
	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		for j, other := range s.Static.Clients {
			if other == nil || !other.Active {
				continue
			}
			if client.OldFrags != int(other.Edict.Vars.Frags) {
				client.Message.WriteByte(byte(SVCUpdateFrags))
				client.Message.WriteByte(byte(j))
				client.Message.WriteShort(int16(other.Edict.Vars.Frags))
				client.OldFrags = int(other.Edict.Vars.Frags)
			}
		}
	}
}

func (s *Server) CleanupEnts() {
	for i := 1; i < s.NumEdicts; i++ {
		ent := s.Edicts[i]
		if ent != nil {
			ent.Vars.Effects = float32(uint32(ent.Vars.Effects) & ^uint32(EffectMuzzleFlash))
		}
	}
}

func (s *Server) CreateBaseline() {
	for entNum := 0; entNum < s.NumEdicts; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free {
			continue
		}
		if entNum > s.Static.MaxClients && ent.Vars.ModelIndex == 0 {
			continue
		}

		for i := 0; i < 3; i++ {
			ent.Baseline.Origin[i] = ent.Vars.Origin[i]
			ent.Baseline.Angles[i] = ent.Vars.Angles[i]
		}
		ent.Baseline.Frame = int(ent.Vars.Frame)
		ent.Baseline.Skin = int(ent.Vars.Skin)

		if entNum > 0 && entNum <= s.Static.MaxClients {
			ent.Baseline.Colormap = entNum
			ent.Baseline.ModelIndex = s.FindModel("progs/player.mdl")
			ent.Baseline.Alpha = 255
			ent.Baseline.Scale = 16
		} else {
			ent.Baseline.Colormap = 0
			ent.Baseline.ModelIndex = s.FindModel(s.GetString(int32(ent.Vars.Model)))
			ent.Baseline.Alpha = ent.Alpha
			ent.Baseline.Scale = 16
		}
	}
}

func (s *Server) SendReconnect() {
	var msg MessageBuffer
	msg.Data = make([]byte, 128)
	msg.WriteByte(byte(SVCStuffText))
	msg.WriteString("reconnect\n")
}

func (s *Server) SaveSpawnParms() {
	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		if s.QCVM != nil {
			if setChangeParms := s.QCVM.FindFunction("SetChangeParms"); setChangeParms >= 0 {
				s.QCVM.Time = float64(s.Time)
				s.QCVM.SetGlobal("self", s.NumForEdict(client.Edict))
				s.QCVM.ExecuteFunction(setChangeParms)
				for i := 0; i < NumSpawnParms; i++ {
					client.SpawnParms[i] = float32(s.QCVM.GetGlobalFloat(fmt.Sprintf("parm%d", i+1)))
				}
			}
		}
	}
}
