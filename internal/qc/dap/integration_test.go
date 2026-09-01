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
	vm.Globals = make([]float32, 100)
	vm.SetGInt(qc.OFSSelf, 1)
	vm.SetGInt(qc.OFSOther, 0)
	vm.Globals[qc.OFSTime] = 12.5

	fnIdx := len(vm.Functions)
	vm.Functions = append(vm.Functions, qc.DFunction{
		Name:           vm.AllocString("test_target_fn"),
		FirstStatement: int32(len(vm.Statements)),
		NumParms:       1,
		ParmStart:      10,
		ParmSize:       [qc.MaxParms]byte{1},
	})
	vm.Globals[qc.OFSParm0] = 77.0 // parm0 passed via OFSParm0

	vm.Statements = append(vm.Statements,
		qc.DStatement{Op: uint16(qc.OPAddF)},
		qc.DStatement{Op: uint16(qc.OPDone)},
	)

	target := &mockTarget{vm: vm}
	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Close() }()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// 1. Initialize Handshake
	if err := WriteMessage(conn, Request{
		Message: Message{Seq: 1, Type: "request"},
		Command: "initialize",
	}); err != nil {
		t.Fatal(err)
	}
	p, err := ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var initResp Response
	if err := DecodeMessage(p, &initResp); err != nil || !initResp.Success {
		t.Fatalf("Init response failed: %v, resp: %+v", err, initResp)
	}

	p, err = ReadMessage(conn) // initialized event
	if err != nil {
		t.Fatal(err)
	}
	var initEvt Event
	if err := DecodeMessage(p, &initEvt); err != nil || initEvt.Event != "initialized" {
		t.Fatalf("Expected initialized event, got: %+v", initEvt)
	}

	// 2. Set breakpoint on test_target_fn
	args, err := json.Marshal(map[string]any{
		"breakpoints": []FunctionBreakpoint{{Name: "test_target_fn"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 2, Type: "request"},
		Command:   "setFunctionBreakpoints",
		Arguments: args,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var bpResp Response
	if err := DecodeMessage(p, &bpResp); err != nil || !bpResp.Success {
		t.Fatalf("SetFunctionBreakpoints failed: %v, resp: %+v", err, bpResp)
	}

	// 3. ConfigurationDone
	if err := WriteMessage(conn, Request{
		Message: Message{Seq: 3, Type: "request"},
		Command: "configurationDone",
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var configResp Response
	if err := DecodeMessage(p, &configResp); err != nil || !configResp.Success {
		t.Fatalf("ConfigurationDone failed: %v, resp: %+v", err, configResp)
	}

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
	var stopBody StoppedEventBody
	if err := json.Unmarshal(stopEvt.Body, &stopBody); err != nil || stopBody.Reason != "breakpoint" {
		t.Fatalf("Expected breakpoint reason in stopped body, got: %+v", stopBody)
	}

	// 6. Inspect StackTrace
	if err := WriteMessage(conn, Request{
		Message: Message{Seq: 4, Type: "request"},
		Command: "stackTrace",
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var stackResp Response
	if err := DecodeMessage(p, &stackResp); err != nil || !stackResp.Success {
		t.Fatalf("StackTrace failed: %v, resp: %+v", err, stackResp)
	}
	var stackBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
		TotalFrames int          `json:"totalFrames"`
	}
	if err := json.Unmarshal(stackResp.Body, &stackBody); err != nil {
		t.Fatal(err)
	}
	if len(stackBody.StackFrames) == 0 || stackBody.StackFrames[0].Name != "test_target_fn" {
		t.Fatalf("Stack trace frame mismatch: %+v", stackBody)
	}

	// 7. Inspect Scopes
	scopesArgs, _ := json.Marshal(map[string]any{"frameId": 0})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 5, Type: "request"},
		Command:   "scopes",
		Arguments: scopesArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var scopesResp Response
	if err := DecodeMessage(p, &scopesResp); err != nil || !scopesResp.Success {
		t.Fatalf("Scopes failed: %v, resp: %+v", err, scopesResp)
	}
	var scopesBody struct {
		Scopes []Scope `json:"scopes"`
	}
	if err := json.Unmarshal(scopesResp.Body, &scopesBody); err != nil || len(scopesBody.Scopes) != 3 {
		t.Fatalf("Scopes response mismatch: %v, body: %+v", err, scopesBody)
	}
	localsRef := scopesBody.Scopes[0].VariablesReference
	globalsRef := scopesBody.Scopes[1].VariablesReference
	edictsRef := scopesBody.Scopes[2].VariablesReference

	// 8. Inspect Variables: Locals, Globals, Edicts, and Edict drill-down
	// Locals (check parm0)
	varsArgs, _ := json.Marshal(map[string]any{"variablesReference": localsRef})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 6, Type: "request"},
		Command:   "variables",
		Arguments: varsArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var localsResp Response
	if err := DecodeMessage(p, &localsResp); err != nil || !localsResp.Success {
		t.Fatalf("Locals variables failed: %v", err)
	}
	var localsBody struct {
		Variables []Variable `json:"variables"`
	}
	if err := json.Unmarshal(localsResp.Body, &localsBody); err != nil {
		t.Fatal(err)
	}
	foundParm0 := false
	for _, v := range localsBody.Variables {
		if v.Name == "parm0" && v.Value == "77" {
			foundParm0 = true
		}
	}
	if !foundParm0 {
		t.Fatalf("Expected parm0=77 in locals: %+v", localsBody.Variables)
	}

	// Globals (check time)
	varsArgs, _ = json.Marshal(map[string]any{"variablesReference": globalsRef})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 7, Type: "request"},
		Command:   "variables",
		Arguments: varsArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var globalsResp Response
	if err := DecodeMessage(p, &globalsResp); err != nil || !globalsResp.Success {
		t.Fatalf("Globals variables failed: %v", err)
	}
	var globalsBody struct {
		Variables []Variable `json:"variables"`
	}
	if err := json.Unmarshal(globalsResp.Body, &globalsBody); err != nil {
		t.Fatal(err)
	}
	foundTime := false
	for _, v := range globalsBody.Variables {
		if v.Name == "time" && v.Value == "12.5" {
			foundTime = true
		}
	}
	if !foundTime {
		t.Fatalf("Expected time=12.5 in globals: %+v", globalsBody.Variables)
	}

	// Edicts list
	varsArgs, _ = json.Marshal(map[string]any{"variablesReference": edictsRef})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 8, Type: "request"},
		Command:   "variables",
		Arguments: varsArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var edictsResp Response
	if err := DecodeMessage(p, &edictsResp); err != nil || !edictsResp.Success {
		t.Fatalf("Edicts variables failed: %v", err)
	}
	var edictsBody struct {
		Variables []Variable `json:"variables"`
	}
	if err := json.Unmarshal(edictsResp.Body, &edictsBody); err != nil || len(edictsBody.Variables) != 2 {
		t.Fatalf("Expected 2 edicts in summary, got %+v", edictsBody.Variables)
	}
	playerEdictRef := edictsBody.Variables[1].VariablesReference

	// Edict Drill-down (inspect player fields)
	varsArgs, _ = json.Marshal(map[string]any{"variablesReference": playerEdictRef})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 9, Type: "request"},
		Command:   "variables",
		Arguments: varsArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var fieldsResp Response
	if err := DecodeMessage(p, &fieldsResp); err != nil || !fieldsResp.Success {
		t.Fatalf("Edict fields failed: %v", err)
	}
	var fieldsBody struct {
		Variables []Variable `json:"variables"`
	}
	if err := json.Unmarshal(fieldsResp.Body, &fieldsBody); err != nil {
		t.Fatal(err)
	}
	foundOrigin := false
	foundHealth := false
	for _, f := range fieldsBody.Variables {
		if f.Name == "origin" && f.Value == "[10, 20, 30]" {
			foundOrigin = true
		}
		if f.Name == "health" && f.Value == "100" {
			foundHealth = true
		}
	}
	if !foundOrigin || !foundHealth {
		t.Fatalf("Expected origin and health in edict fields: %+v", fieldsBody.Variables)
	}

	// 9. Evaluate Expressions (valid & invalid)
	evalArgs, _ := json.Marshal(map[string]any{"expression": "time"})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 10, Type: "request"},
		Command:   "evaluate",
		Arguments: evalArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var evalResp Response
	if err := DecodeMessage(p, &evalResp); err != nil || !evalResp.Success {
		t.Fatalf("Evaluate time failed: %+v", evalResp)
	}
	var evalBody struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(evalResp.Body, &evalBody); err != nil || evalBody.Result != "12.5" {
		t.Fatalf("Expected evaluate(time) == 12.5, got %s", evalBody.Result)
	}

	evalArgs, _ = json.Marshal(map[string]any{"expression": "self.health"})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 11, Type: "request"},
		Command:   "evaluate",
		Arguments: evalArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := DecodeMessage(p, &evalResp); err != nil || !evalResp.Success {
		t.Fatalf("Evaluate self.health failed: %+v", evalResp)
	}
	if err := json.Unmarshal(evalResp.Body, &evalBody); err != nil || evalBody.Result != "100" {
		t.Fatalf("Expected evaluate(self.health) == 100, got %s", evalBody.Result)
	}

	evalArgs, _ = json.Marshal(map[string]any{"expression": "unknown_var_xyz"})
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 12, Type: "request"},
		Command:   "evaluate",
		Arguments: evalArgs,
	}); err != nil {
		t.Fatal(err)
	}
	p, err = ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	if err := DecodeMessage(p, &evalResp); err != nil || evalResp.Success {
		t.Fatalf("Expected evaluate(unknown_var_xyz) to fail, got: %+v", evalResp)
	}

	// 10. Step (next)
	if err := WriteMessage(conn, Request{
		Message: Message{Seq: 13, Type: "request"},
		Command: "next",
	}); err != nil {
		t.Fatal(err)
	}
	nextPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var nextResp Response
	if err := DecodeMessage(nextPayload, &nextResp); err != nil || !nextResp.Success {
		t.Fatalf("Next response failed: %+v", nextResp)
	}

	// 11. Expect stopped event for step
	stepStopPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed reading step stopped event: %v", err)
	}
	if err := DecodeMessage(stepStopPayload, &stopEvt); err != nil || stopEvt.Event != "stopped" {
		t.Fatalf("Expected stopped event for step, got: %+v", stopEvt)
	}
	if err := json.Unmarshal(stopEvt.Body, &stopBody); err != nil || stopBody.Reason != "step" {
		t.Fatalf("Expected step reason in stopped body, got: %+v", stopBody)
	}

	// 12. Continue to end
	if err := WriteMessage(conn, Request{
		Message: Message{Seq: 14, Type: "request"},
		Command: "continue",
	}); err != nil {
		t.Fatal(err)
	}
	contPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var contResp Response
	if err := DecodeMessage(contPayload, &contResp); err != nil || !contResp.Success {
		t.Fatalf("Continue response failed: %+v", contResp)
	}

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("ExecuteFunction returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteFunction timed out")
	}
}

