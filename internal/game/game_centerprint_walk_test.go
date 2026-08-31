package game

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/loc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestCenterprintSimulatedWalk(t *testing.T) {
	baseDir := testutil.SkipIfNoQuakeDir(t)

	g := New()
	args := []string{"-basedir", baseDir}
	if err := g.InitSubsystems(true, false, 1, baseDir, "id1", args); err != nil {
		t.Fatalf("InitSubsystems: %v", err)
	}

	var stdoutLogged []string
	console.SetPrintCallback(func(msg string) {
		formatted := console.TerminalText(msg)
		stdoutLogged = append(stdoutLogged, formatted)
		t.Logf("[Console Output]: %q", formatted)
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	t.Logf("Language string count: %d", loc.Default().Len())
	t.Logf("Translation for $map_skill_normal: %q", loc.GetString("$map_skill_normal"))

	if err := g.Host.CmdMap("start", g.Subs); err != nil {
		t.Fatalf("CmdMap: %v", err)
	}

	player := g.Server.EdictNum(1)
	if player == nil {
		t.Fatalf("player edict 1 is nil")
	}

	t.Logf("Player start origin: %v", player.Origin(g.Server))

	// Walk the player along the Y axis from Y=288 to Y=900 (into normal skill hall)
	// by simulating frames.
	startPos := player.Origin(g.Server)
	for step := 0; step < 50; step++ {
		curY := startPos.Y + float32(step)*15.0
		player.SetOrigin(g.Server, qtypes.Vec3{X: 544, Y: curY, Z: 0})
		player.SetVelocity(g.Server, qtypes.Vec3{X: 0, Y: 100, Z: 0})
		player.SetAngles(g.Server, qtypes.Vec3{X: 0, Y: 90, Z: 0}) // facing North

		// Server frame
		if err := g.Subs.Server.Frame(0.05); err != nil {
			t.Fatalf("Server.Frame: %v", err)
		}

		// Client read
		if err := g.Subs.Client.ReadFromServer(); err != nil {
			t.Fatalf("Client.ReadFromServer: %v", err)
		}

		if g.Client.CenterPrint != "" {
			t.Logf("Step %d (Y=%.1f): Received CenterPrint: %q (CenterPrintAt=%.3f, Client.Time=%.3f)",
				step, curY, g.Client.CenterPrint, g.Client.CenterPrintAt, g.Client.Time)
			break
		}
	}

	if g.Client.CenterPrint == "" {
		t.Fatalf("Player never received CenterPrint during walk!")
	}

	foundInStdout := false
	for _, line := range stdoutLogged {
		if strings.Contains(line, "This hall selects NORMAL skill") {
			foundInStdout = true
			break
		}
	}
	if !foundInStdout {
		t.Errorf("CenterPrint was not logged via SetPrintCallback to stdout! Logged lines: %#v", stdoutLogged)
	}
}
