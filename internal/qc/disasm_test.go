package qc

import (
	"strings"
	"testing"
)

// disasmTestVM builds a minimal VM with a couple of functions, named globals,
// and a named field so disassembly output can be asserted deterministically.
func disasmTestVM() *VM {
	vm := NewVM()
	// String table: offset 0 is empty; then "self", "other", "time", "main",
	// "target_fn", "message", "player", "$hello".
	strs := []string{"", "self", "other", "time", "main", "target_fn", "message", "player", "$hello"}
	vm.Strings = []byte{}
	sIdx := make(map[string]int32)
	for _, s := range strs {
		sIdx[s] = int32(len(vm.Strings))
		vm.Strings = append(vm.Strings, []byte(s)...)
		vm.Strings = append(vm.Strings, 0)
	}

	vm.GlobalDefs = []DDef{
		{Ofs: 28, Name: sIdx["self"]},
		{Ofs: 29, Name: sIdx["other"]},
		{Ofs: 31, Name: sIdx["time"]},
		{Ofs: 99, Name: sIdx["message"]},
		{Ofs: 200, Name: sIdx["target_fn"]},
	}
	vm.FieldDefs = []DDef{
		{Ofs: 28, Name: sIdx["self"]},
		{Ofs: 99, Name: sIdx["message"]},
	}

	// Globals storage: global 200 holds function index 1 (target_fn).
	// Function references are raw int32 bit patterns, so store via SetGInt.
	vm.Globals = make([]float32, 4096)
	vm.SetGInt(200, 1)

	// Function 0 reserved empty; function 1 = target_fn (builtin 73);
	// function 2 = main (bytecode at statement 0).
	vm.Functions = []DFunction{
		{Name: 0},
		{Name: sIdx["target_fn"], FirstStatement: -73},
		{Name: sIdx["main"], FirstStatement: 0},
	}

	// main body:
	//   [0] LOADS  self.message -> g100   (a=28 self, b=g400(field ofs holder), c=100)
	//   [1] EQS    g100 == g300           (g300 holds "$hello")
	//   [2] IFNOT  g401 +2
	//   [3] CALL2  target_fn
	//   [4] RETURN
	//   [5] DONE
	vm.Statements = []DStatement{
		{Op: uint16(OPLoadS), A: 28, B: 400, C: 100},
		{Op: uint16(OPEqS), A: 100, B: 300, C: 401},
		{Op: uint16(OPIFNot), A: 401, B: 2},
		{Op: uint16(OPCall2), A: 200},
		{Op: uint16(OPReturn), A: 1, B: 1, C: 1},
		{Op: uint16(OPDone)},
	}

	// Field-offset indirection: global 400 holds the value 99 (message field).
	vm.SetGFloat(400, 99)
	// String constant: global 300 holds the string index of "$hello".
	vm.SetGInt(300, sIdx["$hello"])

	return vm
}

func TestDisassembleIncludesHeader(t *testing.T) {
	vm := disasmTestVM()
	var sb strings.Builder
	if err := vm.Disassemble(&sb, DisasmOptions{}); err != nil {
		t.Fatalf("Disassemble: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "statements") || !strings.Contains(out, "functions") {
		t.Errorf("header missing counts, got:\n%s", out)
	}
}

func TestDisassembleDecodesFunctionBody(t *testing.T) {
	vm := disasmTestVM()
	var sb strings.Builder
	if err := vm.Disassemble(&sb, DisasmOptions{Function: "main"}); err != nil {
		t.Fatalf("Disassemble: %v", err)
	}
	out := sb.String()
	t.Logf("disasm:\n%s", out)

	for _, want := range []string{
		"main",
		"LOADS",
		"EQS",
		"IFNOT",
		"CALL2",
		"RETURN",
		"DONE",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q, got:\n%s", want, out)
		}
	}
}

func TestDisassembleAnnotatesCallTarget(t *testing.T) {
	vm := disasmTestVM()
	var sb strings.Builder
	if err := vm.Disassemble(&sb, DisasmOptions{Function: "main"}); err != nil {
		t.Fatalf("Disassemble: %v", err)
	}
	out := sb.String()
	// The CALL2 to the builtin should name target_fn and its builtin number.
	if !strings.Contains(out, "target_fn") {
		t.Errorf("call target not annotated, got:\n%s", out)
	}
}

func TestDisassembleUnknownFunctionErrors(t *testing.T) {
	vm := disasmTestVM()
	var sb strings.Builder
	if err := vm.Disassemble(&sb, DisasmOptions{Function: "nope"}); err == nil {
		t.Fatal("expected error for unknown function")
	}
}

func TestOpcodeNameCoversAllDefinedOpcodes(t *testing.T) {
	// Every opcode value in the defined enum range must have a non-empty name.
	for op := OPDone; op <= OPBitOr; op++ {
		if OpcodeName(op) == "" {
			t.Errorf("OpcodeName(%d) is empty", int(op))
		}
	}
	if OpcodeName(Opcode(0xFFFF)) == "" {
		// Out-of-range should still render something non-empty (e.g. "?9999").
		t.Error("OpcodeName(out-of-range) should render a placeholder, got empty")
	}
}
