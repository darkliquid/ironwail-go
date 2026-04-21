package game

import (
	"bytes"
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/menu"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestApplyDemoPlaybackViewAnglesUpdatesCurrentAndPreviousAngles(t *testing.T) {
	g := New()
	clientState := cl.NewClient()
	clientState.MViewAngles[0] = [3]float32{1, 2, 3}
	clientState.ViewAngles = [3]float32{4, 5, 6}

	g.applyDemoPlaybackViewAngles(clientState, [3]float32{10, 20, 30})

	if clientState.MViewAngles[1] != [3]float32{1, 2, 3} {
		t.Fatalf("previous demo angles = %v, want [1 2 3]", clientState.MViewAngles[1])
	}
	if clientState.MViewAngles[0] != [3]float32{10, 20, 30} {
		t.Fatalf("current demo angles = %v, want [10 20 30]", clientState.MViewAngles[0])
	}
	if clientState.ViewAngles != [3]float32{10, 20, 30} {
		t.Fatalf("view angles = %v, want [10 20 30]", clientState.ViewAngles)
	}
}

func TestDemoPlaybackReadsOneFramePerHostFrame(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	originalClient := g.Client
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Client = originalClient
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("single_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("single_step", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil || !clientState.DemoPlayback || clientState.TimeDemoActive {
		t.Fatalf("demo flags at start = %#v, want demo playback true and timedemo false", clientState)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after second host frame = %d, want 2", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame eof: %v", err)
	}
	if demo.Playback {
		t.Fatal("expected demo playback to stop at EOF")
	}
	if clientState.DemoPlayback || clientState.TimeDemoActive {
		t.Fatalf("demo flags after EOF = demo:%v timedemo:%v, want both false", clientState.DemoPlayback, clientState.TimeDemoActive)
	}
}

func TestDemoPlaybackEOFQueuesNextPlaylistDemo(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("playlist_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	cmdBuf := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmdBuf,
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.SetDemoList([]string{"demo2"})
	g.Host.SetDemoNum(0)
	g.Host.CmdPlaydemo("playlist_step", g.Subs)

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame eof: %v", err)
	}

	demo := g.Host.DemoState()
	if demo == nil {
		t.Fatal("expected demo state")
	}
	if demo.Playback {
		t.Fatal("expected playback to stop before queued playlist advance")
	}
	if len(cmdBuf.added) == 0 || cmdBuf.added[len(cmdBuf.added)-1] != "demos\n" {
		t.Fatalf("queued commands = %q, want trailing demos command", cmdBuf.added)
	}
}

func TestPausedDemoPlaybackDoesNotReadFrames(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("paused", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("paused", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	demo.Paused = true

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}
	if demo.FrameIndex != 0 {
		t.Fatalf("frame index while paused = %d, want 0", demo.FrameIndex)
	}
}

func TestDemoPlaybackNegativeSpeedRewindsOneFrame(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("rewind_step", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := recorder.WriteDemoFrame([]byte{0xff}, [3]float32{float32(i), 0, 0}); err != nil {
			t.Fatalf("WriteDemoFrame %d: %v", i, err)
		}
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("rewind_step", g.Subs)

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}
	demo.EnableTimeDemo()

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if got := demo.FrameIndex; got != 2 {
		t.Fatalf("frame index before rewind = %d, want 2", got)
	}

	demo.SetSpeed(-1)
	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame rewind: %v", err)
	}
	if got := demo.FrameIndex; got != 1 {
		t.Fatalf("frame index after rewind = %d, want 1", got)
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame backstop: %v", err)
	}
	if got := demo.FrameIndex; got != 1 {
		t.Fatalf("frame index at rewind backstop = %d, want 1", got)
	}
	if !demo.RewindBackstop() {
		t.Fatal("expected rewind backstop after rewinding to the first frame")
	}
}

