package compiler

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestCompile_Minimal(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/minimal")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	// Parse the header
	header := parseHeader(t, data)

	if header.Version != qc.ProgVersion {
		t.Errorf("version = %d, want %d", header.Version, qc.ProgVersion)
	}
	if got, want := header.CRC, int32(qc.ProgHeaderCRC); got != want {
		t.Errorf("crc = %d, want %d", got, want)
	}

	// Should have at least 1 global def (health)
	if header.NumGlobalDefs < 1 {
		t.Errorf("expected at least 1 global def, got %d", header.NumGlobalDefs)
	}

	// Verify the health global exists with value 100
	globals := parseGlobals(t, data, header)
	gdefs := parseGlobalDefs(t, data, header)
	strings := parseStrings(t, data, header)

	found := false
	for _, def := range gdefs {
		name := stringAt(strings, def.Name)
		if name == "health" {
			found = true
			val := math.Float32frombits(globals[def.Ofs])
			if val != 100.0 {
				t.Errorf("health = %f, want 100.0", val)
			}
		}
	}
	if !found {
		t.Error("global 'health' not found in defs")
	}
}

func TestCompile_Arithmetic(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/arithmetic")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)

	// Should have at least 2 functions (sentinel + Add)
	if header.NumFunctions < 2 {
		t.Errorf("expected at least 2 functions, got %d", header.NumFunctions)
	}

	// Parse functions and find Add
	funcs := parseFunctions(t, data, header)
	strings := parseStrings(t, data, header)

	found := false
	for _, fn := range funcs {
		name := stringAt(strings, fn.Name)
		if name == "Add" {
			found = true
			if fn.NumParms != 2 {
				t.Errorf("Add should have 2 params, got %d", fn.NumParms)
			}
			if fn.FirstStatement <= 0 {
				t.Errorf("Add should have positive first_statement, got %d", fn.FirstStatement)
			}
		}
	}
	if !found {
		t.Error("function 'Add' not found")
	}

	// Verify there's an ADD_F instruction
	stmts := parseStatements(t, data, header)
	hasAddF := false
	for _, s := range stmts {
		if qc.Opcode(s.Op) == qc.OPAddF {
			hasAddF = true
			break
		}
	}
	if !hasAddF {
		t.Error("expected ADD_F instruction in output")
	}
}

// Round-trip tests: compile → load into VM → execute → verify results

func loadVM(t *testing.T, data []byte) *qc.VM {
	t.Helper()
	vm := qc.NewVM()
	if err := vm.LoadProgs(bytes.NewReader(data)); err != nil {
		t.Fatalf("LoadProgs failed: %v", err)
	}
	return vm
}

func TestRoundTrip_MinimalGlobal(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/minimal")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	vm := loadVM(t, data)
	if got, want := vm.CRC, uint16(qc.ProgHeaderCRC); got != want {
		t.Fatalf("vm CRC = %d, want %d", got, want)
	}

	// Find the "health" global and verify its value
	ofs := vm.FindGlobal("health")
	if ofs < 0 {
		t.Fatal("global 'health' not found")
	}
	got := vm.GFloat(ofs)
	if got != 100.0 {
		t.Errorf("health = %f, want 100.0", got)
	}
}

func TestRoundTrip_ArithmeticAdd(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/arithmetic")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	vm := loadVM(t, data)

	fnum := vm.FindFunction("Add")
	if fnum < 0 {
		t.Fatal("function 'Add' not found")
	}

	// Set parameters: a=3.0, b=4.0
	vm.SetGFloat(qc.OFSParm0, 3.0)
	vm.SetGFloat(qc.OFSParm1, 4.0)

	if err := vm.ExecuteProgram(fnum); err != nil {
		t.Fatalf("ExecuteProgram failed: %v", err)
	}

	got := vm.GFloat(qc.OFSReturn)
	if got != 7.0 {
		t.Errorf("Add(3, 4) = %f, want 7.0", got)
	}
}

