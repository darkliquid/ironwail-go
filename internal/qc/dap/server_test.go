package dap

import (
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/qc"
)

func TestDAPServerInitializeAndThreads(t *testing.T) {
	vm := qc.NewVM()
	target := &mockTarget{vm: vm}

	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = srv.Close() }()

	addr := srv.Addr().String()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect to DAP server: %v", err)
	}
	defer func() { _ = conn.Close() }()

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

	// Verify initialized event is sent right after initialize response
	eventPayload, err := ReadMessage(conn)
	if err != nil {
		t.Fatal(err)
	}
	var initEvent Event
	if err := DecodeMessage(eventPayload, &initEvent); err != nil || initEvent.Event != "initialized" {
		t.Fatalf("Expected initialized event, got err=%v, event=%+v", err, initEvent)
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
	if body.Threads[0].ID != 1 || body.Threads[0].Name != "QuakeC VM Thread" {
		t.Fatalf("Unexpected thread details: %+v", body.Threads[0])
	}
}

func TestDAPServerAllCommands(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("main"), FirstStatement: 0},
		{Name: vm.AllocString("player_stand"), FirstStatement: 10},
	}
	vm.Statements = make([]qc.DStatement, 30)
	vm.Globals = make([]float32, 100)
	vm.Globals[qc.OFSTime] = 42.0
	vm.GlobalDefs = []qc.DDef{
		{Name: vm.AllocString("time"), Ofs: uint16(qc.OFSTime), Type: uint16(qc.EvFloat)},
	}

	target := &mockTarget{vm: vm}

	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = srv.Close() }()

	if srv.Session() == nil {
		t.Fatal("Expected srv.Session() to not be nil")
	}

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	sendReq := func(cmd string, args any) Response {
		var rawArgs json.RawMessage
		if args != nil {
			rawArgs, _ = json.Marshal(args)
		}
		req := Request{
			Message:   Message{Seq: 100, Type: "request"},
			Command:   cmd,
			Arguments: rawArgs,
		}
		if err := WriteMessage(conn, req); err != nil {
			t.Fatalf("Failed to send request %s: %v", cmd, err)
		}
		payload, err := ReadMessage(conn)
		if err != nil {
			t.Fatalf("Failed to read response for %s: %v", cmd, err)
		}
		var resp Response
		if err := DecodeMessage(payload, &resp); err != nil {
			t.Fatalf("Failed to decode response for %s: %v", cmd, err)
		}
		return resp
	}

	// 1. attach & launch
	resp := sendReq("attach", nil)
	if !resp.Success {
		t.Fatalf("attach failed: %+v", resp)
	}
	resp = sendReq("launch", nil)
	if !resp.Success {
		t.Fatalf("launch failed: %+v", resp)
	}

	// 2. configurationDone
	resp = sendReq("configurationDone", nil)
	if !resp.Success {
		t.Fatalf("configurationDone failed: %+v", resp)
	}
	if !srv.Session().Initialized() {
		t.Fatal("Expected session to be initialized after configurationDone")
	}

	// 3. setFunctionBreakpoints
	resp = sendReq("setFunctionBreakpoints", map[string]any{
		"breakpoints": []FunctionBreakpoint{
			{Name: "player_stand"},
			{Name: "nonexistent_fn"},
		},
	})
	if !resp.Success {
		t.Fatalf("setFunctionBreakpoints failed: %+v", resp)
	}
	var bpBody struct {
		Breakpoints []Breakpoint `json:"breakpoints"`
	}
	if err := json.Unmarshal(resp.Body, &bpBody); err != nil || len(bpBody.Breakpoints) != 2 {
		t.Fatalf("Breakpoints response mismatch: %v, body=%+v", err, bpBody)
	}
	if !bpBody.Breakpoints[0].Verified || bpBody.Breakpoints[0].Line != 10 {
		t.Errorf("Expected verified player_stand breakpoint, got %+v", bpBody.Breakpoints[0])
	}
	if bpBody.Breakpoints[1].Verified {
		t.Errorf("Expected unverified nonexistent breakpoint, got %+v", bpBody.Breakpoints[1])
	}

	// 4. stackTrace
	vm.XFunction = &vm.Functions[1]
	vm.XStatement = 12
	resp = sendReq("stackTrace", nil)
	if !resp.Success {
		t.Fatalf("stackTrace failed: %+v", resp)
	}
	var stBody struct {
		StackFrames []StackFrame `json:"stackFrames"`
		TotalFrames int          `json:"totalFrames"`
	}
	if err := json.Unmarshal(resp.Body, &stBody); err != nil || stBody.TotalFrames != 1 {
		t.Fatalf("stackTrace body mismatch: %v, body=%+v", err, stBody)
	}

	// 5. scopes
	resp = sendReq("scopes", map[string]any{"frameId": 0})
	if !resp.Success {
		t.Fatalf("scopes failed: %+v", resp)
	}
	var scBody struct {
		Scopes []Scope `json:"scopes"`
	}
	if err := json.Unmarshal(resp.Body, &scBody); err != nil || len(scBody.Scopes) != 3 {
		t.Fatalf("scopes mismatch: %v, body=%+v", err, scBody)
	}

	// 6. variables
	resp = sendReq("variables", map[string]any{"variablesReference": scBody.Scopes[1].VariablesReference})
	if !resp.Success {
		t.Fatalf("variables failed: %+v", resp)
	}
	var varsBody struct {
		Variables []Variable `json:"variables"`
	}
	if err := json.Unmarshal(resp.Body, &varsBody); err != nil || len(varsBody.Variables) == 0 {
		t.Fatalf("variables mismatch: %v, body=%+v", err, varsBody)
	}

	// 7. evaluate valid & invalid
	resp = sendReq("evaluate", map[string]any{"expression": "time"})
	if !resp.Success {
		t.Fatalf("evaluate valid expression failed: %+v", resp)
	}
	var evalBody struct {
		Result             string `json:"result"`
		VariablesReference int    `json:"variablesReference"`
	}
	if err := json.Unmarshal(resp.Body, &evalBody); err != nil || evalBody.Result != "42" {
		t.Fatalf("evaluate result mismatch: %v, body=%+v", err, evalBody)
	}

	resp = sendReq("evaluate", map[string]any{"expression": "unknown_var_xyz"})
	if resp.Success || resp.ErrorMessage == "" {
		t.Fatalf("evaluate invalid expression should fail with error message, got %+v", resp)
	}

	// 8. stepping commands (continue, next, stepIn, stepOut, pause)
	resp = sendReq("continue", nil)
	if !resp.Success {
		t.Fatalf("continue failed: %+v", resp)
	}

	resp = sendReq("next", nil)
	if !resp.Success {
		t.Fatalf("next failed: %+v", resp)
	}

	resp = sendReq("stepIn", nil)
	if !resp.Success {
		t.Fatalf("stepIn failed: %+v", resp)
	}

	resp = sendReq("stepOut", nil)
	if !resp.Success {
		t.Fatalf("stepOut failed: %+v", resp)
	}

	resp = sendReq("pause", nil)
	if !resp.Success {
		t.Fatalf("pause failed: %+v", resp)
	}

	// 9. unsupported command
	resp = sendReq("nonexistentCommand", nil)
	if resp.Success || resp.ErrorMessage == "" {
		t.Fatalf("unsupported command should fail, got %+v", resp)
	}

	// 10. disconnect
	resp = sendReq("disconnect", nil)
	if !resp.Success {
		t.Fatalf("disconnect failed: %+v", resp)
	}
}

