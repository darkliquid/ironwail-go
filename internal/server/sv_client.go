package server

import (
	"errors"
	"fmt"
	"log/slog"

	inet "github.com/ironwail/ironwail-go/internal/net"
)

// writeSpawnStaticMessage emits SVCSpawnStatic(_2) for entities baked into signon world state.
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

// writeSpawnStaticSoundMessage emits ambient/static sound signon messages with large-index fallback.
func (s *Server) writeSpawnStaticSoundMessage(msg *MessageBuffer, snd StaticSound) {
	flags := uint32(s.ProtocolFlags())
	if snd.SoundIndex > 255 {
		msg.WriteByte(byte(inet.SVCSpawnStaticSound2))
		for i := 0; i < 3; i++ {
			msg.WriteCoord(snd.Origin[i], flags)
		}
		msg.WriteShort(int16(snd.SoundIndex))
		msg.WriteByte(byte(snd.Volume))
		msg.WriteByte(byte(snd.Attenuation * 64))
		return
	}
	msg.WriteByte(byte(inet.SVCSpawnStaticSound))
	for i := 0; i < 3; i++ {
		msg.WriteCoord(snd.Origin[i], flags)
	}
	msg.WriteByte(byte(snd.SoundIndex))
	msg.WriteByte(byte(snd.Volume))
	msg.WriteByte(byte(snd.Attenuation * 64))
}

// SendServerInfo writes the initial serverinfo handshake block for a connecting client.
func (s *Server) SendServerInfo(client *Client) {
	client.Message.WriteByte(byte(inet.SVCPrint))
	client.Message.WriteString(fmt.Sprintf("\nFITZQUAKE GO SERVER\n"))

	client.Message.WriteByte(byte(inet.SVCServerInfo))
	client.Message.WriteLong(int32(s.Protocol))
	if s.Protocol == ProtocolRMQ {
		client.Message.WriteLong(int32(s.ProtocolFlags()))
	}
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

	client.Message.WriteByte(byte(inet.SVCSetView))
	client.Message.WriteShort(int16(s.NumForEdict(client.Edict)))

	client.Message.WriteByte(byte(inet.SVCSignOnNum))
	client.Message.WriteByte(1)

	client.SendSignon = SignonFlush
	client.SignonIdx = 0
	client.Spawned = false
}

// GetString resolves a QC string table index into UTF-8 text for game messages and model names.
func (s *Server) GetString(idx int32) string {
	if idx == 0 {
		return ""
	}
	if s.QCVM == nil {
		return ""
	}
	return s.QCVM.GetString(idx)
}

// ConnectClient initializes one client slot, bind its edict, runs spawn parm QC, and starts signon.
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
	client.RespawnTime = 0
	client.SignonIdx = 0
	client.Edict = ent
	client.Name = "unconnected"
	if client.Message != nil {
		client.Message.Clear()
	}
	if client.EntityStates == nil {
		client.EntityStates = make(map[int]EntityState)
	} else {
		clear(client.EntityStates)
	}

	if s.LoadGame {
	} else {
		if s.QCVM != nil {
			if setNewParms := s.QCVM.FindFunction("SetNewParms"); setNewParms >= 0 {
				s.setQCTimeGlobal(s.Time)
				_ = s.executeQCFunction(setNewParms)
				for i := 0; i < NumSpawnParms; i++ {
					client.SpawnParms[i] = float32(s.QCVM.GetGlobalFloat(fmt.Sprintf("parm%d", i+1)))
				}
			}
		}
	}

	s.SendServerInfo(client)
}

// ClearDatagram resets the shared unreliable broadcast packet assembled each simulation frame.
func (s *Server) ClearDatagram() {
	s.Datagram.Clear()
}

// ============================================================================
// SIGNON BUFFER SYSTEM
// ============================================================================

// AddSignonBuffer allocates a new signon buffer and sets it as the current
// write target. Signon buffers hold the initial game state (precache lists,
// static entities, ambient sounds) that is sent to every connecting client.
// This mirrors SV_AddSignonBuffer in C Ironwail (sv_main.c:1485).
func (s *Server) AddSignonBuffer() error {
	if len(s.SignonBuffers) >= MaxSignonBuffers {
		return fmt.Errorf("SV_AddSignonBuffer: overflow (%d buffers)", MaxSignonBuffers)
	}
	buf := NewMessageBuffer(SignonSize)
	s.SignonBuffers = append(s.SignonBuffers, buf)
	s.Signon = buf
	return nil
}

