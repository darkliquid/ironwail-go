// This file belongs to the Entity/QC subsystem: edict allocation, entity accessors, QuakeC field offsets, QC call tracing, and entity state types.

package server

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

type qcExecutionContext struct {
	self           int32
	other          int32
	depth          int
	localUsed      int
	xFunction      *qc.DFunction
	xFunctionIndex int32
	xStatement     int
}

func captureQCExecutionContext(vm *qc.VM) qcExecutionContext {
	if vm == nil {
		return qcExecutionContext{}
	}
	// O4 (plan 27): plain field reads instead of inline closures — the
	// previous func() wrappers could keep closures alive per call. Reading
	// the globals directly is identical and allocation-free.
	hasGlobals := len(vm.Globals) > qc.OFSOther
	var self, other int32
	if hasGlobals {
		self = vm.GInt(qc.OFSSelf)
		other = vm.GInt(qc.OFSOther)
	}
	return qcExecutionContext{
		self:           self,
		other:          other,
		depth:          vm.Depth,
		localUsed:      vm.LocalUsed,
		xFunction:      vm.XFunction,
		xFunctionIndex: vm.XFunctionIndex,
		xStatement:     vm.XStatement,
	}
}

func restoreQCExecutionContext(vm *qc.VM, ctx qcExecutionContext) {
	if vm == nil {
		return
	}
	if len(vm.Globals) > qc.OFSOther {
		vm.SetGInt(qc.OFSSelf, ctx.self)
		vm.SetGInt(qc.OFSOther, ctx.other)
	}
	for vm.Depth > ctx.depth {
		if err := vm.LeaveFunction(); err != nil {
			break
		}
	}
	if vm.Depth != ctx.depth {
		vm.Depth = ctx.depth
	}
	if vm.LocalUsed != ctx.localUsed {
		vm.LocalUsed = ctx.localUsed
	}
	vm.XFunction = ctx.xFunction
	vm.XFunctionIndex = ctx.xFunctionIndex
	vm.XStatement = ctx.xStatement
}

func (s *Server) executeQCFunction(funcIdx int) error {
	if s == nil || s.QCVM == nil {
		return nil
	}
	vm := s.QCVM
	ctx := captureQCExecutionContext(vm)
	prevNumEdicts := s.NumEdicts

	restoreContext := true
	defer func() {
		if restoreContext {
			restoreQCExecutionContext(vm, ctx)
		}
	}()

	// Install the trace hook unconditionally: it feeds the walkthrough
	// inspector's QC event ring (always) and the sv_debug_qc_trace telemetry
	// (only when the cvar is enabled, decided inside logQCTraceEvent).
	previousTraceCallFunc := vm.TraceCallFunc
	vm.TraceCallFunc = func(vm *qc.VM, event qc.TraceCallEvent) {
		if previousTraceCallFunc != nil {
			previousTraceCallFunc(vm, event)
		}
		s.recordQCTraceEvent(vm, event)
		s.logQCTraceEvent(vm, event)
	}
	defer func() {
		vm.TraceCallFunc = previousTraceCallFunc
	}()

	err := vm.ExecuteFunction(funcIdx)
	if err == nil {
		s.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
	}
	return err
}

func (s *Server) executeQCFunctionLeavingGlobals(funcIdx int) error {
	if s == nil || s.QCVM == nil {
		return nil
	}
	vm := s.QCVM
	prevNumEdicts := s.NumEdicts
	err := vm.ExecuteFunction(funcIdx)
	if err == nil {
		s.SyncSpawnedEdictsFromQCVM(prevNumEdicts)
	}
	return err
}

func (s *Server) logQCTraceEvent(vm *qc.VM, event qc.TraceCallEvent) {
	if s == nil || s.DebugTelemetry == nil || vm == nil {
		return
	}

	verbosity := 1
	if event.Phase == "builtin" {
		verbosity = 2
	}

	selfNum := int(vm.GInt(qc.OFSSelf))
	otherNum := int(vm.GInt(qc.OFSOther))
	selfEnt, selfEntNum := s.traceEntityForNum(selfNum)
	otherEnt, otherEntNum := s.traceEntityForNum(otherNum)

	msg := fmt.Sprintf("self=%d other=%d", selfEntNum, otherEntNum)
	if otherEnt != nil {
		msg = fmt.Sprintf("%s other_classname=%q", msg, qcString(vm, otherEnt.ClassName(s)))
	}

	s.DebugTelemetry.LogQCEventf(
		event.Phase,
		verbosity,
		event.Depth,
		event.FunctionIndex,
		vm,
		selfEntNum,
		selfEnt,
		"%s",
		msg,
	)
}

func (s *Server) traceEntityForNum(entNum int) (*Edict, int) {
	if s == nil || entNum < 0 || entNum >= s.NumEdicts {
		return nil, entNum
	}
	return s.EdictNum(entNum), entNum
}
