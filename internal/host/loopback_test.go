package host

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
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
	lc.inner.ViewAngles = [3]float32{0, 90, 0}
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
	if lc.inner.CommandCount != 1 {
		t.Fatalf("CommandCount = %d, want 1 after loopback send", lc.inner.CommandCount)
	}

	got := s.Static.Clients[0].LastCmd
	if got.ForwardMove <= 0 {
		t.Fatalf("forwardmove = %v, want > 0", got.ForwardMove)
	}
	if got.ViewAngles != lc.inner.ViewAngles {
		t.Fatalf("view angles = %v, want %v", got.ViewAngles, lc.inner.ViewAngles)
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

func TestLocalLoopbackClientSendCommandLatchesButtonsAtSendTime(t *testing.T) {
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
	lc.inner.ViewAngles = [3]float32{0, 90, 0}

	if err := lc.Frame(0.1); err != nil {
		t.Fatalf("frame: %v", err)
	}
	if !lc.cmdReady {
		t.Fatal("command was not marked ready after frame")
	}

	// Simulate a press that happens after input accumulation but before send.
	// C Ironwail samples in_attack/in_jump in CL_SendMove, so this press must
	// still be visible to the server command.
	lc.inner.KeyDown(&lc.inner.InputAttack, 1)

	if err := lc.SendCommand(); err != nil {
		t.Fatalf("send command: %v", err)
	}
	got := s.Static.Clients[0].LastCmd
	if got.Buttons&1 == 0 {
		t.Fatalf("buttons = %d, want attack bit set from send-time latch", got.Buttons)
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
	if got := lc.LastServerMessage(); len(got) == 0 || got[0] != byte(inet.SVCStuffText) {
		t.Fatalf("LastServerMessage = %v, want recorded reliable message", got)
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
	if got := lc.LastServerMessage(); got != nil {
		t.Fatalf("LastServerMessage after second read = %v, want nil", got)
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

func TestLocalLoopbackClientPrespawnDrainsChunkedSignonBuffers(t *testing.T) {
	s := server.NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.ConnectClient(0)
	first := server.NewMessageBuffer(server.MaxDatagram)
	first.Write(bytes.Repeat([]byte{byte(inet.SVCNop)}, server.MaxDatagram-1))
	second := server.NewMessageBuffer(server.MaxDatagram)
	second.WriteByte(byte(inet.SVCNop))
	second.WriteByte(byte(inet.SVCNop))
	s.SignonBuffers = []*server.MessageBuffer{first, second}

	lc := newLocalLoopbackClient()
	lc.srv = s
	lc.cmd = s

	if err := lc.LocalServerInfo(); err != nil {
		t.Fatalf("LocalServerInfo: %v", err)
	}
	if err := lc.LocalSignonReply("prespawn"); err != nil {
		t.Fatalf("LocalSignonReply(prespawn): %v", err)
	}
	if got := lc.inner.Signon; got != 2 {
		t.Fatalf("signon after chunked prespawn = %d, want 2", got)
	}
}

type mockCommandBuffer struct {
	added        []string
	executedText []string
	executes     int
	source       cmdsys.CommandSource
}

func (m *mockCommandBuffer) Init()    {}
func (m *mockCommandBuffer) Execute() { m.executes++ }
func (m *mockCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {
	m.executes++
	m.source = source
}
func (m *mockCommandBuffer) ExecuteTextWithSource(text string, source cmdsys.CommandSource) {
	m.executedText = append(m.executedText, text)
	m.source = source
}
func (m *mockCommandBuffer) AddText(text string)    { m.added = append(m.added, text) }
func (m *mockCommandBuffer) InsertText(text string) {}
func (m *mockCommandBuffer) Shutdown()              {}

func TestDispatchLoopbackStuffTextFlushesCompleteLines(t *testing.T) {
	cmd := &mockCommandBuffer{}
	lc := newLocalLoopbackClient()
	lc.inner.StuffCmdBuf = "bf\nrecon"
	subs := &Subsystems{Client: lc, Commands: cmd}

	DispatchLoopbackStuffText(subs)
	if len(cmd.executedText) != 1 || cmd.executedText[0] != "bf\n" {
		t.Fatalf("executed text = %v, want [bf\\n]", cmd.executedText)
	}
	if cmd.executes != 0 {
		t.Fatalf("executes = %d, want 0", cmd.executes)
	}
	if cmd.source != cmdsys.SrcServer {
		t.Fatalf("command source = %v, want %v", cmd.source, cmdsys.SrcServer)
	}
	if got := lc.inner.StuffCmdBuf; got != "recon" {
		t.Fatalf("StuffCmdBuf remainder = %q, want %q", got, "recon")
	}

	lc.inner.StuffCmdBuf += "nect\n"
	DispatchLoopbackStuffText(subs)
	if len(cmd.executedText) != 2 || cmd.executedText[1] != "reconnect\n" {
		t.Fatalf("executed text after second flush = %v", cmd.executedText)
	}
}

type globalExecuteTextCommandBuffer struct{}

func (globalExecuteTextCommandBuffer) Init()    {}
func (globalExecuteTextCommandBuffer) Execute() { cmdsys.Execute() }
func (globalExecuteTextCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {
	cmdsys.ExecuteWithSource(source)
}
func (globalExecuteTextCommandBuffer) ExecuteTextWithSource(text string, source cmdsys.CommandSource) {
	cmdsys.ExecuteTextWithSource(text, source)
}
func (globalExecuteTextCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (globalExecuteTextCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (globalExecuteTextCommandBuffer) Shutdown() {}

func TestDispatchLoopbackStuffTextDoesNotDrainLocalCommandBufferAsServer(t *testing.T) {
	cmdsys.Execute()
	cmdsys.RemoveCommand("test_loopback_localcapture")
	cmdsys.RemoveCommand("test_loopback_servercapture")
	t.Cleanup(func() {
		cmdsys.RemoveCommand("test_loopback_localcapture")
		cmdsys.RemoveCommand("test_loopback_servercapture")
		cmdsys.Execute()
	})

	var seen []string
	cmdsys.AddCommand("test_loopback_localcapture", func(args []string) {
		seen = append(seen, fmt.Sprintf("local:%v", cmdsys.Source()))
	}, "")
	cmdsys.AddServerCommand("test_loopback_servercapture", func(args []string) {
		seen = append(seen, fmt.Sprintf("server:%v", cmdsys.Source()))
	}, "")

	cmdsys.AddText("test_loopback_localcapture\n")

	lc := newLocalLoopbackClient()
	lc.inner.StuffCmdBuf = "test_loopback_servercapture\n"
	subs := &Subsystems{Client: lc, Commands: globalExecuteTextCommandBuffer{}}

	DispatchLoopbackStuffText(subs)
	if got, want := seen, []string{"server:2"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seen after stufftext dispatch = %v, want %v", got, want)
	}

	cmdsys.Execute()
	if got, want := seen, []string{"server:2", "local:0"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("seen after draining local buffer = %v, want %v", got, want)
	}
}
