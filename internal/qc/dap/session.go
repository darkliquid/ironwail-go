package dap

import (
	"fmt"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Session tracks breakpoints, execution state, and variable inspection for a DAP client.
type Session struct {
	target      Target
	barrier     *Barrier
	mu          sync.Mutex
	funcBreaks  map[string]bool
	stmtBreaks  map[int]bool
	vars        *VariableManager
	seq         int
	nextBPID    int
	OnStopped   func(reason string, threadID int)
	initialized bool
}

// NewSession constructs a Session.
func NewSession(target Target) *Session {
	return &Session{
		target:     target,
		barrier:    NewBarrier(),
		funcBreaks: make(map[string]bool),
		stmtBreaks: make(map[int]bool),
		vars:       NewVariableManager(target),
	}
}

// NextSeq generates an incremental sequence number for outgoing responses/events.
func (s *Session) NextSeq() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	return s.seq
}

// SetFunctionBreakpoint registers a function entry breakpoint.
func (s *Session) SetFunctionBreakpoint(name string) Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.funcBreaks[name] = true

	verified := false
	line := 0
	if s.target != nil && s.target.VM() != nil {
		vm := s.target.VM()
		fnIdx := vm.FindFunction(name)
		if fnIdx >= 0 && fnIdx < len(vm.Functions) && vm.Functions[fnIdx].FirstStatement >= 0 {
			verified = true
			line = int(vm.Functions[fnIdx].FirstStatement)
		}
	}
	s.nextBPID++
	return Breakpoint{ID: s.nextBPID, Verified: verified, Line: line}
}

// SetBreakpoint registers a statement index breakpoint.
func (s *Session) SetBreakpoint(stmtIdx int) Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stmtBreaks[stmtIdx] = true

	verified := false
	if s.target != nil && s.target.VM() != nil {
		verified = stmtIdx >= 0 && stmtIdx < len(s.target.VM().Statements)
	}
	s.nextBPID++
	return Breakpoint{ID: s.nextBPID, Verified: verified, Line: stmtIdx}
}

// ClearBreakpoints removes all breakpoints.
func (s *Session) ClearBreakpoints() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.funcBreaks = make(map[string]bool)
	s.stmtBreaks = make(map[int]bool)
}

// Continue resumes normal execution until next breakpoint.
func (s *Session) Continue() {
	s.barrier.Resume(modeContinue, 0)
}

// StepIn resumes execution and stops at the next statement.
func (s *Session) StepIn() {
	s.barrier.Resume(modeStepIn, 0)
}

// StepOver resumes execution and stops at the next statement at or above current depth.
func (s *Session) StepOver() {
	depth := 0
	if s.target != nil && s.target.VM() != nil {
		depth = s.target.VM().Depth
	}
	s.barrier.Resume(modeStepOver, depth)
}

// StepOut resumes execution and stops at depth - 1.
func (s *Session) StepOut() {
	depth := 0
	if s.target != nil && s.target.VM() != nil {
		depth = s.target.VM().Depth - 1
	}
	s.barrier.Resume(modeStepOut, depth)
}

// Pause requests an immediate pause at the next executed statement.
func (s *Session) Pause() {
	s.barrier.Resume(modePause, 0)
}

// Disconnect cleans up and unblocks the execution thread.
func (s *Session) Disconnect() {
	s.ClearBreakpoints()
	s.Continue()
}

// IsPaused returns true if execution is currently paused at the barrier.
func (s *Session) IsPaused() bool {
	return s.barrier.IsPaused()
}

// Initialized reports whether the session initialization handshake is complete.
func (s *Session) Initialized() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.initialized
}

// SetInitialized marks the session as having completed initialization.
func (s *Session) SetInitialized(v bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.initialized = v
}

// Variables returns the variable manager associated with this session.
func (s *Session) Variables() *VariableManager {
	return s.vars
}

// StackTrace constructs the current call stack.
func (s *Session) StackTrace() []StackFrame {
	if s.target == nil || s.target.VM() == nil {
		return nil
	}
	vm := s.target.VM()
	var frames []StackFrame

	// Top frame
	topName := "top"
	if vm.XFunction != nil {
		topName = vm.String(vm.XFunction.Name)
	}
	frames = append(frames, StackFrame{
		ID:     0,
		Name:   topName,
		Line:   vm.XStatement,
		Column: 1,
	})

	// Remaining stack frames
	start := min(vm.Depth-1, len(vm.Stack)-1)
	for i := start; i >= 0; i-- {
		stk := vm.Stack[i]
		fnName := fmt.Sprintf("fn_%d", stk.FuncIndex)
		if stk.Func != nil {
			fnName = vm.String(stk.Func.Name)
		} else if int(stk.FuncIndex) < len(vm.Functions) && stk.FuncIndex >= 0 {
			fnName = vm.String(vm.Functions[stk.FuncIndex].Name)
		}
		frames = append(frames, StackFrame{
			ID:     len(frames),
			Name:   fnName,
			Line:   stk.S,
			Column: 1,
		})
	}
	return frames
}

// BreakHook returns a QCVM statement hook callback wired to this session.
func (s *Session) BreakHook() func(vm *qc.VM, stmtIdx int) bool {
	return func(vm *qc.VM, stmtIdx int) bool {
		mode, targetDep := s.barrier.Mode()

		s.mu.Lock()
		funcBreaks := s.funcBreaks
		stmtBreaks := s.stmtBreaks
		onStopped := s.OnStopped
		s.mu.Unlock()

		stopReason := ""

		if mode == modePause {
			stopReason = "pause"
		} else if mode == modeStepIn {
			stopReason = "step"
		} else if mode == modeStepOver && vm.Depth <= targetDep {
			stopReason = "step"
		} else if mode == modeStepOut && vm.Depth <= targetDep {
			stopReason = "step"
		} else if stmtBreaks[stmtIdx] {
			stopReason = "breakpoint"
		} else if vm.XFunction != nil && funcBreaks[vm.String(vm.XFunction.Name)] && stmtIdx == int(vm.XFunction.FirstStatement) {
			stopReason = "breakpoint"
		}

		if stopReason != "" {
			s.barrier.Arm()
			if onStopped != nil {
				onStopped(stopReason, 1)
			}
			s.barrier.Wait()
		}

		return false
	}
}
