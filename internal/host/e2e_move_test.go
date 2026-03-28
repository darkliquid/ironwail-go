package host

import (
	"bytes"
	"fmt"
	"testing"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/server"
	"github.com/darkliquid/ironwail-go/internal/testutil"
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

func TestE2EJump(t *testing.T) {
	_, srv, _, clientState := setupE2ELoopback(t)

	serverEnt := srv.Static.Clients[0].Edict
	entNum := srv.NumForEdict(serverEnt)

	// Look up QC field offsets from progs.dat
	fieldOfs := map[string]int{}
	for _, def := range srv.QCVM.FieldDefs {
		name := srv.QCVM.GetString(def.Name)
		if name != "" {
			fieldOfs[name] = int(def.Ofs)
		}
	}
	flagsOfs := fieldOfs["flags"]
	button2Ofs := fieldOfs["button2"]
	viewOfsOfs := fieldOfs["view_ofs"]
	waterLevelOfs := fieldOfs["waterlevel"]
	waterJumpOfs := fieldOfs["teleport_time"]
	moveTypeOfs := fieldOfs["movetype"]
	t.Logf("Field offsets: flags=%d button2=%d view_ofs=%d waterlevel=%d teleport_time=%d movetype=%d",
		flagsOfs, button2Ofs, viewOfsOfs, waterLevelOfs, waterJumpOfs, moveTypeOfs)

	// Dump ALL field defs to check for unexpected offsets
	t.Logf("All QC field defs (offset → name):")
	for _, def := range srv.QCVM.FieldDefs {
		name := srv.QCVM.GetString(def.Name)
		if name != "" {
			t.Logf("  ofs=%d type=%d name=%s", def.Ofs, def.Type, name)
		}
	}

	t.Logf("Initial: Origin=%v Flags=0x%x ViewOfs=%v MoveType=%v Health=%v",
		serverEnt.Vars.Origin, uint32(serverEnt.Vars.Flags),
		serverEnt.Vars.ViewOfs, serverEnt.Vars.MoveType, serverEnt.Vars.Health)

	// Settle until player lands (FL_ONGROUND set)
	landed := false
	for i := 0; i < 30; i++ {
		if err := srv.Frame(1.0 / 72.0); err != nil {
			t.Fatalf("settle Frame %d: %v", i, err)
		}
		flags := uint32(serverEnt.Vars.Flags)
		if i < 3 || flags&server.FlagOnGround != 0 {
			t.Logf("  frame %d: z=%.3f vel_z=%.3f Flags=0x%x JumpReleased=%v",
				i, serverEnt.Vars.Origin[2], serverEnt.Vars.Velocity[2],
				flags, flags&server.FlagJumpReleased != 0)
		}
		if flags&server.FlagOnGround != 0 && !landed {
			landed = true
			t.Logf("  >>> Player landed on frame %d", i)

			// Inspect VM state right after the frame where landing happened
			// (syncEdictFromQCVM already ran, so VM has post-frame state)
			vmFlags := srv.QCVM.EFloat(entNum, flagsOfs)
			vmViewOfs := srv.QCVM.EVector(entNum, viewOfsOfs)
			vmButton2 := srv.QCVM.EFloat(entNum, button2Ofs)
			vmWaterLvl := srv.QCVM.EFloat(entNum, waterLevelOfs)
			vmMoveType := srv.QCVM.EFloat(entNum, moveTypeOfs)
			t.Logf("  VM after landing: flags=0x%x view_ofs=%v button2=%v waterlevel=%v movetype=%v",
				uint32(vmFlags), vmViewOfs, vmButton2, vmWaterLvl, vmMoveType)

			// Run a few more frames with button2=0 to let FL_JUMPRELEASED get set
			for j := 0; j < 5; j++ {
				if err := srv.Frame(1.0 / 72.0); err != nil {
					t.Fatalf("post-land Frame %d: %v", j, err)
				}
				flags2 := uint32(serverEnt.Vars.Flags)
				t.Logf("  post-land frame %d: Flags=0x%x JumpReleased=%v OnGround=%v",
					j, flags2, flags2&server.FlagJumpReleased != 0, flags2&server.FlagOnGround != 0)
				if flags2&server.FlagJumpReleased != 0 {
					t.Logf("  >>> FL_JUMPRELEASED set on post-land frame %d", j)
					break
				}
			}
			break
		}
	}

	if !landed {
		t.Fatal("Player never landed after 30 frames")
	}

	flags := uint32(serverEnt.Vars.Flags)
	if flags&server.FlagJumpReleased == 0 {
		t.Logf("FL_JUMPRELEASED NOT set after landing — investigating QC early return")

		// Check QC globals that could cause early return
		intermissionIdx := srv.QCVM.FindGlobal("intermission_running")
		if intermissionIdx >= 0 {
			intermissionVal := srv.QCVM.GFloat(intermissionIdx)
			t.Logf("QC global 'intermission_running' = %v (idx=%d)", intermissionVal, intermissionIdx)
		} else {
			t.Logf("QC global 'intermission_running' not found (idx=%d)", intermissionIdx)
		}

		// Manually write all relevant Go edict fields to VM for PlayerPreThink
		srv.QCVM.SetEFloat(entNum, flagsOfs, serverEnt.Vars.Flags)
		srv.QCVM.SetEVector(entNum, viewOfsOfs, serverEnt.Vars.ViewOfs)
		srv.QCVM.SetEFloat(entNum, button2Ofs, serverEnt.Vars.Button2)
		srv.QCVM.SetEFloat(entNum, waterLevelOfs, serverEnt.Vars.WaterLevel)
		srv.QCVM.SetEFloat(entNum, waterJumpOfs, serverEnt.Vars.TeleportTime)
		srv.QCVM.SetEFloat(entNum, moveTypeOfs, serverEnt.Vars.MoveType)
		srv.QCVM.SetEVector(entNum, fieldOfs["origin"], serverEnt.Vars.Origin)
		srv.QCVM.SetEVector(entNum, fieldOfs["velocity"], serverEnt.Vars.Velocity)
		srv.QCVM.SetEFloat(entNum, fieldOfs["health"], serverEnt.Vars.Health)

		// Check ALL relevant VM fields that could cause early return in PlayerPreThink
		vmViewOfs := srv.QCVM.EVector(entNum, viewOfsOfs)
		vmFlags := srv.QCVM.EFloat(entNum, flagsOfs)
		vmButton2 := srv.QCVM.EFloat(entNum, button2Ofs)
		vmWaterLvl := srv.QCVM.EFloat(entNum, waterLevelOfs)
		vmWaterJump := srv.QCVM.EFloat(entNum, waterJumpOfs)
		vmMoveType := srv.QCVM.EFloat(entNum, moveTypeOfs)

		t.Logf("Pre-PlayerPreThink VM state (after explicit sync):")
		t.Logf("  flags=0x%x view_ofs=%v button2=%v waterlevel=%v teleport_time=%v movetype=%v",
			uint32(vmFlags), vmViewOfs, vmButton2, vmWaterLvl, vmWaterJump, vmMoveType)

		// Compare Go edict values
		t.Logf("Go edict state:")
		t.Logf("  Flags=0x%x ViewOfs=%v Button2=%v WaterLevel=%v TeleportTime=%v MoveType=%v",
			uint32(serverEnt.Vars.Flags), serverEnt.Vars.ViewOfs, serverEnt.Vars.Button2,
			serverEnt.Vars.WaterLevel, serverEnt.Vars.TeleportTime, serverEnt.Vars.MoveType)

		// Run PlayerPreThink manually with QC tracing
		preThink := srv.QCVM.FindFunction("PlayerPreThink")
		srv.QCVM.Time = float64(srv.Time)
		srv.QCVM.SetGlobal("time", srv.Time)
		srv.QCVM.SetGlobal("self", entNum)
		srv.QCVM.SetGlobal("other", 0)

		// Enable tracing to see branches and function calls
		srv.QCVM.Trace = true
		var traceLines []string
		srv.QCVM.TraceFunc = func(vm *qc.VM, stmtIdx int, st *qc.DStatement, op qc.Opcode) {
			fn := ""
			if vm.XFunction != nil {
				fn = vm.GetString(vm.XFunction.Name)
			}
			switch op {
			case qc.OPIF:
				val := vm.GFloat(int(st.A))
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: IF g[%d]=%.4f → %v (jump %d)",
					fn, stmtIdx, st.A, val, val != 0, int16(st.B)))
			case qc.OPIFNot:
				val := vm.GFloat(int(st.A))
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: IFNOT g[%d]=%.4f → %v (jump %d)",
					fn, stmtIdx, st.A, val, val == 0, int16(st.B)))
			case qc.OPDone, qc.OPReturn:
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: RETURN (depth=%d)", fn, stmtIdx, vm.Depth))
			case qc.OPCall0, qc.OPCall1, qc.OPCall2, qc.OPCall3, qc.OPCall4, qc.OPCall5, qc.OPCall6, qc.OPCall7, qc.OPCall8:
				funcIdx := vm.GInt(int(st.A))
				callee := ""
				if int(funcIdx) > 0 && int(funcIdx) < len(vm.Functions) {
					callee = vm.GetString(vm.Functions[int(funcIdx)].Name)
				}
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: CALL%d → %s",
					fn, stmtIdx, op-qc.OPCall0, callee))
			case qc.OPGoto:
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: GOTO %d",
					fn, stmtIdx, int16(st.A)))
			case qc.OPBitOr:
				a, b := vm.GFloat(int(st.A)), vm.GFloat(int(st.B))
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: BITOR g[%d]=%v g[%d]=%v → g[%d]",
					fn, stmtIdx, st.A, a, st.B, b, st.C))
			case qc.OPBitAnd:
				a, b := vm.GFloat(int(st.A)), vm.GFloat(int(st.B))
				traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: BITAND g[%d]=%v g[%d]=%v → g[%d]",
					fn, stmtIdx, st.A, a, st.B, b, st.C))
			default:
				// Trace ALL opcodes within PlayerPreThink for stmts 6640-6670
				if fn == "PlayerPreThink" && stmtIdx >= 6640 && stmtIdx <= 6670 {
					traceLines = append(traceLines, fmt.Sprintf("  [%s] stmt %d: op=%d a=%d b=%d c=%d",
						fn, stmtIdx, op, st.A, st.B, st.C))
				}
			}
		}
		if err := srv.QCVM.ExecuteFunction(preThink); err != nil {
			t.Fatalf("PlayerPreThink failed: %v", err)
		}
		srv.QCVM.Trace = false
		srv.QCVM.TraceFunc = nil

		t.Logf("QC trace (%d entries):", len(traceLines))
		for _, line := range traceLines {
			t.Log(line)
		}

		// Dump the PlayerPreThink function definition
		preThinkFunc := &srv.QCVM.Functions[preThink]
		t.Logf("PlayerPreThink func: firstStatement=%d numParms=%d locals=%d name=%s file=%s",
			preThinkFunc.FirstStatement, preThinkFunc.NumParms, preThinkFunc.Locals,
			srv.QCVM.GetString(preThinkFunc.Name), srv.QCVM.GetString(preThinkFunc.File))
		// Dump first 20 statements of PlayerPreThink
		startStmt := int(preThinkFunc.FirstStatement)
		t.Logf("Statements starting at %d:", startStmt)
		for i := 0; i < 20 && startStmt+i < len(srv.QCVM.Statements); i++ {
			st := &srv.QCVM.Statements[startStmt+i]
			op := qc.Opcode(st.Op)
			t.Logf("  [%d] op=%d(%v) a=%d b=%d c=%d", startStmt+i, st.Op, op, st.A, st.B, st.C)
			if op == qc.OPDone || op == qc.OPReturn {
				break
			}
		}

		vmFlagsAfter := srv.QCVM.EFloat(entNum, flagsOfs)
		t.Logf("After manual PlayerPreThink: VM flags=0x%x JumpReleased=%v",
			uint32(vmFlagsAfter), uint32(vmFlagsAfter)&server.FlagJumpReleased != 0)

		t.Error("FL_JUMPRELEASED never set — jump cannot work")
	}

	// Test actual jump
	if flags&server.FlagJumpReleased != 0 {
		t.Logf("Attempting jump with natural FL_JUMPRELEASED...")
		if err := srv.SubmitLoopbackCmd(0, clientState.ViewAngles, 0, 0, 0, 2, 0, float64(srv.Time)); err != nil {
			t.Fatalf("SubmitLoopbackCmd: %v", err)
		}
		preZ := serverEnt.Vars.Origin[2]
		if err := srv.Frame(1.0 / 72.0); err != nil {
			t.Fatalf("Frame: %v", err)
		}
		t.Logf("After jump: z=%.3f (was %.3f) vel_z=%.3f Flags=0x%x",
			serverEnt.Vars.Origin[2], preZ, serverEnt.Vars.Velocity[2], uint32(serverEnt.Vars.Flags))
		if serverEnt.Vars.Velocity[2] > 0 {
			t.Logf("Jump successful!")
		} else {
			t.Error("Jump failed despite FL_JUMPRELEASED being set")
		}
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
