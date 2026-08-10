package host

import (
	"bytes"
	"fmt"
	"testing"

	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/server"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

// TestQbj3E2EDiagnostics drives the real single-player loopback on
// qbj3_pixeldud and reports exactly what the client parser sees for the
// equipped weapon (stat 2 -> ModelPrecache) and for pickup entities, so the
// root cause of the invisible weapon / invisible keycard regressions can be
// observed end-to-end.
func TestQbj3E2EDiagnostics(t *testing.T) {
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
		GameDir:    "qbj3",
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

	if err := h.CmdMap("qbj3_pixeldud", subs); err != nil {
		t.Fatalf("CmdMap(qbj3_pixeldud): %v", err)
	}

	clientState := LoopbackClientState(subs)
	if clientState == nil {
		t.Fatal("no client state")
	}

	t.Logf("After CmdMap: State=%v Signon=%d ViewEntity=%d", clientState.State, clientState.Signon, clientState.ViewEntity)

	// Dump the model precache list mapping indices to names.
	precache := func(idx int) string {
		p := idx - 1
		if p < 0 || p >= len(clientState.ModelPrecache) {
			return fmt.Sprintf("<invalid idx %d, len=%d>", idx, len(clientState.ModelPrecache))
		}
		return fmt.Sprintf("%q", clientState.ModelPrecache[p])
	}

	// Frame pump: server sim + loopback client read, mirroring Host.Frame's
	// ProcessClient/ProcessServer order from TestE2EHostFrameMovement.
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
			_ = srv.Frame(h.FrameTime())
		},
	}

	frames := 220 // ~3s at 72Hz; enough for ItemPlace to run and pickups to settle
	keycardPresent := 0
	keycardCurrent := 0
	for i := 0; i < frames; i++ {
		if err := h.Frame(1.0/72.0, cb); err != nil {
			t.Fatalf("Frame %d: %v", i, err)
		}
		if es, ok := clientState.Entities[16]; ok {
			keycardPresent++
			if es.MsgTime == clientState.MTime[0] {
				keycardCurrent++
			}
		}
		if i%60 == 0 || i == frames-1 {
			weapon := clientState.Stats[inet.StatWeapon]
			active := clientState.Stats[inet.StatActiveWeapon]
			t.Logf("frame %3d: State=%v signon=%d weaponStat=%d aktiv=%d weapon=%s viewEntAlpha=%d health=%d items=0x%x",
				i, clientState.State, clientState.Signon, int(weapon), int(active), precache(int(weapon)),
				clientState.ViewEntAlpha, int(clientState.Stats[inet.StatHealth]), clientState.Items)
		}
	}
	t.Logf("keycard entity 16 present in %d/%d frames, MsgTime-current in %d/%d",
		keycardPresent, frames, keycardCurrent, frames)

	weapon := clientState.Stats[inet.StatWeapon]
	t.Logf("FINAL weaponStat=%d activeWeaponStat=%d weapon=%s",
		int(weapon), int(clientState.Stats[inet.StatActiveWeapon]), precache(int(weapon)))

	// Enumerate client-side entities with models to see pickups/renderables.
	modelCount := 0
	for entNum, es := range clientState.Entities {
		if es.ModelIndex > 0 {
			modelCount++
			if entNum <= 40 || modelCount <= 40 {
				t.Logf("entity %3d modelIdx=%-3d (%s) frame=%d alpha=%d eff=%#x",
					entNum, int(es.ModelIndex), precache(int(es.ModelIndex)), int(es.Frame), int(es.Alpha), int(es.Effects))
			}
		}
	}
	t.Logf("total client entities with visible models: %d", modelCount)

	// Report server-side visibility: keycard (edict 16) and world items.
	if es, ok := clientState.Entities[16]; ok {
		t.Logf("client entity 16 present: modelIdx=%d -> %s alphaByte=%d alphaDecoded=%v eff=%#x",
			int(es.ModelIndex), precache(int(es.ModelIndex)), int(es.Alpha),
			inet.ENTALPHA_DECODE(es.Alpha), int(es.Effects))
	} else {
		t.Logf("client entity 16 ABSENT from client Entities map")
	}

	// Report server-side weaponmodel resolution.
	srvWeapon := srv.Static.Clients[0].Edict.WeaponModel(srv)
	srvWeaponName := srv.String(srvWeapon)
	srvIdx := srv.FindModel(srvWeaponName)
	t.Logf("server player weaponmodel: %q -> FindModel idx=%d", srvWeaponName, srvIdx)
}
