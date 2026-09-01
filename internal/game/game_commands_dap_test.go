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

	// 5. Test port-only argument parsing ("0")
	g.cmdQCDebugStart([]string{"0"})
	if active := server.ActiveDAPServer(); active != nil {
		if !strings.HasPrefix(active.Addr().String(), "127.0.0.1:") {
			t.Fatalf("Expected 127.0.0.1 address, got %s", active.Addr().String())
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

	// 7. Test when g.Host is nil (fallback to default or explicit arg)
	gNilHost := New()
	gNilHost.Server = server.NewServer()
	gNilHost.cmdQCDebugStart([]string{"127.0.0.1:0"})
	if active := server.ActiveDAPServer(); active == nil {
		t.Fatal("Expected active DAP server even when Host is nil")
	}
	server.StopDAPServer()

	// 8. Test when g.Server is nil
	gNilServer := New()
	gNilServer.cmdQCDebugStart([]string{"127.0.0.1:0"})
	if active := server.ActiveDAPServer(); active != nil {
		t.Fatal("Expected nil DAP server when Server is nil")
	}
}
