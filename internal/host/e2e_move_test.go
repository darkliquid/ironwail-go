package host

import (
	"bytes"
	"testing"

	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/server"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

// setupE2ELoopback creates a fully-initialized Host+Server+loopback client
// with the "start" map loaded, returning the components for testing.
func setupE2ELoopback(t *testing.T) (*Host, *server.Server, *Subsystems, *cl.Client) {
	t.Helper()
	quakeDir := testutil.SkipIfNoQuakeDir(t)

	h := NewHost()
	fileSys := fs.NewFileSystem()
	srv := server.NewServer()
	subs := &Subsystems{
		Files:   fileSys,
		Console: &mockConsole{},
		Server:  srv,
	}
	SetupLoopbackClientServer(subs, srv)

	if err := h.Init(&InitParams{
		BaseDir:    quakeDir,
		GameDir:    "id1",
		MaxClients: 1,
	}, subs); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { fileSys.Close() })

	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("LoadFile(progs.dat): %v", err)
	}
	if err := srv.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(srv.QCVM)

	if err := h.CmdMap("start", subs); err != nil {
		t.Fatalf("CmdMap(start): %v", err)
	}

	clientState := LoopbackClientState(subs)
	if clientState == nil {
		t.Fatal("no client state")
	}

	return h, srv, subs, clientState
}

func TestE2ELoopbackMovement(t *testing.T) {
	_, srv, subs, clientState := setupE2ELoopback(t)

	t.Logf("Client: State=%v Signon=%v ViewEntity=%d", clientState.State, clientState.Signon, clientState.ViewEntity)

	if ent, ok := clientState.Entities[clientState.ViewEntity]; ok {
		t.Logf("Client ViewEntity %d: Origin=%v ModelIndex=%v", clientState.ViewEntity, ent.Origin, ent.ModelIndex)
	}

	serverEnt := srv.Static.Clients[0].Edict
	t.Logf("Server entity: Origin=%v MoveType=%v Health=%v Velocity=%v Flags=%v",
		serverEnt.Vars.Origin, serverEnt.Vars.MoveType, serverEnt.Vars.Health,
		serverEnt.Vars.Velocity, uint32(serverEnt.Vars.Flags))

	lc := subs.Client.(*localLoopbackClient)

	// Set up forward input
	clientState.ViewAngles = [3]float32{0, 0, 0}
	clientState.ForwardSpeed = 400
	clientState.SideSpeed = 350
	clientState.InputForward.State = 1 // key held
	clientState.MoveSpeedKey = 1

	clientState.AccumulateCmd(0.1)
	t.Logf("PendingCmd: Forward=%v Side=%v Up=%v ViewAngles=%v",
		clientState.PendingCmd.Forward, clientState.PendingCmd.Side, clientState.PendingCmd.Up, clientState.PendingCmd.ViewAngles)

	lc.cmdReady = true
	clientState.State = cl.StateActive
	clientState.Signon = cl.Signons
	if err := lc.SendCommand(); err != nil {
		t.Fatalf("SendCommand: %v", err)
	}

	t.Logf("Server LastCmd: Forward=%v Side=%v ViewAngles=%v",
		srv.Static.Clients[0].LastCmd.ForwardMove, srv.Static.Clients[0].LastCmd.SideMove,
		srv.Static.Clients[0].LastCmd.ViewAngles)

	srv.Frame(0.1)
	t.Logf("Server entity after Frame: Origin=%v Velocity=%v",
		serverEnt.Vars.Origin, serverEnt.Vars.Velocity)

	if err := lc.ReadFromServer(); err != nil {
		t.Fatalf("ReadFromServer: %v", err)
	}

	if ent, ok := clientState.Entities[clientState.ViewEntity]; ok {
		t.Logf("Client ViewEntity after frame: Origin=%v", ent.Origin)
	} else {
		t.Log("Client ViewEntity STILL not found after frame")
	}
	t.Logf("Client Velocity after frame: %v", clientState.Velocity)
}

