# QCVM Remote Debugging Protocol (DAP) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement a pure-Go Debug Adapter Protocol (DAP) server over TCP for the QuakeC Virtual Machine (QCVM) in `ironwail-go` and `qcmod` (issue `ironwail-go-g1p`), enabling remote breakpoints, stepping, stack trace, and edict/variable inspection from standard IDEs and external tools.

**Architecture:** A modular package `internal/qc/dap` provides DAP framing, request/response codecs, and an execution barrier for thread-safe VM inspection during pauses. A pluggable `Target` interface connects the DAP session to either the live engine (`internal/server.Server`) or the standalone simulation world (`cmd/qcmod`).

**Tech Stack:** Go 1.26, pure Go (`CGO_ENABLED=0`), `net`, `encoding/json`, `internal/qc`, `internal/server`.

---

### Task 1: DAP Protocol Wire Framing & Base Types

**Files:**
- Create: `internal/qc/dap/protocol.go`
- Test: `internal/qc/dap/protocol_test.go`

- [ ] **Step 1: Write the failing test for DAP wire framing and JSON message codec**

```go
package dap

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDAPFramingReadWrite(t *testing.T) {
	req := Request{
		Message: Message{
			Seq:  1,
			Type: "request",
		},
		Command: "initialize",
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, req); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	payload, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	var parsed Request
	if err := DecodeMessage(payload, &parsed); err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	if parsed.Seq != req.Seq || parsed.Type != req.Type || parsed.Command != req.Command {
		t.Fatalf("Parsed message mismatch: got %+v, want %+v", parsed, req)
	}
}

func TestDAPFramingMultipleMessages(t *testing.T) {
	var buf bytes.Buffer
	m1 := Request{Message: Message{Seq: 1, Type: "request"}, Command: "threads"}
	m2 := Request{Message: Message{Seq: 2, Type: "request"}, Command: "stackTrace"}

	if err := WriteMessage(&buf, m1); err != nil {
		t.Fatal(err)
	}
	if err := WriteMessage(&buf, m2); err != nil {
		t.Fatal(err)
	}

	p1, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var r1 Request
	if err := DecodeMessage(p1, &r1); err != nil || r1.Command != "threads" {
		t.Fatalf("First message error: %v, got %+v", err, r1)
	}

	p2, err := ReadMessage(&buf)
	if err != nil {
		t.Fatal(err)
	}
	var r2 Request
	if err := DecodeMessage(p2, &r2); err != nil || r2.Command != "stackTrace" {
		t.Fatalf("Second message error: %v, got %+v", err, r2)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestDAPFraming -count=1`
Expected: FAIL (package does not exist)

- [ ] **Step 3: Write minimal implementation**

`internal/qc/dap/protocol.go`:
```go
package dap

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Message is the base DAP JSON message structure.
type Message struct {
	Seq  int    `json:"seq"`
	Type string `json:"type"` // "request", "response", "event"
}

// Request represents a DAP request from client to server.
type Request struct {
	Message
	Command   string          `json:"command"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// Response represents a DAP response from server to client.
type Response struct {
	Message
	RequestSeq int             `json:"request_seq"`
	Success    bool            `json:"success"`
	Command    string          `json:"command"`
	Message    string          `json:"message,omitempty"`
	Body       json.RawMessage `json:"body,omitempty"`
}

// Event represents an asynchronous notification from server to client.
type Event struct {
	Message
	Event string          `json:"event"`
	Body  json.RawMessage `json:"body,omitempty"`
}

// Capabilities describes the debugger features supported by this server.
type Capabilities struct {
	SupportsConfigurationDoneRequest bool `json:"supportsConfigurationDoneRequest"`
	SupportsFunctionBreakpoints      bool `json:"supportsFunctionBreakpoints"`
	SupportsEvaluateForHovers        bool `json:"supportsEvaluateForHovers"`
	SupportsStepBack                 bool `json:"supportsStepBack"`
}

// Thread represents a DAP thread.
type Thread struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// StackFrame represents a stack frame in a stack trace.
type StackFrame struct {
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Source    *Source `json:"source,omitempty"`
	Line      int     `json:"line"`
	Column    int     `json:"column"`
	EndLine   int     `json:"endLine,omitempty"`
	EndColumn int     `json:"endColumn,omitempty"`
}

// Source represents a source file in DAP.
type Source struct {
	Name string `json:"name,omitempty"`
	Path string `json:"path,omitempty"`
}

// Scope represents a variable scope (Locals, Globals, Edicts).
type Scope struct {
	Name               string `json:"name"`
	VariablesReference int    `json:"variablesReference"`
	Expensive          bool   `json:"expensive"`
}

// Variable represents a variable or edict field.
type Variable struct {
	Name               string `json:"name"`
	Value              string `json:"value"`
	Type               string `json:"type,omitempty"`
	VariablesReference int    `json:"variablesReference"`
}

