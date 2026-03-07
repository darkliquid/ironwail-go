package server

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
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

func resetLightStyles(values *[64]string) {
	for i := range values {
		values[i] = "m"
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
	s.StaticEntities = nil
	s.StaticSounds = nil
	resetLightStyles(&s.LightStyles)

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
			Edict:        clientEdict,
			Message:      NewMessageBuffer(MaxDatagram),
			Name:         "unconnected",
			EntityStates: make(map[int]EntityState),
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
	s.StaticEntities = nil
	s.StaticSounds = nil
	resetLightStyles(&s.LightStyles)
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
	resetLightStyles(&s.LightStyles)

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
	bspFile, err := bsp.Load(bytes.NewReader(bspData))
	if err != nil {
		return fmt.Errorf("parse collision bsp %q: %w", s.ModelName, err)
	}

	s.WorldModel = worldModelFromBSPTree(s.ModelName, tree)
	if wm, ok := s.WorldModel.(*model.Model); ok {
		populateWorldModelCollision(wm, tree, bspFile)
	}
	s.WorldTree = tree

	if s.Static != nil {
		keep := s.Static.MaxClients + 1
		if keep < 1 {
			keep = 1
		}
		if keep < len(s.Edicts) {
			for i := keep; i < len(s.Edicts); i++ {
				s.Edicts[i] = nil
			}
			s.Edicts = s.Edicts[:keep]
		}
		s.NumEdicts = len(s.Edicts)
	}

	if s.Edicts[0] == nil {
		s.Edicts[0] = &Edict{Vars: &EntVars{}, Scale: 16}
	}
	world := s.Edicts[0]
	world.Free = false
	world.Vars = &EntVars{}
	world.Vars.ModelIndex = 1
	world.Vars.Solid = float32(SolidBSP)
	world.Vars.MoveType = float32(MoveTypePush)
	world.Vars.ClassName = 0
	world.Vars.Model = 0

	s.ModelPrecache[0] = ""
	s.ModelPrecache[1] = s.ModelName
	for i := 1; i < len(tree.Models) && i+1 < len(s.ModelPrecache); i++ {
		s.ModelPrecache[i+1] = string(bytes.TrimRight(LocalModels[i][:], "\x00"))
	}
	if s.FindModel("progs/player.mdl") == 0 {
		for i := 1; i < len(s.ModelPrecache); i++ {
			if s.ModelPrecache[i] != "" {
				continue
			}
			s.ModelPrecache[i] = "progs/player.mdl"
			break
		}
	}
	s.StaticEntities = nil
	s.StaticSounds = nil

	if err := s.loadMapEntities(string(tree.Entities)); err != nil {
		return fmt.Errorf("parse map entities %q: %w", s.ModelName, err)
	}
	if s.QCVM != nil {
		if world.Vars.Model == 0 {
			world.Vars.Model = s.QCVM.AllocString(s.ModelName)
		}
		if world.Vars.ClassName == 0 {
			world.Vars.ClassName = s.QCVM.AllocString("worldspawn")
		}
	}

	s.ClearWorld()
	s.LinkEdict(world, false)
	s.syncQCVMState()

	s.Active = true
	s.State = ServerStateActive

	slog.Info("server spawned map start", "map", mapName)

	return nil
}

func (s *Server) loadMapEntities(raw string) error {
	if strings.Trim(raw, " \t\r\n\x00") == "" {
		return nil
	}
	maxClients := 0
	if s.Static != nil {
		maxClients = s.Static.MaxClients
	}
	em := &EntityManager{
		edicts:     s.Edicts,
		vm:         s.QCVM,
		maxEdicts:  s.MaxEdicts,
		numEdicts:  s.NumEdicts,
		maxClients: maxClients,
		freeTime:   make([]float32, max(s.MaxEdicts, len(s.Edicts))),
	}

	remaining := raw
	for entIndex := 0; ; entIndex++ {
		remaining = strings.TrimLeft(remaining, " \t\r\n\x00")
		if remaining == "" {
			break
		}

		entNum := entIndex
		if entIndex > 0 {
			ent := s.AllocEdict()
			if ent == nil {
				return fmt.Errorf("no free edict for map entity %d", entIndex)
			}
			entNum = s.NumForEdict(ent)
			em.edicts = s.Edicts
			em.numEdicts = s.NumEdicts
		}

		next, err := em.ED_ParseEdict(remaining, entNum)
		if err != nil {
			return err
		}
		remaining = next
	}

	s.NumEdicts = len(s.Edicts)
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
		m.ClipMins = m.Mins
		m.ClipMaxs = m.Maxs
		m.ClipBox = true
	}
	m.NumSubModels = len(tree.Models)
	m.SubModels = append([]bsp.DModel(nil), tree.Models...)

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

var brushHullClipBounds = [model.MaxMapHulls]struct {
	mins [3]float32
	maxs [3]float32
}{
	0: {},
	1: {mins: [3]float32{-16, -16, -24}, maxs: [3]float32{16, 16, 32}},
	2: {mins: [3]float32{-32, -32, -24}, maxs: [3]float32{32, 32, 64}},
}

func populateWorldModelCollision(m *model.Model, tree *bsp.Tree, file *bsp.File) {
	if m == nil || tree == nil || len(m.Planes) == 0 || len(tree.Models) == 0 {
		return
	}

	m.Hulls[0] = buildNodeHull(tree, m.Planes, int(tree.Models[0].HeadNode[0]))

	clipNodes := bspClipNodesToModel(file)
	if len(clipNodes) == 0 {
		return
	}

	m.ClipNodes = clipNodes
	for hullNum := 1; hullNum <= 2; hullNum++ {
		headNode := int(tree.Models[0].HeadNode[hullNum])
		if headNode < 0 {
			continue
		}
		m.Hulls[hullNum] = model.Hull{
			ClipNodes:     clipNodes,
			Planes:        m.Planes,
			FirstClipNode: headNode,
			LastClipNode:  len(clipNodes) - 1,
			ClipMins:      brushHullClipBounds[hullNum].mins,
			ClipMaxs:      brushHullClipBounds[hullNum].maxs,
		}
	}
}

func buildNodeHull(tree *bsp.Tree, planes []model.MPlane, headNode int) model.Hull {
	if tree == nil || len(tree.Nodes) == 0 || headNode < 0 || headNode >= len(tree.Nodes) {
		return model.Hull{FirstClipNode: -1, LastClipNode: -1}
	}

	clipNodes := make([]model.MClipNode, len(tree.Nodes))
	for i, node := range tree.Nodes {
		clipNodes[i].PlaneNum = int(node.PlaneNum)
		for side, child := range node.Children {
			if child.IsLeaf {
				if child.Index >= 0 && child.Index < len(tree.Leafs) {
					clipNodes[i].Children[side] = int(tree.Leafs[child.Index].Contents)
				} else {
					clipNodes[i].Children[side] = bsp.ContentsSolid
				}
				continue
			}
			clipNodes[i].Children[side] = child.Index
		}
	}

	return model.Hull{
		ClipNodes:     clipNodes,
		Planes:        planes,
		FirstClipNode: headNode,
		LastClipNode:  len(clipNodes) - 1,
	}
}

func bspClipNodesToModel(file *bsp.File) []model.MClipNode {
	if file == nil {
		return nil
	}

	switch clipNodes := file.Clipnodes.(type) {
	case []bsp.DSClipNode:
		out := make([]model.MClipNode, len(clipNodes))
		for i, node := range clipNodes {
			out[i] = model.MClipNode{
				PlaneNum: int(node.PlaneNum),
				Children: [2]int{int(node.Children[0]), int(node.Children[1])},
			}
		}
		return out
	case []bsp.DLClipNode:
		out := make([]model.MClipNode, len(clipNodes))
		for i, node := range clipNodes {
			out[i] = model.MClipNode{
				PlaneNum: int(node.PlaneNum),
				Children: [2]int{int(node.Children[0]), int(node.Children[1])},
			}
		}
		return out
	default:
		return nil
	}
}

func (s *Server) modelBounds(modelName string) (mins, maxs [3]float32, ok bool) {
	if modelName == "" {
		return mins, maxs, true
	}

	if wm, ok := s.WorldModel.(*model.Model); ok && wm != nil {
		if modelName == s.ModelName {
			if wm.Type == model.ModBrush && (wm.ClipBox || wm.ClipMins != [3]float32{} || wm.ClipMaxs != [3]float32{}) {
				return wm.ClipMins, wm.ClipMaxs, true
			}
			return wm.Mins, wm.Maxs, true
		}

		if len(modelName) > 1 && modelName[0] == '*' {
			idx, err := strconv.Atoi(modelName[1:])
			if err == nil && idx >= 0 {
				if s.WorldTree != nil && idx < len(s.WorldTree.Models) {
					sub := s.WorldTree.Models[idx]
					return sub.BoundsMin, sub.BoundsMax, true
				}
				if idx < len(wm.SubModels) {
					sub := wm.SubModels[idx]
					return sub.BoundsMin, sub.BoundsMax, true
				}
			}
		}
	}

	return mins, maxs, false
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

	s.Datagram.WriteByte(byte(inet.SVCSound))
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

	client.Message.WriteByte(byte(inet.SVCLocalSound))
	client.Message.WriteByte(byte(fieldMask))
	if fieldMask != 0 {
		client.Message.WriteShort(int16(soundNum))
	} else {
		client.Message.WriteByte(byte(soundNum))
	}
}

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
	for i := 0; i < 3; i++ {
		msg.WriteCoord(ent.Origin[i])
	}
	for i := 0; i < 3; i++ {
		msg.WriteAngle(ent.Angles[i])
	}
	if extended && bits&(1<<2) != 0 {
		msg.WriteByte(ent.Alpha)
	}
	if extended && bits&(1<<3) != 0 {
		msg.WriteByte(ent.Scale)
	}
}

