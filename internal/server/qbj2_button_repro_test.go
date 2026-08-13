// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func TestQBJ2ButtonRepro(t *testing.T) {
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
	if err := s.Init(1); err != nil {
		t.Fatalf("server.Init: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	if err := s.SpawnServer("maps/qbj2_mtsch.bsp", vfs); err != nil {
		t.Skipf("SpawnServer: %v", err)
	}

	t.Logf("Server spawned qbj2_mtsch cleanly with %d edicts", s.NumEdicts)

	// Enable trigger debug output
	s.CVar.Set("sv_debug_trigger", "1")
	s.CVar.Set("sv_debug_qc_trace", "1")
	s.CVar.Set("sv_debug_telemetry", "1")

	s.QCVM.TraceCallFunc = func(vm *qc.VM, event qc.TraceCallEvent) {
		name := ""
		if int(event.FunctionIndex) < len(vm.Functions) {
			name = vm.String(vm.Functions[event.FunctionIndex].Name)
		}
		t.Logf("[QC TRACE] %s fn=%s(#%d) self=%d", event.Phase, name, event.FunctionIndex, vm.GInt(qc.OFSSelf))
	}

	// Search edicts for buttons, counter, and doors
	for i := 1; i < s.NumEdicts; i++ {
		ent := s.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}
		cls := s.QCVM.String(s.QCVM.EString(i, qc.EntFieldClassName))
		tname := s.QCVM.String(s.QCVM.EString(i, qc.EntFieldTargetName))
		target := s.QCVM.String(s.QCVM.EString(i, qc.EntFieldTarget))
		if tname == "west_side_door_counter" || target == "west_side_door_counter" {
			t.Logf("MATCH! Edict #%d: class=%q targetname=%q target=%q touch=%d use=%d think=%d state=%.1f wait=%.1f",
				i, cls, tname, target, ent.Touch(s), ent.Use(s), ent.Think(s), ent.State(s), ent.Wait(s))
		}
		if cls == "trigger_counter" {
			t.Logf("COUNTER Edict #%d: class=%q targetname=%q target=%q", i, cls, tname, target)
		}
	}

	// Create dummy player edict
	player := s.AllocEdict()
	playerNum := s.NumForEdict(player)
	s.QCVM.SetEString(playerNum, qc.EntFieldClassName, s.QCVM.AllocString("player"))

	// Press Button 1 (210)
	btn210 := s.EdictNum(210)
	t.Logf("=== TOUCHING BUTTON #210 ===")
	t.Logf("Counter Edict #39 count before button 1: count=%.1f", s.QCVM.EFloat(39, s.QCVM.FindField("count")))
	s.QCVM.SetGInt(qc.OFSSelf, 210)
	s.QCVM.SetGInt(qc.OFSOther, int32(playerNum))
	if err := s.executeQCFunction(int(btn210.Touch(s))); err != nil {
		t.Fatalf("executeQCFunction btn210 touch: %v", err)
	}
	t.Logf("Counter Edict #39 count after button 1: count=%.1f", s.QCVM.EFloat(39, s.QCVM.FindField("count")))

	// Press Button 2 (211)
	btn211 := s.EdictNum(211)
	t.Logf("=== TOUCHING BUTTON #211 ===")
	t.Logf("Counter Edict #39 count before button 2: count=%.1f", s.QCVM.EFloat(39, s.QCVM.FindField("count")))
	s.QCVM.SetGInt(qc.OFSSelf, 211)
	s.QCVM.SetGInt(qc.OFSOther, int32(playerNum))
	if err := s.executeQCFunction(int(btn211.Touch(s))); err != nil {
		t.Fatalf("executeQCFunction btn211 touch: %v", err)
	}
	t.Logf("Counter Edict #39 count after button 2: count=%.1f", s.QCVM.EFloat(39, s.QCVM.FindField("count")))
}
