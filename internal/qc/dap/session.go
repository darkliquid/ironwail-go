package dap

import (
	"fmt"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Session tracks breakpoints, execution state, and variable inspection for a DAP client.
// Session tracks breakpoints, execution state, and variable inspection for a DAP client.
type Session struct {
	target      Target
	barrier     *Barrier
	mu          sync.Mutex
	funcBreaks  map[string]bool
	stmtBreaks  map[int]bool
	sourceMap   *qc.SourceMap
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

// ClearSourceBreakpoints removes statement breakpoints set via source lines
// (setBreakpoints). Function breakpoints are left intact, matching DAP
// semantics where each breakpoint kind is managed by its own request.
func (s *Session) ClearSourceBreakpoints() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stmtBreaks = make(map[int]bool)
}

// SetSourceMap attaches the compiled side-car source map so line breakpoints
// and stack frames can be mapped between progs statements and QuakeGo source.
// Safe to call again (e.g. after a progs reload); pass nil to detach.
func (s *Session) SetSourceMap(sm *qc.SourceMap) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sourceMap = sm
}

// SourceMap returns the attached source map, or nil.
func (s *Session) SourceMap() *qc.SourceMap {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sourceMap
}

// SetSourceBreakpoints resolves DAP source line breakpoints (a QuakeGo file
// plus 1-based lines) into progs statement breakpoints via the source map.
// Lines with no mapped statements produce an unverified breakpoint rather
// than an error, matching DAP client expectations.
func (s *Session) SetSourceBreakpoints(file string, lines []int) []Breakpoint {
	s.mu.Lock()
	defer s.mu.Unlock()

	sm := s.sourceMap
	var resolved [][]int // statements per requested line, parallel to out
	for _, line := range lines {
		var stmts []int
		if sm != nil {
			stmts = sm.StatementsForLine(qc.SourceFileKey(file), line)
		}
		resolved = append(resolved, stmts)
	}

	out := make([]Breakpoint, 0, len(lines))
	for i, stmts := range resolved {
		verified := len(stmts) > 0
		line := lines[i]
		if verified {
			// Arm every statement on the line; report the first as the
			// canonical breakpoint position.
			for _, stmt := range stmts {
				s.stmtBreaks[stmt] = true
			}
			line = stmts[0]
		}
		s.nextBPID++
		bp := Breakpoint{
			ID:       s.nextBPID,
			Verified: verified,
			Line:     line,
		}
		if !verified {
			bp.Message = "no executable code on this line"
		}
		if sm != nil {
			path := sm.ResolveSource(sm.SourceIndexForFile(file))
			if path != "" {
				bp.Source = &Source{Name: qc.SourceFileKey(file), Path: path}
			}
		}
		out = append(out, bp)
	}
	return out
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

// SetOnStopped sets the callback invoked when execution stops at a breakpoint or pause.
func (s *Session) SetOnStopped(fn func(reason string, threadID int)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.OnStopped = fn
}

// Variables returns the variable manager associated with this session.
func (s *Session) Variables() *VariableManager {
	return s.vars
}

// StackTrace constructs the current call stack. With a source map attached,
// each frame reports the QuakeGo source file/line its statement was lowered
// from and the raw statement index as the fallback line; without one, frames
// report raw statement indices (bytecode-level debugging).
func (s *Session) StackTrace() []StackFrame {
	if s.target == nil || s.target.VM() == nil {
		return nil
	}
	vm := s.target.VM()
	s.mu.Lock()
	sm := s.sourceMap
	s.mu.Unlock()

	frame := func(id int, name string, stmt int) StackFrame {
		f := StackFrame{
			ID:     id,
			Name:   name,
			Line:   stmt,
			Column: 1,
		}
		if m := sm.Lookup(stmt); m != nil && m.Source >= 0 {
			f.Line = m.Line
			f.Column = max(m.Col, 1)
			f.Source = &Source{
				Name: sm.SourceRelName(m.Source),
				Path: sm.ResolveSource(m.Source),
			}
		}
		return f
	}

	var frames []StackFrame

	// Top frame
	topName := "top"
	if vm.XFunction != nil {
		topName = vm.String(vm.XFunction.Name)
	}
	frames = append(frames, frame(0, topName, vm.XStatement))

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
		frames = append(frames, frame(len(frames), fnName, stk.S))
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
