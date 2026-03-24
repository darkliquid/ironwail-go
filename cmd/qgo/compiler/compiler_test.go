package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

func TestCompile_ConstantFloatExpression_IsFoldedInIRPass(t *testing.T) {
	dir := t.TempDir()
	writeQGoModule(t, dir, "module qgoconstfoldtest")
	writeFile(t, filepath.Join(dir, "main.go"), `package main

func Folded() float32 {
	return 2 + 3
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	stmts := parseStatements(t, data, header)
	funcs := parseFunctions(t, data, header)
	stringTable := parseStrings(t, data, header)

	var folded *qc.DFunction
	for i := range funcs {
		if stringAt(stringTable, funcs[i].Name) == "Folded" {
			folded = &funcs[i]
			break
		}
	}
	if folded == nil {
		t.Fatal("function 'Folded' not found")
	}
	if folded.FirstStatement <= 0 {
		t.Fatalf("Folded first_statement = %d, want > 0", folded.FirstStatement)
	}

	start := int(folded.FirstStatement)
	if start >= len(stmts) {
		t.Fatalf("Folded first_statement %d out of range (num statements %d)", start, len(stmts))
	}

	for i := start; i < len(stmts); i++ {
		op := qc.Opcode(stmts[i].Op)
		if op == qc.OPDone {
			break
		}
		if op == qc.OPAddF {
			t.Fatalf("Folded body contains arithmetic opcode %v at statement %d; expected literal-folded store/return only", op, i)
		}
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

func TestRoundTrip_ArithmeticMatchesNativeGo(t *testing.T) {
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

	tests := []struct {
		a, b float32
	}{
		{3, 4},
		{-2, 5},
		{1.5, 2.25},
		{-7.75, -0.25},
	}

	goAdd := func(a, b float32) float32 { return a + b }

	for _, tt := range tests {
		vm.SetGFloat(qc.OFSParm0, tt.a)
		vm.SetGFloat(qc.OFSParm1, tt.b)
		if err := vm.ExecuteProgram(fnum); err != nil {
			t.Fatalf("Add(%v, %v) error: %v", tt.a, tt.b, err)
		}

		got := vm.GFloat(qc.OFSReturn)
		want := goAdd(tt.a, tt.b)
		if got != want {
			t.Fatalf("Add(%v, %v) = %v, want native-Go %v", tt.a, tt.b, got, want)
		}
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

type fixtureSignalSpec struct {
	requiredParms   map[string]int32
	requiredOpcodes []qc.Opcode
}

type compiledFixture struct {
	vm      *qc.VM
	header  qc.DProgs
	funcs   []qc.DFunction
	strings []byte
	stmts   []qc.DStatement
}

type parityMismatch struct {
	category string
	field    string
	want     string
	got      string
}

func TestParitySmoke_QCVMBehaviorBaselines(t *testing.T) {
	c := New()

	type smokeCase struct {
		name     string
		fixture  string
		function string
		args     []float32
		goExpect func(args []float32) float32
	}

	fixtureSignals := map[string]fixtureSignalSpec{
		"../testdata/arithmetic": {
			requiredParms: map[string]int32{
				"Add": 2,
			},
			requiredOpcodes: []qc.Opcode{qc.OPAddF},
		},
		"../testdata/controlflow": {
			requiredParms: map[string]int32{
				"Max": 2,
				"Sum": 1,
			},
			requiredOpcodes: []qc.Opcode{qc.OPGT, qc.OPIFNot, qc.OPGoto, qc.OPAddF},
		},
		"../testdata/maprunner": {
			requiredParms: map[string]int32{
				"MapRunner": 2,
			},
			requiredOpcodes: []qc.Opcode{qc.OPGT, qc.OPIFNot, qc.OPGoto, qc.OPAddF, qc.OPSubF},
		},
	}

	cases := []smokeCase{
		{
			name:     "arithmetic-add-positive",
			fixture:  "../testdata/arithmetic",
			function: "Add",
			args:     []float32{3, 4},
			goExpect: func(args []float32) float32 { return args[0] + args[1] },
		},
		{
			name:     "arithmetic-add-mixed-sign",
			fixture:  "../testdata/arithmetic",
			function: "Add",
			args:     []float32{-2.5, 1.25},
			goExpect: func(args []float32) float32 { return args[0] + args[1] },
		},
		{
			name:     "controlflow-max-descending",
			fixture:  "../testdata/controlflow",
			function: "Max",
			args:     []float32{9, 2},
			goExpect: func(args []float32) float32 {
				if args[0] > args[1] {
					return args[0]
				}
				return args[1]
			},
		},
		{
			name:     "controlflow-max-negative",
			fixture:  "../testdata/controlflow",
			function: "Max",
			args:     []float32{-3, -7},
			goExpect: func(args []float32) float32 {
				if args[0] > args[1] {
					return args[0]
				}
				return args[1]
			},
		},
		{
			name:     "controlflow-sum-five",
			fixture:  "../testdata/controlflow",
			function: "Sum",
			args:     []float32{5},
			goExpect: func(args []float32) float32 {
				n := args[0]
				var result float32
				var i float32
				for i = 0; i < n; i++ {
					result += i
				}
				return result
			},
		},
		{
			name:     "controlflow-sum-zero",
			fixture:  "../testdata/controlflow",
			function: "Sum",
			args:     []float32{0},
			goExpect: func(args []float32) float32 {
				n := args[0]
				var result float32
				var i float32
				for i = 0; i < n; i++ {
					result += i
				}
				return result
			},
		},
		{
			name:     "maprunner-step-sequence",
			fixture:  "../testdata/maprunner",
			function: "MapRunner",
			args:     []float32{1, 4},
			goExpect: func(args []float32) float32 {
				pos := args[0]
				steps := args[1]
				var i float32
				for i = 0; i < steps; i++ {
					if pos > 5 {
						pos = pos - 2
					} else {
						pos = pos + 3
					}
				}
				return pos
			},
		},
	}

	compiled := map[string]compiledFixture{}
	parmSlots := []int{
		qc.OFSParm0,
		qc.OFSParm1,
		qc.OFSParm2,
		qc.OFSParm3,
		qc.OFSParm4,
		qc.OFSParm5,
		qc.OFSParm6,
		qc.OFSParm7,
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			fixtureData, ok := compiled[tc.fixture]
			if !ok {
				data, err := c.Compile(tc.fixture)
				if err != nil {
					t.Fatalf("compile %s failed: %v", tc.fixture, err)
				}
				header := parseHeader(t, data)
				fixtureData = compiledFixture{
					vm:      loadVM(t, data),
					header:  header,
					funcs:   parseFunctions(t, data, header),
					strings: parseStrings(t, data, header),
					stmts:   parseStatements(t, data, header),
				}
				compiled[tc.fixture] = fixtureData
			}

			spec, ok := fixtureSignals[tc.fixture]
			if !ok {
				t.Fatalf("missing fixture signal spec for %s", tc.fixture)
			}
			if mismatches := collectShallowFixtureMismatches(tc.fixture, fixtureData, spec); len(mismatches) > 0 {
				t.Fatalf("%s", formatParitySmokeDiffReport(tc.name, tc.fixture, tc.function, mismatches))
			}

			fnum := fixtureData.vm.FindFunction(tc.function)
			if fnum < 0 {
				t.Fatalf("%s", formatParitySmokeDiffReport(tc.name, tc.fixture, tc.function, []parityMismatch{{
					category: "structural.function_presence",
					field:    "required function",
					want:     tc.function,
					got:      "missing",
				}}))
			}

			if len(tc.args) > len(parmSlots) {
				t.Fatalf("test case has %d args, max supported %d", len(tc.args), len(parmSlots))
			}
			for i, arg := range tc.args {
				fixtureData.vm.SetGFloat(parmSlots[i], arg)
			}

			if err := fixtureData.vm.ExecuteProgram(fnum); err != nil {
				t.Fatalf("%s", formatParitySmokeDiffReport(tc.name, tc.fixture, tc.function, []parityMismatch{{
					category: "runtime.execute_program",
					field:    "ExecuteProgram",
					want:     "no error",
					got:      err.Error(),
				}}))
			}

			want := tc.goExpect(tc.args)
			if got := fixtureData.vm.GFloat(qc.OFSReturn); got != want {
				t.Fatalf("%s", formatParitySmokeDiffReport(tc.name, tc.fixture, tc.function, []parityMismatch{{
					category: "behavior.return_value",
					field:    "OFSReturn",
					want:     fmt.Sprintf("%g (native-go)", want),
					got:      fmt.Sprintf("%g (qcvm)", got),
				}}))
			}
		})
	}
}

func collectShallowFixtureMismatches(fixture string, got compiledFixture, want fixtureSignalSpec) []parityMismatch {
	var mismatches []parityMismatch

	if got.header.Version != int32(qc.ProgVersion) {
		mismatches = append(mismatches, parityMismatch{
			category: "structural.header",
			field:    "version",
			want:     fmt.Sprintf("%d", qc.ProgVersion),
			got:      fmt.Sprintf("%d", got.header.Version),
		})
	}
	if got.header.CRC != int32(qc.ProgHeaderCRC) {
		mismatches = append(mismatches, parityMismatch{
			category: "structural.header",
			field:    "crc",
			want:     fmt.Sprintf("%d", qc.ProgHeaderCRC),
			got:      fmt.Sprintf("%d", got.header.CRC),
		})
	}
	if got.header.NumStatements == 0 || got.header.NumFunctions == 0 || got.header.NumGlobals == 0 {
		mismatches = append(mismatches, parityMismatch{
			category: "structural.sections",
			field:    "non-empty core sections",
			want:     "statements>0 && functions>0 && globals>0",
			got:      fmt.Sprintf("statements=%d functions=%d globals=%d", got.header.NumStatements, got.header.NumFunctions, got.header.NumGlobals),
		})
	}

	funcMeta := make(map[string]qc.DFunction, len(got.funcs))
	for _, fn := range got.funcs {
		name := stringAt(got.strings, fn.Name)
		if name != "" {
			funcMeta[name] = fn
		}
	}
	requiredNames := make([]string, 0, len(want.requiredParms))
	for name := range want.requiredParms {
		requiredNames = append(requiredNames, name)
	}
	sort.Strings(requiredNames)

	for _, name := range requiredNames {
		numParms := want.requiredParms[name]
		fn, ok := funcMeta[name]
		if !ok {
			mismatches = append(mismatches, parityMismatch{
				category: "structural.function_presence",
				field:    "required function",
				want:     name,
				got:      "missing",
			})
			continue
		}
		if fn.NumParms != numParms {
			mismatches = append(mismatches, parityMismatch{
				category: "structural.function_signature",
				field:    fmt.Sprintf("%s.NumParms", name),
				want:     fmt.Sprintf("%d", numParms),
				got:      fmt.Sprintf("%d", fn.NumParms),
			})
		}
		if fn.FirstStatement <= 0 {
			mismatches = append(mismatches, parityMismatch{
				category: "structural.function_anchor",
				field:    fmt.Sprintf("%s.FirstStatement", name),
				want:     "> 0",
				got:      fmt.Sprintf("%d", fn.FirstStatement),
			})
		}
	}

	hasOpcode := make(map[qc.Opcode]bool, len(got.stmts))
	for _, s := range got.stmts {
		hasOpcode[qc.Opcode(s.Op)] = true
	}
	for _, op := range want.requiredOpcodes {
		if !hasOpcode[op] {
			mismatches = append(mismatches, parityMismatch{
				category: "structural.opcode_presence",
				field:    fmt.Sprintf("opcode %d", op),
				want:     "present",
				got:      "missing",
			})
		}
	}

	return mismatches
}

func formatParitySmokeDiffReport(caseName, fixture, function string, mismatches []parityMismatch) string {
	var b strings.Builder
	fmt.Fprintf(&b, "parity smoke structured diff (case=%s fixture=%s function=%s):", caseName, fixture, function)
	for _, m := range mismatches {
		fmt.Fprintf(&b, "\n- [%s] %s: want %s, got %s", m.category, m.field, m.want, m.got)
	}
	return b.String()
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

func TestCompile_ImportedBodiesAreNotLowered(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module qgoimportisotest

go 1.26

`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import (
	"qgoimportisotest/dep"
)

func main() {
	dep.Value(12)
}
`)
	if err := os.MkdirAll(filepath.Join(dir, "dep"), 0o755); err != nil {
		t.Fatalf("mkdir dep: %v", err)
	}
	writeFile(t, filepath.Join(dir, "dep", "dep.go"), `package dep

func Value(v interface{}) float32 {
	switch x := v.(type) {
	case int:
		return float32(x)
	default:
		return 0
	}
}
`)

	c := New()
	_, err := c.Compile(dir)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "*ast.TypeSwitchStmt") || strings.Contains(msg, "*ast.TypeAssertExpr") {
			t.Fatalf("compile should not lower imported package bodies, got: %v", err)
		}
		t.Fatalf("compile failed: %v", err)
	}
}

func TestCompile_ControlFlowStructuralBaseline(t *testing.T) {
	c := New()
	data, err := c.Compile("../testdata/controlflow")
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	if got, want := header.Version, int32(qc.ProgVersion); got != want {
		t.Fatalf("version = %d, want %d", got, want)
	}
	if got, want := header.CRC, int32(qc.ProgHeaderCRC); got != want {
		t.Fatalf("crc = %d, want %d", got, want)
	}

	if header.NumStatements == 0 || header.NumFunctions == 0 || header.NumGlobals == 0 {
		t.Fatalf("unexpected empty sections: statements=%d functions=%d globals=%d", header.NumStatements, header.NumFunctions, header.NumGlobals)
	}

	if header.Statements <= 0 || header.GlobalDefs <= 0 || header.Functions <= 0 || header.Strings <= 0 || header.Globals <= 0 {
		t.Fatalf("invalid section offsets: statements=%d globaldefs=%d functions=%d strings=%d globals=%d", header.Statements, header.GlobalDefs, header.Functions, header.Strings, header.Globals)
	}
	section := []struct {
		name string
		ofs  int32
		end  int32
	}{
		{name: "statements", ofs: header.Statements, end: header.Statements + int32(binary.Size(qc.DStatement{}))*header.NumStatements},
		{name: "globaldefs", ofs: header.GlobalDefs, end: header.GlobalDefs + int32(binary.Size(qc.DDef{}))*header.NumGlobalDefs},
		{name: "fielddefs", ofs: header.FieldDefs, end: header.FieldDefs + int32(binary.Size(qc.DDef{}))*header.NumFieldDefs},
		{name: "functions", ofs: header.Functions, end: header.Functions + int32(binary.Size(qc.DFunction{}))*header.NumFunctions},
		{name: "strings", ofs: header.Strings, end: header.Strings + header.NumStrings},
		{name: "globals", ofs: header.Globals, end: header.Globals + 4*header.NumGlobals},
	}
	prevEnd := int32(0)
	for _, sec := range section {
		if sec.ofs < prevEnd {
			t.Fatalf("section %s starts before previous section end: start=%d prev_end=%d", sec.name, sec.ofs, prevEnd)
		}
		if sec.end < sec.ofs {
			t.Fatalf("section %s has invalid bounds: start=%d end=%d", sec.name, sec.ofs, sec.end)
		}
		prevEnd = sec.end
	}
	if prevEnd > int32(len(data)) {
		t.Fatalf("section layout exceeds binary size: end=%d len=%d", prevEnd, len(data))
	}

	funcs := parseFunctions(t, data, header)
	strings := parseStrings(t, data, header)
	indices := map[string]int{"Max": -1, "Sum": -1}
	wantParms := map[string]int32{"Max": 2, "Sum": 1}
	for i, fn := range funcs {
		name := stringAt(strings, fn.Name)
		if _, ok := indices[name]; ok {
			indices[name] = i
			if got, want := fn.NumParms, wantParms[name]; got != want {
				t.Fatalf("%s NumParms = %d, want %d", name, got, want)
			}
			if fn.FirstStatement <= 0 {
				t.Fatalf("%s FirstStatement = %d, want > 0", name, fn.FirstStatement)
			}
		}
	}
	if indices["Max"] == -1 || indices["Sum"] == -1 {
		t.Fatalf("expected both Max and Sum in function table, got indices: %#v", indices)
	}
	if !(indices["Max"] < indices["Sum"]) {
		t.Fatalf("expected Max before Sum for controlflow baseline ordering, got Max=%d Sum=%d", indices["Max"], indices["Sum"])
	}

	stmts := parseStatements(t, data, header)
	has := map[qc.Opcode]bool{}
	for _, s := range stmts {
		has[qc.Opcode(s.Op)] = true
	}
	for _, op := range []qc.Opcode{qc.OPGT, qc.OPIFNot, qc.OPGoto, qc.OPAddF} {
		if !has[op] {
			t.Fatalf("missing expected opcode %d in controlflow baseline", op)
		}
	}
}

func TestCompile_DeterministicBinaryForSameInput(t *testing.T) {
	c := New()
	first, err := c.Compile("../testdata/controlflow")
	if err != nil {
		t.Fatalf("first compile failed: %v", err)
	}

	second, err := c.Compile("../testdata/controlflow")
	if err != nil {
		t.Fatalf("second compile failed: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("compile output differed between identical runs")
	}
}

func TestCompile_GeneralStructLiteral_DeferredWithClearError(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgostructliteraldeferredtest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Pair struct {
	A float32
	B float32
}

func BuildPair(a float32, b float32) Pair {
	return Pair{A: a, B: b}
}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for general struct literal")
	}
	msg := err.Error()
	if !strings.Contains(msg, "general struct literals are deferred") {
		t.Fatalf("unexpected compile error: %v", err)
	}
	if !strings.Contains(msg, "Pair") {
		t.Fatalf("expected deferred diagnostic to include struct type context, got: %v", err)
	}
}

func TestCompile_Vec3StructLiteral_RemainsSupported(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgovec3literaltest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

type Vec3 struct {
	X float32
	Y float32
	Z float32
}

func BuildVec3(x float32, y float32, z float32) Vec3 {
	return Vec3{x, y, z}
}
`)

	c := New()
	if _, err := c.Compile(dir); err != nil {
		t.Fatalf("expected Vec3 literal compile to succeed, got: %v", err)
	}
}

