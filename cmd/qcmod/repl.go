package main

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// runREPL implements `qcmod sim` — the plan 25 Phase D interactive headless
// debugger/REPL. It boots the In-VM world and reads commands from r:
//
//	run <fn> [self [other [time]]]  — fire a function (breakpoints armed)
//	break <fn>                      — toggle a function-entry breakpoint
//	step                             — single-step the current (paused) frame
//	cont                             — continue until the next break
//	watch <n>.<field>                — toggle a field watch (prints on pause)
//	inspect <n>                      — dump an edict's key fields
//	globals                          — print key QC globals
//	functions [prefix]               — list functions matching a prefix
//	reset                            — clear breaks/watches
//	quit                             — exit
//
// When a breakpoint/step fires, `run` returns ErrBreak with the stack live;
// `step`/`cont` resume via ExecuteFrom. This is the same state machine the
// wasm walkthrough's QuakeC panel will drive (plan 22).
func runREPL(r io.Reader, w io.Writer) int {
	vm, err := newVMWorld(nil)
	if err != nil {
		fmt.Fprintf(w, "qcmod sim: %v\n", err)
		return 1
	}
	dbg := NewDebugger(vm.vm)
	dbg.Continue()
	vm.vm.BreakHook = dbg.hook()

	fmt.Fprintf(w, "qcmod sim — plan 25 REPL. Type 'help' for commands.\n")
	sc := bufio.NewScanner(r)
	for {
		if dbg.Paused() {
			fmt.Fprintf(w, "(%s) qc> ", dbg.Message())
		} else {
			fmt.Fprintf(w, "qc> ")
		}
		if !sc.Scan() {
			if err := sc.Err(); err != nil && err != io.EOF {
				fmt.Fprintf(w, "read error: %v\n", err)
			}
			break // EOF
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		cmd, rest := fields[0], fields[1:]
		switch cmd {
		case "help", "?":
			fmt.Fprintf(w, "commands: run <fn> [self [other [time]]], break <fn>, step, cont, watch <n>.<field>, inspect <n>, globals, functions [prefix], reset, quit\n")
		case "quit", "exit":
			return 0
		case "reset":
			dbg.Reset()
			vm.vm.BreakHook = dbg.hook()
			fmt.Fprintf(w, "debugger reset\n")
		case "run":
			if len(rest) < 1 {
				fmt.Fprintf(w, "usage: run <fn> [self [other [time]]]\n")
				continue
			}
			fnIdx := vm.vm.FindFunction(rest[0])
			if fnIdx < 0 {
				fmt.Fprintf(w, "no function %q\n", rest[0])
				continue
			}
			self, other := 0, 0
			var t float32
			if len(rest) > 1 {
				self, _ = strconv.Atoi(rest[1])
			}
			if len(rest) > 2 {
				other, _ = strconv.Atoi(rest[2])
			}
			if len(rest) > 3 {
				f, _ := strconv.ParseFloat(rest[3], 32)
				t = float32(f)
			}
			vm.vm.SetGInt(qc.OFSSelf, int32(self))
			vm.vm.SetGInt(qc.OFSOther, int32(other))
			vm.vm.SetGInt(qc.OFSTime, int32(math.Float32bits(t)))
			err := vm.vm.ExecuteFunction(fnIdx)
			if err == qc.ErrBreak {
				fmt.Fprintf(w, "paused: %s (depth %d)\n", dbg.Message(), vm.vm.Depth)
				continue
			}
			if err != nil {
				fmt.Fprintf(w, "error: %v\n", err)
				continue
			}
			fmt.Fprintf(w, "ok: %s completed (depth %d)\n", rest[0], vm.vm.Depth)
		case "break":
			if len(rest) < 1 {
				fmt.Fprintf(w, "usage: break <fn>\n")
				continue
			}
			fnIdx := vm.vm.FindFunction(rest[0])
			if fnIdx < 0 {
				fmt.Fprintf(w, "no function %q\n", rest[0])
				continue
			}
			if dbg.BreakFuncs[fnIdx] {
				delete(dbg.BreakFuncs, fnIdx)
				fmt.Fprintf(w, "break removed: %s\n", rest[0])
			} else {
				dbg.SetBreakFunc(fnIdx)
				fmt.Fprintf(w, "break set: %s (%d)\n", rest[0], fnIdx)
			}
			vm.vm.BreakHook = dbg.hook()
		case "step":
			if vm.vm.Depth <= 0 {
				fmt.Fprintf(w, "nothing to step (no active frame)\n")
				continue
			}
			dbg.StepOver(vm.vm.Depth)
			vm.vm.BreakHook = dbg.hook()
			err := vm.vm.ExecuteFrom(currentFnIdx(vm.vm))
			if err == qc.ErrBreak {
				fmt.Fprintf(w, "step: %s (depth %d)\n", dbg.Message(), vm.vm.Depth)
			} else if err != nil {
				fmt.Fprintf(w, "error: %v\n", err)
			} else {
				fmt.Fprintf(w, "step completed (frame returned)\n")
				dbg.paused = false
			}
		case "cont":
			if vm.vm.Depth <= 0 {
				fmt.Fprintf(w, "nothing to continue (no paused frame)\n")
				continue
			}
			dbg.Continue()
			vm.vm.BreakHook = dbg.hook()
			err := vm.vm.ExecuteFrom(currentFnIdx(vm.vm))
			if err == qc.ErrBreak {
				fmt.Fprintf(w, "paused: %s\n", dbg.Message())
			} else if err != nil {
				fmt.Fprintf(w, "error: %v\n", err)
			} else {
				fmt.Fprintf(w, "completed\n")
				dbg.paused = false
			}
		case "watch":
			if len(rest) < 1 {
				fmt.Fprintf(w, "usage: watch <n>.<field>\n")
				continue
			}
			dbg.Watches = append(dbg.Watches, rest[0])
			vm.vm.BreakHook = dbg.hook()
			fmt.Fprintf(w, "watch set: %s\n", rest[0])
		case "inspect":
			if len(rest) < 1 {
				fmt.Fprintf(w, "usage: inspect <n>\n")
				continue
			}
			n, _ := strconv.Atoi(rest[0])
			fmt.Fprintf(w, "edict %d:\n", n)
			for name, ofs := range map[string]int{
				"origin": qc.EntFieldOrigin, "velocity": qc.EntFieldVelocity,
				"angles": qc.EntFieldAngles, "health": qc.EntFieldHealth,
				"nextthink": qc.EntFieldNextThink, "ltime": qc.EntFieldLTime,
				"frame": qc.EntFieldFrame, "solid": qc.EntFieldSolid,
				"movetype": qc.EntFieldMoveType,
			} {
				fmt.Fprintf(w, "  %s = %v\n", name, vm.vm.EFloat(n, ofs))
			}
		case "globals":
			for _, g := range []string{"self", "other", "time", "world"} {
				if ofs := vm.vm.FindGlobal(g); ofs >= 0 {
					fmt.Fprintf(w, "  %s = %v\n", g, vm.vm.Globals[ofs])
				}
			}
		case "functions":
			prefix := ""
			if len(rest) > 0 {
				prefix = rest[0]
			}
			n := 0
			for i, f := range vm.vm.Functions {
				name := vm.vm.String(f.Name)
				if strings.HasPrefix(name, prefix) {
					fmt.Fprintf(w, "%4d %s\n", i, name)
					n++
					if n >= 40 {
						fmt.Fprintf(w, "...\n")
						break
					}
				}
			}
			if n == 0 {
				fmt.Fprintf(w, "no functions matching %q\n", prefix)
			}
		default:
			fmt.Fprintf(w, "unknown command %q (try 'help')\n", cmd)
		}
	}
	return 0
}

// currentFnIdx finds the array index of the VM's current (paused) function.
func currentFnIdx(vm *qc.VM) int {
	if vm == nil || vm.XFunction == nil {
		return -1
	}
	for i := range vm.Functions {
		if &vm.Functions[i] == vm.XFunction {
			return i
		}
	}
	return -1
}
