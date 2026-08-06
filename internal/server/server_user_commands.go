// This file belongs to the Server Lifecycle subsystem: server struct, frame timing, PVS, user commands, spawn parameters, rules, and core server types.

package server

import (
	"fmt"
	"log/slog"
	"math"
	"strings"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	srvcmds "github.com/darkliquid/ironwail-go/internal/server/commands"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

const (
	maxForwardSamples = 6
	idealPitchScale   = 0.8
	edgeFriction      = 2.0
	svMaxSpeed        = 320.0
	svAccelerate      = 10.0
)

var svAllowedUserCommands = []string{
	"status",
	"god",
	"notarget",
	"fly",
	"name",
	"noclip",
	"setpos",
	"say",
	"say_team",
	"tell",
	"color",
	"kill",
	"pause",
	"spawn",
	"begin",
	"prespawn",
	"kick",
	"ping",
	"give",
	"ban",
}

func (s *Server) SetIdealPitch(ent *Edict) {
	if ent == nil || uint32(ent.Flags(s))&FlagOnGround == 0 {
		return
	}

	entAngles := ent.Angles(s)
	angle := float64(entAngles[1]) * math.Pi * 2 / 360
	sinVal := float32(math.Sin(angle))
	cosVal := float32(math.Cos(angle))

	entOrigin := ent.Origin(s)
	entViewOfs := ent.ViewOfs(s)
	var z [maxForwardSamples]float32
	for i := 0; i < maxForwardSamples; i++ {
		top := [3]float32{
			entOrigin[0] + cosVal*float32(i+3)*12,
			entOrigin[1] + sinVal*float32(i+3)*12,
			entOrigin[2] + entViewOfs[2],
		}
		bottom := [3]float32{top[0], top[1], top[2] - 160}

		tr := s.Move(top, [3]float32{}, [3]float32{}, bottom, MoveType(MoveNoMonsters), ent)
		if tr.AllSolid || tr.Fraction == 1 {
			return
		}

		z[i] = top[2] + tr.Fraction*(bottom[2]-top[2])
	}

	var dir float32
	steps := 0
	for i := 1; i < maxForwardSamples; i++ {
		step := z[i] - z[i-1]
		if step > -OneEpsilon && step < OneEpsilon {
			continue
		}

		if dir != 0 && (step-dir > OneEpsilon || step-dir < -OneEpsilon) {
			return
		}

		steps++
		dir = step
	}

	if dir == 0 {
		ent.SetIdealPitch(s, 0)
		return
	}

	if steps < 2 {
		return
	}

	ent.SetIdealPitch(s, -dir*idealPitchScale)
}


func (s *Server) SV_ClientThink(client *Client) {
	s.ensurePhysicsSys()
	s.PhysicsSys.SV_ClientThink(client)
}

func (s *Server) PlayerClient(ent *Edict) *Client {
	if s == nil || s.Static == nil || ent == nil {
		return nil
	}

	entNum := s.NumForEdict(ent)
	if entNum <= 0 || entNum > len(s.Static.Clients) {
		return nil
	}

	client := s.Static.Clients[entNum-1]
	if client == nil || !client.Active || !client.Spawned || client.Edict != ent {
		return nil
	}

	return client
}

func (s *Server) runClientQCThink(client *Client, funcName string) {
	s.RunClientQCThinkWithMode(client, funcName, true)
}

func (s *Server) RunClientQCThinkWithMode(client *Client, funcName string, fullSync bool) {
	if s == nil || s.QCVM == nil || client == nil || client.Edict == nil || client.Edict.Free {
		return
	}

	funcIdx := s.QCVM.FindFunction(funcName)
	if funcIdx < 0 {
		return
	}

	entNum := s.NumForEdict(client.Edict)
	if entNum <= 0 {
		return
	}

	if fullSync {
		s.SyncQCVMGlobals()
	}
	s.syncEdictToQCVM(entNum, client.Edict)
	s.QCVM.Time = float64(s.Time)
	s.QCVM.SetGlobal("time", s.Time)
	s.QCVM.SetGlobal("frametime", s.FrameTime)
	s.QCVM.SetGlobal("self", entNum)
	s.QCVM.SetGlobal("other", 0)
	s.QCVM.SetGlobal("msg_entity", entNum)
	if svDebugMoveLevel() >= 1 {
		org := client.Edict.Origin(s)
		vel := client.Edict.Velocity(s)
		ang := client.Edict.VAngle(s)
		SvdbgMoveLogf("%s/pre ent=%d origin=[%.3f %.3f %.3f] velocity=[%.3f %.3f %.3f] angles=[%.3f %.3f %.3f] flags=%d movetype=%d",
			funcName, entNum,
			org[0], org[1], org[2],
			vel[0], vel[1], vel[2],
			ang[0], ang[1], ang[2],
			int(client.Edict.Flags(s)), int(client.Edict.MoveType(s)))
	}
	if err := s.executeQCFunction(funcIdx); err != nil {
		slog.Warn("client think QC failed", "function", funcName, "entity", entNum, "error", err)
		return
	}
	s.syncEdictFromQCVM(entNum, client.Edict)
	if svDebugMoveLevel() >= 1 {
		org := client.Edict.Origin(s)
		vel := client.Edict.Velocity(s)
		ang := client.Edict.VAngle(s)
		SvdbgMoveLogf("%s/post ent=%d origin=[%.3f %.3f %.3f] velocity=[%.3f %.3f %.3f] angles=[%.3f %.3f %.3f] flags=%d movetype=%d",
			funcName, entNum,
			org[0], org[1], org[2],
			vel[0], vel[1], vel[2],
			ang[0], ang[1], ang[2],
			int(client.Edict.Flags(s)), int(client.Edict.MoveType(s)))
	}
}

func (s *Server) ClientThink(client *Client) {
	s.SV_ClientThink(client)
}

func (s *Server) ReadClientMove(client *Client, buf *MessageBuffer) UserCmd {
	var cmd UserCmd

	pingTime := buf.ReadFloat()
	client.PingTimes[client.NumPings%NumPingTimes] = s.Time - pingTime
	client.NumPings++

	for i := 0; i < 3; i++ {
		// NetQuake uses 8-bit angles, FitzQuake/RMQ use 16-bit
		if s.Protocol == ProtocolNetQuake {
			cmd.ViewAngles[i] = buf.ReadAngle(0)
		} else {
			cmd.ViewAngles[i] = buf.ReadAngle16()
		}
	}

	if client.Edict != nil {
		client.Edict.SetVAngle(s, cmd.ViewAngles)
	}

	cmd.ForwardMove = float32(buf.ReadShort())
	cmd.SideMove = float32(buf.ReadShort())
	cmd.UpMove = float32(buf.ReadShort())

	bits := buf.Byte()
	cmd.Buttons = bits
	if client.Edict != nil {
		client.Edict.SetButton0(s, float32(bits&1))
		client.Edict.SetButton2(s, float32((bits&2)>>1))
	}

	impulse := buf.Byte()
	cmd.Impulse = impulse
	if impulse != 0 && client.Edict != nil {
		client.Edict.SetImpulse(s, float32(impulse))
	}

	return cmd
}

func (s *Server) SV_ExecuteUserCommand(client *Client, cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return true
	}

	lower := strings.ToLower(cmd)
	args := strings.Fields(cmd)
	if len(args) == 0 {
		return true
	}
	verb := strings.ToLower(args[0])

	switch verb {
	case "say":
		if len(args) < 2 {
			return true
		}
		msg := strings.Join(args[1:], " ")
		s.SV_BroadcastPrintf("%s: %s\n", client.Name, msg)
		return true
	case "say_team":
		if len(args) < 2 {
			return true
		}
		msg := strings.Join(args[1:], " ")
		if s.Static == nil {
			return true
		}
		for _, c := range s.Static.Clients {
			if c == nil || !c.Active || c.Edict == nil {
				continue
			}
			if c.Edict.Team(s) == client.Edict.Team(s) {
				s.SV_ClientPrintf(c, "(team) %s: %s\n", client.Name, msg)
			}
		}
		return true
	case "tell":
		if len(args) < 3 {
			return true
		}
		targetName := args[1]
		msg := strings.Join(args[2:], " ")
		if s.Static == nil {
			return true
		}
		for _, c := range s.Static.Clients {
			if c == nil || !c.Active {
				continue
			}
			if strings.EqualFold(c.Name, targetName) {
				s.SV_ClientPrintf(c, "%s tells you: %s\n", client.Name, msg)
				s.SV_ClientPrintf(client, "you tell %s: %s\n", c.Name, msg)
				return true
			}
		}
		s.SV_ClientPrintf(client, "player %s not found\n", targetName)
		return true
	case "name":
		newName := parseClientNameCommand(cmd)
		if newName == "" {
			s.SV_ClientPrintf(client, "name is %s\n", client.Name)
			return true
		}
		oldName := client.Name
		s.SV_BroadcastPrintf("%s changed name to %s\n", oldName, newName)
		if clientNum := s.clientIndex(client); clientNum >= 0 {
			s.SetClientName(clientNum, newName)
		} else {
			client.Name = newName
			if client.Edict != nil && s.QCVM != nil {
				client.Edict.SetNetName(s, s.QCVM.AllocString(client.Name))
			}
		}
		return true
	case "color":
		if clientStringCommandArgs(cmd) == "" {
			s.SV_ClientPrintf(client, "color is %d\n", client.Color)
			return true
		}
		color := parseClientColorCommand(cmd)
		if clientNum := s.clientIndex(client); clientNum >= 0 {
			s.SetClientColor(clientNum, color)
		} else {
			client.Color = color
			if client.Edict != nil {
				client.Edict.SetTeam(s, float32((color&15)+1))
			}
		}
		return true
	case "kill":
		if clientNum := s.clientIndex(client); clientNum >= 0 {
			s.KillClient(clientNum)
		}
		return true
	}

	for _, allowed := range svAllowedUserCommands {
		if strings.HasPrefix(lower, allowed) {
			return true
		}
	}

	return false
}

