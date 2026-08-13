// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQbj2TwinDoorsBothFireViaChain is a REGRESSION GUARD for the intermittently
// reported "only one of a double-door pair triggers".
//
// qbj2_mtsch has twin func_door entities (BSP #32 /*6, #33 /*7) sharing
// targetname "west_door_up". Per the mod's doors.qc (door_link), the chain
// folds: the chained door's targetname is deliberately NULLed and only the
// master keeps it; the master's door_fire walks the Enemy loop and fires ALL
// halves via door_go_up (SUB_CalcMove).
//
// Contract: when the shared counter (west_side_door_counter) reaches 0 and
// fires the relay, BOTH halves must start moving (both origins change after
// physics frames). If only the master moves, the Enemy chain is broken.
//
// Where in C / mod: qbj2/src/doors.qc door_link + door_fire + door_go_up.

func TestQbj2TwinDoorsBothFireViaChain(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}
	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "qbj2"); err != nil {
		t.Fatalf("vfs.Init: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	newServerTestVM(s, 300)
	if err := s.Init(1); err != nil {
		t.Fatalf("server.Init: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)
	if err := s.SpawnServer("maps/qbj2_mtsch.bsp", vfs); err != nil {
		t.Skipf("SpawnServer: %v", err)
	}
	t.Cleanup(s.Shutdown)

	// Locate the twin west_door_up doors by submodel.
	var master, half *Edict
	for i := 1; i < s.NumEdicts; i++ {
		ent := s.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}
		model := s.QCVM.String(s.QCVM.EString(i, qc.EntFieldModel))
		cls := s.QCVM.String(s.QCVM.EString(i, qc.EntFieldClassName))
		if cls != "func_door" {
			continue
		}
		switch model {
		case "*6":
			master = ent
		case "*7":
			half = ent
		}
	}
	if master == nil || half == nil {
		t.Fatalf("twin doors not found: master=%v half=%v", master != nil, half != nil)
	}
	mNum := s.NumForEdict(master)
	hNum := s.NumForEdict(half)
	t.Logf("door pair: master=#%d enemy=%d owner=%d | half=#%d enemy=%d owner=%d",
		mNum, master.Enemy(s), master.Owner(s), hNum, half.Enemy(s), half.Owner(s))
	if master.Enemy(s) != int32(hNum) {
		t.Fatalf("master.enemy=%d, want %d (chain must link master->half)", master.Enemy(s), hNum)
	}

	// Fire the counter's use chain by pressing both buttons (#210/#211) then
	// stepping physics so the counter relay fires the doors and they move.
	prevMaster := master.Origin(s)
	prevHalf := half.Origin(s)

	for _, btn := range []int{210, 211} {
		e := s.EdictNum(btn)
		if e == nil {
			t.Fatalf("button #%d missing", btn)
		}
		s.QCVM.SetGlobal("self", int(s.NumForEdict(e)))
		if fn := e.Use(s); fn != 0 {
			if err := s.ExecuteQCFunction(int(fn)); err != nil {
				t.Fatalf("button #%d use: %v", btn, err)
			}
		} else if fn := e.Touch(s); fn != 0 {
			if err := s.ExecuteQCFunction(int(fn)); err != nil {
				t.Fatalf("button #%d touch: %v", btn, err)
			}
		}
	}

	// Step physics until doors should have started moving.
	for i := 0; i < 8; i++ {
		s.Physics()
	}

	movedMaster := master.Origin(s) != prevMaster
	movedHalf := half.Origin(s) != prevHalf
	t.Logf("after fire: master_moved=%v half_moved=%v master_orig=%v half_orig=%v",
		movedMaster, movedHalf, master.Origin(s), half.Origin(s))
	if !movedMaster || !movedHalf {
		t.Fatalf("twin doors did not BOTH fire: master_moved=%v half_moved=%v", movedMaster, movedHalf)
	}
}
