// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/cmdsys"
	"github.com/darkliquid/ironwail-go/internal/server"
)

type autosaveCommandBuffer struct {
	added []string
}

func (b *autosaveCommandBuffer) Init()                                         {}
func (b *autosaveCommandBuffer) Execute()                                      {}
func (b *autosaveCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {}
func (b *autosaveCommandBuffer) AddText(text string)                           { b.added = append(b.added, text) }
func (b *autosaveCommandBuffer) InsertText(text string)                        {}
func (b *autosaveCommandBuffer) Shutdown()                                     {}

type autosaveTestServer struct {
	mockServer
	maxClients int
	edict      *server.Edict
	realServer *server.Server
}

func (s *autosaveTestServer) MaxClients() int {
	return s.maxClients
}

func (s *autosaveTestServer) EdictNum(n int) *server.Edict {
	if s.edict != nil && n == 1 {
		return s.edict
	}
	return s.mockServer.EdictNum(n)
}

// initRealServer sets up a real *server.Server with QCVM edict storage
// so accessor methods (Health, MoveType, etc.) read/write to the QCVM.
func (s *autosaveTestServer) initRealServer() {
	s.realServer = server.NewServer()
	vm := s.realServer.QCVM
	if vm != nil {
		vm.MaxEdicts = 8
		vm.NumEdicts = 8
		vm.EntityFields = 128
		vm.EdictSize = 28 + vm.EntityFields*4
		vm.Edicts = make([]byte, vm.EdictSize*8)
	}
	s.edict = &server.Edict{Num: 1}
}

func (s *autosaveTestServer) RealServer() *server.Server {
	return s.realServer
}

func setHostAutosaveForTest(t *testing.T, h *Host, value string) {
	t.Helper()
	h.registerHostCVars()
	h.CVar.Set("sv_autosave", "1")
	h.CVar.Set("sv_autosave_interval", value)
}

func TestCheckAutosaveTriggersAtConfiguredInterval(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeWalk))
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if len(commands.added) != 1 || commands.added[0] != "save \"autosave/start\" 0\n" {
		t.Fatalf("first autosave command = %v, want [save \"autosave/start\" 0\\n]", commands.added)
	}

	h.realtime = 105
	h.checkAutosave(subs)
	if got := len(commands.added); got != 1 {
		t.Fatalf("autosave before interval queued %d commands, want 1", got)
	}

	h.realtime = 106
	h.checkAutosave(subs)
	if got := len(commands.added); got != 2 {
		t.Fatalf("autosave at interval queued %d commands, want 2", got)
	}
}

func TestCheckAutosaveSkippedInMultiplayer(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 2,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("multiplayer autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveSkippedWhenDisabled(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "30")
	h.CVar.Set("sv_autosave", "0")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("disabled autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveSkippedDuringIntermission(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	client := newLocalLoopbackClient()
	client.inner.Intermission = 1
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Client: client, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("intermission autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveSkippedForDeadPlayer(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 0)
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("dead-player autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveSkippedForMoveTypeNonePlayer(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeNone))
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("movetype-none autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveSkippedForFastPlayer(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("fast-player autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveWaitsAfterRecentDamage(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "3")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeWalk))
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.realtime = 1
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("initial autosave queued %d commands before minimum elapsed time, want 0", got)
	}

	srv.edict.SetHealth(srv.realServer, 90)
	h.realtime = 2
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands immediately after damage, want 0", got)
	}

	h.realtime = 4
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands during hurt cooldown, want 0", got)
	}

	h.realtime = 6
	h.checkAutosave(subs)
	if got := len(commands.added); got != 1 {
		t.Fatalf("autosave queued %d commands after hurt cooldown, want 1", got)
	}
}

func TestCheckAutosaveWaitsAfterRecentShot(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "3")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeWalk))
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.realtime = 1
	h.checkAutosave(subs)

	srv.edict.SetButton0(srv.realServer, 1)
	h.realtime = 2
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands immediately after shot, want 0", got)
	}

	srv.edict.SetButton0(srv.realServer, 0)
	h.realtime = 4
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands during shot cooldown, want 0", got)
	}

	h.realtime = 6
	h.checkAutosave(subs)
	if got := len(commands.added); got != 1 {
		t.Fatalf("autosave queued %d commands after shot cooldown, want 1", got)
	}
}

func TestCheckAutosaveCheatTimeDoesNotAdvanceScore(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "30")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.frameTime = 10
	h.realtime = 30

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeNoClip))
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("noclip autosave queued %d commands, want 0", got)
	}

	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeWalk))
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands without enough non-cheat elapsed time, want 0", got)
	}

	h.realtime = 40
	h.checkAutosave(subs)
	if got := len(commands.added); got != 1 {
		t.Fatalf("autosave queued %d commands after cheat-adjusted interval, want 1", got)
	}
}

func TestCheckAutosaveTeleportBoostCanTriggerEarlySave(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "6")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 4

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 100)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeWalk))
	srv.edict.SetTeleportTime(srv.realServer, 3.5)
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 1 {
		t.Fatalf("teleport boost autosave queued %d commands, want 1", got)
	}
}

func TestCheckAutosaveWaitsAfterHazardDamageAbove100Health(t *testing.T) {
	h := NewHost()
	setHostAutosaveForTest(t, h, "3")
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1

	srv := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
		edict:      &server.Edict{},
	}
	srv.initRealServer()
	srv.edict.SetHealth(srv.realServer, 120)
	srv.edict.SetMoveType(srv.realServer, float32(server.MoveTypeWalk))
	srv.edict.SetWaterType(srv.realServer, float32(bsp.ContentsLava))
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: srv, Commands: commands}

	h.realtime = 1
	h.checkAutosave(subs)

	srv.edict.SetHealth(srv.realServer, 118)
	h.realtime = 2
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands immediately after hazard damage, want 0", got)
	}

	h.realtime = 4
	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("autosave queued %d commands during hazard hurt cooldown, want 0", got)
	}

	h.realtime = 6
	h.checkAutosave(subs)
	if got := len(commands.added); got != 1 {
		t.Fatalf("autosave queued %d commands after hazard hurt cooldown, want 1", got)
	}
}
