package server

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/qc"
)

type qcExecutionContext struct {
	self           int32
	other          int32
	xFunction      *qc.DFunction
	xFunctionIndex int32
}

func captureQCExecutionContext(vm *qc.VM) qcExecutionContext {
	if vm == nil {
		return qcExecutionContext{}
	}
	hasGlobals := len(vm.Globals) > qc.OFSOther
	return qcExecutionContext{
		self: func() int32 {
			if hasGlobals {
				return vm.GInt(qc.OFSSelf)
			}
			return 0
		}(),
		other: func() int32 {
			if hasGlobals {
				return vm.GInt(qc.OFSOther)
			}
			return 0
		}(),
		xFunction:      vm.XFunction,
		xFunctionIndex: vm.XFunctionIndex,
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
	vm.XFunction = ctx.xFunction
	vm.XFunctionIndex = ctx.xFunctionIndex
}

func (s *Server) executeQCFunction(funcIdx int) error {
	if s == nil || s.QCVM == nil {
		return nil
	}
	snapshots := s.captureNonPusherQCVMEdictSnapshots()

	if s.DebugTelemetry == nil || !s.DebugTelemetry.QCTraceVerbosityEnabled(1) {
		err := s.QCVM.ExecuteFunction(funcIdx)
		if err == nil {
			s.syncMutatedNonPushersFromQCVM(snapshots)
		}
		return err
	}

	vm := s.QCVM
	previousTraceCallFunc := vm.TraceCallFunc
	vm.TraceCallFunc = func(vm *qc.VM, event qc.TraceCallEvent) {
		if previousTraceCallFunc != nil {
			previousTraceCallFunc(vm, event)
		}
		s.logQCTraceEvent(vm, event)
	}
	defer func() {
		vm.TraceCallFunc = previousTraceCallFunc
	}()

	err := vm.ExecuteFunction(funcIdx)
	if err == nil {
		s.syncMutatedNonPushersFromQCVM(snapshots)
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
	if otherEnt != nil && otherEnt.Vars != nil {
		msg = fmt.Sprintf("%s other_classname=%q", msg, qcString(vm, otherEnt.Vars.ClassName))
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