func TestCompile_FunctionTableOrderFollowsFilenameOrder(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `module qgosourceordertest

go 1.26
`)
	writeFile(t, filepath.Join(dir, "z_last.qgo"), `package main

func Zed() float32 { return 2 }
`)
	writeFile(t, filepath.Join(dir, "a_first.qgo"), `package main

func Able() float32 { return 1 }
`)
	writeFile(t, filepath.Join(dir, "main.qgo"), `package main

func MainValue() float32 { return Able() + Zed() }
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	funcs := parseFunctions(t, data, header)
	stringTable := parseStrings(t, data, header)

	mustAppearInOrder(t, funcs, stringTable, []string{"Able", "MainValue", "Zed"})
}

func TestCompile_BuiltinDirectiveNamedAlias_EmitsNegativeBuiltinStatement(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgobuiltinaliasnamedtest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

//qgo:builtin bprint
func Broadcast() {}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	funcs := parseFunctions(t, data, header)
	stringTable := parseStrings(t, data, header)

	for _, fn := range funcs {
		if stringAt(stringTable, fn.Name) != "Broadcast" {
			continue
		}
		if got, want := fn.FirstStatement, int32(-23); got != want {
			t.Fatalf("Broadcast first_statement = %d, want %d for bprint builtin", got, want)
		}
		return
	}

	t.Fatal("function 'Broadcast' not found")
}

func TestCompile_BuiltinDirectiveNamedAlias_CaseInsensitive(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgobuiltinaliascasetest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

//qgo:builtin SPAWN
func SpawnAlias() {}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	funcs := parseFunctions(t, data, header)
	stringTable := parseStrings(t, data, header)

	for _, fn := range funcs {
		if stringAt(stringTable, fn.Name) != "SpawnAlias" {
			continue
		}
		if got, want := fn.FirstStatement, int32(-14); got != want {
			t.Fatalf("SpawnAlias first_statement = %d, want %d for spawn builtin", got, want)
		}
		return
	}

	t.Fatal("function 'SpawnAlias' not found")
}

func TestCompile_BuiltinDirective_UnknownAlias_FailsWithDiagnostic(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgobuiltinunknownaliastest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

//qgo:builtin definitely_not_a_real_builtin
func UnknownAliasBuiltin() {}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for unknown builtin alias")
	}
	if !strings.Contains(err.Error(), `unknown //qgo:builtin alias "definitely_not_a_real_builtin"`) {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompile_BuiltinDirective_AmbiguousMultiple_FailsWithDiagnostic(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgobuiltinambiguoustest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

//qgo:builtin spawn
//qgo:builtin remove
func AmbiguousBuiltin() {}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for ambiguous builtin directives")
	}
	if !strings.Contains(err.Error(), "ambiguous //qgo:builtin directives for AmbiguousBuiltin: 14 and 15") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompile_BuiltinDirective_DuplicateSameBuiltin_FailsWithDiagnostic(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgobuiltinduplicatetest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

//qgo:builtin 23
//qgo:builtin bprint
func DuplicateBuiltin() {}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for duplicate builtin directives")
	}
	if !strings.Contains(err.Error(), "duplicate //qgo:builtin directive for DuplicateBuiltin (builtin 23)") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompile_BuiltinDirective_MalformedPayload_FailsWithDiagnostic(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgobuiltinmalformedtest`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

//qgo:builtin
func MalformedBuiltin() {}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for malformed builtin directive")
	}
	if !strings.Contains(err.Error(), "malformed //qgo:builtin directive: expected one builtin number or alias") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompile_FieldOffsetIntrinsic_FieldFloatOpcodes(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgofieldintrinsictest`)
	writeFieldIntrinsicStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgofieldintrinsictest/quake"