func (s *Server) writeSpawnStaticMessage(msg *MessageBuffer, ent EntityState) {
	extended := ent.ModelIndex > 255 || ent.Frame > 255 || ent.Alpha != 0 || (ent.Scale != 0 && ent.Scale != 16)
	if extended {
		msg.WriteByte(byte(inet.SVCSpawnStatic2))
		s.writeEntityState(msg, ent, true, false, 0)
		return
	}
	msg.WriteByte(byte(inet.SVCSpawnStatic))
	s.writeEntityState(msg, ent, false, false, 0)
}

func (s *Server) writeSpawnStaticSoundMessage(msg *MessageBuffer, snd StaticSound) {
	if snd.SoundIndex > 255 {
		msg.WriteByte(byte(inet.SVCSpawnStaticSound2))
		for i := 0; i < 3; i++ {
			msg.WriteCoord(snd.Origin[i])
		}
		msg.WriteShort(int16(snd.SoundIndex))
		msg.WriteByte(byte(snd.Volume))
		msg.WriteByte(byte(snd.Attenuation * 64))
		return
	}
	msg.WriteByte(byte(inet.SVCSpawnStaticSound))
	for i := 0; i < 3; i++ {
		msg.WriteCoord(snd.Origin[i])
	}
	msg.WriteByte(byte(snd.SoundIndex))
	msg.WriteByte(byte(snd.Volume))
	msg.WriteByte(byte(snd.Attenuation * 64))
}

