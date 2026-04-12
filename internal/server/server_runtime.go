package server

import (
	"fmt"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/internal/console"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func (s *Server) raiseQCRuntimeError(vm *qc.VM, format string, args ...any) {
	if vm == nil {
		return
	}
	vm.SetBuiltinError(fmt.Errorf(format, args...))
}

func (s *Server) ensureSpawnPrecache(kind, value string) error {
	if value == "" {
		return fmt.Errorf("%s: empty string", kind)
	}
	if s.State != ServerStateLoading {
		return fmt.Errorf("%s: precache can only be done in spawn functions", kind)
	}
	return nil
}

func insertPrecache(cache []string, value string) (int, error) {
	for i := 1; i < len(cache); i++ {
		if cache[i] == value {
			return i, nil
		}
	}
	for i := 1; i < len(cache); i++ {
		if cache[i] == "" {
			cache[i] = value
			return i, nil
		}
	}
	return 0, fmt.Errorf("precache overflow for %q", value)
}

func (s *Server) precacheSound(sample string) error {
	if err := s.ensureSpawnPrecache("PF_precache_sound", sample); err != nil {
		return err
	}
	if len(s.SoundPrecache) == 0 {
		s.SoundPrecache = make([]string, MaxSounds)
	}
	_, err := insertPrecache(s.SoundPrecache, sample)
	return err
}

func (s *Server) precacheModel(modelName string) error {
	if err := s.ensureSpawnPrecache("PF_precache_model", modelName); err != nil {
		return err
	}
	if len(s.ModelPrecache) == 0 {
		s.ModelPrecache = make([]string, MaxModels)
	}
	if _, err := s.cacheModelInfo(modelName); err != nil {
		return fmt.Errorf("PF_precache_model: %w", err)
	}
	_, err := insertPrecache(s.ModelPrecache, modelName)
	return err
}

func (s *Server) SetCompatRNG(rng *compatrand.RNG) {
	if rng == nil {
		rng = compatrand.New()
	}
	s.compatRNG = rng
	if s.QCVM != nil {
		s.QCVM.SetCompatRNG(rng)
	}
}

// AllocEdict allocates a new entity.
func (s *Server) AllocEdict() *Edict {
	for i, e := range s.Edicts {
		if e.Free && (e.FreeTime < 2 || s.Time-e.FreeTime > 0.5) {
			UnlinkEdict(e)
			*e = Edict{Vars: &EntVars{}}
			s.NumEdicts = max(s.NumEdicts, i+1)
			s.ensureQCVMEdictStorage()
			clearQCVMEdictData(s.QCVM, i)
			syncEdictToQCVM(s.QCVM, i, e)
			return e
		}
	}

	if len(s.Edicts) >= s.MaxEdicts {
		return nil
	}

	e := &Edict{Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, e)
	s.NumEdicts = len(s.Edicts)
	s.ensureQCVMEdictStorage()
	clearQCVMEdictData(s.QCVM, s.NumEdicts-1)
	syncEdictToQCVM(s.QCVM, s.NumEdicts-1, e)
	return e
}

// FreeEdict marks an entity as free.
func (s *Server) FreeEdict(e *Edict) {
	if e == nil {
		return
	}
	UnlinkEdict(e)
	e.Vars = &EntVars{}
	e.Alpha = inet.ENTALPHA_DEFAULT
	e.Scale = inet.ENTSCALE_DEFAULT
	e.Free = true
	e.FreeTime = s.Time
	if s.QCVM != nil {
		entNum := s.NumForEdict(e)
		if entNum >= 0 && entNum < s.QCVM.NumEdicts {
			clearQCVMEdictData(s.QCVM, entNum)
			syncEdictToQCVM(s.QCVM, entNum, e)
		}
	}
}

// EdictNum returns the entity at the given index.
func (s *Server) EdictNum(n int) *Edict {
	if n < 0 || n >= len(s.Edicts) {
		return nil
	}
	return s.Edicts[n]
}

// NumForEdict returns the index for the given entity.
func (s *Server) NumForEdict(e *Edict) int {
	for i, ent := range s.Edicts {
		if ent == e {
			return i
		}
	}
	return -1
}

// GetMaxClients returns configured client slot count from persistent server static state.
func (s *Server) GetMaxClients() int {
	if s.Static == nil {
		return 0
	}
	return s.Static.MaxClients
}

// IsClientActive reports whether a client slot is currently occupied by an active connection.
func (s *Server) IsClientActive(clientNum int) bool {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return false
	}
	client := s.Static.Clients[clientNum]
	return client != nil && client.Active
}

// GetClientName returns the user-visible name for a connected client slot.
func (s *Server) GetClientName(clientNum int) string {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return ""
	}
	if s.Static.Clients[clientNum] == nil {
		return ""
	}
	return s.Static.Clients[clientNum].Name
}

// SetClientName updates a client's display name used by chat, scoreboards, and prints.
func (s *Server) SetClientName(clientNum int, name string) {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return
	}
	if len(name) > 15 {
		name = name[:15]
	}
	client.Name = name
	if client.Edict != nil && client.Edict.Vars != nil && s.QCVM != nil {
		client.Edict.Vars.NetName = s.QCVM.AllocString(name)
	}
	s.broadcastClientNameUpdate(clientNum, name)
}

// GetClientColor returns the top/bottom shirt color bits for the given client slot.
func (s *Server) GetClientColor(clientNum int) int {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return 0
	}
	if s.Static.Clients[clientNum] == nil {
		return 0
	}
	return s.Static.Clients[clientNum].Color
}