func Read(ent *quake.Entity, ofs quake.FieldOffset) float32 {
	return quake.FieldFloat(ent, ofs)
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	stmts := parseStatements(t, data, header)
	hasLoadF := false
	for _, s := range stmts {
		switch qc.Opcode(s.Op) {
		case qc.OPLoadF:
			hasLoadF = true
		}
	}

	if !hasLoadF {
		t.Fatal("expected OP_LOAD_F from quake.FieldFloat intrinsic lowering")
	}
}

func TestCompile_FieldOffsetIntrinsic_RejectsNonFieldOffsetArg(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgofieldintrinsicbadargtest`)
	writeFieldIntrinsicStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgofieldintrinsicbadargtest/quake"

func Bad(ent *quake.Entity, notField float32) float32 {
	return quake.FieldFloat(ent, notField)
}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for non-field offset argument")
	}
	if !strings.Contains(err.Error(), "quake.FieldFloat arg 2 must be field offset") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompile_FieldOffsetIntrinsic_RejectsWrongArity(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgofieldintrinsicaritytest`)
	writeFieldIntrinsicStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgofieldintrinsicaritytest/quake"

func BadArity(ent *quake.Entity, ofs quake.FieldOffset) float32 {
	return quake.FieldFloat(ent, ofs, 1)
}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for wrong intrinsic arity")
	}
	msg := err.Error()
	if !strings.Contains(msg, "quake.FieldFloat expects 2 args") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestCompile_FieldOffsetIntrinsic_DefersNonFloatDynamicHelpers(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgofieldintrinsicdeferredtest`)
	if err := os.MkdirAll(filepath.Join(dir, "quake"), 0o755); err != nil {
		t.Fatalf("mkdir quake stub package: %v", err)
	}
	writeFile(t, filepath.Join(dir, "quake", "quake.go"), `package quake

type Entity struct{}
type FieldOffset any
type Vec3 [3]float32

func FieldFloat(entity *Entity, args ...any) float32 { return 0 }
func SetFieldFloat(entity *Entity, args ...any) {}

func FieldVector(entity *Entity, args ...any) Vec3 { return Vec3{} }
func SetFieldVector(entity *Entity, args ...any) {}
`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgofieldintrinsicdeferredtest/quake"

func ReadVector(ent *quake.Entity, ofs quake.FieldOffset) quake.Vec3 {
	return quake.FieldVector(ent, ofs)
}

func WriteVector(ent *quake.Entity, ofs quake.FieldOffset, value quake.Vec3) {
	quake.SetFieldVector(ent, ofs, value)
}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for deferred non-float dynamic helper")
	}
	msg := err.Error()
	if !strings.Contains(msg, "quake.FieldVector is deferred for dynamic field access; only quake.FieldFloat and quake.SetFieldFloat are currently supported") &&
		!strings.Contains(msg, "quake.SetFieldVector is deferred for dynamic field access; only quake.FieldFloat and quake.SetFieldFloat are currently supported") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func TestRoundTrip_FieldOffsetIntrinsic_FieldFloatRead(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgofieldintrinsicroundtriptest`)
	writeFieldIntrinsicRuntimeStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgofieldintrinsicroundtriptest/quake"

