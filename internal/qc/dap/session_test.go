package dap

import (
	"sync"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestSessionBreakAndStep(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("test_func"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

	target := &mockTarget{vm: vm}
	session := NewSession(target)

	// Set breakpoint on function test_func
	bp := session.SetFunctionBreakpoint("test_func")
	if !bp.Verified {
		t.Fatalf("Expected function breakpoint to be verified")
	}

	var stoppedReason string
	var stopMu sync.Mutex
	session.OnStopped = func(reason string, threadID int) {
		stopMu.Lock()
		stoppedReason = reason
		stopMu.Unlock()
	}

	hook := session.BreakHook()

	// Simulate VM entering statement 0
	vm.XFunction = &vm.Functions[0]
	vm.XFunctionIndex = 0
	vm.Depth = 1

	go func() {
		// Wait for pause
		for {
			stopMu.Lock()
			r := stoppedReason
			stopMu.Unlock()
			if r != "" {
				break
			}
			time.Sleep(1 * time.Millisecond)
		}
		// Send StepOver
		session.StepOver()
	}()

	// Hook must block until StepOver, then return
	hook(vm, 0)

	stopMu.Lock()
	r := stoppedReason
	stopMu.Unlock()
	if r != "breakpoint" {
		t.Fatalf("Expected stopped reason 'breakpoint', got %q", r)
	}
}

func TestSessionBreakpointsManagement(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("known_func"), FirstStatement: 10},
		{Name: vm.AllocString("builtin_func"), FirstStatement: -1},
	}
	vm.Statements = make([]qc.DStatement, 50)

	target := &mockTarget{vm: vm}
	session := NewSession(target)

	// Known function breakpoint -> verified
	bp1 := session.SetFunctionBreakpoint("known_func")
	if !bp1.Verified || bp1.Line != 10 || bp1.ID != 1 {
		t.Fatalf("Expected verified function breakpoint at line 10 with ID 1, got %+v", bp1)
	}

	// Unknown function breakpoint -> not verified
	bp2 := session.SetFunctionBreakpoint("unknown_func")
	if bp2.Verified || bp2.ID != 2 {
		t.Fatalf("Expected unverified function breakpoint with ID 2, got %+v", bp2)
	}

	// Builtin function (FirstStatement < 0) -> not verified
	bpBuiltin := session.SetFunctionBreakpoint("builtin_func")
	if bpBuiltin.Verified || bpBuiltin.ID != 3 {
		t.Fatalf("Expected unverified builtin function breakpoint with ID 3, got %+v", bpBuiltin)
	}

	// Valid statement breakpoint -> verified
	bp3 := session.SetBreakpoint(25)
	if !bp3.Verified || bp3.Line != 25 || bp3.ID != 4 {
		t.Fatalf("Expected verified statement breakpoint at line 25 with ID 4, got %+v", bp3)
	}

	// Invalid statement breakpoint -> not verified
	bp4 := session.SetBreakpoint(999)
	if bp4.Verified || bp4.ID != 5 {
		t.Fatalf("Expected unverified statement breakpoint for out of bounds line with ID 5, got %+v", bp4)
	}

	// Clear breakpoints
	session.ClearBreakpoints()
	if len(session.funcBreaks) != 0 || len(session.stmtBreaks) != 0 {
		t.Fatalf("Expected all breakpoints cleared, got func=%d stmt=%d", len(session.funcBreaks), len(session.stmtBreaks))
	}

	// Monotonic ID counter persists across clear
	bp5 := session.SetBreakpoint(5)
	if bp5.ID != 6 {
		t.Fatalf("Expected monotonic breakpoint ID 6 after clear, got %d", bp5.ID)
	}
}