func clientStringCommandVerb(cmd string) string {
	return srvcmds.ClientStringCommandVerb(cmd)
}

func clientStringCommandArgs(cmd string) string {
	return srvcmds.ClientStringCommandArgs(cmd)
}

func parseClientNameCommand(cmd string) string {
	return srvcmds.ParseClientNameCommand(cmd)
}

func parseClientColorCommand(cmd string) int {
	return srvcmds.ParseClientColorCommand(cmd)
}

func (s *Server) ExecuteClientString(client *Client, cmd string) bool {
	return s.executeClientStringCommand(client, cmd) == nil
}

func (s *Server) executeClientStringCommand(client *Client, cmd string) error {
	if !s.SV_ExecuteUserCommand(client, cmd) {
		if err := s.runClientParseClientCommandQC(client, cmd); err != nil {
			return err
		}
		return nil
	}
	return s.handleClientStringCommand(client, cmd)
}

func (s *Server) handleClientStringCommand(client *Client, cmd string) error {
	switch clientStringCommandVerb(cmd) {
	case "prespawn":
		if client.SendSignon != SignonFlush {
			return fmt.Errorf("prespawn out of order")
		}
		client.SignonIdx = 0
		client.SendSignon = SignonPrespawn
	case "spawn":
		if client.SendSignon != SignonSignonBufs {
			return fmt.Errorf("spawn out of order")
		}
		if !s.LoadGame {
			if err := s.runClientSpawnQC(client); err != nil {
				return err
			}
		}
		s.writeSpawnSnapshot(client)
		client.SendSignon = SignonNone
	case "begin":
		if client.SendSignon != SignonNone {
			return fmt.Errorf("begin out of order")
		}
		client.Spawned = true
	case "ban":
		if s.CVar.IntValue("deathmatch") != 0 {
			return nil
		}
		args := strings.Fields(clientStringCommandArgs(cmd))
		switch len(args) {
		case 0:
			s.SV_ClientPrintf(client, "%s\n", s.Net.IPBanStatus())
		case 1:
			if err := s.Net.SetIPBan(args[0], ""); err != nil {
				s.SV_ClientPrintf(client, "%s\n", err.Error())
			}
		case 2:
			if err := s.Net.SetIPBan(args[0], args[1]); err != nil {
				s.SV_ClientPrintf(client, "%s\n", err.Error())
			}
		default:
			s.SV_ClientPrintf(client, "BAN ip_address [mask]\n")
		}
	}
	return nil
}