func TestDAPServerStoppedEventOnBreakpoint(t *testing.T) {
	vm := qc.NewVM()
	vm.Functions = []qc.DFunction{
		{Name: vm.AllocString("main"), FirstStatement: 0},
	}
	vm.Statements = make([]qc.DStatement, 10)
	target := &mockTarget{vm: vm}

	srv, err := NewServer("127.0.0.1:0", target)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer func() { _ = srv.Close() }()

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() { _ = conn.Close() }()

	// Initialize handshake
	_ = WriteMessage(conn, Request{Message: Message{Seq: 1, Type: "request"}, Command: "initialize"})
	_, _ = ReadMessage(conn) // init resp
	_, _ = ReadMessage(conn) // initialized event

	// Set function breakpoint on main
	_ = WriteMessage(conn, Request{
		Message: Message{Seq: 2, Type: "request"},
		Command: "setFunctionBreakpoints",
		Arguments: json.RawMessage(`{"breakpoints":[{"name":"main"}]}`),
	})
	_, _ = ReadMessage(conn)

	// Trigger hook in a goroutine
	vm.XFunction = &vm.Functions[0]
	hook := vm.BreakHook
	if hook == nil {
		t.Fatal("Expected vm.BreakHook to be wired by NewServer")
	}

	hookDone := make(chan struct{})
	go func() {
		hook(vm, 0)
		close(hookDone)
	}()

	// DAP client should receive stopped event
	payload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read stopped event: %v", err)
	}

	var stoppedEvent Event
	if err := DecodeMessage(payload, &stoppedEvent); err != nil || stoppedEvent.Event != "stopped" {
		t.Fatalf("Expected stopped event, got err=%v, event=%+v", err, stoppedEvent)
	}

	var stoppedBody StoppedEventBody
	if err := json.Unmarshal(stoppedEvent.Body, &stoppedBody); err != nil {
		t.Fatalf("Failed to unmarshal stopped body: %v", err)
	}
	if stoppedBody.Reason != "breakpoint" || stoppedBody.ThreadID != 1 || !stoppedBody.AllThreadsStopped {
		t.Fatalf("Unexpected stopped event body: %+v", stoppedBody)
	}

	// Client sends continue request to unblock hook
	_ = WriteMessage(conn, Request{Message: Message{Seq: 3, Type: "request"}, Command: "continue"})
	_, _ = ReadMessage(conn) // continue resp

	select {
	case <-hookDone:
		// Success!
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Hook did not resume after continue request")
	}
}

