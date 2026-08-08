package main

import (
	"os"
	"testing"
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
	defer os.Chdir(cwd)

	data, err := compileProgs()
	if err != nil {
		t.Fatalf("compileProgs from nested cwd: %v", err)
	}
	if len(data) < 1024 {
		t.Fatalf("compiled progs suspiciously small: %d bytes", len(data))
	}
}