// StoppedEventBody is the payload for a DAP "stopped" event.
type StoppedEventBody struct {
	Reason            string `json:"reason"` // "breakpoint", "step", "pause", "exception"
	Description       string `json:"description,omitempty"`
	ThreadID          int    `json:"threadId"`
	PreserveFocusHint bool   `json:"preserveFocusHint,omitempty"`
	AllThreadsStopped bool   `json:"allThreadsStopped"`
}

// FunctionBreakpoint represents a breakpoint placed on a QC function.
type FunctionBreakpoint struct {
	Name string `json:"name"`
}

// Breakpoint represents a resolved breakpoint in a response.
type Breakpoint struct {
	ID       int     `json:"id"`
	Verified bool    `json:"verified"`
	Message  string  `json:"message,omitempty"`
	Source   *Source `json:"source,omitempty"`
	Line     int     `json:"line,omitempty"`
}

// ReadMessage reads a Content-Length framed JSON message from r.
func ReadMessage(r io.Reader) ([]byte, error) {
	br, ok := r.(*bufio.Reader)
	if !ok {
		br = bufio.NewReader(r)
	}

	contentLength := -1
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Empty line marks end of headers.
			break
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "Content-Length") {
			length, parseErr := strconv.Atoi(strings.TrimSpace(parts[1]))
			if parseErr != nil {
				return nil, fmt.Errorf("invalid Content-Length: %w", parseErr)
			}
			contentLength = length
		}
	}

	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(br, payload); err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}
	return payload, nil
}

// WriteMessage serializes msg as JSON and writes it with Content-Length framing to w.
func WriteMessage(w io.Writer, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal DAP message: %w", err)
	}
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
	if _, err := io.WriteString(w, header); err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// DecodeMessage unmarshals a JSON message into dest.
