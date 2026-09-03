package dap

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

// sourceMapTestVM builds a two-function VM whose statements are the ones the
// test source map maps, plus the map itself (matching what qgo emits for a
// two-file QuakeGo mod).
func sourceMapTestVM() (*qc.VM, *qc.SourceMap) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: 0},
		{Name: vm.AllocString("multi_trigger"), FirstStatement: 10},
	}
	vm.Statements = make([]qc.DStatement, 20)

	sm := &qc.SourceMap{
		Version:    qc.SourceMapVersion,
		File:       "progs.dat",
		SourceRoot: "/mods/testmod",
		Sources:    []string{"triggers.go", "client.qc.go"},
		Mappings: []qc.SourceMapping{
			{Stmt: 0, Source: -1}, // sentinel
			{Stmt: 10, Source: 0, Line: 45, Col: 2, Func: "multi_trigger"},
			{Stmt: 11, Source: 0, Line: 46, Col: 2, Func: "multi_trigger"},
			{Stmt: 12, Source: 0, Line: 47, Col: 9, Func: "multi_trigger"},
			{Stmt: 15, Source: 1, Line: 100, Col: 2, Func: "ClientConnect"},
		},
	}
	return vm, sm
}

func TestSetSourceBreakpointsResolveLinesToStatements(t *testing.T) {
	vm, sm := sourceMapTestVM()
	session := NewSession(&mockTarget{vm: vm})
	session.SetSourceMap(sm)

	bps := session.SetSourceBreakpoints("/mods/testmod/triggers.go", []int{46, 999})
	if len(bps) != 2 {
		t.Fatalf("breakpoints = %d, want 2", len(bps))
	}
	if !bps[0].Verified {
		t.Errorf("line 46 breakpoint not verified")
	}
	if bps[0].Line != 11 {
		t.Errorf("line 46 resolved to statement %d, want 11", bps[0].Line)
	}
	if bps[1].Verified {
		t.Errorf("line 999 breakpoint should be unverified (no mapped statements)")
	}
	if bps[1].Message == "" {
		t.Errorf("unverified breakpoint should carry an explanation")
	}

	// Arming: the BreakHook must stop on statement 11 (line 46's statement).
	// The hook blocks on the barrier when it stops, so it runs on a goroutine
	// and the test resumes it via Continue (the same pattern the existing
	// session tests use).
	hook := session.BreakHook()
	vm.XFunction = &vm.Functions[1]
	vm.XFunctionIndex = 1
	vm.Depth = 1

	stopReason := make(chan string, 4)
	session.SetOnStopped(func(reason string, threadID int) {
		stopReason <- reason
	})

	go func() {
		hook(vm, 11)     // blocks until resumed
		stopReason <- "" // hook returned: execution resumed
	}()

	if reason := <-stopReason; reason != "breakpoint" {
		t.Errorf("expected stop at stmt 11 with reason 'breakpoint', got %q", reason)
	}
	session.Continue()
	if reason := <-stopReason; reason != "" {
		t.Errorf("hook returned while still stopped: %q", reason)
	}

	// Statement 12 has no breakpoint: the hook must return without blocking
	// or firing OnStopped. (The hook always returns false — halting is
	// signalled by blocking on the barrier, not by the return value.)
	select {
	case reason := <-stopReason:
		t.Errorf("unexpected stop at stmt 12 (no breakpoint): %q", reason)
	default:
	}
	hook(vm, 12)
	select {
	case reason := <-stopReason:
		t.Errorf("unexpected stop at stmt 12 (no breakpoint): %q", reason)
	default:
	}
}

func TestSetSourceBreakpointsWithoutMapAreUnverified(t *testing.T) {
	vm, _ := sourceMapTestVM()
	session := NewSession(&mockTarget{vm: vm})

	bps := session.SetSourceBreakpoints("triggers.go", []int{46})
	if len(bps) != 1 {
		t.Fatalf("breakpoints = %d, want 1", len(bps))
	}
	if bps[0].Verified {
		t.Error("breakpoint should be unverified without a source map")
	}
}

