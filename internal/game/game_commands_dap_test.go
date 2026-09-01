package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestQCDebugCommands(t *testing.T) {
	g := New()
	h := host.NewHost()
	h.CVar = cvar.NewCVarSystem()
	h.CVar.Register("qc_debug_port", "0", cvar.FlagArchive, "")
	g.Host = h
	g.Server = server.NewServer()

	// 1. Status when stopped
	g.cmdQCDebugStatus(nil)
	if status := server.DAPServerStatus(); status != "DAP debug server is inactive" {
		t.Fatalf("Expected inactive DAP server, got %q", status)
	}

	// 2. Start with port 0 (dynamic port)
	g.cmdQCDebugStart([]string{"qc_debug_start", "127.0.0.1:0"})
	if active := server.ActiveDAPServer(); active == nil {
		t.Fatal("Expected active DAP server after start")
	}

	// 3. Status when active
	g.cmdQCDebugStatus(nil)

	// 4. Stop
	g.cmdQCDebugStop(nil)
	if active := server.ActiveDAPServer(); active != nil {
		t.Fatal("Expected nil DAP server after stop")
	}
}