//qgo:entity
type Entity struct {
	Health float32 `+"`qgo:\"health\"`"+`
}

type FieldOffset any

func Read(ent *Entity, ofs FieldOffset) float32 {
	return quake.FieldFloat(ent, ofs)
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	vm := loadVM(t, data)
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)

	healthOfs := vm.FindField("health")
	if healthOfs < 0 {
		t.Fatal("field 'health' not found")
	}

	const initial = float32(33.5)
	const entNum = 1

	vm.SetEFloat(entNum, healthOfs, initial)

	fnum := vm.FindFunction("Read")
	if fnum < 0 {
		t.Fatal("function 'Read' not found")
	}

	vm.SetGInt(qc.OFSParm0, entNum)
	vm.SetGInt(qc.OFSParm1, int32(healthOfs))

	if err := vm.ExecuteProgram(fnum); err != nil {
		t.Fatalf("ExecuteProgram failed: %v", err)
	}

	if got := vm.GFloat(qc.OFSReturn); got != initial {
		t.Fatalf("Read return = %v, want initial value %v", got, initial)
	}
}

func TestRoundTrip_FieldOffsetIntrinsic_ReceiverFieldFloatRead(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgoreceiverfieldintrinsicroundtriptest`)
	writeFieldIntrinsicRuntimeStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgoreceiverfieldintrinsicroundtriptest/quake"

