package dap

import (
	"encoding/json"
	"fmt"
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
	defer func() { _ = conn.Close() }()

	var writeMu sync.Mutex
	writeSafe := func(msg any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return WriteMessage(conn, msg)
	}

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
		_ = writeSafe(event)
	}

	for {
		payload, err := ReadMessage(conn)
		if err != nil {
			session.Disconnect()
			return
		}

		var req Request
		if err := DecodeMessage(payload, &req); err != nil {
			continue
		}

		s.dispatchRequest(writeSafe, req)
	}
}

func (s *Server) dispatchRequest(write func(msg any) error, req Request) {
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
		_ = write(resp)

		// Send initialized event
		initEvent := Event{
			Message: Message{Seq: session.NextSeq(), Type: "event"},
			Event:   "initialized",
		}
		_ = write(initEvent)
		return

	case "attach", "launch":
		// Success empty body
	case "configurationDone":
		session.SetInitialized(true)
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
			resp.ErrorMessage = err.Error()
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
		_ = write(resp)
		return

	default:
		resp.Success = false
		resp.ErrorMessage = fmt.Sprintf("unsupported command %s", req.Command)
	}

	_ = write(resp)
}