func TestSessionSteppingModes(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("main"), FirstStatement: 0},
		{Name: vm.AllocString("callee"), FirstStatement: 10},
	}
	vm.Statements = make([]qc.DStatement, 30)

	target := &mockTarget{vm: vm}
	session := NewSession(target)

	var stoppedReasons []string
	var stopMu sync.Mutex
	session.OnStopped = func(reason string, threadID int) {
		stopMu.Lock()
		stoppedReasons = append(stoppedReasons, reason)
		stopMu.Unlock()
	}

	hook := session.BreakHook()
	vm.XFunction = &vm.Functions[0]
	vm.Depth = 1

	// 1. StepIn
	session.StepIn()
	done := make(chan struct{})
	go func() {
		hook(vm, 0)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Hook should have paused on StepIn")
	case <-time.After(20 * time.Millisecond):
		if !session.IsPaused() {
			t.Fatal("Expected session to be paused")
		}
	}

	// Resume with StepOver at current depth 1
	session.StepOver()
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Hook did not resume after StepOver")
	}

	// 2. StepOver when depth is deeper (depth = 2 > targetDep = 1) -> should NOT stop
	vm.Depth = 2
	hook(vm, 1) // Should return false immediately without pausing

	// 3. StepOver when depth returns to 1 (depth <= targetDep = 1) -> should stop
	vm.Depth = 1
	done2 := make(chan struct{})
	go func() {
		hook(vm, 2)
		close(done2)
	}()

	select {
	case <-done2:
		t.Fatal("Hook should have paused on StepOver at depth 1")
	case <-time.After(20 * time.Millisecond):
		if !session.IsPaused() {
			t.Fatal("Expected session to be paused")
		}
	}

	// 4. StepOut at depth 1 -> targetDep = 0
	session.StepOut()
	select {
	case <-done2:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Hook did not resume after StepOut")
	}

	// At depth 1 > 0 -> should not stop
	vm.Depth = 1
	hook(vm, 3)

	// At depth 0 <= 0 -> should stop
	vm.Depth = 0
	done3 := make(chan struct{})
	go func() {
		hook(vm, 4)
		close(done3)
	}()

	select {
	case <-done3:
		t.Fatal("Hook should have paused on StepOut at depth 0")
	case <-time.After(20 * time.Millisecond):
		if !session.IsPaused() {
			t.Fatal("Expected session to be paused")
		}
	}

	// 5. Continue
	session.Continue()
	select {
	case <-done3:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Hook did not resume after Continue")
	}

	// Further statements without breakpoints should not pause
	hook(vm, 5)
	hook(vm, 6)

	stopMu.Lock()
	count := len(stoppedReasons)
	stopMu.Unlock()
	if count != 3 {
		t.Fatalf("Expected 3 stop events, got %d: %v", count, stoppedReasons)
	}
}

func TestSessionPauseAndDisconnect(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("worker"), FirstStatement: 0},
	}
	vm.Statements = make([]qc.DStatement, 10)

	target := &mockTarget{vm: vm}
	session := NewSession(target)

	var lastReason string
	var mu sync.Mutex
	session.OnStopped = func(reason string, threadID int) {
		mu.Lock()
		lastReason = reason
		mu.Unlock()
	}

	hook := session.BreakHook()
	vm.XFunction = &vm.Functions[0]

	// Request pause while running
	session.Pause()

	done := make(chan struct{})
	go func() {
		hook(vm, 1)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Expected hook to block on Pause")
	case <-time.After(20 * time.Millisecond):
		mu.Lock()
		r := lastReason
		mu.Unlock()
		if r != "pause" {
			t.Fatalf("Expected stop reason 'pause', got %q", r)
		}
	}

	// Test Disconnect unblocks and clears breakpoints
	session.SetBreakpoint(2)
	session.Disconnect()

	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Disconnect did not unblock paused session")
	}

	// Ensure breakpoints were cleared on disconnect
	if len(session.stmtBreaks) != 0 {
		t.Fatalf("Expected statement breakpoints to be empty after disconnect")
	}

	// Subsequent hook at stmt 2 should not stop
	hook(vm, 2)
}

