package qc

import "testing"

func TestExecuteProgramCallRunsCalleeFirstStatement(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 64)

	const (
		mainFuncNum   = 0
		calleeFuncNum = 1

		funcRefOfs = 10
		valueOfs   = 20
		targetOfs  = 21
	)

	vm.Functions = []DFunction{
		{
			// ExecuteProgram initializes XStatement to FirstStatement-1.
			// Use 1 so execution starts at statement 0.
			FirstStatement: 1,
		},
		{
			FirstStatement: 3,
		},
	}

	vm.Statements = []DStatement{
		// main: call callee(), then return target
		{Op: uint16(OPCall0), A: uint16(funcRefOfs)},
		{Op: uint16(OPDone), A: uint16(targetOfs)},

		// filler: never executed; keeps callee first_statement at index 3
		{Op: uint16(OPDone), A: 0},

		// callee first statement: must run before OPDone in callee
		{Op: uint16(OPStoreF), A: uint16(valueOfs), B: uint16(targetOfs)},
		{Op: uint16(OPDone), A: uint16(targetOfs)},
	}

	vm.SetGInt(funcRefOfs, calleeFuncNum)
	vm.SetGFloat(valueOfs, 42)

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}

	if got := vm.GFloat(targetOfs); got != 42 {
		t.Fatalf("target global = %v, want 42 (callee first statement must execute)", got)
	}
	if got := vm.GFloat(OFSReturn); got != 42 {
		t.Fatalf("return value = %v, want 42", got)
	}
}