func (s *Server) RunClients() {
	if s.Static == nil {
		return
	}

	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		if client.Loopback {
			if !client.LoopbackCmdPending {
				client.LastCmd = UserCmd{}
			}
			client.LoopbackCmdPending = false
		} else {
			if client.NetConnection == nil && client.Message != nil && client.Message.Len() > 0 {
				if !s.SV_ReadClientMessage(client, client.Message) {
					s.DropClient(client, false)
					continue
				}
				client.Message.Clear()
				client.LastMessage = float64(s.Time)
				goto processedInput
			}
			if client.NetConnection == nil {
				s.DropClient(client, false)
				continue
			}
			for {
				msgType, payload := s.Net.Message(client.NetConnection)
				if msgType == 0 {
					break
				}
				if msgType != 1 && msgType != 2 {
					s.DropClient(client, false)
					break
				}
				incoming := NewMessageBuffer(len(payload))
				incoming.Write(payload)
				if !s.SV_ReadClientMessage(client, incoming) {
					s.DropClient(client, false)
					break
				}
				client.LastMessage = float64(s.Time)
			}
		}

	processedInput:
		if !client.Spawned {
			client.LastCmd = UserCmd{}
			continue
		}
		if s.handleDeathmatchRespawn(client) {
			client.LastCmd = UserCmd{}
			continue
		}

		if !s.Paused {
			s.SV_ClientThink(client)
		}
	}
}

