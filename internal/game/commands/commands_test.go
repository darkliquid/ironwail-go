package commands

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cmdsys"
)

func TestNewDispatcher(t *testing.T) {
	cmdSys := cmdsys.NewCmdSystem()
	d := NewDispatcher(cmdSys)
	if d == nil {
		t.Fatal("expected non-nil Dispatcher")
	}
}
