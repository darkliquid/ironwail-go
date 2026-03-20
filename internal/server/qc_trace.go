package server

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/qc"
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
	ctx := captureQCExecutionContext(s.QCVM)
	snapshots := s.captureNonPusherQCVMEdictSnapshots()

	if s.DebugTelemetry == nil || !s.DebugTelemetry.QCTraceVerbosityEnabled(1) {
		err := s.QCVM.ExecuteFunction(funcIdx)
		if err == nil {
			s.syncMutatedNonPushersFromQCVM(snapshots)
		} else {
			restoreQCExecutionContext(s.QCVM, ctx)
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
	} else {
		restoreQCExecutionContext(vm, ctx)
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