func TestSessionStackTrace(t *testing.T) {
	// Nil target / nil VM
	nilSession := NewSession(nil)
	if frames := nilSession.StackTrace(); frames != nil {
		t.Fatalf("Expected nil stack frames for nil target, got %+v", frames)
	}

	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("main"), FirstStatement: 0},
		{Name: vm.AllocString("foo"), FirstStatement: 10},
		{Name: vm.AllocString("bar"), FirstStatement: 20},
	}
	vm.Statements = make([]qc.DStatement, 30)

	target := &mockTarget{vm: vm}
	session := NewSession(target)

	// Single top frame (Depth = 0)
	vm.XFunction = &vm.Functions[2]
	vm.XStatement = 22
	vm.Depth = 0

	frames := session.StackTrace()
	if len(frames) != 1 {
		t.Fatalf("Expected 1 frame, got %d", len(frames))
	}
	if frames[0].Name != "bar" || frames[0].Line != 22 {
		t.Fatalf("Frame 0 mismatch: %+v", frames[0])
	}

	// Multi-frame call stack: main -> foo -> bar
	vm.Depth = 2
	vm.Stack[0] = qc.PRStack{
		S:         3,
		Func:      &vm.Functions[0],
		FuncIndex: 0,
	}
	vm.Stack[1] = qc.PRStack{
		S:         15,
		Func:      &vm.Functions[1],
		FuncIndex: 1,
	}

	frames = session.StackTrace()
	if len(frames) != 3 {
		t.Fatalf("Expected 3 frames, got %d: %+v", len(frames), frames)
	}

	// Top frame (bar at 22)
	if frames[0].Name != "bar" || frames[0].Line != 22 || frames[0].ID != 0 {
		t.Errorf("Frame 0 mismatch: %+v", frames[0])
	}
	// Middle frame (foo at 15)
	if frames[1].Name != "foo" || frames[1].Line != 15 || frames[1].ID != 1 {
		t.Errorf("Frame 1 mismatch: %+v", frames[1])
	}
	// Bottom frame (main at 3)
	if frames[2].Name != "main" || frames[2].Line != 3 || frames[2].ID != 2 {
		t.Errorf("Frame 2 mismatch: %+v", frames[2])
	}

	// Anonymous / fallback function name when Func is nil
	vm.Stack[0] = qc.PRStack{
		S:         1,
		Func:      nil,
		FuncIndex: 99,
	}
	frames = session.StackTrace()
	if frames[2].Name != "fn_99" {
		t.Errorf("Expected fallback frame name 'fn_99', got %q", frames[2].Name)
	}
}

func TestSessionNextSeqAndVariables(t *testing.T) {
	vm := qc.NewVM()
	target := &mockTarget{vm: vm}
	session := NewSession(target)

	seq1 := session.NextSeq()
	seq2 := session.NextSeq()
	seq3 := session.NextSeq()

	if seq1 != 1 || seq2 != 2 || seq3 != 3 {
		t.Fatalf("NextSeq() not monotonically increasing: %d, %d, %d", seq1, seq2, seq3)
	}

	if session.Variables() == nil {
		t.Fatal("Variables() returned nil")
	}

	if session.Initialized() {
		t.Fatal("Expected Initialized() to be false initially")
	}
	session.SetInitialized(true)
	if !session.Initialized() {
		t.Fatal("Expected Initialized() to be true after SetInitialized(true)")
	}
}

func TestSessionStackTraceClamping(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("main"), FirstStatement: 0},
	}
	target := &mockTarget{vm: vm}
	session := NewSession(target)

	// vm.Depth is excessively large compared to stack length (1024)
	vm.Depth = len(vm.Stack) + 50
	for i := range vm.Stack {
		vm.Stack[i] = qc.PRStack{
			S:         i,
			FuncIndex: 0,
		}
	}

	frames := session.StackTrace()
	// Should not panic, and number of frames should be 1 (top) + len(vm.Stack) = 1025
	if len(frames) != 1+len(vm.Stack) {
		t.Fatalf("Expected %d frames, got %d", 1+len(vm.Stack), len(frames))
	}
}

func TestBarrierModeAndArmEarlyResume(t *testing.T) {
	b := NewBarrier()

	mode, dep := b.Mode()
	if mode != modeContinue || dep != 0 {
		t.Fatalf("Expected (modeContinue, 0), got (%v, %d)", mode, dep)
	}

	b.Arm()
	if !b.IsPaused() {
		t.Fatal("Expected barrier to be paused after Arm()")
	}

	// Immediate Resume before Wait should clear pause and set mode
	b.Resume(modeStepOver, 3)
	mode, dep = b.Mode()
	if mode != modeStepOver || dep != 3 {
		t.Fatalf("Expected (modeStepOver, 3), got (%v, %d)", mode, dep)
	}
	if b.IsPaused() {
		t.Fatal("Expected barrier to not be paused after Resume()")
	}

	// Wait should not block since pause was already cleared
	done := make(chan struct{})
	go func() {
		b.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Wait() blocked even though Resume() was called after Arm()")
	}
}
