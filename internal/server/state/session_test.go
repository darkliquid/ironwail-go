package state_test

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/server/state"
	srvtypes "github.com/darkliquid/ironwail-go/internal/server/types"
)

func TestClientSessionStateLifecycle(t *testing.T) {
	cs := state.NewClientSessionState(0, 1024)
	if cs.ClientNum != 0 {
		t.Fatalf("ClientNum = %d, want 0", cs.ClientNum)
	}
	if cs.Signon != srvtypes.SignonNone {
		t.Fatalf("Signon = %v, want SignonNone", cs.Signon)
	}

	if !cs.AdvanceSignon(srvtypes.SignonNone, srvtypes.SignonPrespawn) {
		t.Fatalf("AdvanceSignon failed")
	}
	if cs.Signon != srvtypes.SignonPrespawn {
		t.Fatalf("Signon = %v, want SignonPrespawn", cs.Signon)
	}

	cs.AdvanceSignon(srvtypes.SignonPrespawn, srvtypes.SignonSignonBufs)
	cs.AdvanceSignon(srvtypes.SignonSignonBufs, srvtypes.SignonSignonMsg)
	cs.AdvanceSignon(srvtypes.SignonSignonMsg, srvtypes.SignonFlush)
	cs.AdvanceSignon(srvtypes.SignonFlush, srvtypes.SignonDone)

	if !cs.Active || !cs.Spawned {
		t.Fatalf("Expected Active and Spawned to be true after advancing to SignonDone")
	}

	cs.Reset()
	if cs.Active || cs.Spawned || cs.Signon != srvtypes.SignonNone {
		t.Fatalf("Reset failed to restore initial state")
	}
}

func TestSessionManagerLifecycle(t *testing.T) {
	sm := state.NewSessionManager(8, 1024)

	cs, err := sm.AddClient(0)
	if err != nil || cs == nil {
		t.Fatalf("AddClient(0) error: %v", err)
	}

	cs.AdvanceSignon(srvtypes.SignonNone, srvtypes.SignonDone)
	if sm.ActiveCount() != 1 {
		t.Fatalf("ActiveCount = %d, want 1", sm.ActiveCount())
	}

	if sm.Client(0) != cs {
		t.Fatalf("Client(0) mismatch")
	}

	sm.RemoveClient(0)
	if sm.Client(0) != nil {
		t.Fatalf("expected nil client after RemoveClient")
	}
	if sm.ActiveCount() != 0 {
		t.Fatalf("ActiveCount = %d, want 0 after removal", sm.ActiveCount())
	}
}
