package qc

import (
	"math"
	"testing"
)

func TestExecuteProgramDivByZeroBehaviorMatrixMatchesC(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 64)

	const (
		mainFuncNum = 0
		numerOfs    = 10
		denomOfs    = 11
		resultOfs   = 12
	)

	vm.Functions = []DFunction{{FirstStatement: 0}}
	vm.Statements = []DStatement{
		{Op: uint16(OPDivF), A: uint16(numerOfs), B: uint16(denomOfs), C: uint16(resultOfs)},
		{Op: uint16(OPDone), A: uint16(resultOfs)},
	}

	negZero := float32(math.Copysign(0, -1))
	tests := []struct {
		name   string
		numer  float32
		denom  float32
		check  func(float32) bool
		checkR func(float32) bool
	}{
		{
			name:   "1/+0 -> +Inf",
			numer:  1,
			denom:  0,
			check:  func(v float32) bool { return math.IsInf(float64(v), 1) },
			checkR: func(v float32) bool { return math.IsInf(float64(v), 1) },
		},
		{
			name:   "-1/+0 -> -Inf",
			numer:  -1,
			denom:  0,
			check:  func(v float32) bool { return math.IsInf(float64(v), -1) },
			checkR: func(v float32) bool { return math.IsInf(float64(v), -1) },
		},
		{
			name:   "1/-0 -> -Inf",
			numer:  1,
			denom:  negZero,
			check:  func(v float32) bool { return math.IsInf(float64(v), -1) },
			checkR: func(v float32) bool { return math.IsInf(float64(v), -1) },
		},
		{
			name:   "-1/-0 -> +Inf",
			numer:  -1,
			denom:  negZero,
			check:  func(v float32) bool { return math.IsInf(float64(v), 1) },
			checkR: func(v float32) bool { return math.IsInf(float64(v), 1) },
		},
		{
			name:   "0/+0 -> NaN",
			numer:  0,
			denom:  0,
			check:  func(v float32) bool { return math.IsNaN(float64(v)) },
			checkR: func(v float32) bool { return math.IsNaN(float64(v)) },
		},
		{
			name:   "0/-0 -> NaN",
			numer:  0,
			denom:  negZero,
			check:  func(v float32) bool { return math.IsNaN(float64(v)) },
			checkR: func(v float32) bool { return math.IsNaN(float64(v)) },
		},
		{
			name:   "finite divide still works",
			numer:  5,
			denom:  -2,
			check:  func(v float32) bool { return v == -2.5 },
			checkR: func(v float32) bool { return v == -2.5 },
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			vm.SetGFloat(numerOfs, tc.numer)
			vm.SetGFloat(denomOfs, tc.denom)
			vm.SetGFloat(resultOfs, 0)
			vm.SetGFloat(OFSReturn, 0)

			if err := vm.ExecuteProgram(mainFuncNum); err != nil {
				t.Fatalf("ExecuteProgram() error = %v", err)
			}

			got := vm.GFloat(resultOfs)
			if !tc.check(got) {
				t.Fatalf("result = %v, case %q failed", got, tc.name)
			}
			if gotReturn := vm.GFloat(OFSReturn); !tc.checkR(gotReturn) {
				t.Fatalf("OFSReturn = %v, case %q failed", gotReturn, tc.name)
			}
		})
	}
}