// SetClientColor updates a client's color setting used by player model colormap rendering.
func (s *Server) SetClientColor(clientNum int, color int) {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return
	}
	client := s.Static.Clients[clientNum]
	if client == nil {
		return
	}
	client.Color = color
	if client.Edict != nil && client.Edict.Vars != nil {
		client.Edict.Vars.Team = float32((color & 15) + 1)
	}
	s.broadcastClientColorUpdate(clientNum, color)
}

// GetClientPing returns average ping (ms) from the client's rolling network sample window.
func (s *Server) GetClientPing(clientNum int) float32 {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return 0
	}
	c := s.Static.Clients[clientNum]
	if c == nil || c.NumPings == 0 {
		return 0
	}

	var total float32
	count := min(c.NumPings, 16)
	for i := 0; i < count; i++ {
		total += c.PingTimes[i]
	}
	return total / float32(count) * 1000
}

// KillClient triggers QC ClientKill for a live player, mirroring the classic
// server-side "kill" command behavior used by both local and remote clients.
func (s *Server) KillClient(clientNum int) bool {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return false
	}
	client := s.Static.Clients[clientNum]
	if client == nil || !client.Active || client.Edict == nil || client.Edict.Free {
		return false
	}
	if client.Edict.Vars.Health <= 0 {
		s.SV_ClientPrintf(client, "Can't suicide -- already dead!\n")
		return false
	}
	if err := s.runClientKillQC(client); err != nil {
		client.Edict.Vars.Health = 0
		client.Edict.Vars.DeadFlag = float32(DeadDead)
		return true
	}
	return true
}

// KickClient sends a kick reason to a client then drops the connection from the server.
func (s *Server) KickClient(clientNum int, who, reason string) bool {
	if s.Static == nil || clientNum < 0 || clientNum >= len(s.Static.Clients) {
		return false
	}
	client := s.Static.Clients[clientNum]
	if client == nil || !client.Active {
		return false
	}
	if who == "" {
		who = "Console"
	}

	if client.Message != nil {
		message := "Kicked by " + who
		if reason != "" {
			message += ": " + reason
		}
		client.Message.WriteByte(byte(inet.SVCPrint))
		client.Message.WriteString(message + "\n")
	}

	s.DropClient(client, false)
	return true
}

// GetMapName returns the currently loaded map short name (without maps/ path or .bsp suffix).
func (s *Server) GetMapName() string {
	return s.Name
}

// DevStatsSnapshot returns current and peak developer counters.
func (s *Server) DevStatsSnapshot() (curr, peak DevStats) {
	if s == nil {
		return DevStats{}, DevStats{}
	}
	return s.devStats, s.devPeak
}

// DevStatsEdictCounters returns active and configured maximum edict counters.
func (s *Server) DevStatsEdictCounters() (active, max int) {
	if s == nil {
		return 0, 0
	}
	return s.devStats.Edicts, s.MaxEdicts
}

func (s *Server) recordDevStatsEdicts(active int) {
	if active > 600 && s.devPeak.Edicts <= 600 {
		slog.Warn("edict count exceeds standard limit",
			"active", active, "limit", 600, "max", s.MaxEdicts)
	}
	s.devStats.Edicts = active
	if active > s.devPeak.Edicts {
		s.devPeak.Edicts = active
	}
	s.peakEdicts = s.devPeak.Edicts
}

func (s *Server) recordDevStatsFrame() {
	s.devStats.Frames++
	if s.devStats.Frames > s.devPeak.Frames {
		s.devPeak.Frames = s.devStats.Frames
	}
}

func (s *Server) recordDevStatsPacketSize(size int) {
	if size > 1024 && s.devPeak.PacketSize <= 1024 {
		slog.Warn("packet size exceeds standard limit",
			"packet", size, "limit", 1024, "max", MaxDatagram)
	}
	s.devStats.PacketSize = size
	if size > s.devPeak.PacketSize {
		s.devPeak.PacketSize = size
	}
}

func (s *Server) QCProfileResults(top int) []qc.ProfileResult {
	if s == nil || !s.Active || s.QCVM == nil || top <= 0 {
		return nil
	}
	return s.QCVM.ProfileResults(top)
}

// SV_BroadcastPrintf prints formatted text to server console and all active clients reliably.
func (s *Server) SV_BroadcastPrintf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	console.Printf("%s", msg)
	if s.Static == nil {
		return
	}
	for _, client := range s.Static.Clients {
		if client == nil || !client.Active || client.Message == nil {
			continue
		}
		client.Message.WriteByte(byte(inet.SVCPrint))
		client.Message.WriteString(msg)
	}
}

// SV_ClientPrintf queues a formatted print message to a single client's reliable stream.
func (s *Server) SV_ClientPrintf(client *Client, format string, args ...any) {
	if client == nil || !client.Active || client.Message == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	client.Message.WriteByte(byte(inet.SVCPrint))
	client.Message.WriteString(msg)
}

func (s *Server) broadcastClientNameUpdate(clientNum int, name string) {
	if s == nil || s.Static == nil {
		return
	}
	for _, receiver := range s.Static.Clients {
		if receiver == nil || !receiver.Active || receiver.Message == nil {
			continue
		}
		receiver.Message.WriteByte(byte(inet.SVCUpdateName))
		receiver.Message.WriteByte(byte(clientNum))
		receiver.Message.WriteString(name)
	}
}

func (s *Server) broadcastClientColorUpdate(clientNum int, color int) {
	if s == nil || s.Static == nil {
		return
	}
	for _, receiver := range s.Static.Clients {
		if receiver == nil || !receiver.Active || receiver.Message == nil {
			continue
		}
		receiver.Message.WriteByte(byte(inet.SVCUpdateColors))
		receiver.Message.WriteByte(byte(clientNum))
		receiver.Message.WriteByte(byte(color))
	}
}