func (s *Server) SendServerInfo(client *Client) {
	client.Message.WriteByte(byte(inet.SVCPrint))
	client.Message.WriteString(fmt.Sprintf("\nFITZQUAKE GO SERVER\n"))

	client.Message.WriteByte(byte(inet.SVCServerInfo))
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
		client.Message.WriteByte(byte(inet.SVCCDTrack))
		client.Message.WriteByte(byte(s.Edicts[0].Vars.Sounds))
		client.Message.WriteByte(byte(s.Edicts[0].Vars.Sounds))
	}

	for _, ent := range s.StaticEntities {
		s.writeSpawnStaticMessage(client.Message, ent)
	}
	for _, snd := range s.StaticSounds {
		s.writeSpawnStaticSoundMessage(client.Message, snd)
	}

	client.Message.WriteByte(byte(inet.SVCSetView))
	client.Message.WriteShort(int16(s.NumForEdict(client.Edict)))

	client.Message.WriteByte(byte(inet.SVCSignOnNum))
	client.Message.WriteByte(1)

	client.SendSignon = SignonFlush
	client.Spawned = false
}

func (s *Server) GetString(idx int32) string {
	if idx == 0 {
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
	if client.EntityStates == nil {
		client.EntityStates = make(map[int]EntityState)
	} else {
		clear(client.EntityStates)
	}

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
		if state.ModelIndex > 255 {
			bits |= inet.U_MODEL2
		}
	}
	if force || state.Frame != prev.Frame {
		bits |= inet.U_FRAME
		if state.Frame > 255 {
			bits |= inet.U_FRAME2
		}
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
	if force || state.Alpha != prev.Alpha {
		if state.Alpha != 0 || prev.Alpha != 0 || force {
			bits |= inet.U_ALPHA
		}
	}
	if force || state.Scale != prev.Scale {
		if state.Scale != 16 || prev.Scale != 16 || force {
			bits |= inet.U_SCALE
		}
	}

	if bits == 0 {
		return false
	}
	if bits&0x0000ff00 != 0 {
		bits |= inet.U_MOREBITS
	}
	if bits&0x00ff0000 != 0 {
		bits |= inet.U_EXTEND1
	}
	if bits&0xff000000 != 0 {
		bits |= inet.U_EXTEND2
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
	if bits&inet.U_MODEL != 0 {
		msg.WriteByte(byte(state.ModelIndex))
	}
	if bits&inet.U_MODEL2 != 0 {
		msg.WriteByte(byte(state.ModelIndex >> 8))
	}
	if bits&inet.U_FRAME != 0 {
		msg.WriteByte(byte(state.Frame))
	}
	if bits&inet.U_FRAME2 != 0 {
		msg.WriteByte(byte(state.Frame >> 8))
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
	if bits&inet.U_ORIGIN1 != 0 {
		msg.WriteCoord(state.Origin[0])
	}
	if bits&inet.U_ORIGIN2 != 0 {
		msg.WriteCoord(state.Origin[1])
	}
	if bits&inet.U_ORIGIN3 != 0 {
		msg.WriteCoord(state.Origin[2])
	}
	if bits&inet.U_ANGLE1 != 0 {
		msg.WriteAngle(state.Angles[0])
	}
	if bits&inet.U_ANGLE2 != 0 {
		msg.WriteAngle(state.Angles[1])
	}
	if bits&inet.U_ANGLE3 != 0 {
		msg.WriteAngle(state.Angles[2])
	}
	if bits&inet.U_ALPHA != 0 {
		msg.WriteByte(state.Alpha)
	}
	if bits&inet.U_SCALE != 0 {
		msg.WriteByte(state.Scale)
	}

	return true
}

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

func (s *Server) buildClientDatagram(client *Client, msg *MessageBuffer) {
	msg.WriteByte(byte(inet.SVCTime))
	msg.WriteFloat(s.Time)

	s.WriteClientDataToMessage(client.Edict, msg)
	s.writeEntitiesToClient(client, msg)

	if s.Datagram.Len() > 0 && msg.Len()+s.Datagram.Len()+1 < MaxDatagram {
		msg.Write(s.Datagram.Data[:s.Datagram.Len()])
	}
	msg.WriteByte(0xff)
}

func (s *Server) SendClientDatagram(client *Client) bool {
	var msg MessageBuffer
	msg.Data = make([]byte, MaxDatagram)
	s.buildClientDatagram(client, &msg)

	return true
}

// GetClientDatagram builds and returns the per-frame datagram bytes for the
// given client slot. Returns nil if the client is not active/spawned.
// Used by the loopback client to feed server messages into the client parser.
func (s *Server) GetClientDatagram(clientNum int) []byte {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return nil
	}
	client := s.Static.Clients[clientNum]
	if client == nil || !client.Active || !client.Spawned {
		return nil
	}
	var msg MessageBuffer
	msg.Data = make([]byte, MaxDatagram)
	s.buildClientDatagram(client, &msg)

	result := make([]byte, msg.Len())
	copy(result, msg.Data[:msg.Len()])
	return result
}

func (s *Server) GetClientLoopbackMessage(clientNum int) []byte {
	if clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return nil
	}
	client := s.Static.Clients[clientNum]
	if client == nil || !client.Active {
		return nil
	}

	var msg MessageBuffer
	msg.Data = make([]byte, MaxDatagram)

	if client.Message != nil && client.Message.Len() > 0 {
		msg.Write(client.Message.Data[:client.Message.Len()])
		client.Message.Clear()
	}

	if client.Spawned {
		var frame MessageBuffer
		frame.Data = make([]byte, MaxDatagram)
		s.buildClientDatagram(client, &frame)
		msg.Write(frame.Data[:frame.Len()])
	} else if msg.Len() > 0 {
		msg.WriteByte(0xff)
	}

	if msg.Len() == 0 {
		return nil
	}

	result := make([]byte, msg.Len())
	copy(result, msg.Data[:msg.Len()])
	return result
}

func (s *Server) SendNop(client *Client) {
	var msg MessageBuffer
	msg.Data = make([]byte, 4)
	msg.WriteByte(byte(inet.SVCNop))
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
				client.Message.WriteByte(byte(inet.SVCUpdateFrags))
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
	msg.WriteByte(byte(inet.SVCStuffText))
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
