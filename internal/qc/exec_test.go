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

func TestExecuteProgramCallCopiesParametersIntoCalleeLocals(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 128)

	const (
		mainFuncNum   = 0
		calleeFuncNum = 1

		calleeRefOfs = 10
		argValueOfs  = 20
		capturedOfs  = 21
		calleeParm0  = 40
	)

	vm.Functions = []DFunction{
		{
			FirstStatement: 1, // start at statement 0
		},
		{
			FirstStatement: 3, // start at statement 3
			ParmStart:      calleeParm0,
			Locals:         1,
			NumParms:       1,
			ParmSize:       [MaxParms]byte{1},
		},
	}

	vm.Statements = []DStatement{
		// main: parm0 = argValue; call callee(); return captured
		{Op: uint16(OPStoreF), A: uint16(argValueOfs), B: uint16(OFSParm0)},
		{Op: uint16(OPCall1), A: uint16(calleeRefOfs)},
		{Op: uint16(OPDone), A: uint16(capturedOfs)},

		// callee: captured = local parm0 (at ParmStart), then return captured
		{Op: uint16(OPStoreF), A: uint16(calleeParm0), B: uint16(capturedOfs)},
		{Op: uint16(OPDone), A: uint16(capturedOfs)},
	}

	vm.SetGInt(calleeRefOfs, calleeFuncNum)
	vm.SetGFloat(argValueOfs, 37)

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}

	if got := vm.GFloat(capturedOfs); got != 37 {
		t.Fatalf("callee received = %v, want 37", got)
	}
	if got := vm.GFloat(OFSReturn); got != 37 {
		t.Fatalf("return value = %v, want 37", got)
	}
}

func TestExecuteProgramNestedCallRestoresCallerLocals(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 128)

	const (
		mainFuncNum   = 0
		callerFuncNum = 1
		calleeFuncNum = 2

		callerRefOfs = 10
		calleeRefOfs = 11
		initialOfs   = 20
		calleeSetOfs = 21
		resultOfs    = 22

		sharedLocalOfs = 40
	)

	vm.Functions = []DFunction{
		{
			FirstStatement: 1, // start at statement 0
		},
		{
			FirstStatement: 3, // start at statement 3
			ParmStart:      sharedLocalOfs,
			Locals:         1,
		},
		{
			FirstStatement: 8, // start at statement 8
			ParmStart:      sharedLocalOfs,
			Locals:         1,
		},
	}

	vm.Statements = []DStatement{
		// main: call caller(); return caller result
		{Op: uint16(OPCall0), A: uint16(callerRefOfs)},
		{Op: uint16(OPDone), A: uint16(resultOfs)},

		// filler (unused)
		{Op: uint16(OPDone), A: 0},

		// caller:
		//   sharedLocal = initial
		//   call callee() which overwrites sharedLocal
		//   result = sharedLocal (must still be initial after callee returns)
		//   return result
		{Op: uint16(OPStoreF), A: uint16(initialOfs), B: uint16(sharedLocalOfs)},
		{Op: uint16(OPCall0), A: uint16(calleeRefOfs)},
		{Op: uint16(OPStoreF), A: uint16(sharedLocalOfs), B: uint16(resultOfs)},
		{Op: uint16(OPDone), A: uint16(resultOfs)},

		// filler (unused)
		{Op: uint16(OPDone), A: 0},

		// callee: sharedLocal = calleeSet; return sharedLocal
		{Op: uint16(OPStoreF), A: uint16(calleeSetOfs), B: uint16(sharedLocalOfs)},
		{Op: uint16(OPDone), A: uint16(sharedLocalOfs)},
	}

	vm.SetGInt(callerRefOfs, callerFuncNum)
	vm.SetGInt(calleeRefOfs, calleeFuncNum)
	vm.SetGFloat(initialOfs, 99)
	vm.SetGFloat(calleeSetOfs, 7)

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}

	if got := vm.GFloat(resultOfs); got != 99 {
		t.Fatalf("caller local after nested call = %v, want 99", got)
	}
	if got := vm.GFloat(OFSReturn); got != 99 {
		t.Fatalf("return value = %v, want 99", got)
	}
}

func TestExecuteProgramOPReturnCopiesThreeSlotsToOFSReturn(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 128)

	const (
		mainFuncNum   = 0
		calleeFuncNum = 1

		calleeRefOfs = 10
		returnVecOfs = 30
	)

	vm.Functions = []DFunction{
		{
			FirstStatement: 1, // start at statement 0
		},
		{
			FirstStatement: 3, // start at statement 3
		},
	}

	vm.Statements = []DStatement{
		// main: call callee(); return OFSReturn unchanged
		{Op: uint16(OPCall0), A: uint16(calleeRefOfs)},
		{Op: uint16(OPDone), A: uint16(OFSReturn)},

		// filler (unused)
		{Op: uint16(OPDone), A: 0},

		// callee: explicit OPReturn from returnVecOfs
		{Op: uint16(OPReturn), A: uint16(returnVecOfs)},
	}

	vm.SetGInt(calleeRefOfs, calleeFuncNum)
	vm.SetGVector(returnVecOfs, [3]float32{11, 22, 33})
	vm.SetGVector(OFSReturn, [3]float32{-1, -2, -3}) // stale seed; must be replaced

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}

	if got := vm.GVector(OFSReturn); got != [3]float32{11, 22, 33} {
		t.Fatalf("OFSReturn vector = %v, want [11 22 33]", got)
	}
}
