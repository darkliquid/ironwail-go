package qc

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/fs"
)

// buildStubVM returns a VM whose string table holds the given function names
// and whose Functions table contains one empty "extension stub" per name
// (FirstStatement=0, ParmStart=0, Locals=0), mirroring how the 2021 rerelease
// progs.dat declares engine-extension functions it expects the engine to bind.
func buildStubVM(names ...string) *VM {
	vm := NewVM()
	vm.Globals = make([]float32, 4096)
	vm.Strings = []byte{0}              // offset 0 = empty string; real strings start at 1
	vm.Functions = make([]DFunction, 1) // function 0 is the reserved empty entry
	for _, name := range names {
		nameOfs := int32(len(vm.Strings))
		vm.Strings = append(vm.Strings, []byte(name)...)
		vm.Strings = append(vm.Strings, 0)
		vm.Functions = append(vm.Functions, DFunction{Name: nameOfs, NumParms: 2})
	}
	return vm
}

// TestLoadProgsRemapsExtensionStubs verifies that empty stub functions whose
// name matches a known engine-extension alias are rebound to the implementing
// builtin (negative FirstStatement), matching C Ironwail's PR_InitBuiltins
// remap of "progs functions with id 0" (pr_edict.c).
func TestLoadProgsRemapsExtensionStubs(t *testing.T) {
	vm := buildStubVM("ex_centerprint", "ex_bprint", "ex_sprint", "ex_localsound", "ex_finaleFinished")
	vm.remapExtensionBuiltins()

	want := map[string]int32{
		"ex_centerprint":    -73, // centerprint
		"ex_bprint":         -23, // bprint
		"ex_sprint":         -24, // sprint
		"ex_localsound":     -80, // localsound
		"ex_finaleFinished": -79, // finaleFinished
	}
	for i := 1; i < len(vm.Functions); i++ {
		name := vm.String(vm.Functions[i].Name)
		w, ok := want[name]
		if !ok {
			t.Fatalf("unexpected function %q", name)
		}
		if got := vm.Functions[i].FirstStatement; got != w {
			t.Errorf("%s FirstStatement = %d, want %d", name, got, w)
		}
	}
}

// TestRemapExtensionBuiltinsLeavesRealFunctions verifies the remap never
// touches functions that already have a body or are already builtins.
func TestRemapExtensionBuiltinsLeavesRealFunctions(t *testing.T) {
	vm := buildStubVM("ex_centerprint")
	// A real bytecode function that happens to share the alias name must not be
	// remapped (it has a body: FirstStatement > 0).
	vm.Functions = append(vm.Functions, DFunction{Name: vm.Functions[1].Name, FirstStatement: 100, ParmStart: 200, Locals: 4})
	// A function already bound to a builtin must not be changed.
	vm.Functions = append(vm.Functions, DFunction{Name: vm.Functions[1].Name, FirstStatement: -73})

	vm.remapExtensionBuiltins()

	if got := vm.Functions[2].FirstStatement; got != 100 {
		t.Errorf("bytecode function FirstStatement = %d, want 100 (untouched)", got)
	}
	if got := vm.Functions[3].FirstStatement; got != -73 {
		t.Errorf("builtin function FirstStatement = %d, want -73 (untouched)", got)
	}
}

// TestRemapExtensionBuiltinsIgnoresUnknownStubs verifies stubs with names that
// are not known extension aliases are left alone (no Go builtin exists).
func TestRemapExtensionBuiltinsIgnoresUnknownStubs(t *testing.T) {
	vm := buildStubVM("ex_draw_point", "some_mod_thing")
	vm.remapExtensionBuiltins()
	for i := 1; i < len(vm.Functions); i++ {
		if got := vm.Functions[i].FirstStatement; got != 0 {
			t.Errorf("%q FirstStatement = %d, want 0 (left as stub)", vm.String(vm.Functions[i].Name), got)
		}
	}
}