//qgo:entity
type Entity struct {
	Health float32 `+"`qgo:\"health\"`"+`
}

type FieldOffset any

func Read(ent *quake.Entity, ofs FieldOffset) float32 {
	return ent.FieldFloat(ofs)
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	vm := loadVM(t, data)
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)

	healthOfs := vm.FindField("health")
	if healthOfs < 0 {
		t.Fatal("field 'health' not found")
	}

	const initial = float32(48.25)
	const entNum = 1
	vm.SetEFloat(entNum, healthOfs, initial)

	fnum := vm.FindFunction("Read")
	if fnum < 0 {
		t.Fatal("function 'Read' not found")
	}

	vm.SetGInt(qc.OFSParm0, entNum)
	vm.SetGInt(qc.OFSParm1, int32(healthOfs))

	if err := vm.ExecuteProgram(fnum); err != nil {
		t.Fatalf("ExecuteProgram failed: %v", err)
	}
	if got := vm.GFloat(qc.OFSReturn); got != initial {
		t.Fatalf("Read return = %v, want %v", got, initial)
	}
}

func TestCompile_FieldOffsetIntrinsic_ReceiverFieldFloatOpcodes_NoCallFallback(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgoreceiverfieldintrinsictest`)
	writeFieldIntrinsicStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgoreceiverfieldintrinsictest/quake"

func Read(ent *quake.Entity, ofs quake.FieldOffset) float32 {
	return ent.FieldFloat(ofs)
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	stmts := parseStatements(t, data, header)
	hasLoadF := false
	hasCall := false
	for _, s := range stmts {
		switch qc.Opcode(s.Op) {
		case qc.OPLoadF:
			hasLoadF = true
		case qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
			hasCall = true
		}
	}

	if !hasLoadF {
		t.Fatal("expected OP_LOAD_F from quake.Entity.FieldFloat intrinsic lowering")
	}
	if hasCall {
		t.Fatal("did not expect OP_CALL* fallback for quake.Entity.FieldFloat intrinsic lowering")
	}
}

func TestCompile_FieldOffsetIntrinsic_ReceiverSetFieldFloatOpcodes_NoCallFallback(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgoreceiversetfieldintrinsictest`)
	writeFieldIntrinsicRuntimeStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgoreceiversetfieldintrinsictest/quake"

func Write(ent *quake.Entity, ofs quake.FieldOffset, value float32) {
	ent.SetFieldFloat(ofs, value)
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	header := parseHeader(t, data)
	stmts := parseStatements(t, data, header)
	hasAddress := false
	hasStorePF := false
	hasCall := false
	for _, s := range stmts {
		switch qc.Opcode(s.Op) {
		case qc.OPAddress:
			hasAddress = true
		case qc.OPStorePF:
			hasStorePF = true
		case qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
			hasCall = true
		}
	}

	if !hasAddress {
		t.Fatal("expected OP_ADDRESS from quake.Entity.SetFieldFloat intrinsic lowering")
	}
	if !hasStorePF {
		t.Fatal("expected OP_STOREP_F from quake.Entity.SetFieldFloat intrinsic lowering")
	}
	if hasCall {
		t.Fatal("did not expect OP_CALL* fallback for quake.Entity.SetFieldFloat intrinsic lowering")
	}
}

func TestRoundTrip_FieldOffsetIntrinsic_ReceiverSetFieldFloatWrite(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgoreceiversetfieldintrinsicroundtriptest`)
	writeFieldIntrinsicRuntimeStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgoreceiversetfieldintrinsicroundtriptest/quake"