func TestExecuteProgramOPAddressAllowsWorldEntityFieldStores(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 128)
	vm.MaxEdicts = 2
	vm.NumEdicts = 1
	vm.EntityFields = 64
	vm.EdictSize = 28 + vm.EntityFields*4
	vm.Edicts = make([]byte, vm.EdictSize*2)

	const (
		mainFuncNum = 0
		fieldOfs    = 10
		ptrOfs      = 11
		valueOfs    = 12
	)

	vm.Functions = []DFunction{{FirstStatement: 0}}
	vm.Statements = []DStatement{
		{Op: uint16(OPAddress), A: uint16(OFSSelf), B: uint16(fieldOfs), C: uint16(ptrOfs)},
		{Op: uint16(OPStorePF), A: uint16(valueOfs), B: uint16(ptrOfs)},
		{Op: uint16(OPDone)},
	}
	vm.SetGInt(OFSSelf, 0)
	vm.SetGInt(fieldOfs, EntFieldHealth)
	vm.SetGFloat(valueOfs, 42)

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}
	if got := vm.EFloat(0, EntFieldHealth); got != 42 {
		t.Fatalf("world health = %v, want 42", got)
	}
}

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
			// ExecuteProgram starts at FirstStatement directly.
			FirstStatement: 0,
		},
		{
			// callFunction uses FirstStatement-1 because the loop's
			// bottom XStatement++ fires after it returns.
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
			FirstStatement: 0, // entry point starts at statement 0
		},
		{
			FirstStatement: 3, // callees use FirstStatement-1 internally
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
			FirstStatement: 0, // entry point starts at statement 0
		},
		{
			FirstStatement: 3, // callee
			ParmStart:      sharedLocalOfs,
			Locals:         1,
		},
		{
			FirstStatement: 8, // callee
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
			FirstStatement: 0, // entry point starts at statement 0
		},
		{
			FirstStatement: 3, // callee
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

func TestProfileResultsAccumulatesPerFunction(t *testing.T) {
	// Set up a VM where main calls callee twice in a loop (via goto).
	// After execution, ProfileResults should show both functions with
	// non-zero statement counts, callee having more than main.
	vm := NewVM()
	vm.Globals = make([]float32, 64)

	// String table: "main\0callee\0"
	vm.Strings = []byte("main\x00callee\x00")

	const (
		mainFuncNum   = 0
		calleeFuncNum = 1
		calleeRefOfs  = 10
		counterOfs    = 11
		oneOfs        = 12
		twoOfs        = 13
	)

	vm.Functions = []DFunction{
		{FirstStatement: 0, Name: 0}, // "main" at string offset 0
		{FirstStatement: 6, Name: 5}, // "callee" at string offset 5
	}

	// main: counter=2; loop: call callee; counter--; if counter>0 goto loop; done
	vm.Statements = []DStatement{
		// stmt 0: counter = 2
		{Op: uint16(OPStoreF), A: uint16(twoOfs), B: uint16(counterOfs)},
		// stmt 1: call callee
		{Op: uint16(OPCall0), A: uint16(calleeRefOfs)},
		// stmt 2: counter = counter - 1
		{Op: uint16(OPSubF), A: uint16(counterOfs), B: uint16(oneOfs), C: uint16(counterOfs)},
		// stmt 3: if counter != 0, goto stmt 1 (offset = 1-4 = -3)
		{Op: uint16(OPIF), A: uint16(counterOfs), B: uint16(65534)},
		// stmt 4: done
		{Op: uint16(OPDone), A: 0},

		// filler
		{Op: uint16(OPDone), A: 0},

		// callee: stmt 6: just return
		{Op: uint16(OPDone), A: 0},
	}

	vm.SetGInt(calleeRefOfs, calleeFuncNum)
	vm.SetGFloat(oneOfs, 1)
	vm.SetGFloat(twoOfs, 2)

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}

	results := vm.ProfileResults(10)
	if len(results) == 0 {
		t.Fatal("ProfileResults returned no results")
	}

	// Both main and callee should appear with non-zero counts.
	found := map[string]int32{}
	for _, r := range results {
		found[r.Name] = r.Profile
	}

	if found["main"] == 0 {
		t.Error("main should have non-zero profile count")
	}
	if found["callee"] == 0 {
		t.Error("callee should have non-zero profile count")
	}

	// After ProfileResults, counters should be reset.
	results2 := vm.ProfileResults(10)
	if len(results2) != 0 {
		t.Errorf("after reset, ProfileResults should be empty, got %d entries", len(results2))
	}
}

func TestExecuteProgramRunawayLoopProtectionMatchesC(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 16)
	vm.Functions = []DFunction{{FirstStatement: 0}}
	vm.Statements = []DStatement{{Op: uint16(OPGoto), A: uint16(0)}}

	if err := vm.ExecuteProgram(0); err == nil || err.Error() != "runaway loop error" {
		t.Fatalf("ExecuteProgram() error = %v, want runaway loop error", err)
	}
	if got := vm.XStatement; got != 0 {
		t.Fatalf("XStatement = %d, want 0 at runaway loop trap", got)
	}
}

