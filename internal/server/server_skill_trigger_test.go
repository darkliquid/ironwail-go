// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/loc"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

func TestSkillTriggerCenterprint(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	loc.Init(baseDir, "english")

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	s.Datagram = NewMessageBuffer(MaxDatagram)
	s.Static = &ServerStatic{Clients: []*Client{{Active: true, Message: NewMessageBuffer(MaxDatagram)}}}
	newServerTestVM(s, 512)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	progsData, err := vfs.LoadFile("progs.dat")
	if err != nil {
		t.Fatalf("load progs.dat: %v", err)
	}
	if err := s.QCVM.LoadProgs(bytes.NewReader(progsData)); err != nil {
		t.Fatalf("LoadProgs: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)

	var loggedToConsole []string
	console.SetPrintCallback(func(msg string) {
		loggedToConsole = append(loggedToConsole, console.TerminalText(msg))
	})
	t.Cleanup(func() {
		console.SetPrintCallback(nil)
	})

	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	// Find the normal skill trigger entity
	var normalTrigger *Edict
	for i := 0; i < s.NumEdicts; i++ {
		e := s.Edicts[i]
		if e == nil || e.Free {
			continue
		}
		cname := s.String(e.ClassName(s))
		msg := s.String(e.Message(s))
		if cname == "trigger_multiple" && (msg == "$map_skill_normal" || msg == "This hall selects NORMAL skill") {
			normalTrigger = e
			t.Logf("Found normal skill trigger entity #%d: classname=%q, msg=%q, origin=%v, absmin=%v, absmax=%v, solid=%v, touch=%d",
				i, cname, msg, e.Origin(s), e.AbsMin(s), e.AbsMax(s), e.Solid(s), e.Touch(s))
			break
		}
	}

	if normalTrigger == nil {
		t.Fatal("could not find normal skill trigger in start.bsp")
	}

	// Create and spawn player edict
	player := s.Edicts[1]
	player.Free = false
	player.SetClassName(s, s.QCVM.AllocString("player"))
	player.SetSolid(s, float32(SolidSlideBox))
	player.SetMoveType(s, float32(MoveTypeWalk))
	center := qtypes.Vec3{
		X: (normalTrigger.AbsMin(s).X + normalTrigger.AbsMax(s).X) / 2,
		Y: (normalTrigger.AbsMin(s).Y + normalTrigger.AbsMax(s).Y) / 2,
		Z: (normalTrigger.AbsMin(s).Z + normalTrigger.AbsMax(s).Z) / 2,
	}
	player.SetOrigin(s, center)
	player.SetAbsMin(s, center.Sub(qtypes.Vec3{X: 16, Y: 16, Z: 24}))
	player.SetAbsMax(s, center.Add(qtypes.Vec3{X: 16, Y: 16, Z: 32}))
	player.SetAngles(s, qtypes.Vec3{X: 0, Y: 90, Z: 0}) // facing 90 degrees

	s.LinkEdict(player, true)

	// Call SV_TouchLinks or touch
	touchFunc := normalTrigger.Touch(s)
	t.Logf("normalTrigger.Touch func index = %d", touchFunc)

	s.QCVM.SetGInt(qc.OFSSelf, int32(s.NumForEdict(normalTrigger)))
	s.QCVM.SetGInt(qc.OFSOther, int32(s.NumForEdict(player)))
	s.QCVM.SetGFloat(qc.OFSTime, 1)

	if err := s.QCVM.ExecuteProgram(int(touchFunc)); err != nil {
		t.Fatalf("QCVM.ExecuteProgram(touch) failed: %v", err)
	}

	t.Logf("Client message buffer length after touch = %d", s.Static.Clients[0].Message.Len())

	foundCenterprint := false
	msgBuf := s.Static.Clients[0].Message.Data[:s.Static.Clients[0].Message.Len()]
	for i := 0; i < len(msgBuf); i++ {
		if msgBuf[i] == byte(inet.SVCCenterPrint) {
			foundCenterprint = true
			end := i + 1
			for end < len(msgBuf) && msgBuf[end] != 0 {
				end++
			}
			text := string(msgBuf[i+1 : end])
			t.Logf("Received SVCCenterPrint payload: %q", text)
			if text != "This hall selects NORMAL skill" {
				t.Errorf("SVCCenterPrint text = %q, want %q", text, "This hall selects NORMAL skill")
			}
			break
		}
	}

	if !foundCenterprint {
		t.Errorf("SVCCenterPrint was not written to client message buffer! Buffer hex: %x", msgBuf)
	}
}