func TestDAPServerErrorsAndEdgeCases(t *testing.T) {
	// 1. Invalid listen address
	_, err := NewServer("invalid_ip_addr_that_fails:999999", nil)
	if err == nil {
		t.Fatal("Expected error with invalid listen address")
	}

	// 2. Nil target server
	srv, err := NewServer("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("Failed to create server with nil target: %v", err)
	}

	conn, err := net.Dial("tcp", srv.Addr().String())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	// Send malformed JSON message (valid framing, invalid JSON body)
	malformedHeader := "Content-Length: 15\r\n\r\n{not valid json"
	_, _ = conn.Write([]byte(malformedHeader))

	// Follow up with valid initialize request to verify server is still responsive
	_ = WriteMessage(conn, Request{Message: Message{Seq: 1, Type: "request"}, Command: "initialize"})
	payload, err := ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read after malformed message: %v", err)
	}
	var initResp Response
	if err := DecodeMessage(payload, &initResp); err != nil || !initResp.Success {
		t.Fatalf("Expected valid init response, got err=%v, resp=%+v", err, initResp)
	}
	_, _ = ReadMessage(conn) // init event

	// StackTrace and Evaluate with nil target
	_ = WriteMessage(conn, Request{Message: Message{Seq: 2, Type: "request"}, Command: "stackTrace"})
	payload, err = ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read stackTrace response: %v", err)
	}
	var stResp Response
	if err := DecodeMessage(payload, &stResp); err != nil || !stResp.Success {
		t.Fatalf("Expected success for stackTrace with nil target, got %+v", stResp)
	}

	_ = WriteMessage(conn, Request{Message: Message{Seq: 3, Type: "request"}, Command: "evaluate", Arguments: json.RawMessage(`{"expression":"foo"}`)})
	payload, err = ReadMessage(conn)
	if err != nil {
		t.Fatalf("Failed to read evaluate response: %v", err)
	}
	var evalResp Response
	if err := DecodeMessage(payload, &evalResp); err != nil || evalResp.Success {
		t.Fatalf("Expected evaluate on nil target to fail, got %+v", evalResp)
	}

	_ = conn.Close()

	// 3. Multiple Close calls
	if err := srv.Close(); err != nil {
		t.Fatalf("First Close() failed: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("Second Close() failed: %v", err)
	}
}

