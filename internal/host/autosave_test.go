// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package host

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

type autosaveCommandBuffer struct {
	added []string
}

func (b *autosaveCommandBuffer) Init()                  {}
func (b *autosaveCommandBuffer) Execute()               {}
func (b *autosaveCommandBuffer) AddText(text string)    { b.added = append(b.added, text) }
func (b *autosaveCommandBuffer) InsertText(text string) {}
func (b *autosaveCommandBuffer) Shutdown()              {}

type autosaveTestServer struct {
	mockServer
	maxClients int
}

func (s *autosaveTestServer) GetMaxClients() int {
	return s.maxClients
}

func setHostAutosaveForTest(t *testing.T, value string) {
	t.Helper()
	hostCVarsOnce.Do(registerHostCVars)
	previous := cvar.StringValue("host_autosave")
	cvar.Set("host_autosave", value)
	t.Cleanup(func() {
		cvar.Set("host_autosave", previous)
	})
}

func TestCheckAutosaveTriggersAtConfiguredInterval(t *testing.T) {
	setHostAutosaveForTest(t, "0.1") // 6 seconds

	h := NewHost()
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	server := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
	}
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: server, Commands: commands}

	h.checkAutosave(subs)
	if len(commands.added) != 1 || commands.added[0] != "save auto\n" {
		t.Fatalf("first autosave command = %v, want [save auto\\n]", commands.added)
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
	setHostAutosaveForTest(t, "0.1")

	h := NewHost()
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	server := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 2,
	}
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: server, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("multiplayer autosave queued %d commands, want 0", got)
	}
}

func TestCheckAutosaveSkippedWhenDisabled(t *testing.T) {
	setHostAutosaveForTest(t, "0")

	h := NewHost()
	h.serverActive = true
	h.clientState = caActive
	h.signOns = 1
	h.realtime = 100

	server := &autosaveTestServer{
		mockServer: mockServer{active: true},
		maxClients: 1,
	}
	commands := &autosaveCommandBuffer{}
	subs := &Subsystems{Server: server, Commands: commands}

	h.checkAutosave(subs)
	if got := len(commands.added); got != 0 {
		t.Fatalf("disabled autosave queued %d commands, want 0", got)
	}
}
