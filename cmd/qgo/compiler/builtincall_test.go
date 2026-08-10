package compiler

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// TestCompile_BuiltinCallFunctionValues is the plan-28 sentinel round-trip:
// a module that (a) declares a func-typed package var, (b) assigns a target
// function to an entity's func-valued field via `th.Think = Target`, and
// (c) calls through that field (`th.Think()`). It proves that:
//   - function-valued cells (globals AND fields) hold the function table
//     index, not a raw numeric float (plan D1/D3);
//   - OP_CALL through a stored func field executes the target function;
//   - engine builtins remain callable from the same program.
func TestCompile_BuiltinCallFunctionValues(t *testing.T) {
	data, err := New().Compile("../testdata/builtincall")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	funcs := parseFunctions(t, data, header)
	strings := parseStrings(t, data, header)

	// Both the target and the caller must exist as real function records.
	byName := make(map[string]qc.DFunction)
	for _, fn := range funcs {
		name := stringAt(strings, fn.Name)
		if name != "" {
			byName[name] = fn
		}
	}
	for _, name := range []string{"Target", "RunFunc"} {
		if _, ok := byName[name]; !ok {
			t.Fatalf("function %q not emitted", name)
		}
	}

	// FuncVar must be a global whose cell starts at 0 (null function).
	globals := parseGlobals(t, data, header)
	gdefs := parseGlobalDefs(t, data, header)
	foundFuncVar := false
	for _, def := range gdefs {
		if stringAt(strings, def.Name) != "FuncVar" {
			continue
		}
		foundFuncVar = true
		if v := math.Float32frombits(globals[def.Ofs]); v != 0 {
			t.Fatalf("FuncVar cell = %v, want 0 (null func)", v)
		}
	}
	if !foundFuncVar {
		t.Fatalf("global %q not emitted", "FuncVar")
	}

	// The think field must exist as an EvFunction field def.
	fields := parseFieldDefs(t, data, header)
	thinkIsFunc := false
	for _, f := range fields {
		if stringAt(strings, f.Name) == "think" {
			if qc.EType(f.Type) != qc.EvFunction {
				t.Fatalf("think field type = %d, want EvFunction(%d)", f.Type, qc.EvFunction)
			}
			thinkIsFunc = true
			break
		}
	}
	if !thinkIsFunc {
		t.Fatalf("field def %q not found", "think")
	}

	// Run the program through the real VM and verify the call chain works.
	vm := qc.NewVM()
	qc.RegisterBuiltins(vm)
	if err := vm.LoadProgs(bytes.NewReader(data)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	runIdx := -1
	targetIdx := -1
	for i := range funcs {
		switch stringAt(strings, funcs[i].Name) {
		case "RunFunc":
			runIdx = i
		case "Target":
			targetIdx = i
		}
	}
	if runIdx < 0 || targetIdx < 0 {
		t.Fatalf("RunFunc/Target not in function table (run=%d target=%d)", runIdx, targetIdx)
	}

	// Use a real edict number (say 3) for the th parameter. The VM's edict
	// storage grows lazily from EdictSize; zeroed fields are fine because
	// RunFunc only stores into Think and calls it.
	thEdict := 3
	if vm.EdictSize <= 0 {
		t.Fatalf("LoadProgs left EdictSize=%d", vm.EdictSize)
	}
	vm.NumEdicts = thEdict + 1
	vm.Edicts = make([]byte, vm.EdictSize*(thEdict+1))
	vm.SetGInt(qc.OFSParm0, int32(thEdict))
	if err := vm.ExecuteProgram(runIdx); err != nil {
		t.Fatalf("ExecuteProgram(RunFunc): %v", err)
	}
	got := vm.GFloat(qc.OFSReturn)
	if got != 42 {
		t.Fatalf("RunFunc returned %v, want 42 (func-cell/indirect-call path broken)", got)
	}

	// Sanity: the Think field on edict thEdict must now hold the index of
	// Target — proof the store path wrote the function index, not a float.
	thinkOfs := -1
	for _, f := range fields {
		if stringAt(strings, f.Name) == "think" {
			thinkOfs = int(f.Ofs)
			break
		}
	}
	if thinkOfs < 0 {
		t.Fatalf("think field offset not found")
	}
	if got := vm.EInt(thEdict, thinkOfs); got != int32(targetIdx) {
		t.Fatalf("th.Think = %d, want %d (Target index)", got, targetIdx)
	}

	// Sprintf intrinsic: run SprintfStr with a name and hp and verify the
	// concatenated string equals the literal expansion.
	sprintfIdx := -1
	for i := range funcs {
		if stringAt(strings, funcs[i].Name) == "SprintfStr" {
			sprintfIdx = i
			break
		}
	}
	if sprintfIdx < 0 {
		t.Fatalf("SprintfStr not in function table")
	}
	// UV (unused param slot): set string parm0 -> "quux", float parm1 -> 2.5.
	nameStr := "quux"
	nameOfs := int32(vm.AllocString(nameStr))
	vm.SetGInt(qc.OFSParm0, nameOfs)
	vm.SetGFloat(qc.OFSParm1, 2.5)
	if err := vm.ExecuteProgram(sprintfIdx); err != nil {
		t.Fatalf("ExecuteProgram(SprintfStr): %v", err)
	}
	gotStr := vm.String(vm.GInt(qc.OFSReturn))
	// ftos(2.5) = "%5.1f" → "  2.5" (C-correct padding); stracat joins it.
	wantStr := "$qc_test quux   2.5"
	if gotStr != wantStr {
		t.Fatalf("SprintfStr = %q, want %q", gotStr, wantStr)
	}
}

func parseFieldDefs(t *testing.T, data []byte, h qc.DProgs) []qc.DDef {
	t.Helper()
	defs := make([]qc.DDef, h.NumFieldDefs)
	r := bytes.NewReader(data)
	if _, err := r.Seek(int64(h.FieldDefs), 0); err != nil {
		t.Fatalf("seek field defs: %v", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &defs); err != nil {
		t.Fatalf("read field defs: %v", err)
	}
	return defs
}
