package host

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// TestNewHostIsolatesCVarAndCmdState ensures two independent Host instances
// own distinct cvar/command registries and do not share mutable state via
// package-level globals. This is a regression guard for the DI refactor that
// removed the singleton facades in internal/cvar and internal/cmdsys.
func TestNewHostIsolatesCVarAndCmdState(t *testing.T) {
	h1 := NewHost()
	h2 := NewHost()

	if h1.CVar == h2.CVar {
		t.Fatal("expected independent cvar systems per Host; got shared pointer")
	}
	if h1.Cmd == h2.Cmd {
		t.Fatal("expected independent command systems per Host; got shared pointer")
	}

	h1.CVar.Register("isolation_probe", "alpha", cvar.FlagNone, "host1 probe")
	if h2.CVar.Get("isolation_probe") != nil {
		t.Fatal("cvar registration on h1 leaked into h2")
	}

	probeCalled := 0
	h1.Cmd.AddCommand("isolation_probe_cmd", func(args []string) {
		probeCalled++
	}, "host1 probe command")
	if h2.Cmd.Exists("isolation_probe_cmd") {
		t.Fatal("command registration on h1 leaked into h2")
	}
	_ = probeCalled
}
