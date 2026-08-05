package commands

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestHandler_AllowedCommands(t *testing.T) {
	cmds := AllowedCommands()
	if len(cmds) == 0 {
		t.Fatal("expected non-empty list of allowed user commands")
	}

	cvars := cvar.NewCVarSystem()
	h := NewHandler(cvars)
	if h == nil {
		t.Fatal("expected non-nil Handler")
	}
}