// ReserveSignonSpace ensures the current signon buffer has room for size bytes.
// If the current buffer would overflow, a new buffer is allocated.
// This mirrors SV_ReserveSignonSpace in C Ironwail (sv_main.c:1503).
func (s *Server) ReserveSignonSpace(size int) error {
	if s.Signon == nil {
		return s.AddSignonBuffer()
	}
	if s.Signon.Len()+size > len(s.Signon.Data) {
		return s.AddSignonBuffer()
	}
	return nil
}

// WriteSignonByte writes a single byte to the current signon buffer,
// allocating a new buffer if needed.
func (s *Server) WriteSignonByte(b byte) error {
	if err := s.ReserveSignonSpace(1); err != nil {
		return err
	}
	s.Signon.WriteByte(b)
	return nil
}

// WriteSignonShort writes a 16-bit integer to the current signon buffer.
func (s *Server) WriteSignonShort(v int16) error {
	if err := s.ReserveSignonSpace(2); err != nil {
		return err
	}
	s.Signon.WriteShort(v)
	return nil
}

// WriteSignonLong writes a 32-bit integer to the current signon buffer.
func (s *Server) WriteSignonLong(v int32) error {
	if err := s.ReserveSignonSpace(4); err != nil {
		return err
	}
	s.Signon.WriteLong(v)
	return nil
}

// WriteSignonFloat writes a 32-bit float to the current signon buffer.
func (s *Server) WriteSignonFloat(f float32) error {
	if err := s.ReserveSignonSpace(4); err != nil {
		return err
	}
	s.Signon.WriteFloat(f)
	return nil
}

// WriteSignonString writes a null-terminated string to the current signon buffer.
func (s *Server) WriteSignonString(str string) error {
	if err := s.ReserveSignonSpace(len(str) + 1); err != nil {
		return err
	}
	s.Signon.WriteString(str)
	return nil
}

// WriteSignonCoord writes a coordinate to the current signon buffer.
func (s *Server) WriteSignonCoord(c float32) error {
	flags := uint32(s.ProtocolFlags())
	if err := s.ReserveSignonSpace(coordWireSize(flags)); err != nil {
		return err
	}
	s.Signon.WriteCoord(c, flags)
	return nil
}

// WriteSignonAngle writes an angle to the current signon buffer.
func (s *Server) WriteSignonAngle(a float32) error {
	flags := uint32(s.ProtocolFlags())
	if err := s.ReserveSignonSpace(angleWireSize(flags)); err != nil {
		return err
	}
	s.Signon.WriteAngle(a, flags)
	return nil
}

// WriteSignonData writes raw bytes to the current signon buffer.
func (s *Server) WriteSignonData(data []byte) error {
	if err := s.ReserveSignonSpace(len(data)); err != nil {
		return err
	}
	s.Signon.Write(data)
	return nil
}

// SendSignonBuffers copies all signon buffer data to the client's message
// buffer. This is called during the prespawn phase to send the initial game
// state (precache lists, static entities, ambient sounds) to a connecting client.
func (s *Server) SendSignonBuffers(client *Client) {
	for _, buf := range s.SignonBuffers {
		if buf.Len() > 0 {
			client.Message.Write(buf.Data[:buf.Len()])
		}
	}
}

