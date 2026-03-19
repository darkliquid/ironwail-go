package server

import (
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/qc"
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