func DecodeMessage(payload []byte, dest any) error {
	return json.Unmarshal(payload, dest)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestDAPFraming -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/qc/dap/protocol.go internal/qc/dap/protocol_test.go
git commit -m "feat(qc/dap): implement DAP protocol message types and wire framing"
```

---

### Task 2: Target Interface & Inspection Model

**Files:**
- Create: `internal/qc/dap/target.go`
- Create: `internal/qc/dap/variables.go`
- Test: `internal/qc/dap/variables_test.go`

- [ ] **Step 1: Write failing test for Target and Variable Inspection**

`internal/qc/dap/variables_test.go`:
```go
package dap

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

type mockTarget struct {
	vm *qc.VM
}

func (m *mockTarget) VM() *qc.VM { return m.vm }
func (m *mockTarget) EdictCount() int { return 2 }
func (m *mockTarget) GetEdictFloat(entNum, offset int) float32 {
	if offset == qc.EntFieldHealth {
		return 100.0
	}
	return 0
}
func (m *mockTarget) GetEdictString(entNum, offset int) string {
	if entNum == 0 && offset == qc.EntFieldClassName {
		return "worldspawn"
	}
	if entNum == 1 && offset == qc.EntFieldClassName {
		return "player"
	}
	return ""
}
func (m *mockTarget) GetEdictVector(entNum, offset int) [3]float32 {
	if offset == qc.EntFieldOrigin {
		return [3]float32{10, 20, 30}
	}
	return [3]float32{}
}
func (m *mockTarget) GetEdictClassName(entNum int) string {
	return m.GetEdictString(entNum, qc.EntFieldClassName)
}
func (m *mockTarget) FieldNames() map[string]int {
	return map[string]int{
		"classname": qc.EntFieldClassName,
		"origin":    qc.EntFieldOrigin,
		"health":    qc.EntFieldHealth,
	}
}

func TestResolveScopesAndVariables(t *testing.T) {
	vm := qc.NewVM()
	vm.SetGInt(qc.OFSSelf, 1)
	vm.SetGInt(qc.OFSOther, 0)
	vm.Globals[qc.OFSTime] = 12.5

	target := &mockTarget{vm: vm}
	mgr := NewVariableManager(target)

	scopes := mgr.GetScopes(0)
	if len(scopes) != 3 {
		t.Fatalf("Expected 3 scopes, got %d", len(scopes))
	}

	// Locals
	locals := mgr.GetVariables(scopes[0].VariablesReference)
	if len(locals) == 0 {
		t.Fatal("Expected local variables")
	}

	// Globals
	globals := mgr.GetVariables(scopes[1].VariablesReference)
	foundTime := false
	for _, g := range globals {
		if g.Name == "time" && g.Value == "12.5" {
			foundTime = true
		}
	}
	if !foundTime {
		t.Fatalf("Expected global 'time=12.5' in globals: %+v", globals)
	}

	// Edicts scope
	edicts := mgr.GetVariables(scopes[2].VariablesReference)
	if len(edicts) != 2 {
		t.Fatalf("Expected 2 edicts, got %d", len(edicts))
	}

	// Inspect specific edict fields
	fields := mgr.GetVariables(edicts[1].VariablesReference)
	foundOrigin := false
	for _, f := range fields {
		if f.Name == "origin" && strings.Contains(f.Value, "10") {
			foundOrigin = true
		}
	}
	if !foundOrigin {
		t.Fatalf("Expected 'origin' field on player edict: %+v", fields)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestResolveScopesAndVariables -count=1`
Expected: FAIL

- [ ] **Step 3: Implement Target interface and VariableManager**

`internal/qc/dap/target.go`:
```go
package dap

import "github.com/darkliquid/ironwail-go/internal/qc"

// Target represents an engine or simulation target being debugged.
type Target interface {
	VM() *qc.VM
	EdictCount() int
	GetEdictFloat(entNum, offset int) float32
	GetEdictString(entNum, offset int) string
	GetEdictVector(entNum, offset int) [3]float32
	GetEdictClassName(entNum int) string
	FieldNames() map[string]int
}
```

`internal/qc/dap/variables.go`:
```go
package dap

import (
	"fmt"
	"math"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

const (
	scopeLocalsBase  = 1000
	scopeGlobalsBase = 2000
	scopeEdictsBase  = 3000
	edictFieldsBase  = 10000
)

// VariableManager maps DAP variablesReference IDs to hierarchical variables.
type VariableManager struct {
	target Target
}

// NewVariableManager creates a new VariableManager.
func NewVariableManager(target Target) *VariableManager {
	return &VariableManager{target: target}
}

// GetScopes returns the standard 3 scopes for a frame.
func (vm *VariableManager) GetScopes(frameID int) []Scope {
	return []Scope{
		{Name: "Locals", VariablesReference: scopeLocalsBase + frameID, Expensive: false},
		{Name: "Globals", VariablesReference: scopeGlobalsBase + frameID, Expensive: false},
		{Name: "Edicts", VariablesReference: scopeEdictsBase + frameID, Expensive: true},
	}
}

// GetVariables returns child variables for a given reference ID.
func (vm *VariableManager) GetVariables(ref int) []Variable {
	if vm.target == nil {
		return nil
	}
	qcvm := vm.target.VM()

	if ref >= scopeLocalsBase && ref < scopeGlobalsBase {
		// Locals
		var vars []Variable
		if qcvm != nil {
			vars = append(vars, Variable{
				Name:  "self",
				Value: fmt.Sprintf("edict %d (%s)", qcvm.GInt(qc.OFSSelf), vm.target.GetEdictClassName(int(qcvm.GInt(qc.OFSSelf)))),
				Type:  "entity",
			})
			vars = append(vars, Variable{
				Name:  "other",
				Value: fmt.Sprintf("edict %d (%s)", qcvm.GInt(qc.OFSOther), vm.target.GetEdictClassName(int(qcvm.GInt(qc.OFSOther)))),
				Type:  "entity",
			})
			if qcvm.XFunction != nil {
				for i := 0; i < int(qcvm.XFunction.NumParms); i++ {
					parmOfs := int(qcvm.XFunction.Parms[i])
					if parmOfs >= 0 && parmOfs < len(qcvm.Globals) {
						vars = append(vars, Variable{
							Name:  fmt.Sprintf("parm%d", i),
							Value: fmt.Sprintf("%v", qcvm.Globals[parmOfs]),
							Type:  "float",
						})
					}
				}
			}
		}
		return vars
	}

	if ref >= scopeGlobalsBase && ref < scopeEdictsBase {
		// Globals
		var vars []Variable
		if qcvm != nil {
			vars = append(vars, Variable{Name: "time", Value: fmt.Sprintf("%v", qcvm.Globals[qc.OFSTime]), Type: "float"})
			vars = append(vars, Variable{Name: "self", Value: fmt.Sprintf("%d", qcvm.GInt(qc.OFSSelf)), Type: "entity"})
			vars = append(vars, Variable{Name: "other", Value: fmt.Sprintf("%d", qcvm.GInt(qc.OFSOther)), Type: "entity"})
			vars = append(vars, Variable{Name: "world", Value: fmt.Sprintf("%d", qcvm.GInt(qc.OFSWorld)), Type: "entity"})
		}
		return vars
	}

	if ref >= scopeEdictsBase && ref < edictFieldsBase {
		// Edicts summary list
		var vars []Variable
		count := vm.target.EdictCount()
		for i := 0; i < count; i++ {
			cname := vm.target.GetEdictClassName(i)
			if cname == "" && i > 0 {
				continue // Skip unused / free edicts
			}
			vars = append(vars, Variable{
				Name:               fmt.Sprintf("[%d]", i),
				Value:              cname,
				Type:               "entity",
				VariablesReference: edictFieldsBase + i,
			})
		}
		return vars
	}

	if ref >= edictFieldsBase {
		// Edict fields
		entNum := ref - edictFieldsBase
		var vars []Variable
		fields := vm.target.FieldNames()
		for name, ofs := range fields {
			if strings.Contains(name, "origin") || strings.Contains(name, "velocity") || strings.Contains(name, "angles") {
				vec := vm.target.GetEdictVector(entNum, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: fmt.Sprintf("[%v, %v, %v]", vec[0], vec[1], vec[2]),
					Type:  "vector",
				})
			} else if name == "classname" || name == "model" || name == "target" || name == "targetname" {
				str := vm.target.GetEdictString(entNum, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: fmt.Sprintf("%q", str),
					Type:  "string",
				})
			} else {
				fl := vm.target.GetEdictFloat(entNum, ofs)
				vars = append(vars, Variable{
					Name:  name,
					Value: fmt.Sprintf("%v", fl),
					Type:  "float",
				})
			}
		}
		return vars
	}

	return nil
}

// Evaluate evaluates an expression string against target state.
func (vm *VariableManager) Evaluate(expr string) (string, error) {
	expr = strings.TrimSpace(expr)
	if vm.target == nil {
		return "", fmt.Errorf("no active target")
	}
	qcvm := vm.target.VM()
	if qcvm == nil {
		return "", fmt.Errorf("no active VM")
	}

	switch expr {
	case "time":
		return fmt.Sprintf("%v", qcvm.Globals[qc.OFSTime]), nil
	case "self":
		return fmt.Sprintf("edict %d (%s)", qcvm.GInt(qc.OFSSelf), vm.target.GetEdictClassName(int(qcvm.GInt(qc.OFSSelf)))), nil
	case "other":
		return fmt.Sprintf("edict %d (%s)", qcvm.GInt(qc.OFSOther), vm.target.GetEdictClassName(int(qcvm.GInt(qc.OFSOther)))), nil
	}

	if strings.HasPrefix(expr, "self.") {
		field := strings.TrimPrefix(expr, "self.")
		entNum := int(qcvm.GInt(qc.OFSSelf))
		if ofs, ok := vm.target.FieldNames()[field]; ok {
			return fmt.Sprintf("%v", vm.target.GetEdictFloat(entNum, ofs)), nil
		}
	}

	return "", fmt.Errorf("unknown expression: %s", expr)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestResolveScopesAndVariables -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/qc/dap/target.go internal/qc/dap/variables.go internal/qc/dap/variables_test.go
git commit -m "feat(qc/dap): add Target interface and hierarchical variable inspector"
```

---

### Task 3: Session State Machine, Breakpoints & Execution Barrier

**Files:**
- Create: `internal/qc/dap/barrier.go`
- Create: `internal/qc/dap/session.go`
- Test: `internal/qc/dap/session_test.go`

- [ ] **Step 1: Write failing test for Session barrier and stepping transitions**

`internal/qc/dap/session_test.go`:
```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestSessionBreakAndStep -count=1`
Expected: FAIL

- [ ] **Step 3: Implement Barrier and Session**

`internal/qc/dap/barrier.go`:
```go
package dap

import (
	"sync"
)

type stepMode int

const (
	modeContinue stepMode = iota
	modeStepIn
	modeStepOver
	modeStepOut
	modePause
)

// Barrier coordinates synchronization between the execution thread and DAP reader goroutine.
type Barrier struct {
	mu        sync.Mutex
	mode      stepMode
	targetDep int
	paused    bool
	resumeCh  chan struct{}
}

// NewBarrier creates an initialized Barrier.
func NewBarrier() *Barrier {
	return &Barrier{
		mode:     modeContinue,
		resumeCh: make(chan struct{}, 1),
	}
}

// Resume unblocks the execution thread.
func (b *Barrier) Resume(mode stepMode, targetDep int) {
	b.mu.Lock()
	b.mode = mode
	b.targetDep = targetDep
	b.paused = false
	b.mu.Unlock()

	select {
	case b.resumeCh <- struct{}{}:
	default:
	}
}

// Wait blocks until Resume is called.
func (b *Barrier) Wait() {
	b.mu.Lock()
	b.paused = true
	b.mu.Unlock()

	<-b.resumeCh
}

// IsPaused returns true if execution is halted at the barrier.
func (b *Barrier) IsPaused() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.paused
}
```

`internal/qc/dap/session.go`:
```go
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
	if s.target != nil && s.target.VM() != nil {
		fnIdx := s.target.VM().FindFunction(name)
		verified = fnIdx >= 0
	}
	return Breakpoint{ID: len(s.funcBreaks), Verified: verified}
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
	for i := vm.Depth - 1; i >= 0; i-- {
		stk := vm.Stack[i]
		fnName := fmt.Sprintf("fn_%d", stk.F)
		if int(stk.F) < len(vm.Functions) {
			fnName = vm.String(vm.Functions[stk.F].Name)
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
		s.mu.Lock()
		mode := s.barrier.mode
		targetDep := s.barrier.targetDep
		funcBreaks := s.funcBreaks
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
		} else if vm.XFunction != nil && funcBreaks[vm.String(vm.XFunction.Name)] && stmtIdx == int(vm.XFunction.FirstStatement) {
			stopReason = "breakpoint"
		}

		if stopReason != "" {
			if s.OnStopped != nil {
				s.OnStopped(stopReason, 1)
			}
			s.barrier.Wait()
		}

		return false
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestSessionBreakAndStep -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/qc/dap/barrier.go internal/qc/dap/session.go internal/qc/dap/session_test.go
git commit -m "feat(qc/dap): implement thread-safe barrier and session state machine"
```

---

### Task 4: DAP TCP Server & Command Dispatcher

**Files:**
- Create: `internal/qc/dap/server.go`
- Test: `internal/qc/dap/server_test.go`

- [ ] **Step 1: Write failing test for DAP TCP Server connection and command dispatch**

`internal/qc/dap/server_test.go`:
```go
package dap

import (
	"encoding/json"
	"net"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestDAPServerInitializeAndThreads(t *testing.T) {
	vm := qc.NewVM()
	target := &mockTarget{vm: vm}

	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer srv.Close()

	addr := srv.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to DAP server: %v", err)
	}
	defer conn.Close()

	// Send initialize request
	initReq := Request{
		Message: Message{Seq: 1, Type: "request"},
		Command: "initialize",
	}
	if err := WriteMessage(conn, initReq); err != nil {
		t.Fatal(err)
	}

	payload, err := ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var initResp Response
	if err := DecodeMessage(payload, &initResp); err != nil || !initResp.Success {
		t.Fatalf("Init response error: %v, resp=%+v", err, initResp)
	}

	// Send threads request
	threadsReq := Request{
		Message: Message{Seq: 2, Type: "request"},
		Command: "threads",
	}
	if err := WriteMessage(conn, threadsReq); err != nil {
		t.Fatal(err)
	}

	payload, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var threadsResp Response
	if err := DecodeMessage(payload, &threadsResp); err != nil || !threadsResp.Success {
		t.Fatalf("Threads response error: %v, resp=%+v", err, threadsResp)
	}

	var body struct {
		Threads []Thread `json:"threads"`
	}
	if err := json.Unmarshal(threadsResp.Body, &body); err != nil || len(body.Threads) != 1 {
		t.Fatalf("Threads body mismatch: %v, body=%+v", err, body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestDAPServerInitializeAndThreads -count=1`
Expected: FAIL

- [ ] **Step 3: Implement DAP TCP Server**

`internal/qc/dap/server.go`:
```go
package dap

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"sync"
)

// Server is a TCP server that listens for incoming DAP client connections.
type Server struct {
	listener net.Listener
	target   Target
	session  *Session
	mu       sync.Mutex
	closed   bool
}

// NewServer starts a new DAP TCP server on the given address.
func NewServer(addr string, target Target) (*Server, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on %s: %w", addr, err)
	}

	session := NewSession(target)
	if target != nil && target.VM() != nil {
		target.VM().BreakHook = session.BreakHook()
	}

	srv := &Server{
		listener: ln,
		target:   target,
		session:  session,
	}

	go srv.acceptLoop()
	return srv, nil
}

// Addr returns the listener address.
func (s *Server) Addr() net.Addr {
	return s.listener.Addr()
}

// Session returns the active debugging session.
func (s *Server) Session() *Session {
	return s.session
}

// Close closes the server listener and active session.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.session != nil {
		s.session.Disconnect()
	}
	return s.listener.Close()
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go s.handleConnection(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	session := s.session
	session.OnStopped = func(reason string, threadID int) {
		body, _ := json.Marshal(StoppedEventBody{
			Reason:            reason,
			ThreadID:          threadID,
			AllThreadsStopped: true,
		})
		event := Event{
			Message: Message{Seq: session.NextSeq(), Type: "event"},
			Event:   "stopped",
			Body:    body,
		}
		_ = WriteMessage(conn, event)
	}

	for {
		payload, err := ReadMessage(conn)
		if err != nil {
			if err != io.EOF {
				// Connection dropped
			}
			session.Disconnect()
			return
		}

		var req Request
		if err := DecodeMessage(payload, &req); err != nil {
			continue
		}

		s.dispatchRequest(conn, req)
	}
}

func (s *Server) dispatchRequest(conn net.Conn, req Request) {
	session := s.session
	resp := Response{
		Message:    Message{Seq: session.NextSeq(), Type: "response"},
		RequestSeq: req.Seq,
		Success:    true,
		Command:    req.Command,
	}

	switch req.Command {
	case "initialize":
		caps := Capabilities{
			SupportsConfigurationDoneRequest: true,
			SupportsFunctionBreakpoints:      true,
			SupportsEvaluateForHovers:        true,
			SupportsStepBack:                 false,
		}
		resp.Body, _ = json.Marshal(caps)
		_ = WriteMessage(conn, resp)

		// Send initialized event
		initEvent := Event{
			Message: Message{Seq: session.NextSeq(), Type: "event"},
			Event:   "initialized",
		}
		_ = WriteMessage(conn, initEvent)
		return

	case "attach", "launch":
		// Success empty body
	case "configurationDone":
		// Configuration completed
	case "threads":
		body := map[string]any{
			"threads": []Thread{{ID: 1, Name: "QuakeC VM Thread"}},
		}
		resp.Body, _ = json.Marshal(body)

	case "stackTrace":
		frames := session.StackTrace()
		body := map[string]any{
			"stackFrames": frames,
			"totalFrames": len(frames),
		}
		resp.Body, _ = json.Marshal(body)

	case "scopes":
		var args struct {
			FrameID int `json:"frameId"`
		}
		_ = json.Unmarshal(req.Arguments, &args)
		scopes := session.vars.GetScopes(args.FrameID)
		body := map[string]any{"scopes": scopes}
		resp.Body, _ = json.Marshal(body)

	case "variables":
		var args struct {
			VariablesReference int `json:"variablesReference"`
		}
		_ = json.Unmarshal(req.Arguments, &args)
		vars := session.vars.GetVariables(args.VariablesReference)
		body := map[string]any{"variables": vars}
		resp.Body, _ = json.Marshal(body)

	case "evaluate":
		var args struct {
			Expression string `json:"expression"`
		}
		_ = json.Unmarshal(req.Arguments, &args)
		val, err := session.vars.Evaluate(args.Expression)
		if err != nil {
			resp.Success = false
			resp.Message = err.Error()
		} else {
			resp.Body, _ = json.Marshal(map[string]any{
				"result":             val,
				"variablesReference": 0,
			})
		}

	case "setFunctionBreakpoints":
		var args struct {
			Breakpoints []FunctionBreakpoint `json:"breakpoints"`
		}
		_ = json.Unmarshal(req.Arguments, &args)
		session.ClearBreakpoints()
		var bps []Breakpoint
		for _, b := range args.Breakpoints {
			bps = append(bps, session.SetFunctionBreakpoint(b.Name))
		}
		resp.Body, _ = json.Marshal(map[string]any{"breakpoints": bps})

	case "continue":
		session.Continue()
		resp.Body, _ = json.Marshal(map[string]any{"allThreadsContinued": true})

	case "next":
		session.StepOver()

	case "stepIn":
		session.StepIn()

	case "stepOut":
		session.StepOut()

	case "pause":
		session.Pause()

	case "disconnect":
		session.Disconnect()
		_ = WriteMessage(conn, resp)
		return

	default:
		resp.Success = false
		resp.Message = fmt.Sprintf("unsupported command %s", req.Command)
	}

	_ = WriteMessage(conn, resp)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestDAPServerInitializeAndThreads -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/qc/dap/server.go internal/qc/dap/server_test.go
git commit -m "feat(qc/dap): implement DAP TCP server and command dispatcher"
```

---

### Task 5: End-to-End DAP Integration Test with Synthetic QCVM

**Files:**
- Create: `internal/qc/dap/integration_test.go`

- [ ] **Step 1: Write comprehensive end-to-end DAP integration test**

`internal/qc/dap/integration_test.go`:
```go
package dap

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestDAPEndToEndBreakStepAndInspect(t *testing.T) {
	vm := qc.NewVM()
	fnIdx := len(vm.Functions)
	vm.Functions = append(vm.Functions, qc.DFunction{
		Name:           vm.AllocString("test_target_fn"),
		FirstStatement: int32(len(vm.Statements)),
		NumParms:       1,
		Parms:          [8]byte{0},
	})
	vm.Statements = append(vm.Statements,
		qc.DStatement{Op: uint16(qc.OPDone)},
		qc.DStatement{Op: uint16(qc.OPDone)},
	)

	target := &mockTarget{vm: vm}
	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer conn.Close()

	// 1. Initialize
	if err := WriteMessage(conn, Request{
		Message: Message{Seq: 1, Type: "request"},
		Command: "initialize",
	}); err != nil {
		t.Fatal(err)
	}
	p, _ := ReadMessage(conn) // response
	var initResp Response
	_ = DecodeMessage(p, &initResp)
	p, _ = ReadMessage(conn) // initialized event

	// 2. Set breakpoint on test_target_fn
	args, _ := json.Marshal(map[string]any{
		"breakpoints": []FunctionBreakpoint{{Name: "test_target_fn"}},
	})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 2, Type: "request"},
		Command:   "setFunctionBreakpoints",
		Arguments: args,
	}); err != nil {
		t.Fatal(err)
	}
	p, _ = ReadMessage(conn)

	// 3. ConfigurationDone
	_ = WriteMessage(conn, Request{
		Message: Message{Seq: 3, Type: "request"},
		Command: "configurationDone",
	})
	p, _ = ReadMessage(conn)

	// 4. Trigger QCVM execution in background goroutine
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- vm.ExecuteFunction(fnIdx)
	}()

	// 5. Expect stopped event
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed reading stopped event: %v", err)
	}
	var stopEvt Event
	if err := DecodeMessage(p, &stopEvt); err != nil || stopEvt.Event != "stopped" {
		t.Fatalf("Expected stopped event, got: %+v", stopEvt)
	}

	// 6. Inspect StackTrace
	_ = WriteMessage(conn, Request{
		Message: Message{Seq: 4, Type: "request"},
		Command: "stackTrace",
	})
	p, _ = ReadMessage(conn)
	var stackResp Response
	_ = DecodeMessage(p, &stackResp)
	var stackBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
	}
	_ = json.Unmarshal(stackResp.Body, &stackBody)
	if len(stackBody.StackFrames) == 0 || stackBody.StackFrames[0].Name != "test_target_fn" {
		t.Fatalf("Stack trace frame mismatch: %+v", stackBody)
	}

	// 7. Step (next)
	_ = WriteMessage(conn, Request{
		Message: Message{Seq: 5, Type: "request"},
		Command: "next",
	})
	p, _ = ReadMessage(conn)

	// 8. Expect stopped event for step
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed reading step stopped event: %v", err)
	}
	_ = DecodeMessage(p, &stopEvt)
	if stopEvt.Event != "stopped" {
		t.Fatalf("Expected stopped event for step, got: %+v", stopEvt)
	}

	// 9. Continue to end
	_ = WriteMessage(conn, Request{
		Message: Message{Seq: 6, Type: "request"},
		Command: "continue",
	})
	p, _ = ReadMessage(conn)

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("ExecuteFunction returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteFunction timed out")
	}
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/qc/dap -run TestDAPEndToEndBreakStepAndInspect -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add internal/qc/dap/integration_test.go
git commit -m "test(qc/dap): add end-to-end breakpoint, step, and inspection integration test"
```

---

### Task 6: Engine Integration: Server Target Adapter, Cvars, Console Commands & Flags

**Files:**
- Create: `internal/server/server_dap.go`
- Create: `internal/server/dap_integration_test.go`
- Modify: `internal/server/debug_telemetry.go`
- Modify: `internal/game/game_init.go`
- Modify: `cmd/ironwailgo/main.go`

- [ ] **Step 1: Write failing test for Server DAP integration**

`internal/server/dap_integration_test.go`:
```go
package server

