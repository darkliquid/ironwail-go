package server

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestExecuteQCFunctionLogsTraceChain(t *testing.T) {
	s := NewServer()
	vm := s.QCVM
	vm.Globals = make([]float32, 128)

	self := s.AllocEdict()
	other := s.AllocEdict()
	self.Vars.ClassName = vm.AllocString("monster_ogre")
	other.Vars.ClassName = vm.AllocString("trigger_once")

	const (
		mainFuncNum   = 2
		calleeFuncNum = 1
		calleeRefOfs  = 10
	)
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("trigger_relay"), FirstStatement: 2},
		{Name: vm.AllocString("monster_use"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(calleeRefOfs)},
		{Op: uint16(qc.OPDone), A: 0},
		{Op: uint16(qc.OPDone), A: 0},
	}
	vm.SetGInt(calleeRefOfs, calleeFuncNum)
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))
	vm.SetGInt(qc.OFSOther, int32(s.NumForEdict(other)))

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			QCTrace:     true,
			QCVerbosity: 1,
			EventMask:   debugEventMaskQC,
			SummaryMode: 0,
			EntityFilter: debugEntityFilter{
				all: true,
			},
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	s.DebugTelemetry.BeginFrame(1.0, 0.1)

	if err := s.executeQCFunction(mainFuncNum); err != nil {
		t.Fatalf("executeQCFunction() error = %v", err)
	}

	if len(lines) != 4 {
		t.Fatalf("logged %d lines, want 4: %#v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"phase=enter",
		"phase=leave",
		"depth=1",
		"depth=2",
		"fn=monster_use[#2]",
		"fn=trigger_relay[#1]",
		`classname="monster_ogre"`,
		"self=1 other=2",
		`other_classname="trigger_once"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in trace output:\n%s", want, joined)
		}
	}
}

func TestExecuteQCFunctionBuiltinTracingHonorsVerbosity(t *testing.T) {
	s := NewServer()
	vm := s.QCVM
	vm.Globals = make([]float32, 128)

	self := s.AllocEdict()
	self.Vars.ClassName = vm.AllocString("func_button")

	const (
		mainFuncNum = 0
		funcRefOfs  = 10
	)
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("button_use"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(funcRefOfs)},
		{Op: uint16(qc.OPDone), A: 0},
	}
	vm.SetGInt(funcRefOfs, -1)
	vm.Builtins[1] = func(vm *qc.VM) {}
	vm.SetGInt(qc.OFSSelf, int32(s.NumForEdict(self)))

	lines := make([]string, 0, 3)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			QCTrace:      true,
			QCVerbosity:  1,
			EventMask:    debugEventMaskQC,
			SummaryMode:  0,
			EntityFilter: debugEntityFilter{all: true},
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	s.DebugTelemetry.BeginFrame(2.0, 0.1)

	if err := s.executeQCFunction(mainFuncNum); err != nil {
		t.Fatalf("executeQCFunction() error = %v", err)
	}

	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "phase=builtin") {
		t.Fatalf("builtin trace should be filtered at verbosity=1:\n%s", joined)
	}
	if !strings.Contains(joined, "phase=enter") || !strings.Contains(joined, "phase=leave") {
		t.Fatalf("missing function enter/leave events:\n%s", joined)
	}
}

func TestExecuteQCFunctionRestoresVMStateAfterError(t *testing.T) {
	s := NewServer()
	vm := s.QCVM
	vm.Globals = make([]float32, 128)

	self := s.AllocEdict()
	selfNum := int32(s.NumForEdict(self))

	const (
		badFuncNum  = 1
		goodFuncNum = 2
		fieldOfs    = 16
		ptrOfs      = 17
	)
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("bad_touch"), FirstStatement: 0},
		{Name: vm.AllocString("good_touch"), FirstStatement: 2},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPAddress), A: uint16(qc.OFSNull), B: uint16(fieldOfs), C: uint16(ptrOfs)},
		{Op: uint16(qc.OPDone), A: 0},
		{Op: uint16(qc.OPDone), A: 0},
	}
	vm.SetGInt(fieldOfs, qc.EntFieldHealth)
	vm.SetGInt(qc.OFSSelf, selfNum)

	if err := s.executeQCFunction(badFuncNum); err == nil {
		t.Fatal("bad QC function unexpectedly succeeded")
	}
	if vm.Depth != 0 {
		t.Fatalf("Depth after error = %d, want 0", vm.Depth)
	}
	if vm.LocalUsed != 0 {
		t.Fatalf("LocalUsed after error = %d, want 0", vm.LocalUsed)
	}
	if vm.XFunction != nil {
		t.Fatalf("XFunction after error = %#v, want nil", vm.XFunction)
	}
	if vm.XFunctionIndex != -1 {
		t.Fatalf("XFunctionIndex after error = %d, want -1", vm.XFunctionIndex)
	}
	if got := vm.GInt(qc.OFSSelf); got != selfNum {
		t.Fatalf("self after error = %d, want %d", got, selfNum)
	}

	if err := s.executeQCFunction(goodFuncNum); err != nil {
		t.Fatalf("good QC function failed after prior error: %v", err)
	}
}

func TestExecuteQCFunctionRestoresVMStateAfterSuccess(t *testing.T) {
	s := NewServer()
	vm := s.QCVM
	vm.Globals = make([]float32, 128)

	const funcNum = 1
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("successful_touch"), FirstStatement: 0},
		{Name: vm.AllocString("outer_context"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone), A: 0},
		{Op: uint16(qc.OPDone), A: 0},
	}

	vm.SetGInt(qc.OFSSelf, 77)
	vm.SetGInt(qc.OFSOther, 88)
	vm.Depth = 0
	vm.LocalUsed = 5
	vm.XFunction = &vm.Functions[2]
	vm.XFunctionIndex = 2
	vm.XStatement = 123

	if err := s.executeQCFunction(funcNum); err != nil {
		t.Fatalf("successful QC function failed: %v", err)
	}
	if got := vm.GInt(qc.OFSSelf); got != 77 {
		t.Fatalf("self after success = %d, want 77", got)
	}
	if got := vm.GInt(qc.OFSOther); got != 88 {
		t.Fatalf("other after success = %d, want 88", got)
	}
	if vm.Depth != 0 {
		t.Fatalf("Depth after success = %d, want 0", vm.Depth)
	}
	if vm.LocalUsed != 5 {
		t.Fatalf("LocalUsed after success = %d, want 5", vm.LocalUsed)
	}
	if vm.XFunction != &vm.Functions[2] {
		t.Fatalf("XFunction after success = %#v, want outer context", vm.XFunction)
	}
	if vm.XFunctionIndex != 2 {
		t.Fatalf("XFunctionIndex after success = %d, want 2", vm.XFunctionIndex)
	}
	if vm.XStatement != 123 {
		t.Fatalf("XStatement after success = %d, want 123", vm.XStatement)
	}
}