// buildSignonBuffers populates the server's signon buffers with static entity
// and sound data that is shared across all connecting clients. Called once
// during SpawnServer after map entities have been loaded.
func (s *Server) buildSignonBuffers() error {
	s.SignonBuffers = nil
	s.Signon = nil
	if err := s.AddSignonBuffer(); err != nil {
		return err
	}

	// Snapshot dynamic entity baselines and write svc_spawnbaseline(_2) signon data.
	s.CreateBaseline()
	for entNum := 0; entNum < s.NumEdicts; entNum++ {
		ent := s.Edicts[entNum]
		if ent == nil || ent.Free {
			continue
		}
		if s.Static != nil && entNum > s.Static.MaxClients && ent.Baseline.ModelIndex == 0 {
			continue
		}
		if err := s.writeSpawnBaselineToSignon(entNum, ent.Baseline); err != nil {
			return err
		}
	}

	// Write static entity baselines.
	for _, ent := range s.StaticEntities {
		if err := s.writeSpawnStaticToSignon(ent); err != nil {
			return err
		}
	}

	// Write static/ambient sounds.
	for _, snd := range s.StaticSounds {
		if err := s.writeSpawnStaticSoundToSignon(snd); err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) writeSpawnBaselineToSignon(entNum int, ent EntityState) error {
	extended := s.Protocol != ProtocolNetQuake &&
		(ent.ModelIndex > 255 || ent.Frame > 255 || ent.Alpha != 0 || (ent.Scale != 0 && ent.Scale != 16))

	if extended {
		if err := s.WriteSignonByte(byte(inet.SVCSpawnBaseline2)); err != nil {
			return err
		}
	} else {
		if err := s.WriteSignonByte(byte(inet.SVCSpawnBaseline)); err != nil {
			return err
		}
	}

	payload := NewMessageBuffer(64)
	s.writeEntityState(payload, ent, extended, true, entNum)
	return s.WriteSignonData(payload.Data[:payload.Len()])
}

// writeSpawnStaticToSignon writes a static entity spawn message into the
// current signon buffer, using SVCSpawnStatic2 for extended entities.
func (s *Server) writeSpawnStaticToSignon(ent EntityState) error {
	extended := ent.ModelIndex > 255 || ent.Frame > 255 || ent.Alpha != 0 || (ent.Scale != 0 && ent.Scale != 16)

	if extended {
		if err := s.WriteSignonByte(byte(inet.SVCSpawnStatic2)); err != nil {
			return err
		}
	} else {
		if err := s.WriteSignonByte(byte(inet.SVCSpawnStatic)); err != nil {
			return err
		}
	}

	// Write entity state bits and fields (mirrors writeEntityState logic).
	var bits byte
	if extended {
		if ent.ModelIndex > 255 {
			bits |= 1 << 0
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
		if err := s.WriteSignonByte(bits); err != nil {
			return err
		}
	}

	// Model index.
	if extended && bits&(1<<0) != 0 {
		if err := s.WriteSignonShort(int16(ent.ModelIndex)); err != nil {
			return err
		}
	} else {
		if err := s.WriteSignonByte(byte(ent.ModelIndex)); err != nil {
			return err
		}
	}

	// Frame.
	if extended && bits&(1<<1) != 0 {
		if err := s.WriteSignonShort(int16(ent.Frame)); err != nil {
			return err
		}
	} else {
		if err := s.WriteSignonByte(byte(ent.Frame)); err != nil {
			return err
		}
	}

	// Colormap and skin.
	if err := s.WriteSignonByte(byte(ent.Colormap)); err != nil {
		return err
	}
	if err := s.WriteSignonByte(byte(ent.Skin)); err != nil {
		return err
	}

	// Origin and angles.
	for i := 0; i < 3; i++ {
		if err := s.WriteSignonCoord(ent.Origin[i]); err != nil {
			return err
		}
	}
	for i := 0; i < 3; i++ {
		if err := s.WriteSignonAngle(ent.Angles[i]); err != nil {
			return err
		}
	}

	// Extended fields.
	if extended && bits&(1<<2) != 0 {
		if err := s.WriteSignonByte(ent.Alpha); err != nil {
			return err
		}
	}
	if extended && bits&(1<<3) != 0 {
		if err := s.WriteSignonByte(ent.Scale); err != nil {
			return err
		}
	}

	return nil
}

// writeSpawnStaticSoundToSignon writes a static sound spawn message into the
// current signon buffer, using SVCSpawnStaticSound2 for large sound indices.
func (s *Server) writeSpawnStaticSoundToSignon(snd StaticSound) error {
	if snd.SoundIndex > 255 {
		if err := s.WriteSignonByte(byte(inet.SVCSpawnStaticSound2)); err != nil {
			return err
		}
		for i := 0; i < 3; i++ {
			if err := s.WriteSignonCoord(snd.Origin[i]); err != nil {
				return err
			}
		}
		if err := s.WriteSignonShort(int16(snd.SoundIndex)); err != nil {
			return err
		}
		if err := s.WriteSignonByte(byte(snd.Volume)); err != nil {
			return err
		}
		return s.WriteSignonByte(byte(snd.Attenuation * 64))
	}

	if err := s.WriteSignonByte(byte(inet.SVCSpawnStaticSound)); err != nil {
		return err
	}
	for i := 0; i < 3; i++ {
		if err := s.WriteSignonCoord(snd.Origin[i]); err != nil {
			return err
		}
	}
	if err := s.WriteSignonByte(byte(snd.SoundIndex)); err != nil {
		return err
	}
	if err := s.WriteSignonByte(byte(snd.Volume)); err != nil {
		return err
	}
	return s.WriteSignonByte(byte(snd.Attenuation * 64))
}

// SendClientDatagram builds and would transmit one frame datagram for a spawned network client.
func (s *Server) SendClientDatagram(client *Client) bool {
	var msg MessageBuffer
	msg.Data = make([]byte, MaxDatagram)
	s.buildClientDatagram(client, &msg)
	if client == nil || client.NetConnection == nil || msg.Len() == 0 {
		return true
	}
	if inet.SendUnreliableMessage(client.NetConnection, msg.Data[:msg.Len()]) == -1 {
		return false
	}
	client.LastMessage = float64(s.Time)
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

// GetClientLoopbackMessage merges reliable + frame data for the in-process loopback client path.
func (s *Server) GetClientLoopbackMessage(clientNum int) []byte {
	if clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return nil
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return nil
	}

	var msg MessageBuffer
	msg.Data = make([]byte, MaxDatagram)

	if client.Message != nil && client.Message.Len() > 0 {
		msg.Write(client.Message.Data[:client.Message.Len()])
		client.Message.Clear()
	}

	if client.Active && client.Spawned {
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

// SendNop constructs a keepalive NOP packet used to avoid connection timeout during idle periods.
func (s *Server) SendNop(client *Client) {
	if client == nil || client.NetConnection == nil {
		return
	}
	if inet.SendUnreliableMessage(client.NetConnection, []byte{byte(inet.SVCNop)}) != -1 {
		client.LastMessage = float64(s.Time)
	}
}

// SendClientMessages drives per-client send policy: spawned datagrams vs. signon/reliable traffic.
func (s *Server) SendClientMessages() {
	s.UpdateToReliableMessages()

	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		if client.Spawned {
			if client.Loopback {
				continue
			}
			if !s.SendClientDatagram(client) {
				s.DropClient(client, false)
				continue
			}
		} else {
			s.queuePendingSignon(client)
			if client.Message.Len() == 0 && client.SendSignon == SignonNone {
				if !client.Loopback && float64(s.Time)-client.LastMessage > 5 {
					s.SendNop(client)
				}
			}
			if client.Message.Len() == 0 && client.SendSignon == SignonNone {
				continue
			}
			if client.Message.Len() > 0 && client.SendSignon == SignonNone {
				continue
			}
		}

		if client.Message != nil && client.Message.Overflowed {
			s.DropClient(client, true)
			if client.Message != nil {
				client.Message.Clear()
			}
			continue
		}

		if client.Loopback {
			continue
		}
		if client.Message.Len() == 0 && !client.DropASAP {
			continue
		}
		if client.NetConnection == nil || !inet.CanSendMessage(client.NetConnection) {
			continue
		}
		if client.DropASAP {
			s.DropClient(client, false)
			continue
		}
		if inet.SendMessage(client.NetConnection, client.Message.Data[:client.Message.Len()]) == -1 {
			s.DropClient(client, true)
			continue
		}
		client.Message.Clear()
		client.LastMessage = float64(s.Time)
	}

	s.CleanupEnts()
}

func (s *Server) queuePendingSignon(client *Client) {
	if client == nil || client.Message == nil {
		return
	}
	if client.SendSignon != SignonPrespawn {
		return
	}

	local := client.Loopback || (client.NetConnection != nil && client.NetConnection.Address() == "localhost")
	for client.SignonIdx < len(s.SignonBuffers) {
		buf := s.SignonBuffers[client.SignonIdx]
		if buf == nil || buf.Len() == 0 {
			client.SignonIdx++
			continue
		}
		if client.Message.Len()+buf.Len() > len(client.Message.Data) {
			break
		}
		client.Message.Write(buf.Data[:buf.Len()])
		client.SignonIdx++
		if !local {
			break
		}
	}

	if client.SignonIdx == len(s.SignonBuffers) && client.Message.Len()+2 <= len(client.Message.Data) {
		client.Message.WriteByte(byte(inet.SVCSignOnNum))
		client.Message.WriteByte(2)
		client.SendSignon = SignonSignonBufs
	}
}

// UpdateToReliableMessages queues scoreboard frag changes on reliable channels for all clients.
func (s *Server) UpdateToReliableMessages() {
	for playerNum, changedClient := range s.Static.Clients {
		if changedClient == nil || !changedClient.Active {
			continue
		}
		currentFrags := int(changedClient.Edict.Vars.Frags)
		if changedClient.OldFrags == currentFrags {
			continue
		}

		for _, receiver := range s.Static.Clients {
			if receiver == nil || !receiver.Active {
				continue
			}
			receiver.Message.WriteByte(byte(inet.SVCUpdateFrags))
			receiver.Message.WriteByte(byte(playerNum))
			receiver.Message.WriteShort(int16(currentFrags))
		}

		changedClient.OldFrags = currentFrags
	}
}

// CleanupEnts clears one-frame transient effect bits (e.g. muzzleflash) after packets are built.
func (s *Server) CleanupEnts() {
	for i := 1; i < s.NumEdicts; i++ {
		ent := s.Edicts[i]
		if ent != nil {
			ent.Vars.Effects = float32(uint32(ent.Vars.Effects) & ^uint32(EffectMuzzleFlash))
		}
	}
}

// CreateBaseline snapshots initial entity states used as delta baselines for future updates.
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
			ent.Baseline.Alpha = 0 // ENTALPHA_DEFAULT
			ent.Baseline.Scale = 16
		} else {
			ent.Baseline.Colormap = 0
			ent.Baseline.ModelIndex = s.FindModel(s.GetString(int32(ent.Vars.Model)))
			ent.Baseline.Alpha = ent.Alpha
			ent.Baseline.Scale = 16
		}
	}
}