//qgo:entity
type Entity struct {
	Health float32 `+"`qgo:\"health\"`"+`
}

type FieldOffset any

func Write(ent *quake.Entity, ofs FieldOffset, value float32) {
	ent.SetFieldFloat(ofs, value)
}
`)

	c := New()
	data, err := c.Compile(dir)
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}

	vm := loadVM(t, data)
	vm.NumEdicts = 4
	vm.Edicts = make([]byte, vm.EdictSize*vm.NumEdicts)

	healthOfs := vm.FindField("health")
	if healthOfs < 0 {
		t.Fatal("field 'health' not found")
	}

	const initial = float32(10.0)
	const updated = float32(96.5)
	const entNum = 1
	vm.SetEFloat(entNum, healthOfs, initial)

	fnum := vm.FindFunction("Write")
	if fnum < 0 {
		t.Fatal("function 'Write' not found")
	}

	vm.SetGInt(qc.OFSParm0, entNum)
	vm.SetGInt(qc.OFSParm1, int32(healthOfs))
	vm.SetGFloat(qc.OFSParm2, updated)

	if err := vm.ExecuteProgram(fnum); err != nil {
		t.Fatalf("ExecuteProgram failed: %v", err)
	}
	if got := vm.EFloat(entNum, healthOfs); got != updated {
		t.Fatalf("entity health after receiver write = %v, want %v", got, updated)
	}
}

