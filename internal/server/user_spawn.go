// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.

package server

import (
	"fmt"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	srvnet "github.com/darkliquid/ironwail-go/internal/server/net"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func (s *Server) writeSpawnSnapshot(client *Client) {
	if client == nil || client.Message == nil {
		return
	}

	client.Message.Clear()
	client.Message.PutByte(byte(inet.SVCTime))
	client.Message.WriteFloat(s.Time)
	s.writeSpawnClientRoster(client, client.Message)
	s.writeSpawnLightStyles(client.Message)
	s.writeSpawnGlobalStats(client, client.Message)
	s.writeSpawnSetAngle(client, client.Message)
	s.WriteClientDataToMessage(client.Edict, client.Message)
	s.writeSpawnSkybox(client, client.Message)
	client.Message.PutByte(byte(inet.SVCSignOnNum))
	client.Message.PutByte(3)
}

func (s *Server) writeSpawnSkybox(_ *Client, msg *MessageBuffer) {
	if s.SkyboxName != "" && msg != nil {
		// External skyboxes are renderer state, but in Go's real client/server
		// split the client must receive the worldspawn sky name over the protocol.
		msg.PutByte(byte(inet.SVCSkyBox))
		msg.WriteString(s.SkyboxName)
	}
}

func (s *Server) writeSpawnClientRoster(_ *Client, msg *MessageBuffer) {
	if s.Static == nil {
		return
	}
	srvnet.WriteSpawnClientRoster(msg, s.Static.Clients, s)
}

func (s *Server) writeSpawnLightStyles(msg *MessageBuffer) {
	srvnet.WriteSpawnLightStyles(msg, s.LightStyles)
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
	msg.PutByte(byte(inet.SVCUpdateStat))
	msg.PutByte(byte(inet.StatTotalSecrets))
	msg.WriteLong(stats[inet.StatTotalSecrets])
	msg.PutByte(byte(inet.SVCUpdateStat))
	msg.PutByte(byte(inet.StatTotalMonsters))
	msg.WriteLong(stats[inet.StatTotalMonsters])
	msg.PutByte(byte(inet.SVCUpdateStat))
	msg.PutByte(byte(inet.StatSecrets))
	msg.WriteLong(stats[inet.StatSecrets])
	msg.PutByte(byte(inet.SVCUpdateStat))
	msg.PutByte(byte(inet.StatMonsters))
	msg.WriteLong(stats[inet.StatMonsters])
}

func (s *Server) writeSpawnSetAngle(client *Client, msg *MessageBuffer) {
	srvnet.WriteSpawnSetAngle(msg, client, uint32(s.ProtocolFlags()), s, s.LoadGame)
}

func (s *Server) findLocalSpawnPoint() *Edict {
	for _, className := range []string{"info_player_start", "testplayerstart"} {
		for entNum := 1; entNum < s.NumEdicts; entNum++ {
			ent := s.Edicts[entNum]
			if ent == nil || ent.Free {
				continue
			}
			if s.String(ent.ClassName(s)) == className {
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
	savedFrags := ent.Frags(s)
	ent.SetColormap(s, float32(entNum))
	ent.SetTeam(s, float32((client.Color&15)+1))
	ent.SetFrags(s, savedFrags)
	ent.SetHealth(s, 100)
	ent.SetTakeDamage(s, 1)
	ent.SetDeadFlag(s, 0)
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetViewOfs(s, qtypes.Vec3{Z: ViewHeight})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSize(s, qtypes.Vec3{X: 32, Y: 32, Z: 56})
	ent.SetVelocity(s, qtypes.Vec3{})
	ent.SetAVelocity(s, qtypes.Vec3{})
	ent.SetFixAngle(s, 1)

	if spawn := s.findLocalSpawnPoint(); spawn != nil {
		spawnOrigin := spawn.Origin(s)
		spawnAngles := spawn.Angles(s)
		ent.SetOrigin(s, spawnOrigin)
		ent.SetAngles(s, spawnAngles)
		ent.SetVAngle(s, spawnAngles)
	}
	entOrigin := ent.Origin(s)
	entMins := ent.Mins(s)
	entMaxs := ent.Maxs(s)
	ent.SetAbsMin(s, entOrigin.Add(entMins))
	ent.SetAbsMax(s, entOrigin.Add(entMaxs))

	if client.Name == "" {
		client.Name = "player"
	}
	if s.QCVM != nil {
		ent.SetClassName(s, s.QCVM.AllocString("player"))
		ent.SetNetName(s, s.QCVM.AllocString(client.Name))
		if playerModel := s.FindModel("progs/player.mdl"); playerModel != 0 {
			ent.SetModelIndex(s, float32(playerModel))
			ent.SetModel(s, s.QCVM.AllocString("progs/player.mdl"))
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
	SvdbgMultiplayerLogf("connect ent=%d name=%q", s.NumForEdict(client.Edict), client.Name)
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
	s.SyncQCVMGlobals()

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

	if functionName == "ClientConnect" {
		s.syncClientAnimControllerFromQCVM(entNum)
	}

	return nil
}

func (s *Server) syncClientAnimControllerFromQCVM(clientEntNum int) {
	if s == nil || s.QCVM == nil {
		return
	}
	// Some mods create a client-owned helper entity during ClientConnect and
	// store it in a custom player field. Import that helper before the next QC
	// entry point so syncQCVMState does not republish stale Go-side fields.
	animOfs := s.QCVM.FindField("animcontroller")
	if animOfs < 0 {
		return
	}
	animEntNum := int(s.QCVM.EEntity(clientEntNum, animOfs))
	if animEntNum <= 0 || animEntNum >= s.NumEdicts {
		return
	}
	animEnt := s.EdictNum(animEntNum)
	if animEnt == nil || animEnt.Free {
		return
	}
}

func (s *Server) runClientPutInServerQC(client *Client) error {
	if s.QCVM == nil || s.QCVM.FindFunction("PutClientInServer") < 0 {
		return s.initClientSpawnFallback(client)
	}
	if err := s.runClientQCFunction(client, "PutClientInServer", true); err != nil {
		return err
	}
	if client == nil || client.Edict == nil {
		return nil
	}
	if client.Edict.Health(s) <= 0 || s.String(client.Edict.ClassName(s)) == "" {
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

	s.SyncQCVMGlobals()
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	s.QCVM.SetGString(qc.OFSParm0, cmd)
	if err := s.executeQCFunction(funcNum); err != nil {
		return fmt.Errorf("SV_ParseClientCommand execution failed: %w", err)
	}
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

func (s *Server) SubmitLoopbackCmd(clientNum int, viewAngles qtypes.Vec3, forward, side, up float32, buttons, impulse int, sentTime float64) error {
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
		client.Edict.SetVAngle(s, viewAngles)
		client.Edict.SetButton0(s, float32(uint8(buttons)&1))
		client.Edict.SetButton2(s, float32((uint8(buttons)&2)>>1))
		if impulse != 0 {
			client.Edict.SetImpulse(s, float32(uint8(impulse)))
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
