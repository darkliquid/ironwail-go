package main

import (
	"os"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// TestVMWorldBootsProgs verifies the In-VM runner (plan 25 Phase B) can boot
// compiled progs and execute plain engine functions — the foundation the
// REPL and walkthrough will build on.
func TestVMWorldBootsProgs(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skips qgo compile")
	}
	w, err := newVMWorld(nil)
	if err != nil {
		t.Fatalf("newVMWorld: %v", err)
	}
	// StartFrame exists in every progs and should run as a no-op.
	idx := w.vm.FindFunction("StartFrame")
	if idx < 0 {
		t.Fatal("StartFrame not found in compiled progs")
	}
	if _, err := w.fire(idx, 0, 0, 1.0); err != nil {
		t.Fatalf("fire StartFrame: %v", err)
	}
	// SUB_Null (a plain think) must run and touch nothing.
	if err := w.fireByName("SUB_Null", 1, 0, 1.0); err != nil {
		t.Fatalf("fire SUB_Null: %v", err)
	}
}

// TestVMWorldCompileProgsResolvesRoot runs qcmod from a nested cwd and
// confirms compileProgs still finds the repo root.
func TestVMWorldCompileProgsResolvesRoot(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	cwd, _ := os.Getwd()
	tmp := t.TempDir()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Errorf("failed to restore cwd: %v", err)
		}
	}()

	data, err := compileProgs()
	if err != nil {
		t.Fatalf("compileProgs from nested cwd: %v", err)
	}
	if len(data) < 1024 {
		t.Fatalf("compiled progs suspiciously small: %d bytes", len(data))
	}
}

// TestDebuggerBreakResume exercises the plan 25 Phase C statement debugger
// against a synthetic two-statement bytecode function (no builtins, so the
// runner needs no engine hooks): BreakHook fires at the first statement,
// ExecuteFunction returns ErrBreak with the stack live, then ExecuteFrom
// resumes to completion.
func TestDebuggerBreakResume(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode")
	}
	w, err := newVMWorld(nil)
	if err != nil {
		t.Fatalf("newVMWorld: %v", err)
	}
	// Register a synthetic function: add 1.0 to a global, then done. Use a
	// high offset (append past the progs-sized globals) so the test's value
	// can't collide with progs globals.
	const gOfs = 4000
	for len(w.vm.Globals) < gOfs+2 {
		w.vm.Globals = append(w.vm.Globals, 0)
	}
	w.vm.Globals[gOfs] = 0
	w.vm.Globals[gOfs+1] = 1
	w.vm.Functions = append(w.vm.Functions, qc.DFunction{Name: w.vm.AllocString("test_inc"), FirstStatement: int32(len(w.vm.Statements))})
	fidx := len(w.vm.Functions) - 1
	one := gOfs + 1
	w.vm.Statements = append(w.vm.Statements,
		qc.DStatement{Op: uint16(qc.OPAddF), A: uint16(gOfs), B: uint16(one), C: uint16(gOfs)},
		qc.DStatement{Op: uint16(qc.OPDone)},
	)
	first := int(w.vm.Functions[fidx].FirstStatement)

	// Fire once to completion (0 -> 1), proving the synthetic fn runs.
	if err := w.vm.ExecuteFunction(fidx); err != nil {
		t.Fatalf("ExecuteFunction (warm) = %v", err)
	}
	if got := w.vm.Globals[gOfs]; got != 1 {
		t.Fatalf("globals[%d] after warm run = %v, want 1", gOfs, got)
	}

	// Arm a one-shot break at the first statement; run again (1 -> 2).
	broke := 0
	w.vm.BreakHook = func(vm *qc.VM, stmtIdx int) bool {
		if stmtIdx == first && broke == 0 {
			broke++
			return true
		}
		return false
	}
	err = w.vm.ExecuteFunction(fidx)
	if err != qc.ErrBreak {
		t.Fatalf("ExecuteFunction = %v, want ErrBreak", err)
	}
	if broke != 1 {
		t.Fatalf("break fired %d times, want 1", broke)
	}
	if w.vm.Depth <= 0 {
		t.Fatal("stack unwound at break — cannot resume")
	}
	// The statement did NOT execute yet (break fires before the op).
	if got := w.vm.Globals[gOfs]; got != 1 {
		t.Fatalf("globals[%d] after break = %v, want 1 (statement not executed)", gOfs, got)
	}

	// Disarm the hook, resume to completion (1 -> 2).
	w.vm.BreakHook = nil
	if err := w.vm.ExecuteFrom(fidx); err != nil {
		t.Fatalf("ExecuteFrom: %v", err)
	}
	if w.vm.Depth != 0 {
		t.Fatalf("depth after resume = %d, want 0", w.vm.Depth)
	}
	if got := w.vm.Globals[gOfs]; got != 2 {
		t.Fatalf("globals[%d] after resume = %v, want 2", gOfs, got)
	}
}
