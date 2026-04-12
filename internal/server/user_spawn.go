package server

import (
	"fmt"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func (s *Server) writeSpawnSnapshot(client *Client) {
	if client == nil || client.Message == nil {
		return
	}

	client.Message.Clear()
	client.Message.WriteByte(byte(inet.SVCTime))
	client.Message.WriteFloat(s.Time)
	s.writeSpawnClientRoster(client, client.Message)
	s.writeSpawnLightStyles(client.Message)
	s.writeSpawnGlobalStats(client, client.Message)
	s.writeSpawnSetAngle(client, client.Message)
	s.WriteClientDataToMessage(client.Edict, client.Message)
	client.Message.WriteByte(byte(inet.SVCSignOnNum))
	client.Message.WriteByte(3)
}

func (s *Server) writeSpawnClientRoster(_ *Client, msg *MessageBuffer) {
	if s.Static == nil || msg == nil {
		return
	}
	for playerNum, rosterClient := range s.Static.Clients {
		name := ""
		frags := 0
		color := 0
		if rosterClient != nil {
			name = rosterClient.Name
			if rosterClient.Edict != nil {
				frags = int(rosterClient.Edict.Vars.Frags)
			}
			color = rosterClient.Color
		}
		msg.WriteByte(byte(inet.SVCUpdateName))
		msg.WriteByte(byte(playerNum))
		msg.WriteString(name)
		msg.WriteByte(byte(inet.SVCUpdateFrags))
		msg.WriteByte(byte(playerNum))
		msg.WriteShort(int16(frags))
		msg.WriteByte(byte(inet.SVCUpdateColors))
		msg.WriteByte(byte(playerNum))
		msg.WriteByte(byte(color))
	}
}

func (s *Server) writeSpawnLightStyles(msg *MessageBuffer) {
	if msg == nil {
		return
	}
	for style, value := range s.LightStyles {
		msg.WriteByte(byte(inet.SVCLightStyle))
		msg.WriteByte(byte(style))
		msg.WriteString(value)
	}
}

func (s *Server) writeSpawnGlobalStats(client *Client, msg *MessageBuffer) {
	if client == nil || msg == nil {
		return
	}
	s.updateClientGlobalStats(client)
	stats := [32]int32{}
	for i := range stats {
		stats[i] = client.Stats[i]
	}
	msg.WriteByte(byte(inet.SVCUpdateStat))
	msg.WriteByte(byte(inet.StatTotalSecrets))
	msg.WriteLong(stats[inet.StatTotalSecrets])
	msg.WriteByte(byte(inet.SVCUpdateStat))
	msg.WriteByte(byte(inet.StatTotalMonsters))
	msg.WriteLong(stats[inet.StatTotalMonsters])
	msg.WriteByte(byte(inet.SVCUpdateStat))
	msg.WriteByte(byte(inet.StatSecrets))
	msg.WriteLong(stats[inet.StatSecrets])
	msg.WriteByte(byte(inet.SVCUpdateStat))
	msg.WriteByte(byte(inet.StatMonsters))
	msg.WriteLong(stats[inet.StatMonsters])
}

func (s *Server) writeSpawnSetAngle(client *Client, msg *MessageBuffer) {
	if client == nil || client.Edict == nil || msg == nil {
		return
	}
	msg.WriteByte(byte(inet.SVCSetAngle))
	flags := uint32(s.ProtocolFlags())
	angles := client.Edict.Vars.Angles
	if s.LoadGame {
		angles = client.Edict.Vars.VAngle
	}
	msg.WriteAngle(angles[0], flags)
	msg.WriteAngle(angles[1], flags)
	msg.WriteAngle(0, flags)
}

func (s *Server) findLocalSpawnPoint() *Edict {
	for _, className := range []string{"info_player_start", "testplayerstart"} {
		for entNum := 1; entNum < s.NumEdicts; entNum++ {
			ent := s.Edicts[entNum]
			if ent == nil || ent.Free || ent.Vars == nil {
				continue
			}
			if s.GetString(ent.Vars.ClassName) == className {
				return ent
			}
		}
	}
	return nil
}

func (s *Server) initClientSpawnFallback(client *Client) error {
	if client == nil || client.Edict == nil {
		return fmt.Errorf("client edict missing")
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return fmt.Errorf("invalid client edict %d", entNum)
	}

	ent := client.Edict
	ent.Free = false
	savedFrags := float32(0)
	if ent.Vars != nil {
		savedFrags = ent.Vars.Frags
	}
	if ent.Vars == nil {
		ent.Vars = &EntVars{}
	} else {
		*ent.Vars = EntVars{}
	}
	ent.Vars.Colormap = float32(entNum)
	ent.Vars.Team = float32((client.Color & 15) + 1)
	ent.Vars.Frags = savedFrags
	ent.Vars.Health = 100
	ent.Vars.TakeDamage = 1
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.ViewOfs = [3]float32{0, 0, ViewHeight}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Size = [3]float32{32, 32, 56}
	ent.Vars.Velocity = [3]float32{}
	ent.Vars.AVelocity = [3]float32{}
	ent.Vars.FixAngle = 1

	if spawn := s.findLocalSpawnPoint(); spawn != nil && spawn.Vars != nil {
		ent.Vars.Origin = spawn.Vars.Origin
		ent.Vars.Angles = spawn.Vars.Angles
		ent.Vars.VAngle = spawn.Vars.Angles
	}
	ent.Vars.AbsMin = [3]float32{ent.Vars.Origin[0] + ent.Vars.Mins[0], ent.Vars.Origin[1] + ent.Vars.Mins[1], ent.Vars.Origin[2] + ent.Vars.Mins[2]}
	ent.Vars.AbsMax = [3]float32{ent.Vars.Origin[0] + ent.Vars.Maxs[0], ent.Vars.Origin[1] + ent.Vars.Maxs[1], ent.Vars.Origin[2] + ent.Vars.Maxs[2]}

	if client.Name == "" {
		client.Name = "player"
	}
	if s.QCVM != nil {
		ent.Vars.ClassName = s.QCVM.AllocString("player")
		ent.Vars.NetName = s.QCVM.AllocString(client.Name)
		if playerModel := s.FindModel("progs/player.mdl"); playerModel != 0 {
			ent.Vars.ModelIndex = float32(playerModel)
			ent.Vars.Model = s.QCVM.AllocString("progs/player.mdl")
		}
	}

	s.LinkEdict(ent, true)
	return nil
}

func (s *Server) runClientSpawnQC(client *Client) error {
	if client == nil || client.Edict == nil {
		return fmt.Errorf("client edict missing")
	}
	if err := s.initClientSpawnFallback(client); err != nil {
		return err
	}
	if s.QCVM == nil {
		return nil
	}
	if err := s.runClientQCFunction(client, "ClientConnect", true); err != nil {
		return err
	}
	if s.QCVM.FindFunction("PutClientInServer") < 0 {
		return nil
	}
	return s.runClientPutInServerQC(client)
}

func (s *Server) runClientQCFunction(client *Client, functionName string, includeSpawnParms bool) error {
	if client == nil || client.Edict == nil {
		return fmt.Errorf("client edict missing")
	}
	if s.QCVM == nil {
		return nil
	}

	funcNum := s.QCVM.FindFunction(functionName)
	if funcNum < 0 {
		return nil
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return fmt.Errorf("invalid client edict %d", entNum)
	}

	// Sync QCVM state and prepare for function call
	s.syncQCVMState()
	syncEdictToQCVM(s.QCVM, entNum, client.Edict)

	// Set up global variables for PutClientInServer
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("frametime", s.FrameTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	if includeSpawnParms {
		for i := 0; i < len(client.SpawnParms); i++ {
			s.QCVM.SetGlobal(fmt.Sprintf("parm%d", i+1), client.SpawnParms[i])
		}
	}

	if err := s.executeQCFunction(funcNum); err != nil {
		return fmt.Errorf("%s execution failed: %w", functionName, err)
	}

	syncEdictFromQCVM(s.QCVM, entNum, client.Edict)

	return nil
}

func (s *Server) runClientPutInServerQC(client *Client) error {
	if s.QCVM == nil || s.QCVM.FindFunction("PutClientInServer") < 0 {
		return s.initClientSpawnFallback(client)
	}
	if err := s.runClientQCFunction(client, "PutClientInServer", true); err != nil {
		return err
	}
	if client == nil || client.Edict == nil || client.Edict.Vars == nil {
		return nil
	}
	if client.Edict.Vars.Health <= 0 || s.GetString(client.Edict.Vars.ClassName) == "" {
		return s.initClientSpawnFallback(client)
	}
	s.LinkEdict(client.Edict, true)
	return nil
}

func (s *Server) runClientParseClientCommandQC(client *Client, cmd string) error {
	if client == nil || client.Edict == nil {
		return fmt.Errorf("command %q rejected", cmd)
	}
	if s.QCVM == nil {
		return fmt.Errorf("command %q rejected", cmd)
	}
	funcNum := s.QCVM.FindFunction("SV_ParseClientCommand")
	if funcNum < 0 {
		return fmt.Errorf("command %q rejected", cmd)
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return fmt.Errorf("command %q rejected", cmd)
	}

	s.syncQCVMState()
	syncEdictToQCVM(s.QCVM, entNum, client.Edict)
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	s.QCVM.SetGString(qc.OFSParm0, cmd)
	if err := s.executeQCFunction(funcNum); err != nil {
		return fmt.Errorf("SV_ParseClientCommand execution failed: %w", err)
	}
	syncEdictFromQCVM(s.QCVM, entNum, client.Edict)
	return nil
}

func (s *Server) runClientKillQC(client *Client) error {
	return s.runClientQCFunction(client, "ClientKill", false)
}

func (s *Server) clientIndex(target *Client) int {
	if s == nil || s.Static == nil || target == nil {
		return -1
	}
	for i, client := range s.Static.Clients {
		if client == target {
			return i
		}
	}
	return -1
}

func (s *Server) SubmitLoopbackStringCommand(clientNum int, cmd string) error {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return fmt.Errorf("invalid client number %d", clientNum)
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return fmt.Errorf("client %d is nil", clientNum)
	}
	client.Loopback = true
	if err := s.executeClientStringCommand(client, cmd); err != nil {
		return err
	}
	if client.Message == nil {
		client.Message = NewMessageBuffer(MaxDatagram)
	}
	if client.SendSignon == SignonPrespawn {
		s.queuePendingSignon(client)
	}

	return nil
}

func (s *Server) SubmitLoopbackCmd(clientNum int, viewAngles [3]float32, forward, side, up float32, buttons, impulse int, sentTime float64) error {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return fmt.Errorf("invalid client number %d", clientNum)
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return fmt.Errorf("client %d is nil", clientNum)
	}
	client.Loopback = true

	client.LastCmd = UserCmd{
		ViewAngles:  viewAngles,
		ForwardMove: forward,
		SideMove:    side,
		UpMove:      up,
		Buttons:     uint8(buttons),
		Impulse:     uint8(impulse),
	}
	client.LoopbackCmdPending = true
	client.PingTimes[client.NumPings%NumPingTimes] = s.Time - float32(sentTime)
	client.NumPings++

	if client.Edict != nil {
		client.Edict.Vars.VAngle = viewAngles
		client.Edict.Vars.Button0 = float32(uint8(buttons) & 1)
		client.Edict.Vars.Button2 = float32((uint8(buttons) & 2) >> 1)
		if impulse != 0 {
			client.Edict.Vars.Impulse = float32(uint8(impulse))
		}
	}

	return nil
}

func (s *Server) SV_ReadClientMessage(client *Client, buf *MessageBuffer) bool {
	for buf.ReadPos < buf.Len() {
		ccmd := int(buf.ReadChar())
		if buf.BadRead {
			return false
		}

		switch ccmd {
		case -1:
			return true
		case int(CLCNop):
			continue
		case int(CLCStringCmd):
			cmd := buf.ReadString()
			if err := s.executeClientStringCommand(client, cmd); err != nil {
				return false
			}
		case int(CLCDisconnect):
			return false
		case int(CLCMove):
			client.LastCmd = s.ReadClientMove(client, buf)
		default:
			return false
		}

		if !client.Active {
			return false
		}
	}
	return !buf.BadRead
}

func (s *Server) ReadClientMessage(client *Client, buf *MessageBuffer) bool {
	return s.SV_ReadClientMessage(client, buf)
}
