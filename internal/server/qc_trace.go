package server

import (
	"fmt"

	"github.com/ironwail/ironwail-go/internal/qc"
)

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
