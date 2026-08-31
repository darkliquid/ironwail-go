// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestCenterprintE2EStartMap(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	g := New()

	args := []string{"-basedir", baseDir}
	if err := g.InitSubsystems(true, false, 1, baseDir, "id1", args); err != nil {
		t.Fatalf("InitSubsystems: %v", err)
	}

	var logged []string
	console.SetPrintCallback(func(msg string) {
		logged = append(logged, console.TerminalText(msg))
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	if err := g.Host.CmdMap("start", g.Subs); err != nil {
		t.Fatalf("CmdMap: %v", err)
	}

	// Run frames to let player spawn and signon complete
	for i := 0; i < 30; i++ {
		_ = g.Host.Frame(0.05, gameCallbacks{g: g})
	}

	t.Logf("Client state: %v, signon: %d", g.Client.State, g.Client.Signon)

	// Move player into normal skill trigger: absmin={416 832 -24}, absmax={688 1232 -8}
	if g.Server != nil && g.Server.Active && g.Server.NumEdicts > 1 {
		playerEnt := g.Server.Edicts[1]
		playerEnt.SetOrigin(g.Server, qtypes.Vec3{X: 500, Y: 900, Z: -20})
		playerEnt.SetAngles(g.Server, qtypes.Vec3{X: 0, Y: 90, Z: 0})
		g.Server.LinkEdict(playerEnt, true)
	}

	// Run more frames to execute physics and process client loopback
	for i := 0; i < 10; i++ {
		_ = g.Host.Frame(0.05, gameCallbacks{g: g})
	}

	t.Logf("Client CenterPrint: %q", g.Client.CenterPrint)
	t.Logf("Client CenterPrintAt: %f, Client.Time: %f", g.Client.CenterPrintAt, g.Client.Time)

	foundLog := false
	for _, l := range logged {
		if strings.Contains(l, "This hall selects NORMAL skill") {
			foundLog = true
			t.Logf("Found in console log: %q", l)
		}
	}

	if !foundLog {
		t.Errorf("Centerprint was NOT logged to console! All console logs:\n%s", strings.Join(logged, ""))
	}

	if g.Client.CenterPrint != "This hall selects NORMAL skill" {
		t.Errorf("Client.CenterPrint = %q, want %q", g.Client.CenterPrint, "This hall selects NORMAL skill")
	}
}