func TestDemoPlaybackWaitsForRecordedServerTime(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	writeDemoTimeFrame := func(seconds float32) []byte {
		var frame bytes.Buffer
		frame.WriteByte(byte(inet.SVCTime))
		if err := binary.Write(&frame, binary.LittleEndian, seconds); err != nil {
			t.Fatalf("binary.Write(time): %v", err)
		}
		frame.WriteByte(0xff)
		return frame.Bytes()
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timed", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.1), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.2), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("timed", g.Subs)

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback {
		t.Fatal("expected active demo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index after first host frame = %d, want 1", demo.FrameIndex)
	}

	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 1 {
		t.Fatalf("frame index before recorded time elapses = %d, want 1", demo.FrameIndex)
	}

	for i := 0; i < 6; i++ {
		if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
			t.Fatalf("Host.Frame catch-up %d: %v", i, err)
		}
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after recorded time elapses = %d, want 2", demo.FrameIndex)
	}
}

func TestDemoPlaybackTimeDemoIgnoresRecordedServerTime(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	originalClient := g.Client
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Client = originalClient
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	writeDemoTimeFrame := func(seconds float32) []byte {
		var msg bytes.Buffer
		msg.WriteByte(byte(inet.SVCTime))
		if err := binary.Write(&msg, binary.LittleEndian, seconds); err != nil {
			t.Fatalf("Write(time): %v", err)
		}
		msg.WriteByte(0xff)
		return msg.Bytes()
	}

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("timedemo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(0.1), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame first: %v", err)
	}
	if err := recorder.WriteDemoFrame(writeDemoTimeFrame(2.0), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame second: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Server: &demoPlaybackNoopServer{}, Console: &demoPlaybackConsole{}}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdTimedemo("timedemo", g.Subs)

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	if !clientState.DemoPlayback || !clientState.TimeDemoActive {
		t.Fatalf("timedemo flags at start = demo:%v timedemo:%v, want both true", clientState.DemoPlayback, clientState.TimeDemoActive)
	}
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons

	demo := g.Host.DemoState()
	if demo == nil || !demo.Playback || !demo.TimeDemo {
		t.Fatal("expected active timedemo playback")
	}

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame first: %v", err)
	}
	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame second: %v", err)
	}
	if demo.FrameIndex != 2 {
		t.Fatalf("frame index after timedemo frames = %d, want 2", demo.FrameIndex)
	}

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame eof: %v", err)
	}
	if demo.Playback {
		t.Fatal("expected timedemo playback to stop at EOF")
	}
	output := strings.Join(g.Subs.Console.(*demoPlaybackConsole).messages, "")
	if !strings.Contains(output, "timedemo: 1 frames") {
		t.Fatalf("console output = %q, want timedemo summary", output)
	}
	if clientState.DemoPlayback || clientState.TimeDemoActive {
		t.Fatalf("timedemo flags after EOF = demo:%v timedemo:%v, want both false", clientState.DemoPlayback, clientState.TimeDemoActive)
	}
}

func TestLoadWorldModelAndLitUsesOptionalLoader(t *testing.T) {
	world, lit, err := loadWorldModelAndLit(demoBootstrapLitFS{
		worldData: []byte("bsp"),
		litData:   []byte("lit"),
	}, "maps/start.bsp")
	if err != nil {
		t.Fatalf("loadWorldModelAndLit error: %v", err)
	}
	if got := string(world); got != "bsp" {
		t.Fatalf("world data = %q, want %q", got, "bsp")
	}
	if got := string(lit); got != "lit" {
		t.Fatalf("lit data = %q, want %q", got, "lit")
	}
}

