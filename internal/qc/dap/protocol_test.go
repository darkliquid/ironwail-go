package dap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestDAPFramingReadWrite(t *testing.T) {
	req := Request{
		Message: Message{
			Seq:  1,
			Type: "request",
		},
		Command:   "initialize",
		Arguments: json.RawMessage(`{"clientID":"vscode"}`),
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
	if string(parsed.Arguments) != string(req.Arguments) {
		t.Fatalf("Arguments mismatch: got %s, want %s", string(parsed.Arguments), string(req.Arguments))
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

func TestDAPResponseAndEvent(t *testing.T) {
	res := Response{
		Message:    Message{Seq: 3, Type: "response"},
		RequestSeq: 1,
		Success:    true,
		Command:    "initialize",
		Body:       json.RawMessage(`{"supportsConfigurationDoneRequest":true}`),
	}

	var buf bytes.Buffer
	if err := WriteMessage(&buf, res); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	payload, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	var parsedRes Response
	if err := DecodeMessage(payload, &parsedRes); err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	if !reflect.DeepEqual(parsedRes, res) {
		t.Fatalf("Response mismatch: got %+v, want %+v", parsedRes, res)
	}

	evt := Event{
		Message: Message{Seq: 4, Type: "event"},
		Event:   "stopped",
		Body:    json.RawMessage(`{"reason":"breakpoint","threadId":1}`),
	}
	buf.Reset()
	if err := WriteMessage(&buf, evt); err != nil {
		t.Fatalf("WriteMessage failed: %v", err)
	}

	payload, err = ReadMessage(&buf)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}

	var parsedEvt Event
	if err := DecodeMessage(payload, &parsedEvt); err != nil {
		t.Fatalf("DecodeMessage failed: %v", err)
	}

	if !reflect.DeepEqual(parsedEvt, evt) {
		t.Fatalf("Event mismatch: got %+v, want %+v", parsedEvt, evt)
	}
}

func TestDAPDataTypesJSON(t *testing.T) {
	caps := Capabilities{
		SupportsConfigurationDoneRequest: true,
		SupportsFunctionBreakpoints:      true,
		SupportsEvaluateForHovers:        true,
		SupportsStepBack:                 false,
	}
	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("Marshal Caps failed: %v", err)
	}
	var caps2 Capabilities
	if err := json.Unmarshal(data, &caps2); err != nil {
		t.Fatalf("Unmarshal Caps failed: %v", err)
	}
	if !reflect.DeepEqual(caps, caps2) {
		t.Fatalf("Caps mismatch: got %+v, want %+v", caps2, caps)
	}

	frame := StackFrame{
		ID:     1,
		Name:   "player_stand1",
		Source: &Source{Name: "player.qc", Path: "/path/to/player.qc"},
		Line:   42,
		Column: 1,
	}
	data, err = json.Marshal(frame)
	if err != nil {
		t.Fatalf("Marshal StackFrame failed: %v", err)
	}
	var frame2 StackFrame
	if err := json.Unmarshal(data, &frame2); err != nil {
		t.Fatalf("Unmarshal StackFrame failed: %v", err)
	}
	if !reflect.DeepEqual(frame, frame2) {
		t.Fatalf("StackFrame mismatch: got %+v, want %+v", frame2, frame)
	}

	stopped := StoppedEventBody{
		Reason:            "breakpoint",
		Description:       "Paused on breakpoint",
		ThreadID:          1,
		PreserveFocusHint: false,
		AllThreadsStopped: true,
	}
	data, err = json.Marshal(stopped)
	if err != nil {
		t.Fatalf("Marshal StoppedEventBody failed: %v", err)
	}
	var stopped2 StoppedEventBody
	if err := json.Unmarshal(data, &stopped2); err != nil {
		t.Fatalf("Unmarshal StoppedEventBody failed: %v", err)
	}
	if !reflect.DeepEqual(stopped, stopped2) {
		t.Fatalf("StoppedEventBody mismatch: got %+v, want %+v", stopped2, stopped)
	}
}

func TestReadMessageErrors(t *testing.T) {
	// Missing Content-Length header
	inputNoHeader := "Header: Value\r\n\r\n{}"
	_, err := ReadMessage(strings.NewReader(inputNoHeader))
	if err == nil || !strings.Contains(err.Error(), "missing Content-Length") {
		t.Fatalf("Expected missing Content-Length error, got %v", err)
	}

	// Invalid Content-Length header
	inputInvalidHeader := "Content-Length: abc\r\n\r\n{}"
	_, err = ReadMessage(strings.NewReader(inputInvalidHeader))
	if err == nil || !strings.Contains(err.Error(), "invalid Content-Length") {
		t.Fatalf("Expected invalid Content-Length error, got %v", err)
	}

	// Truncated body (unexpected EOF)
	inputTruncated := "Content-Length: 50\r\n\r\n{\"short\":1}"
	_, err = ReadMessage(strings.NewReader(inputTruncated))
	if err == nil || !errors.Is(err, io.ErrUnexpectedEOF) && !strings.Contains(err.Error(), "failed reading body") {
		t.Fatalf("Expected truncated body error, got %v", err)
	}

	// Header case insensitivity and extra headers
	inputCustomHeaders := "X-Extra: header\r\ncontent-length: 4\r\nOther: test\r\n\r\ntest"
	payload, err := ReadMessage(strings.NewReader(inputCustomHeaders))
	if err != nil {
		t.Fatalf("Expected success with case-insensitive header, got %v", err)
	}
	if string(payload) != "test" {
		t.Fatalf("Expected payload 'test', got %q", string(payload))
	}
}

func TestWriteMessageError(t *testing.T) {
	// Channels cannot be marshaled to JSON
	ch := make(chan int)
	var buf bytes.Buffer
	err := WriteMessage(&buf, ch)
	if err == nil {
		t.Fatal("Expected error when marshaling unmarshalable type, got nil")
	}

	// Writer failure on header
	err = WriteMessage(&failingWriter{failOn: 0}, Request{})
	if err == nil {
		t.Fatal("Expected error when writer fails on header")
	}

	// Writer failure on body
	err = WriteMessage(&failingWriter{failOn: 1}, Request{})
	if err == nil {
		t.Fatal("Expected error when writer fails on body")
	}
}

type plainReader struct {
	data []byte
	pos  int
}

func (r *plainReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

type failingWriter struct {
	calls  int
	failOn int
}

func (w *failingWriter) Write(p []byte) (n int, err error) {
	if w.calls == w.failOn {
		return 0, io.ErrClosedPipe
	}
	w.calls++
	return len(p), nil
}

func TestReadMessagePlainReader(t *testing.T) {
	raw := "Content-Length: 5\r\n\r\nhello"
	pr := &plainReader{data: []byte(raw)}
	payload, err := ReadMessage(pr)
	if err != nil {
		t.Fatalf("ReadMessage failed: %v", err)
	}
	if string(payload) != "hello" {
		t.Fatalf("Got %s, want hello", string(payload))
	}
}

func TestOtherDAPTypesJSON(t *testing.T) {
	scope := Scope{
		Name:               "Locals",
		VariablesReference: 10,
		Expensive:          false,
	}
	data, err := json.Marshal(scope)
	if err != nil {
		t.Fatal(err)
	}
	var scope2 Scope
	if err := json.Unmarshal(data, &scope2); err != nil || scope2 != scope {
		t.Fatalf("Scope mismatch: got %+v, want %+v", scope2, scope)
	}

	v := Variable{
		Name:               "self",
		Value:              "entity 1 (player)",
		Type:               "entity",
		VariablesReference: 2,
	}
	data, err = json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var v2 Variable
	if err := json.Unmarshal(data, &v2); err != nil || v2 != v {
		t.Fatalf("Variable mismatch: got %+v, want %+v", v2, v)
	}

	fb := FunctionBreakpoint{Name: "player_stand1"}
	data, err = json.Marshal(fb)
	if err != nil {
		t.Fatal(err)
	}
	var fb2 FunctionBreakpoint
	if err := json.Unmarshal(data, &fb2); err != nil || fb2 != fb {
		t.Fatalf("FunctionBreakpoint mismatch: got %+v, want %+v", fb2, fb)
	}

	bp := Breakpoint{
		ID:       1,
		Verified: true,
		Message:  "verified",
		Source:   &Source{Name: "player.qc", Path: "/qc/player.qc"},
		Line:     10,
	}
	data, err = json.Marshal(bp)
	if err != nil {
		t.Fatal(err)
	}
	var bp2 Breakpoint
	if err := json.Unmarshal(data, &bp2); err != nil || !reflect.DeepEqual(bp2, bp) {
		t.Fatalf("Breakpoint mismatch: got %+v, want %+v", bp2, bp)
	}
}