func TestExecuteProgramRunawayLoopLimitConstantMatchesC(t *testing.T) {
	if runawayLoopLimit != 0x1000000 {
		t.Fatalf("runawayLoopLimit = %#x, want %#x", runawayLoopLimit, 0x1000000)
	}
}

func TestExecuteProgramRunawayLoopLimitOverrideUsesVMFixture(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 16)
	vm.Functions = []DFunction{{FirstStatement: 0}}
	vm.Statements = []DStatement{{Op: uint16(OPGoto), A: uint16(0)}}
	vm.RunawayLoopLimit = 3

	if got := vm.statementBudgetLimit(); got != 3 {
		t.Fatalf("statementBudgetLimit() = %d, want 3", got)
	}

	if err := vm.ExecuteProgram(0); err == nil || err.Error() != "runaway loop error" {
		t.Fatalf("ExecuteProgram() error = %v, want runaway loop error", err)
	}
	if got := vm.XStatement; got != 0 {
		t.Fatalf("XStatement = %d, want 0 at override runaway loop trap", got)
	}
}

func TestExecuteProgramTraceCallEventsNested(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 64)

	const (
		mainFuncNum   = 0
		calleeFuncNum = 1
		calleeRefOfs  = 10
	)

	vm.Functions = []DFunction{
		{FirstStatement: 0},
		{FirstStatement: 2},
	}
	vm.Statements = []DStatement{
		{Op: uint16(OPCall0), A: uint16(calleeRefOfs)},
		{Op: uint16(OPDone), A: 0},
		{Op: uint16(OPDone), A: 0},
	}
	vm.SetGInt(calleeRefOfs, calleeFuncNum)

	var got []TraceCallEvent
	vm.TraceCallFunc = func(vm *VM, event TraceCallEvent) {
		got = append(got, event)
	}

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}
	if vm.XFunction != nil {
		t.Fatalf("XFunction = %#v, want nil after unwind", vm.XFunction)
	}
	if vm.XFunctionIndex != -1 {
		t.Fatalf("XFunctionIndex = %d, want -1 after unwind", vm.XFunctionIndex)
	}

	want := []TraceCallEvent{
		{Phase: "enter", Depth: 1, FunctionIndex: 0},
		{Phase: "enter", Depth: 2, FunctionIndex: 1},
		{Phase: "leave", Depth: 2, FunctionIndex: 1},
		{Phase: "leave", Depth: 1, FunctionIndex: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("trace events len=%d, want %d; got=%#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trace event[%d]=%#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestExecuteProgramTraceCallEventsBuiltin(t *testing.T) {
	vm := NewVM()
	vm.Globals = make([]float32, 64)

	const (
		mainFuncNum = 0
		funcRefOfs  = 10
	)

	vm.Functions = []DFunction{
		{FirstStatement: 0},
	}
	vm.Statements = []DStatement{
		{Op: uint16(OPCall0), A: uint16(funcRefOfs)},
		{Op: uint16(OPDone), A: 0},
	}
	vm.SetGInt(funcRefOfs, -1)
	vm.Builtins[1] = func(vm *VM) {}

	var got []TraceCallEvent
	vm.TraceCallFunc = func(vm *VM, event TraceCallEvent) {
		got = append(got, event)
	}

	if err := vm.ExecuteProgram(mainFuncNum); err != nil {
		t.Fatalf("ExecuteProgram() error = %v", err)
	}

	want := []TraceCallEvent{
		{Phase: "enter", Depth: 1, FunctionIndex: 0},
		{Phase: "builtin", Depth: 2, FunctionIndex: -1},
		{Phase: "leave", Depth: 1, FunctionIndex: 0},
	}
	if len(got) != len(want) {
		t.Fatalf("trace events len=%d, want %d; got=%#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("trace event[%d]=%#v, want %#v", i, got[i], want[i])
		}
	}
}