func TestDemoPlaybackBootstrapsWorldAfterServerInfo(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	originalServer := g.Server
	originalClient := g.Client
	originalInput := g.Input
	originalMenu := g.Menu
	originalGrabbed := g.MouseGrabbed
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Server = originalServer
		g.Client = originalClient
		g.Input = originalInput
		g.Menu = originalMenu
		g.MouseGrabbed = originalGrabbed
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	serverInfoMsg := bytes.NewBuffer(nil)
	serverInfoMsg.WriteByte(byte(inet.SVCServerInfo))
	if err := binary.Write(serverInfoMsg, binary.LittleEndian, int32(inet.PROTOCOL_FITZQUAKE)); err != nil {
		t.Fatalf("binary.Write(protocol): %v", err)
	}
	serverInfoMsg.WriteByte(1)
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteString("Demo Test")
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteString("maps/start.bsp")
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteByte(0)
	serverInfoMsg.WriteByte(0)

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("demo_bootstrap", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(serverInfoMsg.Bytes(), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{byte(inet.SVCSignOnNum), 0x02}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame(signon2): %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{byte(inet.SVCSignOnNum), 0x03}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame(signon3): %v", err)
	}
	if err := recorder.WriteDemoFrame([]byte{byte(inet.SVCTime), 0, 0, 0, 0}, [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame(time): %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	wantTree := &bsp.Tree{Models: []bsp.DModel{{}}}
	var loadedModel string
	previousLoadDemoWorldTree := loadDemoWorldTree
	loadDemoWorldTree = func(files host.Filesystem, worldModel string) (*bsp.Tree, error) {
		loadedModel = worldModel
		return wantTree, nil
	}
	defer func() { loadDemoWorldTree = previousLoadDemoWorldTree }()

	g.Host = host.NewHost()
	g.Server = &server.Server{}
	g.Subs = &host.Subsystems{
		Server:  &demoPlaybackNoopServer{},
		Console: &demoPlaybackConsole{},
		Files:   demoBootstrapTestFS{},
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.MouseGrabbed = false
	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Input.OnMenuChar = g.handleMenuCharEvent
	g.Input.OnKey = g.handleGameKeyEvent
	g.Input.OnChar = g.handleGameCharEvent
	g.registerGameplayBindCommands()
	g.applyDefaultGameplayBindings()
	g.Menu.ShowMenu()
	g.syncGameplayInputMode()
	g.Host.CmdPlaydemo("demo_bootstrap", g.Subs)

	for i := 0; i < 4; i++ {
		if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
			t.Fatalf("Host.Frame(%d): %v", i, err)
		}
	}
	if loadedModel != "maps/start.bsp" {
		t.Fatalf("loaded model = %q, want maps/start.bsp", loadedModel)
	}
	if g.Server.ModelName != "maps/start.bsp" {
		t.Fatalf("server model name = %q, want maps/start.bsp", g.Server.ModelName)
	}
	if g.Server.WorldTree != wantTree {
		t.Fatalf("server world tree = %p, want %p", g.Server.WorldTree, wantTree)
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil || clientState.State != cl.StateActive || clientState.Signon != cl.Signons {
		t.Fatalf("client state/signon = %#v, want active/%d", clientState, cl.Signons)
	}
	if g.Menu.IsActive() {
		t.Fatal("expected startup menu to hide once demo playback became active")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after demo startup = %v, want game", got)
	}
}

func TestDemoPlaybackFlushesStuffTextSameFrame(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	message := bytes.NewBuffer(nil)
	message.WriteByte(byte(inet.SVCStuffText))
	message.WriteString("bf\n")
	message.WriteByte(0)
	message.WriteByte(0xff)

	recorder := cl.NewDemoState()
	if err := recorder.StartDemoRecording("stuffcmd", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	if err := recorder.WriteDemoFrame(message.Bytes(), [3]float32{}); err != nil {
		t.Fatalf("WriteDemoFrame: %v", err)
	}
	if err := recorder.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	cmd := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}
	g.Host.CmdPlaydemo("stuffcmd", g.Subs)

	if err := g.Host.Frame(0.016, gameCallbacks{g: g}); err != nil {
		t.Fatalf("Host.Frame: %v", err)
	}

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if cmd.executes < 2 {
		t.Fatalf("executes = %d, want at least 2", cmd.executes)
	}
	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after same-frame flush", clientState.StuffCmdBuf)
	}
}

func TestProcessClientFlushesLiveStuffTextSameFrame(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
	})

	cmd := &demoPlaybackCommandBuffer{}
	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{
		Server:   &demoPlaybackNoopServer{},
		Console:  &demoPlaybackConsole{},
		Commands: cmd,
	}
	tmpDir := t.TempDir()
	if err := g.Host.Init(&host.InitParams{BaseDir: tmpDir, UserDir: tmpDir}, g.Subs); err != nil {
		t.Fatalf("Host.Init: %v", err)
	}

	clientState := host.LoopbackClientState(g.Subs)
	if clientState == nil {
		t.Fatal("expected loopback client state")
	}
	clientState.StuffCmdBuf = "bf\n"

	gameCallbacks{g: g}.ProcessClient()

	if len(cmd.added) != 1 || cmd.added[0] != "bf\n" {
		t.Fatalf("added commands = %v, want [bf\\n]", cmd.added)
	}
	if clientState.StuffCmdBuf != "" {
		t.Fatalf("StuffCmdBuf = %q, want empty after live-frame flush", clientState.StuffCmdBuf)
	}
}

func TestProcessClientSendPhaseOnlySendsCommand(t *testing.T) {
	g := New()
	originalSubs := g.Subs
	originalPhase := runtimeProcessClientPhase
	t.Cleanup(func() {
		g.Subs = originalSubs
		runtimeProcessClientPhase = originalPhase
	})

	client := &processClientPhaseTestClient{state: host.ClientState(3)}
	g.Subs = &host.Subsystems{Client: client}
	runtimeProcessClientPhase = "send"

	gameCallbacks{g: g}.ProcessClient()

	if client.sendCalls != 1 || client.readCalls != 0 {
		t.Fatalf("send/read calls = %d/%d, want 1/0", client.sendCalls, client.readCalls)
	}
}

func TestProcessClientReadPhaseOnlyReadsServer(t *testing.T) {
	g := New()
	originalSubs := g.Subs
	originalPhase := runtimeProcessClientPhase
	t.Cleanup(func() {
		g.Subs = originalSubs
		runtimeProcessClientPhase = originalPhase
	})

	client := &processClientPhaseTestClient{state: host.ClientState(3)}
	g.Subs = &host.Subsystems{Client: client}
	runtimeProcessClientPhase = "read"

	gameCallbacks{g: g}.ProcessClient()

	if client.sendCalls != 0 || client.readCalls != 1 {
		t.Fatalf("send/read calls = %d/%d, want 0/1", client.sendCalls, client.readCalls)
	}
}

func TestProcessClientAppliesGameplayInputWhenClientBecomesActive(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalSubs := g.Subs
	originalInput := g.Input
	originalMenu := g.Menu
	originalClient := g.Client
	originalGrabbed := g.MouseGrabbed
	originalPhase := runtimeProcessClientPhase
	t.Cleanup(func() {
		g.Host = originalHost
		g.Subs = originalSubs
		g.Input = originalInput
		g.Menu = originalMenu
		g.Client = originalClient
		g.MouseGrabbed = originalGrabbed
		runtimeProcessClientPhase = originalPhase
	})

	clientState := cl.NewClient()
	clientState.State = cl.StateConnected
	clientState.Signon = cl.Signons - 1
	client := &activatingProcessClientTestClient{
		state:       host.ClientState(2),
		clientState: clientState,
	}

	g.Host = host.NewHost()
	g.Subs = &host.Subsystems{Client: client}
	g.Input = input.NewSystem(nil)
	g.Menu = menu.NewManager(nil, g.Input)
	g.MouseGrabbed = false

	g.Input.OnMenuKey = g.handleMenuKeyEvent
	g.Input.OnMenuChar = g.handleMenuCharEvent
	g.Input.OnKey = g.handleGameKeyEvent
	g.Input.OnChar = g.handleGameCharEvent
	g.registerGameplayBindCommands()
	g.applyDefaultGameplayBindings()
	g.Menu.ShowMenu()
	g.syncGameplayInputMode()
	runtimeProcessClientPhase = "read"

	gameCallbacks{g: g}.ProcessClient()

	if client.readCalls != 1 || client.sendCalls != 0 {
		t.Fatalf("send/read calls = %d/%d, want 0/1", client.sendCalls, client.readCalls)
	}
	if g.Menu.IsActive() {
		t.Fatal("menu should hide when client becomes active during ProcessClient")
	}
	if got := g.Input.GetKeyDest(); got != input.KeyGame {
		t.Fatalf("key destination after activation = %v, want game", got)
	}
	if !g.MouseGrabbed {
		t.Fatal("mouse should be grabbed when client becomes active during ProcessClient")
	}
}

func TestRecordRuntimeDemoFrameWritesLatestServerMessage(t *testing.T) {
	g := New()
	originalHost := g.Host
	originalClient := g.Client
	originalSubs := g.Subs
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	t.Cleanup(func() {
		g.Host = originalHost
		g.Client = originalClient
		g.Subs = originalSubs
		_ = os.Chdir(cwd)
	})

	tmpDir := t.TempDir()
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}

	g.Host = host.NewHost()
	demo := cl.NewDemoState()
	if err := demo.StartDemoRecording("runtime_demo", 0); err != nil {
		t.Fatalf("StartDemoRecording: %v", err)
	}
	t.Cleanup(func() {
		_ = demo.StopRecording()
	})
	g.Host.SetDemoState(demo)

	g.Client = cl.NewClient()
	g.Client.ViewAngles = [3]float32{10, 20, 30}
	g.Subs = &host.Subsystems{Client: &demoMessageClient{message: []byte{1, 2, 3}}}

	g.recordRuntimeDemoFrame()
	if err := demo.StopRecording(); err != nil {
		t.Fatalf("StopRecording: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "demos", "runtime_demo.dem"))
	if err != nil {
		t.Fatalf("ReadFile(demo): %v", err)
	}
	newline := bytes.IndexByte(data, '\n')
	if newline < 0 || string(data[:newline+1]) != "0\n" {
		t.Fatalf("demo header = %q, want %q", string(data), "0\\n")
	}

	reader := bytes.NewReader(data[newline+1:])
	var msgSize int32
	if err := binary.Read(reader, binary.LittleEndian, &msgSize); err != nil {
		t.Fatalf("Read(msgSize): %v", err)
	}
	if msgSize != 3 {
		t.Fatalf("msgSize = %d, want 3", msgSize)
	}
	for i, want := range [3]float32{10, 20, 30} {
		var got float32
		if err := binary.Read(reader, binary.LittleEndian, &got); err != nil {
			t.Fatalf("Read(viewAngle %d): %v", i, err)
		}
		if got != want {
			t.Fatalf("view angle %d = %v, want %v", i, got, want)
		}
	}
	frame := make([]byte, msgSize)
	if _, err := reader.Read(frame); err != nil {
		t.Fatalf("Read(frame): %v", err)
	}
	if !bytes.Equal(frame, []byte{1, 2, 3}) {
		t.Fatalf("frame = %v, want [1 2 3]", frame)
	}
}

func TestRuntimeAngleVectorsYawNinety(t *testing.T) {
	g := New()
	forward, right, up := g.runtimeAngleVectors([3]float32{0, 90, 0})
	if math.Abs(float64(forward[0])) > 0.0001 || math.Abs(float64(forward[1]-1)) > 0.0001 || math.Abs(float64(forward[2])) > 0.0001 {
		t.Fatalf("forward = %v, want [0 1 0]", forward)
	}
	if math.Abs(float64(right[0]-1)) > 0.0001 || math.Abs(float64(right[1])) > 0.0001 || math.Abs(float64(right[2])) > 0.0001 {
		t.Fatalf("right = %v, want [1 0 0]", right)
	}
	if math.Abs(float64(up[0])) > 0.0001 || math.Abs(float64(up[1])) > 0.0001 || math.Abs(float64(up[2]-1)) > 0.0001 {
		t.Fatalf("up = %v, want [0 0 1]", up)
	}
}
