package server

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

func mapnameGlobal(vm *qc.VM) string {
	ofs := vm.FindGlobal("mapname")
	if ofs < 0 {
		return ""
	}
	return vm.GString(ofs)
}

// TestSyntheticMapNameGlobalStaysSynthetic is the plan-28 follow-up
// (docs/plans/28_qgo_compiler_function_values.md §9): after client active on
// the no-assets demo, respawn() issues `changelevel MapName` where MapName is
// the QC `//qgo:mapname` global. If that global picks up the world-model
// string ("*0") instead of the map name ("synthetic"), the changelevel target
// is garbage and the demo reloads forever instead of staying in the room.
func TestSyntheticMapNameGlobalStaysSynthetic(t *testing.T) {
	vfs := fs.NewFileSystem()
	srv := NewServer()
	qc.RegisterBuiltins(srv.QCVM)
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init server: %v", err)
	}
	if err := srv.SpawnServer(SyntheticMapName, vfs); err != nil {
		t.Fatalf("SpawnServer(%q): %v", SyntheticMapName, err)
	}
	defer vfs.Close()

	got := mapnameGlobal(srv.QCVM)
	t.Logf("mapname global after SpawnServer = %q (want %q)", got, SyntheticMapName)
	if strings.TrimSpace(got) != SyntheticMapName {
		t.Fatalf("mapname global = %q, want %q; respawn() would changelevel to garbage",
			got, SyntheticMapName)
	}

	srv.ConnectClient(0)
	client := srv.Static.Clients[0]
	client.Name = "Player"

	connectFunc := srv.QCVM.FindFunction("ClientConnect")
	if connectFunc >= 0 {
		srv.QCVM.SetGlobal("self", 1)
		if err := srv.executeQCFunction(connectFunc); err != nil {
			t.Fatalf("ClientConnect: %v", err)
		}
	}
	putFunc := srv.QCVM.FindFunction("PutClientInServer")
	if putFunc >= 0 {
		srv.QCVM.SetGlobal("self", 1)
		if err := srv.executeQCFunction(putFunc); err != nil {
			t.Fatalf("PutClientInServer: %v", err)
		}
	}

	// mapname must still be "synthetic" after the full client spawn path.
	got = mapnameGlobal(srv.QCVM)
	t.Logf("mapname global after PutClientInServer = %q", got)
	if strings.TrimSpace(got) != SyntheticMapName {
		t.Fatalf("mapname global after client spawn = %q, want %q", got, SyntheticMapName)
	}
}

// TestCrossFunctionPackageVarCell is the real regression for the plan-28
// changelevel *0 quirk. NextMap is a PLAIN package var (no //qgo: tag): it is
// written by NextLevel() (NextMap = o.Map) and read by GotoNextMap()
// (engine.Changelevel(NextMap)). If the lowering gave it a per-function
// virtual local instead of a real global cell, the read in GotoNextMap never
// sees the write in NextLevel and the demo changelevels to garbage.
func TestCrossFunctionPackageVarCell(t *testing.T) {
	vfs := fs.NewFileSystem()
	srv := NewServer()
	qc.RegisterBuiltins(srv.QCVM)
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init server: %v", err)
	}
	if err := srv.SpawnServer(SyntheticMapName, vfs); err != nil {
		t.Fatalf("SpawnServer(%q): %v", SyntheticMapName, err)
	}
	defer vfs.Close()

	// Capture the level passed to IssueChangeLevel (engine.Changelevel).
	var issued []string
	var wrapper = srv.QCVM.ServerHooks.IssueChangeLevel
	srv.QCVM.ServerHooks.IssueChangeLevel = func(vm *qc.VM, level string) bool {
		issued = append(issued, level)
		if wrapper != nil {
			return wrapper(vm, level + "\n")
		}
		return false
	}

	nextLevelFunc := srv.QCVM.FindFunction("NextLevel")
	if nextLevelFunc < 0 {
		t.Fatalf("NextLevel not found in compiled progs")
	}
	if err := srv.executeQCFunction(nextLevelFunc); err != nil {
		t.Fatalf("NextLevel: %v", err)
	}

	gotoNextMapFunc := srv.QCVM.FindFunction("GotoNextMap")
	if gotoNextMapFunc < 0 {
		t.Fatalf("GotoNextMap not found in compiled progs")
	}
	if err := srv.executeQCFunction(gotoNextMapFunc); err != nil {
		t.Fatalf("GotoNextMap: %v", err)
	}

	if len(issued) == 0 {
		t.Fatalf("GotoNextMap never issued a changelevel")
	}
	for _, lvl := range issued {
		t.Logf("changelevel issued: %q", strings.TrimSpace(lvl))
	}
	if want := SyntheticMapName; !strings.Contains(strings.Join(issued, " "), want) {
		t.Fatalf("changelevel targets %v, want at least one %q (the grue: NextMap plain-var read is broken)", issued, want)
	}
}