// SendReconnect writes a reconnect command broadcast used during map/server transitions.
func (s *Server) SendReconnect() {
	var msg MessageBuffer
	msg.Data = make([]byte, 128)
	msg.WriteByte(byte(inet.SVCStuffText))
	msg.WriteString("reconnect\n")
}

// SaveSpawnParms runs SetChangeParms QC to persist per-client parms across level transitions.
func (s *Server) SaveSpawnParms() {
	for _, client := range s.Static.Clients {
		if client == nil || !client.Active {
			continue
		}

		if s.QCVM != nil {
			if setChangeParms := s.QCVM.FindFunction("SetChangeParms"); setChangeParms >= 0 {
				s.setQCTimeGlobal(s.Time)
				s.QCVM.SetGlobal("self", s.NumForEdict(client.Edict))
				_ = s.executeQCFunction(setChangeParms)
				for i := 0; i < NumSpawnParms; i++ {
					client.SpawnParms[i] = float32(s.QCVM.GetGlobalFloat(fmt.Sprintf("parm%d", i+1)))
				}
			}
		}
	}
}

// CheckForNewClients polls the network layer for incoming connections and
// assigns them to a free client slot. Returns an error if a connection arrives
// but no free slot is available.
func (s *Server) CheckForNewClients() error {
	for {
		sock := inet.CheckNewConnections()
		if sock == nil {
			break
		}

		// Find a free client slot
		freeSlot := -1
		for i := 0; i < s.Static.MaxClients; i++ {
			if s.Static.Clients[i] == nil || !s.Static.Clients[i].Active {
				freeSlot = i
				break
			}
		}
		if freeSlot < 0 {
			slog.Warn("CheckForNewClients: no free client slots")
			inet.Close(sock)
			return errors.New("no free client slots")
		}

		if s.Static.Clients[freeSlot] == nil {
			s.Static.Clients[freeSlot] = &Client{
				Message: NewMessageBuffer(MaxDatagram),
			}
		}
		s.Static.Clients[freeSlot].NetConnection = sock
		s.ConnectClient(freeSlot)
		slog.Info("CheckForNewClients: client connected", "slot", freeSlot)
	}
	return nil
}
