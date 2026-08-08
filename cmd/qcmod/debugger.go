package main

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// Debugger implements the plan 25 Phase C statement debugger on top of the
// VM's existing per-statement TraceFunc hook. It is engine-agnostic: given a
// query API (function index + edict field readers) it can be used by the REPL
// and, later, the wasm walkthrough.
//
// State machine:
//
//	mode Continue   — run until a breakpoint hits
//	mode StepInto   — stop at the next statement (any depth)
//	mode StepOver   — stop after returning to the current depth
//	mode StepOut    — stop after returning to depth-1
//
// Watches are field expressions ("12.origin" = edict 12 field "origin") that
// re-evaluate every statement; a change is reported and optionally pauses.
type Debugger struct {
	// BreakFuncs is a set of function indices to break on entry.
	BreakFuncs map[int]bool

	// BreakStmts is a set of (function index, statement index) points.
	BreakStmts map[[2]int]bool

	// Watches are evaluated via the WatchEval hook every statement.
	Watches []string

	// WatchEval evaluates a watch expression; nil disables watches.
	WatchEval func(expr string) (any, error)

	mode    dbgMode
	stopAt  int // statement depth to stop at (StepOver/StepOut target)
	paused  bool
	hit     bool
	message string
}

type dbgMode int

const (
	dbgContinue dbgMode = iota
	dbgStepInto
	dbgStepOver
	dbgStepOut
)

// NewDebugger builds a debugger wired to the VM's TraceFunc.
func NewDebugger(vm *qc.VM) *Debugger {
	d := &Debugger{BreakFuncs: map[int]bool{}, BreakStmts: map[[2]int]bool{}}
	vm.Trace = true
	vm.TraceFunc = d.statement
	return d
}

// SetBreakFunc adds a function-index breakpoint (fires on function entry).
func (d *Debugger) SetBreakFunc(fn int) { d.BreakFuncs[fn] = true }

// SetBreakStmt adds a (function, statement) breakpoint.
func (d *Debugger) SetBreakStmt(fn, stmt int) { d.BreakStmts[[2]int{fn, stmt}] = true }

// Continue resumes execution until the next breakpoint (or step target).
func (d *Debugger) Continue() { d.mode = dbgContinue; d.hit = false }

// StepInto stops at the very next statement.
func (d *Debugger) StepInto() { d.mode = dbgStepInto; d.hit = false }

// StepOver stops after returning to the current depth.
func (d *Debugger) StepOver(depth int) { d.mode = dbgStepOver; d.stopAt = depth; d.hit = false }

// StepOut stops after returning one level up.
func (d *Debugger) StepOut(depth int) { d.mode = dbgStepOut; d.stopAt = depth - 1; d.hit = false }

// Paused reports whether the debugger has halted execution at a breakpoint.
func (d *Debugger) Paused() bool { return d.paused }

// Message returns the last halt reason.
func (d *Debugger) Message() string { return d.message }

// hook returns a BreakHook that the REPL installs as vm.BreakHook: it calls
// the per-statement decision and returns true to pause the VM (ErrBreak).
// nil-safe so a debugger with no breakpoints/steps/watches does not pause.
func (d *Debugger) hook() func(vm *qc.VM, stmtIdx int) bool {
	if len(d.BreakFuncs) == 0 && len(d.BreakStmts) == 0 && d.mode == dbgContinue && len(d.Watches) == 0 {
		return nil
	}
	return func(vm *qc.VM, stmtIdx int) bool {
		d.statement(vm, stmtIdx, nil, 0)
		return d.paused
	}
}

// statement is the per-statement TraceFunc hook. It decides "should we stop".
func (d *Debugger) statement(vm *qc.VM, stmtIdx int, st *qc.DStatement, op qc.Opcode) {
	// Function entry breakpoints: fire at the function's FIRST statement,
	// but not while stepping (step controls the pause; an entry break at the
	// same statement would loop forever).
	stepping := d.mode == dbgStepInto || d.mode == dbgStepOver || d.mode == dbgStepOut
	if !stepping && vm.XFunction != nil && len(d.BreakFuncs) > 0 {
		if d.BreakFuncs[int(vm.XFunctionIndex)] && stmtIdx == int(vm.XFunction.FirstStatement) {
			d.hit = true
			d.paused = true
			d.message = fmt.Sprintf("break at function %d", vm.XFunctionIndex)
			return
		}
	}
	if d.BreakStmts[[2]int{int(vm.XFunctionIndex), stmtIdx}] {
		d.hit = true
		d.paused = true
		d.message = fmt.Sprintf("break at %d:%d", vm.XFunctionIndex, stmtIdx)
		return
	}
	switch d.mode {
	case dbgStepInto:
		d.hit = true
		d.paused = true
		d.message = "step"
	case dbgStepOver:
		if vm.Depth <= d.stopAt {
			d.hit = true
			d.paused = true
			d.message = "step over"
		}
	case dbgStepOut:
		if vm.Depth <= d.stopAt {
			d.hit = true
			d.paused = true
			d.message = "step out"
		}
	}

	if d.WatchEval != nil && len(d.Watches) > 0 {
		for _, expr := range d.Watches {
			if v, err := d.WatchEval(expr); err == nil {
				d.message = fmt.Sprintf("watch %s = %v", expr, v)
			}
		}
	}
}

// Reset clears all breakpoints/watches and resumes.
func (d *Debugger) Reset() {
	d.BreakFuncs = map[int]bool{}
	d.BreakStmts = map[[2]int]bool{}
	d.Watches = nil
	d.mode = dbgContinue
	d.paused = false
	d.hit = false
	d.message = ""
}