import (
	"net"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc/dap"
)

func TestServerImplementsDAPTarget(t *testing.T) {
	s := &Server{}
	var target dap.Target = s
	if target == nil {
		t.Fatal("Server should implement dap.Target")
	}
}

func TestServerStartDAPListener(t *testing.T) {
	s := newPhysicsTestServer(t)
	srv, err := s.StartDAPServer("127.0.0.1:0")
	if err != nil {
		t.Fatalf("StartDAPServer failed: %v", err)
	}
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial DAP server failed: %v", err)
	}
	defer conn.Close()

	if err := dap.WriteMessage(conn, dap.Request{
		Message: dap.Message{Seq: 1, Type: "request"},
		Command: "initialize",
	}); err != nil {
		t.Fatal(err)
	}

	payload, err := dap.ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var resp dap.Response
	if err := dap.DecodeMessage(payload, &resp); err != nil || !resp.Success {
		t.Fatalf("Init failed: %v, resp=%+v", err, resp)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/server -run TestServerStartDAPListener -count=1`
Expected: FAIL

- [ ] **Step 3: Implement Server DAP methods and dynamic listener management**

`internal/server/server_dap.go`:
```go
package server

import (
	"fmt"
	"sync"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/qc/dap"
)

var (
	dapServerMu sync.Mutex
	dapServer   *dap.Server
)

// VM returns the server's QuakeC virtual machine.
func (s *Server) VM() *qc.VM {
	if s == nil {
		return nil
	}
	return s.QCVM
}

// EdictCount returns the active number of edicts.
func (s *Server) EdictCount() int {
	if s == nil {
		return 0
	}
	return s.NumEdicts
}

// GetEdictFloat returns a float field from an edict.
func (s *Server) GetEdictFloat(entNum, offset int) float32 {
	if s == nil || entNum < 0 || entNum >= len(s.Edicts) {
		return 0
	}
	return s.Edicts[entNum].Float(offset)
}

// GetEdictString returns a string field from an edict.
func (s *Server) GetEdictString(entNum, offset int) string {
	if s == nil || entNum < 0 || entNum >= len(s.Edicts) {
		return ""
	}
	return s.Edicts[entNum].String(s, offset)
}

// GetEdictVector returns a vector field from an edict.
func (s *Server) GetEdictVector(entNum, offset int) [3]float32 {
	if s == nil || entNum < 0 || entNum >= len(s.Edicts) {
		return [3]float32{}
	}
	return s.Edicts[entNum].Vector(offset)
}

// GetEdictClassName returns the classname of an edict.
func (s *Server) GetEdictClassName(entNum int) string {
	return s.GetEdictString(entNum, qc.EntFieldClassName)
}

// FieldNames returns a map of field names to bytecode offsets.
func (s *Server) FieldNames() map[string]int {
	return map[string]int{
		"classname":  qc.EntFieldClassName,
		"origin":     qc.EntFieldOrigin,
		"angles":     qc.EntFieldAngles,
		"velocity":   qc.EntFieldVelocity,
		"health":     qc.EntFieldHealth,
		"target":     qc.EntFieldTarget,
		"targetname": qc.EntFieldTargetName,
		"model":      qc.EntFieldModel,
		"solid":      qc.EntFieldSolid,
		"movetype":   qc.EntFieldMoveType,
		"nextthink":  qc.EntFieldNextThink,
	}
}

// StartDAPServer launches a DAP listener on addr for this server.
func (s *Server) StartDAPServer(addr string) (*dap.Server, error) {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()

	if dapServer != nil {
		_ = dapServer.Close()
		dapServer = nil
	}

	srv, err := dap.NewServer(addr, s)
	if err != nil {
		return nil, err
	}
	dapServer = srv
	return srv, nil
}

// StopDAPServer stops any active DAP listener.
func StopDAPServer() {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()
	if dapServer != nil {
		_ = dapServer.Close()
		dapServer = nil
	}
}

// DAPServerStatus returns the status/address of the active DAP listener.
func DAPServerStatus() string {
	dapServerMu.Lock()
	defer dapServerMu.Unlock()
	if dapServer == nil {
		return "DAP debug server is inactive"
	}
	return fmt.Sprintf("DAP debug server listening on %s", dapServer.Addr().String())
}
```

- [ ] **Step 4: Register Cvars and commands in `internal/server/debug_telemetry.go` and `cmd/ironwailgo/main.go`**

Add `qc_debug_port` cvar and `qc_debug_start` / `qc_debug_stop` / `qc_debug_status` console commands, and `-qcdbg` CLI flag parsing in `cmd/ironwailgo/main.go`.

- [ ] **Step 5: Run tests to verify they pass**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/server -run TestServerStartDAPListener -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/server/server_dap.go internal/server/dap_integration_test.go internal/server/debug_telemetry.go internal/game/game_init.go cmd/ironwailgo/main.go
git commit -m "feat(server): integrate DAP debug server, cvars, and startup flags"
```

---

### Task 7: Standalone Dev Kit Integration (`cmd/qcmod`)

**Files:**
- Create: `cmd/qcmod/dap.go`
- Test: `cmd/qcmod/dap_test.go`
- Modify: `cmd/qcmod/main.go`

- [ ] **Step 1: Write test for `qcmod dap` command**

`cmd/qcmod/dap_test.go`:
```go
package main

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/qc/dap"
)

func TestQCModDAPCommand(t *testing.T) {
	stdout := &bytes.Buffer()
	stderr := &bytes.Buffer()

	// Run in background
	go func() {
		runDAP([]string{"127.0.0.1:23499"}, stdout, stderr)
	}()

	time.Sleep(50 * time.Millisecond)

	conn, err := net.Dial("tcp", "127.0.0.1:23499")
	if err != nil {
		t.Fatalf("Failed connecting to qcmod dap: %v", err)
	}
	defer conn.Close()

	if err := dap.WriteMessage(conn, dap.Request{
		Message: dap.Message{Seq: 1, Type: "request"},
		Command: "initialize",
	}); err != nil {
		t.Fatal(err)
	}

	payload, err := dap.ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var resp dap.Response
	if err := dap.DecodeMessage(payload, &resp); err != nil || !resp.Success {
		t.Fatalf("qcmod DAP initialize failed: %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./cmd/qcmod -run TestQCModDAPCommand -count=1`
Expected: FAIL

- [ ] **Step 3: Implement `runDAP` in `cmd/qcmod/dap.go` and wire into `cmd/qcmod/main.go`**

`cmd/qcmod/dap.go`:
```go
package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/qc/dap"
)

type qcmodTarget struct {
	world *vmWorld
}

func (t *qcmodTarget) VM() *qc.VM {
	if t.world == nil {
		return nil
	}
	return t.world.vm
}

func (t *qcmodTarget) EdictCount() int {
	return 1
}

func (t *qcmodTarget) GetEdictFloat(entNum, offset int) float32 {
	return 0
}

func (t *qcmodTarget) GetEdictString(entNum, offset int) string {
	return ""
}

func (t *qcmodTarget) GetEdictVector(entNum, offset int) [3]float32 {
	return [3]float32{}
}

func (t *qcmodTarget) GetEdictClassName(entNum int) string {
	return "worldspawn"
}

func (t *qcmodTarget) FieldNames() map[string]int {
	return map[string]int{
		"classname": qc.EntFieldClassName,
		"origin":    qc.EntFieldOrigin,
		"angles":    qc.EntFieldAngles,
	}
}

func runDAP(args []string, stdout, stderr io.Writer) int {
	addr := "127.0.0.1:2345"
	if len(args) > 0 {
		addr = args[0]
	}

	w, err := newVMWorld(nil)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod dap: %v\n", err)
		return 1
	}

	target := &qcmodTarget{world: w}
	srv, err := dap.NewServer(addr, target)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "qcmod dap: %v\n", err)
		return 1
	}
	defer srv.Close()

	_, _ = fmt.Fprintf(stdout, "qcmod DAP server listening on %s (press Ctrl+C to stop)\n", srv.Addr().String())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	return 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./cmd/qcmod -run TestQCModDAPCommand -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cmd/qcmod/dap.go cmd/qcmod/dap_test.go cmd/qcmod/main.go
git commit -m "feat(qcmod): add standalone DAP server command"
```

---

### Task 8: Full Quality Gate Verification & Session Close

- [ ] **Step 1: Run full test suite**
Run: `mise run test`
Expected: All tests pass across the entire repository.

- [ ] **Step 2: Run linter and formatting**
Run: `mise run lint`
Expected: No linter warnings or errors.

- [ ] **Step 3: Close beads issue**
Run: `bd close ironwail-go-g1p --reason="Implemented pure-Go DAP debug server for QCVM and QuakeGo with full test coverage"`