// TestE2EHostFrameMovement uses Host.Frame() with callbacks, simulating the
// exact runtime flow: GetEvents→AccumulateCmd→SendCommand→ServerFrame.
// The key press is delivered via KeyDown just as the GLFW callback would.
func TestE2EHostFrameMovement(t *testing.T) {
	h, srv, subs, clientState := setupE2ELoopback(t)

	// Record starting position
	startOrigin := srv.Static.Clients[0].Edict.Vars.Origin
	t.Logf("Start origin: %v", startOrigin)
	t.Logf("Client state: State=%v Signon=%v", clientState.State, clientState.Signon)

	// Simulate the key press: "+forward" with key code 119 ('w')
	// This mirrors what handleGameKeyEvent does at runtime.
	clientState.KeyDown(&clientState.InputForward, 119)
	t.Logf("After KeyDown: InputForward.State=%v Down=%v",
		clientState.InputForward.State, clientState.InputForward.Down)

	clientState.ViewAngles = [3]float32{0, 0, 0}
	clientState.ForwardSpeed = 400
	clientState.MoveSpeedKey = 1
	clientState.SideSpeed = 350

	// Use Host.Frame with real callbacks that drive the loopback pipeline
	frameCount := 0
	cb := &testFrameCallbacks{
		getEvents: func() {
			_ = subs.Client.Frame(h.FrameTime())
		},
		processConsoleCommands: func() {
			DispatchLoopbackStuffText(subs)
		},
		processClient: func() {
			_ = subs.Client.ReadFromServer()
			_ = subs.Client.SendCommand()
		},
		processServer: func() {
			dt := h.FrameTime()
			srv.Frame(dt)
		},
	}

	// Run several frames through Host.Frame
	for i := 0; i < 10; i++ {
		if err := h.Frame(1.0/72.0, cb); err != nil {
			t.Fatalf("Frame %d: %v", i, err)
		}
		frameCount++
	}

	endOrigin := srv.Static.Clients[0].Edict.Vars.Origin
	t.Logf("After %d frames: Origin=%v (was %v)", frameCount, endOrigin, startOrigin)
	t.Logf("Client Velocity: %v", clientState.Velocity)
	t.Logf("Server entity Velocity: %v", srv.Static.Clients[0].Edict.Vars.Velocity)

	// The origin should have changed — player should have moved forward
	dx := endOrigin[0] - startOrigin[0]
	dy := endOrigin[1] - startOrigin[1]
	dz := endOrigin[2] - startOrigin[2]
	totalDist := dx*dx + dy*dy
	t.Logf("Displacement: dx=%v dy=%v dz=%v (lateral dist²=%v)", dx, dy, dz, totalDist)

	if totalDist < 0.01 {
		t.Error("Player did not move laterally after 10 frames with +forward held")
		// Debug: check what happened frame by frame
		t.Logf("  PendingCmd: Forward=%v Side=%v", clientState.PendingCmd.Forward, clientState.PendingCmd.Side)
		t.Logf("  InputForward.State=%v Down=%v", clientState.InputForward.State, clientState.InputForward.Down)
		t.Logf("  Server LastCmd: Forward=%v Side=%v",
			srv.Static.Clients[0].LastCmd.ForwardMove, srv.Static.Clients[0].LastCmd.SideMove)
		t.Logf("  Server Paused: %v", srv.Paused)
		t.Logf("  Client Active=%v Spawned=%v",
			srv.Static.Clients[0].Active, srv.Static.Clients[0].Spawned)
	}
}

// testFrameCallbacks implements FrameCallbacks for tests.
type testFrameCallbacks struct {
	getEvents              func()
	processConsoleCommands func()
	processClient          func()
	processServer          func()
}

func (c *testFrameCallbacks) GetEvents() {
	if c.getEvents != nil {
		c.getEvents()
	}
}
func (c *testFrameCallbacks) ProcessConsoleCommands() {
	if c.processConsoleCommands != nil {
		c.processConsoleCommands()
	}
}
func (c *testFrameCallbacks) ProcessClient() {
	if c.processClient != nil {
		c.processClient()
	}
}
func (c *testFrameCallbacks) ProcessServer() {
	if c.processServer != nil {
		c.processServer()
	}
}
func (c *testFrameCallbacks) UpdateScreen()                                     {}
func (c *testFrameCallbacks) UpdateAudio(origin, forward, right, up [3]float32) {}