// TestExtensionStubDispatchesToBuiltin verifies that after remapping, calling
// the ex_centerprint stub dispatches the registered centerprint builtin.
func TestExtensionStubDispatchesToBuiltin(t *testing.T) {
	vm := buildStubVM("ex_centerprint")
	RegisterBuiltins(vm)

	var gotEnt int
	var gotMsg string
	vm.ServerHooks = ServerBuiltinHooks{
		CenterPrint: func(vm *VM, entNum int, msg string) {
			gotEnt = entNum
			gotMsg = msg
		},
	}

	// Simulate the engine rebinding stubs at load, then QC calling the stub by
	// function index with (entity, string) parms.
	vm.remapExtensionBuiltins()
	vm.SetGInt(OFSParm0, 1)
	vm.SetGString(OFSParm1, "hello")
	if err := vm.ExecuteFunction(1); err != nil {
		t.Fatalf("ExecuteFunction(ex_centerprint): %v", err)
	}
	if gotEnt != 1 || gotMsg != "hello" {
		t.Fatalf("centerprint hook got ent=%d msg=%q, want ent=1 msg=%q", gotEnt, gotMsg, "hello")
	}
}

// loadRereleaseProgs reads the progs.dat shipped in the 2021 rerelease's
// id1/pak0.pak, which declares ex_centerprint as an engine-extension stub. The
// rerelease pak is located via QUAKE_PAK0_PATH or the repo's ./quake-data
// symlink; the test skips when only the classic (testdata) pak0 is available,
// since that progs.dat uses a real centerprint builtin and has no stub.
func loadRereleaseProgs(t *testing.T) []byte {
	t.Helper()
	candidates := []string{}
	if env := os.Getenv("QUAKE_PAK0_PATH"); env != "" {
		candidates = append(candidates, env)
	}
	// Walk up from the test's package dir to the repo root's quake-data symlink.
	for dir, _ := os.Getwd(); ; {
		candidates = append(candidates, filepath.Join(dir, "quake-data", "id1", "pak0.pak"))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	for _, pak0 := range candidates {
		pakBytes, err := os.ReadFile(pak0)
		if err != nil {
			continue
		}
		pack, err := fs.LoadPackFromBytes(pak0, pakBytes)
		if err != nil {
			continue
		}
		data, err := fs.NewPakFS(pack).ReadFile("progs.dat")
		if err != nil {
			continue
		}
		// Only the rerelease progs.dat declares ex_centerprint as a stub.
		if bytes.Contains(data, []byte("ex_centerprint")) {
			return data
		}
	}
	t.Skip("rerelease progs.dat (with ex_centerprint) not found; set QUAKE_PAK0_PATH")
	return nil
}

// TestRereleaseProgsCenterprintRemapped loads the real rerelease progs.dat from
// pak0.pak and verifies ex_centerprint (a load-time stub) is rebound to the
// centerprint builtin, and that calling it drives the CenterPrint server hook.
// This is the regression test for rerelease centerprints never firing.
func TestRereleaseProgsCenterprintRemapped(t *testing.T) {
	data := loadRereleaseProgs(t)

	vm := NewVM()
	if err := vm.LoadProgs(bytes.NewReader(data)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	RegisterBuiltins(vm)

	idx := -1
	for i, f := range vm.Functions {
		if vm.String(f.Name) == "ex_centerprint" {
			idx = i
			break
		}
	}
	t.Logf("progs.dat bytes=%d functions=%d ex_centerprint idx=%d", len(data), len(vm.Functions), idx)
	if idx < 0 {
		t.Skip("progs.dat has no ex_centerprint stub (not rerelease data)")
	}
	if got := vm.Functions[idx].FirstStatement; got != -73 {
		t.Fatalf("ex_centerprint FirstStatement = %d after LoadProgs, want -73 (remapped to centerprint)", got)
	}

	var gotEnt int
	var gotMsg string
	vm.MaxEdicts = 4
	vm.NumEdicts = 2
	vm.Edicts = make([]byte, vm.EdictSize*vm.MaxEdicts)
	vm.ServerHooks = ServerBuiltinHooks{
		CenterPrint: func(vm *VM, entNum int, msg string) {
			gotEnt = entNum
			gotMsg = msg
		},
	}
	vm.SetGInt(OFSParm0, 1)
	vm.SetGString(OFSParm1, "$map_skill_normal")
	if err := vm.ExecuteFunction(idx); err != nil {
		t.Fatalf("ExecuteFunction(ex_centerprint): %v", err)
	}
	if gotEnt != 1 {
		t.Errorf("centerprint hook ent = %d, want 1", gotEnt)
	}
	if gotMsg == "" {
		t.Errorf("centerprint hook received empty message, want localized skill text")
	}
}