func TestRoundTrip_ControlFlowMax(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/controlflow")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	vm := loadVM(t, data)

	fnum := vm.FindFunction("Max")
	if fnum < 0 {
		t.Fatal("function 'Max' not found")
	}

	tests := []struct {
		a, b, want float32
	}{
		{5, 3, 5},
		{2, 8, 8},
		{4, 4, 4},
		{-1, 0, 0},
	}

	for _, tt := range tests {
		vm.SetGFloat(qc.OFSParm0, tt.a)
		vm.SetGFloat(qc.OFSParm1, tt.b)

		if err := vm.ExecuteProgram(fnum); err != nil {
			t.Fatalf("Max(%v, %v) error: %v", tt.a, tt.b, err)
		}

		got := vm.GFloat(qc.OFSReturn)
		if got != tt.want {
			t.Errorf("Max(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCompile_ControlFlow(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/controlflow")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	stmts := parseStatements(t, data, header)
	funcs := parseFunctions(t, data, header)
	strings := parseStrings(t, data, header)

	// Should have Max and Sum functions
	funcNames := make(map[string]bool)
	for _, fn := range funcs {
		name := stringAt(strings, fn.Name)
		if name != "" {
			funcNames[name] = true
		}
	}
	for _, name := range []string{"Max", "Sum"} {
		if !funcNames[name] {
			t.Errorf("function %q not found", name)
		}
	}

	// Should have GT, IFNOT, GOTO instructions (from if/for)
	opcodes := make(map[qc.Opcode]bool)
	for _, s := range stmts {
		opcodes[qc.Opcode(s.Op)] = true
	}
	for _, op := range []qc.Opcode{qc.OPGT, qc.OPIFNot, qc.OPGoto} {
		if !opcodes[op] {
			t.Errorf("expected opcode %d in output", op)
		}
	}
}

// Binary parsing helpers

func parseHeader(t *testing.T, data []byte) qc.DProgs {
	t.Helper()
	var h qc.DProgs
	r := bytes.NewReader(data)
	if err := binary.Read(r, binary.LittleEndian, &h); err != nil {
		t.Fatalf("failed to read header: %v", err)
	}
	return h
}

func parseStatements(t *testing.T, data []byte, h qc.DProgs) []qc.DStatement {
	t.Helper()
	stmts := make([]qc.DStatement, h.NumStatements)
	r := bytes.NewReader(data[h.Statements:])
	if err := binary.Read(r, binary.LittleEndian, &stmts); err != nil {
		t.Fatalf("failed to read statements: %v", err)
	}
	return stmts
}

func parseGlobalDefs(t *testing.T, data []byte, h qc.DProgs) []qc.DDef {
	t.Helper()
	defs := make([]qc.DDef, h.NumGlobalDefs)
	r := bytes.NewReader(data[h.GlobalDefs:])
	if err := binary.Read(r, binary.LittleEndian, &defs); err != nil {
		t.Fatalf("failed to read global defs: %v", err)
	}
	return defs
}

func parseGlobals(t *testing.T, data []byte, h qc.DProgs) []uint32 {
	t.Helper()
	globals := make([]uint32, h.NumGlobals)
	r := bytes.NewReader(data[h.Globals:])
	if err := binary.Read(r, binary.LittleEndian, &globals); err != nil {
		t.Fatalf("failed to read globals: %v", err)
	}
	return globals
}

func parseFunctions(t *testing.T, data []byte, h qc.DProgs) []qc.DFunction {
	t.Helper()
	funcs := make([]qc.DFunction, h.NumFunctions)
	r := bytes.NewReader(data[h.Functions:])
	if err := binary.Read(r, binary.LittleEndian, &funcs); err != nil {
		t.Fatalf("failed to read functions: %v", err)
	}
	return funcs
}

func parseStrings(t *testing.T, data []byte, h qc.DProgs) []byte {
	t.Helper()
	return data[h.Strings : h.Strings+h.NumStrings]
}

func stringAt(table []byte, ofs int32) string {
	if ofs < 0 || int(ofs) >= len(table) {
		return ""
	}
	end := ofs
	for int(end) < len(table) && table[end] != 0 {
		end++
	}
	return string(table[ofs:end])
}

func TestCompile_Modules(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/modules")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Produced empty output")
	}
}

