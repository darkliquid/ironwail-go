// Command qcmod is the standalone QuakeGo/QCVM mod-development toolkit
// (plan 25). It lets mod authors write and run deterministic tests for
// QuakeC/QuakeGo gameplay code WITHOUT booting the full engine.
//
// Current commands:
//
//	qcmod test <moddir>   — run In-Go mod tests (wraps `go test` on the mod
//	                         directory; mod sources import "quake",
//	                         "quake/engine", and "quake/sim").
//	qcmod vm <fn> [self [other [time]]]
//	                       — In-VM runner: boot compiled progs, fire a real
//	                         QC bytecode function with globals set (plan 25
//	                         Phase B).
//	qcmod sim <moddir>    — interactive headless REPL (WIP; see plan 25.6).
//	qcmod disasm [progs]  — disassemble progs.dat bytecode (default: compile
//	                         pkg/qgo/quakego); -func <name> filters to one
//	                         function, -o <path> writes to a file.
//	qcmod dap [addr]      — start standalone DAP debug server.
//	qcmod docs            — print this guide.
//
// In-Go mode runs mod functions natively (via quake/engine's Backend wired by
// sim.World), so a mod test is ordinary Go: spawn entities, fire thinks,
// assert fields. No progs.dat, no GPU, no assets.
//
// Usage requires the quake module (pkg/qgo/quake) to be resolvable from the
// mod's go.mod, exactly like pkg/qgo/quakego does with its `replace quake =>
// ../quake` directive.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 2
	}
	switch args[0] {
	case "test":
		return runTest(args[1:], stdout, stderr)
	case "vm", "run-vm":
		return runVM(args[1:], stdout, stderr)
	case "sim":
		return runREPL(os.Stdin, stdout)
	case "disasm":
		return runDisasm(args[1:], stdout, stderr)
	case "qc2go":
		return runQC2Go(args[1:], stdout, stderr)
	case "dap":
		return runDAP(args[1:], stdout, stderr)
	case "docs", "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		_, _ = fmt.Fprintf(stderr, "qcmod: unknown command %q\n\n", args[0])
		printUsage(stdout)
		return 2
	}
}

// runVM implements `qcmod vm <function> [self [other [time]]]`: boots the
// In-VM runner (plan 25 Phase B) against the compiled progs, sets globals,
// fires the named QC function, and prints the resulting globals/edict count.
// This is the headless bytecode entry point that the REPL and the walkthrough
// will later build on.
func runVM(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod vm: missing <function>")
		return 2
	}
	// Optional numeric args: self other time.
	var self, other int
	var time float32
	if len(args) > 1 {
		if _, err := fmt.Sscanf(args[1], "%d", &self); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod vm: bad self %q\n", args[1])
			return 2
		}
	}
	if len(args) > 2 {
		if _, err := fmt.Sscanf(args[2], "%d", &other); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod vm: bad other %q\n", args[2])
			return 2
		}
	}
	if len(args) > 3 {
		if _, err := fmt.Sscanf(args[3], "%f", &time); err != nil {
			_, _ = fmt.Fprintf(stderr, "qcmod vm: bad time %q\n", args[3])
			return 2
		}
	}

	w, err := newVMWorld(nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod vm: %v\n", err)
		return 1
	}
	fn := w.vm.FindFunction(args[0])
	if fn < 0 {
		_, _ = fmt.Fprintf(stderr, "qcmod vm: no function %q in progs\n", args[0])
		return 1
	}
	spawned, err := w.fire(fn, self, other, time)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod vm: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "ok %s self=%d other=%d time=%.3f spawned=%d\n", args[0], self, other, time, spawned)
	return 0
}

func printUsage(w io.Writer) {
	// io.WriteString avoids vet's printf check (the usage block contains a
	// literal %v from the example test code).
	_, _ = io.WriteString(w, `qcmod — standalone QuakeGo/QCVM mod dev kit (plan 25)

Commands:
  qcmod test <moddir>   run In-Go mod tests (wraps go test on the mod dir)
  qcmod vm <fn> [self [other [time]]]
                        boot compiled progs + fire a QC bytecode function
  qcmod sim <moddir>    interactive headless REPL (WIP)
  qcmod dap [addr]      start standalone DAP debug server
  qcmod docs            print this guide

In-Go mode: mod tests import "quake", "quake/engine", and "quake/sim";
sim.New() builds a deterministic world whose engine.Backend records builtin
side effects, so you can spawn entities, fire Think/Touch/Use closures, and
assert fields with plain Go — no progs.dat, no GPU, no assets.

Example mod test (door chain):

    package mymod

    import (
        "testing"
        "quake"
        "quake/sim"
    )

    func TestDoorSchedulesMove(t *testing.T) {
        w := sim.New()
        door := w.Spawn("func_door")
        door.Think = func() {
            door.Velocity = quake.MakeVec3(0, 0, 100)
            door.NextThink = w.Time + 1.0
        }
        if err := w.Fire(door, nil, door.Think); err != nil {
            t.Fatal(err)
        }
        if door.NextThink != w.Time+1.0 {
            t.Fatalf("nextthink = %v", door.NextThink)
        }
    }

The mod dir must have a go.mod that resolves the quake module (e.g. a
replace directive pointing at pkg/qgo/quake), like pkg/qgo/quakego does.
`)
}

func runTest(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		_, _ = fmt.Fprintln(stderr, "qcmod test: missing <moddir>")
		return 2
	}
	dir := args[0]
	abs, err := filepath.Abs(dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod test: %v\n", err)
		return 1
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod test: %s has no go.mod (mod dirs are Go modules importing quake/quake/sim)\n", abs)
		return 1
	}

	// Run `go test ./...` inside the mod dir so mod tests behave exactly
	// like normal Go tests (table-driven, parallel, subtests). GOWORK=off
	// isolates the mod from the repo root's go.work (the mod is an
	// independent module resolving quake via its own replace directive).
	cmd := exec.Command("go", "test", "./...", "-count=1")
	cmd.Dir = abs
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOWORK=off")
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "qcmod test: OK (%s)\n", abs)
	return 0
}
