package dap

import (
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
	RequestSeq   int             `json:"request_seq"`
	Success      bool            `json:"success"`
	Command      string          `json:"command"`
	ErrorMessage string          `json:"message,omitempty"`
	Body         json.RawMessage `json:"body,omitempty"`
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
	SupportsBreakpointsRequest       bool `json:"supportsBreakpointsRequest"`
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
	contentLength := -1
	for {
		line, err := readHeaderLine(r)
		if err != nil {
			return nil, err
		}
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
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("failed reading body: %w", err)
	}
	return payload, nil
}

func readHeaderLine(r io.Reader) (string, error) {
	var line []byte
	var b [1]byte
	for {
		var c byte
		if br, ok := r.(io.ByteReader); ok {
			var err error
			c, err = br.ReadByte()
			if err != nil {
				if len(line) > 0 && err == io.EOF {
					return string(line), nil
				}
				return "", err
			}
		} else {
			if _, err := io.ReadFull(r, b[:]); err != nil {
				if len(line) > 0 && err == io.EOF {
					return string(line), nil
				}
				return "", err
			}
			c = b[0]
		}
		if c == '\n' {
			break
		}
		line = append(line, c)
	}
	return strings.TrimRight(string(line), "\r"), nil
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
