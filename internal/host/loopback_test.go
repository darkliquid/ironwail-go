package host

import (
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/server"
)

func TestLocalLoopbackClientFrameAndSendCommand(t *testing.T) {
	s := server.NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true

	lc := newLocalLoopbackClient()
	lc.srv = s
	lc.cmd = s
	lc.inner.State = cl.StateActive
	lc.inner.Time = 1.25
	lc.inner.InputForward.State = 1
	lc.inner.InputAttack.State = 3
	lc.inner.InputJump.State = 3
	lc.inner.InImpulse = 7

	if err := lc.Frame(0.1); err != nil {
		t.Fatalf("frame: %v", err)
	}
	if !lc.cmdReady {
		t.Fatal("command was not marked ready after frame")
	}

	if err := lc.SendCommand(); err != nil {
		t.Fatalf("send command: %v", err)
	}
	if lc.cmdReady {
		t.Fatal("command still marked ready after send")
	}

	got := s.Static.Clients[0].LastCmd
	if got.ForwardMove <= 0 {
		t.Fatalf("forwardmove = %v, want > 0", got.ForwardMove)
	}
	if got.Buttons != 3 {
		t.Fatalf("buttons = %d, want 3", got.Buttons)
	}
	if got.Impulse != 7 {
		t.Fatalf("impulse = %d, want 7", got.Impulse)
	}
	if s.Static.Clients[0].Edict.Vars.Button0 != 1 || s.Static.Clients[0].Edict.Vars.Button2 != 1 {
		t.Fatalf("edict button state = (%v,%v), want (1,1)", s.Static.Clients[0].Edict.Vars.Button0, s.Static.Clients[0].Edict.Vars.Button2)
	}
	if s.Static.Clients[0].NumPings != 1 {
		t.Fatalf("NumPings = %d, want 1", s.Static.Clients[0].NumPings)
	}

	if err := lc.SendCommand(); err != nil {
		t.Fatalf("second send command: %v", err)
	}
	if s.Static.Clients[0].NumPings != 1 {
		t.Fatalf("NumPings after duplicate send = %d, want 1", s.Static.Clients[0].NumPings)
	}
}

func TestLocalLoopbackClientReadFromServerConsumesReliableMessage(t *testing.T) {
	s := server.NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = false
	s.Static.Clients[0].Message.WriteByte(byte(inet.SVCStuffText))
	s.Static.Clients[0].Message.WriteString("bf\n")

	lc := newLocalLoopbackClient()
	lc.srv = s
	lc.inner.State = cl.StateActive

	if err := lc.ReadFromServer(); err != nil {
		t.Fatalf("ReadFromServer: %v", err)
	}
	if got := lc.inner.StuffCmdBuf; got != "bf\n" {
		t.Fatalf("StuffCmdBuf = %q, want %q", got, "bf\n")
	}
	if s.Static.Clients[0].Message.Len() != 0 {
		t.Fatalf("reliable message buffer len = %d, want 0", s.Static.Clients[0].Message.Len())
	}

	if err := lc.ReadFromServer(); err != nil {
		t.Fatalf("second ReadFromServer: %v", err)
	}
	if got := lc.inner.StuffCmdBuf; got != "bf\n" {
		t.Fatalf("StuffCmdBuf after second read = %q, want unchanged", got)
	}
}

func TestLocalLoopbackClientRealSignonFlow(t *testing.T) {
	s := server.NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ConnectClient(0)

	lc := newLocalLoopbackClient()
	lc.srv = s
	lc.cmd = s

	if err := lc.LocalServerInfo(); err != nil {
		t.Fatalf("LocalServerInfo: %v", err)
	}
	if got := lc.inner.Signon; got != 1 {
		t.Fatalf("signon after serverinfo = %d, want 1", got)
	}
	if got := lc.State(); got != caConnected {
		t.Fatalf("state after serverinfo = %v, want connected", got)
	}
	if err := lc.LocalSignonReply("begin"); err == nil {
		t.Fatal("begin succeeded before spawn")
	}

	for _, step := range []struct {
		cmd        string
		wantSignon int
	}{
		{cmd: "prespawn", wantSignon: 2},
		{cmd: "spawn", wantSignon: 3},
		{cmd: "begin", wantSignon: 4},
	} {
		if err := lc.LocalSignonReply(step.cmd); err != nil {
			t.Fatalf("LocalSignonReply(%q): %v", step.cmd, err)
		}
		if got := lc.inner.Signon; got != step.wantSignon {
			t.Fatalf("signon after %s = %d, want %d", step.cmd, got, step.wantSignon)
		}
	}

	if got := lc.State(); got != caActive {
		t.Fatalf("final client state = %v, want active", got)
	}
	if !s.Static.Clients[0].Spawned {
		t.Fatal("server client not marked spawned after begin")
	}
}

type mockCommandBuffer struct {
	added    []string
	executes int
}

func (m *mockCommandBuffer) Init()                  {}
func (m *mockCommandBuffer) Execute()               { m.executes++ }
func (m *mockCommandBuffer) AddText(text string)    { m.added = append(m.added, text) }
func (m *mockCommandBuffer) InsertText(text string) {}
func (m *mockCommandBuffer) Shutdown()              {}

func TestDispatchLoopbackStuffTextFlushesCompleteLines(t *testing.T) {
	cmd := &mockCommandBuffer{}
	lc := newLocalLoopbackClient()
	lc.inner.StuffCmdBuf = "bf\nrecon"
	subs := &Subsystems{Client: lc, Commands: cmd}

	DispatchLoopbackStuffText(subs)
	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if cmd.executes != 1 {
		t.Fatalf("executes = %d, want 1", cmd.executes)
	}
	if got := lc.inner.StuffCmdBuf; got != "recon" {
		t.Fatalf("StuffCmdBuf remainder = %q, want %q", got, "recon")
	}

	lc.inner.StuffCmdBuf += "nect\n"
	DispatchLoopbackStuffText(subs)
	if len(cmd.added) != 2 || cmd.added[1] != "reconnect\n" {
		t.Fatalf("added commands after second flush = %v", cmd.added)
	}
	if cmd.executes != 2 {
		t.Fatalf("executes after second flush = %d, want 2", cmd.executes)
	}
}