func (s *Server) DropClient(client *Client, crash bool) {
	if client == nil || !client.Active {
		return
	}

	if !crash && client.NetConnection != nil && s.Net.CanSendMessage(client.NetConnection) {
		client.Message.PutByte(byte(inet.SVCDisconnect))
		_ = s.Net.SendMessage(client.NetConnection, client.Message.Data[:client.Message.Len()])
	}

	if !crash && client.Spawned && client.Edict != nil && s.QCVM != nil {
		funcIdx := s.QCVM.FindFunction("ClientDisconnect")
		if funcIdx >= 0 {
			s.SetQCTimeGlobal(s.Time)
			s.QCVM.SetGlobal("self", s.NumForEdict(client.Edict))
			_ = s.executeQCFunction(funcIdx)
		}
		SvdbgMultiplayerLogf("disconnect ent=%d name=%q crash=%v", s.NumForEdict(client.Edict), client.Name, crash)
	}

	if client.NetConnection != nil {
		s.Net.Close(client.NetConnection)
		client.NetConnection = nil
	}

	clientName := client.Name
	clientColor := client.Color
	if client.Edict != nil {
		client.Edict.SetNetName(s, 0)
		client.Edict.SetTeam(s, 0)
		client.Edict.SetFrags(s, 0)
	}

	client.Active = false
	client.Spawned = false
	client.RespawnTime = 0
	client.DropASAP = false
	client.Name = ""
	client.Color = 0
	client.OldFrags = -999999

	if s.Static != nil {
		clientNum := s.clientIndex(client)
		if clientNum >= 0 {
			s.broadcastClientNameUpdate(clientNum, "")
			for _, receiver := range s.Static.Clients {
				if receiver == nil || !receiver.Active || receiver.Message == nil {
					continue
				}
				receiver.Message.PutByte(byte(inet.SVCUpdateFrags))
				receiver.Message.PutByte(byte(clientNum))
				receiver.Message.WriteShort(0)
			}
			s.broadcastClientColorUpdate(clientNum, 0)
		}
	}

	if clientName != "" && !crash {
		slog.Info("client dropped", "name", clientName, "color", clientColor)
	}
}

func AngleVectors(angles [3]float32, forward, right, up *[3]float32) {
	srvtypes.AngleVectors(angles, forward, right, up)
}
