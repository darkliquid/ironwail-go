package compiler

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
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
		"../testdata/vec3methods": {
			requiredParms: map[string]int32{
				"Compose": 7,
			},
			requiredOpcodes: []qc.Opcode{qc.OPAddV, qc.OPSubV, qc.OPMulVF, qc.OPMulV},
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
		{
			name:     "vec3-methods-compose-deterministic",
			fixture:  "../testdata/vec3methods",
			function: "Compose",
			args:     []float32{2, 0, 0, -1, 0, 0, 2.25},
			goExpect: func(args []float32) float32 {
				ax, ay, az := args[0], args[1], args[2]
				bx, by, bz := args[3], args[4], args[5]
				s := args[6]
				return (ax*s)*bx + (ay*s)*by + (az*s)*bz
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