func TestCompile_FieldOffsetIntrinsic_ReceiverSetFieldVectorDeferred(t *testing.T) {
	dir := makeCompilerTempDir(t)
	writeQGoModule(t, dir, `module qgoreceiversetfielddeferredtest`)
	writeFieldIntrinsicRuntimeStubPackage(t, filepath.Join(dir, "quake"))
	writeFile(t, filepath.Join(dir, "quake", "extra.go"), `package quake

type Vec3 [3]float32

func (e *Entity) SetFieldVector(fieldOffset any, value Vec3) {}
`)
	writeFile(t, filepath.Join(dir, "main.go"), `package main

import "qgoreceiversetfielddeferredtest/quake"

func Write(ent *quake.Entity, ofs quake.FieldOffset, value quake.Vec3) {
	ent.SetFieldVector(ofs, value)
}
`)

	c := New()
	_, err := c.Compile(dir)
	if err == nil {
		t.Fatal("expected compile to fail for deferred receiver dynamic helper")
	}
	if !strings.Contains(err.Error(), "quake.Entity.SetFieldVector is deferred for dynamic field access; only quake.Entity.FieldFloat and quake.Entity.SetFieldFloat receiver forms are currently supported") {
		t.Fatalf("unexpected compile error: %v", err)
	}
}

func mustAppearInOrder(t *testing.T, funcs []qc.DFunction, stringTable []byte, names []string) {
	t.Helper()

	indices := make(map[string]int, len(names))
	for _, name := range names {
		indices[name] = -1
	}

	for i, fn := range funcs {
		name := stringAt(stringTable, fn.Name)
		if _, ok := indices[name]; ok && indices[name] == -1 {
			indices[name] = i
		}
	}

	for _, name := range names {
		if indices[name] == -1 {
			t.Fatalf("function %q not found in function table", name)
		}
	}

	for i := 1; i < len(names); i++ {
		prev := indices[names[i-1]]
		curr := indices[names[i]]
		if prev >= curr {
			t.Fatalf("function order mismatch: %q index=%d should come before %q index=%d", names[i-1], prev, names[i], curr)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeQGoModule(t *testing.T, dir, moduleDecl string) {
	t.Helper()
	writeFile(t, filepath.Join(dir, "go.mod"), moduleDecl+`

go 1.26
`)
}

func makeCompilerTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp(".", "fieldoffset-intrinsic-*")
	if err != nil {
		t.Fatalf("mkdirtemp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})
	return dir
}

func writeFieldIntrinsicStubPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir quake stub package: %v", err)
	}
	writeFile(t, filepath.Join(dir, "quake.go"), `package quake

type Entity struct{}
type FieldOffset any

func FieldFloat(entity *Entity, args ...any) float32 { return 0 }
func SetFieldFloat(entity *Entity, args ...any) {}
func (e *Entity) FieldFloat(args ...any) float32 { return 0 }
func (e *Entity) SetFieldFloat(args ...any) {}
`)
}

func writeFieldIntrinsicRuntimeStubPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir quake stub package: %v", err)
	}
	writeFile(t, filepath.Join(dir, "quake.go"), `package quake

type FieldOffset any

func FieldFloat(entity any, args ...any) float32 { return 0 }
func SetFieldFloat(entity any, args ...any) {}

type Entity struct{}

func (e *Entity) FieldFloat(args ...any) float32 { return 0 }
func (e *Entity) SetFieldFloat(args ...any) {}
`)
}