func TestClearSourceBreakpointsKeepsFunctionBreakpoints(t *testing.T) {
	vm, sm := sourceMapTestVM()
	session := NewSession(&mockTarget{vm: vm})
	session.SetSourceMap(sm)

	fbp := session.SetFunctionBreakpoint("multi_trigger")
	if !fbp.Verified {
		t.Fatal("function breakpoint not verified")
	}
	sbps := session.SetSourceBreakpoints("triggers.go", []int{46})
	if !sbps[0].Verified {
		t.Fatal("source breakpoint not verified")
	}

	session.ClearSourceBreakpoints()

	hook := session.BreakHook()
	vm.XFunction = &vm.Functions[1]
	vm.XFunctionIndex = 1
	vm.Depth = 1

	// The function breakpoint (first statement of multi_trigger, stmt 10)
	// must still fire after clearing source breakpoints.
	stopReason := make(chan string, 4)
	session.SetOnStopped(func(reason string, threadID int) {
		stopReason <- reason
	})
	go func() {
		hook(vm, 10)
		stopReason <- ""
	}()
	if reason := <-stopReason; reason != "breakpoint" {
		t.Errorf("function breakpoint lost after ClearSourceBreakpoints: got %q", reason)
	}
	session.Continue()
	<-stopReason

	// Source line breakpoint must be gone: no stop at stmt 11.
	hook(vm, 11)
	select {
	case reason := <-stopReason:
		t.Errorf("source breakpoint survived ClearSourceBreakpoints: %q", reason)
	default:
	}
}

func TestStackTraceReportsQuakeGoSourceLocations(t *testing.T) {
	vm, sm := sourceMapTestVM()
	session := NewSession(&mockTarget{vm: vm})
	session.SetSourceMap(sm)

	// Pause inside multi_trigger at statement 12.
	vm.XFunction = &vm.Functions[1]
	vm.XFunctionIndex = 1
	vm.Depth = 1
	vm.XStatement = 12

	frames := session.StackTrace()
	if len(frames) == 0 {
		t.Fatal("no stack frames")
	}
	top := frames[0]
	if top.Source == nil {
		t.Fatal("top frame missing Source")
	}
	if top.Line != 47 {
		t.Errorf("top frame line = %d, want 47 (stmt 12 maps to triggers.go:47)", top.Line)
	}
	if top.Column != 9 {
		t.Errorf("top frame column = %d, want 9", top.Column)
	}
	if top.Source.Name != "triggers.go" {
		t.Errorf("source name = %q, want triggers.go", top.Source.Name)
	}
	if top.Source.Path != "/mods/testmod/triggers.go" {
		t.Errorf("source path = %q, want SourceRoot joined with the relative source", top.Source.Path)
	}
}

func TestStackTraceWithoutMapReportsStatementIndices(t *testing.T) {
	vm, _ := sourceMapTestVM()
	session := NewSession(&mockTarget{vm: vm})

	vm.XFunction = &vm.Functions[1]
	vm.XFunctionIndex = 1
	vm.Depth = 1
	vm.XStatement = 12

	frames := session.StackTrace()
	if len(frames) == 0 {
		t.Fatal("no stack frames")
	}
	if frames[0].Source != nil {
		t.Errorf("unexpected Source without a source map: %+v", frames[0].Source)
	}
	if frames[0].Line != 12 {
		t.Errorf("line = %d, want raw statement index 12", frames[0].Line)
	}
}

func TestSourceMapFileKeyMatchesClientPaths(t *testing.T) {
	// DAP clients send absolute or workspace-relative paths; the map stores
	// mod-dir-relative names. Base-name matching must bridge that gap.
	vm, sm := sourceMapTestVM()
	session := NewSession(&mockTarget{vm: vm})
	session.SetSourceMap(sm)

	if sm.SourceIndexForFile("/home/dev/mods/testmod/triggers.go") != 0 {
		t.Error("absolute client path should match by base name")
	}

	bps := session.SetSourceBreakpoints("/home/dev/mods/testmod/client.qc.go", []int{100})
	if len(bps) != 1 || !bps[0].Verified {
		t.Fatalf("absolute-path breakpoint not resolved: %+v", bps)
	}
	if !strings.HasPrefix(bps[0].Source.Path, "/mods/testmod/") {
		t.Errorf("resolved path = %q, want under SourceRoot", bps[0].Source.Path)
	}
}
