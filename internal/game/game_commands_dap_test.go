package game

import (
	"strings"
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
	g.registerGameplayBindCommands()
	t.Cleanup(func() {
		server.StopDAPServer()
	})

	// 1. Status when stopped
	g.cmdQCDebugStatus(nil)
	if status := server.DAPServerStatus(); status != "DAP debug server is inactive" {
		t.Fatalf("Expected inactive DAP server, got %q", status)
	}

	// 2. Start with port 0 (dynamic port)
	g.cmdQCDebugStart([]string{"127.0.0.1:0"})
	if active := server.ActiveDAPServer(); active == nil {
		t.Fatal("Expected active DAP server after start")
	}

	// 3. Status when active
	g.cmdQCDebugStatus(nil)
	if status := server.DAPServerStatus(); !strings.HasPrefix(status, "DAP debug server listening on 127.0.0.1:") {
		t.Fatalf("Unexpected active DAP server status: %q", status)
	}

	// 4. Stop
	g.cmdQCDebugStop(nil)
	if active := server.ActiveDAPServer(); active != nil {
		t.Fatal("Expected nil DAP server after stop")
	}

	// 5. Test port-only argument parsing ("5555")
	g.cmdQCDebugStart([]string{"5555"})
	if active := server.ActiveDAPServer(); active != nil {
		if !strings.HasSuffix(active.Addr().String(), ":5555") {
			t.Fatalf("Expected port 5555, got %s", active.Addr().String())
		}
		g.cmdQCDebugStop(nil)
	}

	// 6. Test executing via Host.Cmd.ExecuteText
	g.Host.Cmd.ExecuteText("qc_debug_start 127.0.0.1:0")
	if active := server.ActiveDAPServer(); active == nil {
		t.Fatal("Expected active DAP server after Host.Cmd.ExecuteText")
	}
	g.Host.Cmd.ExecuteText("qc_debug_stop")
	if active := server.ActiveDAPServer(); active != nil {
		t.Fatal("Expected nil DAP server after Host.Cmd.ExecuteText stop")
	}
}