func TestDAPEndToEndStepInStepOutAndNestedCalls(t *testing.T) {
	vm := qc.NewVM()
	vm.Globals = make([]float32, 100)

	const calleeFuncSlot = 20
	vm.SetGInt(calleeFuncSlot, 1) // Store function index 1 in global offset 20

	// Function 0: caller_fn
	// Statements:
	// 0: OPCall0 (call function at calleeFuncSlot)
	// 1: OPDone
	vm.Functions = append(vm.Functions, qc.DFunction{
		Name:           vm.AllocString("caller_fn"),
		FirstStatement: 0,
	})

	// Function 1: callee_fn
	// Statements:
	// 2: OPAddF
	// 3: OPDone
	vm.Functions = append(vm.Functions, qc.DFunction{
		Name:           vm.AllocString("callee_fn"),
		FirstStatement: 2,
	})

	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: calleeFuncSlot}, // Stmt 0: call callee_fn
		{Op: uint16(qc.OPDone)},                     // Stmt 1: return from caller_fn
		{Op: uint16(qc.OPAddF)},                     // Stmt 2: inside callee_fn
		{Op: uint16(qc.OPDone)},                     // Stmt 3: return from callee_fn
	}

	target := &mockTarget{vm: vm}
	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	defer func() { _ = srv.Close() }()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer func() { _ = conn.Close() }()
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline failed: %v", err)
	}

	// Initialize Handshake
	if err := WriteMessage(conn, Request{Message: Message{Seq: 1, Type: "request"}, Command: "initialize"}); err != nil {
		t.Fatalf("WriteMessage initialize failed: %v", err)
	}
	initPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage initialize response failed: %v", err)
	}
	var initResp Response
	if err := DecodeMessage(initPayload, &initResp); err != nil {
		t.Fatalf("DecodeMessage initialize response failed: %v", err)
	}
	if !initResp.Success {
		t.Fatalf("Initialize response reported failure: %+v", initResp)
	}

	initEvtPayload, err := ReadMessage(conn) // initialized event
	if err != nil {
		t.Fatalf("ReadMessage initialized event failed: %v", err)
	}
	var initEvt Event
	if err := DecodeMessage(initEvtPayload, &initEvt); err != nil {
		t.Fatalf("DecodeMessage initialized event failed: %v", err)
	}
	if initEvt.Event != "initialized" {
		t.Fatalf("Expected initialized event, got: %+v", initEvt)
	}

	// Set Breakpoint on caller_fn
	args, err := json.Marshal(map[string]any{
		"breakpoints": []FunctionBreakpoint{{Name: "caller_fn"}},
	})
	if err != nil {
		t.Fatalf("json.Marshal breakpoints args failed: %v", err)
	}
	if err := WriteMessage(conn, Request{
		Message:   Message{Seq: 2, Type: "request"},
		Command:   "setFunctionBreakpoints",
		Arguments: args,
	}); err != nil {
		t.Fatalf("WriteMessage setFunctionBreakpoints failed: %v", err)
	}
	bpPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage setFunctionBreakpoints response failed: %v", err)
	}
	var bpResp Response
	if err := DecodeMessage(bpPayload, &bpResp); err != nil {
		t.Fatalf("DecodeMessage setFunctionBreakpoints response failed: %v", err)
	}
	if !bpResp.Success {
		t.Fatalf("SetFunctionBreakpoints failed: %+v", bpResp)
	}

	// ConfigurationDone
	if err := WriteMessage(conn, Request{Message: Message{Seq: 3, Type: "request"}, Command: "configurationDone"}); err != nil {
		t.Fatalf("WriteMessage configurationDone failed: %v", err)
	}
	configPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage configurationDone response failed: %v", err)
	}
	var configResp Response
	if err := DecodeMessage(configPayload, &configResp); err != nil {
		t.Fatalf("DecodeMessage configurationDone response failed: %v", err)
	}
	if !configResp.Success {
		t.Fatalf("ConfigurationDone failed: %+v", configResp)
	}

	// Execute caller_fn in background
	doneCh := make(chan error, 1)
	go func() {
		doneCh <- vm.ExecuteFunction(0)
	}()

	// 1. Expect stopped event at caller_fn entry
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline failed: %v", err)
	}
	stopPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed reading initial stop: %v", err)
	}
	var stopEvt Event
	if err := DecodeMessage(stopPayload, &stopEvt); err != nil {
		t.Fatalf("DecodeMessage initial stop event failed: %v", err)
	}
	if stopEvt.Event != "stopped" {
		t.Fatalf("Expected stopped event, got: %+v", stopEvt)
	}
	var stopBody StoppedEventBody
	if err := json.Unmarshal(stopEvt.Body, &stopBody); err != nil {
		t.Fatalf("json.Unmarshal initial stop body failed: %v", err)
	}
	if stopBody.Reason != "breakpoint" {
		t.Fatalf("Expected breakpoint reason, got: %+v", stopBody)
	}

	// 2. StepIn (step into callee_fn)
	if err := WriteMessage(conn, Request{Message: Message{Seq: 4, Type: "request"}, Command: "stepIn"}); err != nil {
		t.Fatalf("WriteMessage stepIn failed: %v", err)
	}
	stepInRespPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage stepIn response failed: %v", err)
	}
	var stepInResp Response
	if err := DecodeMessage(stepInRespPayload, &stepInResp); err != nil {
		t.Fatalf("DecodeMessage stepIn response failed: %v", err)
	}
	if !stepInResp.Success {
		t.Fatalf("StepIn response reported failure: %+v", stepInResp)
	}

	// Expect stopped event inside callee_fn
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline failed: %v", err)
	}
	stepInPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed reading stepIn stop: %v", err)
	}
	if err := DecodeMessage(stepInPayload, &stopEvt); err != nil {
		t.Fatalf("DecodeMessage stepIn stop event failed: %v", err)
	}
	if stopEvt.Event != "stopped" {
		t.Fatalf("Expected stopped event after stepIn, got: %+v", stopEvt)
	}
	if err := json.Unmarshal(stopEvt.Body, &stopBody); err != nil {
		t.Fatalf("json.Unmarshal stepIn stop body failed: %v", err)
	}
	if stopBody.Reason != "step" {
		t.Fatalf("Expected step reason, got: %+v", stopBody)
	}

	// Verify StackTrace has callee_fn on top and caller_fn beneath
	if err := WriteMessage(conn, Request{Message: Message{Seq: 5, Type: "request"}, Command: "stackTrace"}); err != nil {
		t.Fatalf("WriteMessage stackTrace failed: %v", err)
	}
	stPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage stackTrace response failed: %v", err)
	}
	var stackResp Response
	if err := DecodeMessage(stPayload, &stackResp); err != nil {
		t.Fatalf("DecodeMessage stackTrace response failed: %v", err)
	}
	if !stackResp.Success {
		t.Fatalf("StackTrace response reported failure: %+v", stackResp)
	}
	var stackBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
		TotalFrames int          `json:"totalFrames"`
	}
	if err := json.Unmarshal(stackResp.Body, &stackBody); err != nil {
		t.Fatalf("json.Unmarshal stackTrace body failed: %v", err)
	}
	if len(stackBody.StackFrames) < 2 {
		t.Fatalf("Expected at least 2 stack frames, got %d: %+v", len(stackBody.StackFrames), stackBody)
	}
	if stackBody.StackFrames[0].Name != "callee_fn" || stackBody.StackFrames[1].Name != "caller_fn" {
		t.Fatalf("Stack frames hierarchy mismatch: %+v", stackBody)
	}

	// 3. StepOut (step out of callee_fn back to caller_fn)
	if err := WriteMessage(conn, Request{Message: Message{Seq: 6, Type: "request"}, Command: "stepOut"}); err != nil {
		t.Fatalf("WriteMessage stepOut failed: %v", err)
	}
	stepOutRespPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage stepOut response failed: %v", err)
	}
	var stepOutResp Response
	if err := DecodeMessage(stepOutRespPayload, &stepOutResp); err != nil {
		t.Fatalf("DecodeMessage stepOut response failed: %v", err)
	}
	if !stepOutResp.Success {
		t.Fatalf("StepOut response reported failure: %+v", stepOutResp)
	}

	// Expect stopped event back in caller_fn
	if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("SetDeadline failed: %v", err)
	}
	stepOutPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed reading stepOut stop: %v", err)
	}
	if err := DecodeMessage(stepOutPayload, &stopEvt); err != nil {
		t.Fatalf("DecodeMessage stepOut stop event failed: %v", err)
	}
	if stopEvt.Event != "stopped" {
		t.Fatalf("Expected stopped event after stepOut, got: %+v", stopEvt)
	}
	if err := json.Unmarshal(stopEvt.Body, &stopBody); err != nil {
		t.Fatalf("json.Unmarshal stepOut stop body failed: %v", err)
	}
	if stopBody.Reason != "step" {
		t.Fatalf("Expected step reason after stepOut, got: %+v", stopBody)
	}

	// Verify StackTrace top frame is caller_fn
	if err := WriteMessage(conn, Request{Message: Message{Seq: 7, Type: "request"}, Command: "stackTrace"}); err != nil {
		t.Fatalf("WriteMessage stackTrace failed: %v", err)
	}
	stOutPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage stackTrace response failed: %v", err)
	}
	if err := DecodeMessage(stOutPayload, &stackResp); err != nil {
		t.Fatalf("DecodeMessage stackTrace response failed: %v", err)
	}
	if !stackResp.Success {
		t.Fatalf("StackTrace response reported failure: %+v", stackResp)
	}
	if err := json.Unmarshal(stackResp.Body, &stackBody); err != nil {
		t.Fatalf("json.Unmarshal stackTrace body failed: %v", err)
	}
	if len(stackBody.StackFrames) == 0 || stackBody.StackFrames[0].Name != "caller_fn" {
		t.Fatalf("Expected caller_fn top stack frame after stepOut, got: %+v", stackBody)
	}

	// 4. Continue to completion
	if err := WriteMessage(conn, Request{Message: Message{Seq: 8, Type: "request"}, Command: "continue"}); err != nil {
		t.Fatalf("WriteMessage continue failed: %v", err)
	}
	contPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("ReadMessage continue response failed: %v", err)
	}
	var contResp Response
	if err := DecodeMessage(contPayload, &contResp); err != nil {
		t.Fatalf("DecodeMessage continue response failed: %v", err)
	}
	if !contResp.Success {
		t.Fatalf("Continue response reported failure: %+v", contResp)
	}

	select {
	case err := <-doneCh:
		if err != nil {
			t.Fatalf("ExecuteFunction error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteFunction timed out")
	}
}
