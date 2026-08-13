// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package game

import (
	"encoding/json"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/server"
)

func TestDumpStateSchemaFullEntityState(t *testing.T) {
	g := New()
	s := server.NewServer()
	s.Active = true
	s.Time = 12.34
	s.Gravity = 800
	s.Static = &server.ServerStatic{MaxClients: 4}
	vm := qc.NewVM()
	vm.Globals = make([]float32, 256)
	vm.MaxEdicts = 64
	vm.NumEdicts = 64
	vm.EntityFields = 128
	vm.EdictSize = 28 + vm.EntityFields*4
	vm.Edicts = make([]byte, vm.EdictSize*64)
	s.QCVM = vm
	s.Edicts = []*server.Edict{{Num: 0}, {Num: 1}, {Num: 2}}
	s.NumEdicts = 3
	g.Server = s

	// Configure edict 1
	e1 := s.EdictNum(1)
	if e1 != nil {
		e1.Free = false
		e1.SetOrigin(s, [3]float32{100, 200, 300})
		e1.SetAngles(s, [3]float32{10, 20, 30})
		e1.SetVelocity(s, [3]float32{50, 0, -100})
		e1.SetModelIndex(s, 5)
		e1.SetFrame(s, 2)
		e1.SetSkin(s, 1)
		e1.SetColormap(s, 0)
		e1.SetEffects(s, 8)
		e1.SetSolid(s, 3)    // SOLID_SLIDEBOX
		e1.SetMoveType(s, 3) // MOVETYPE_WALK
		e1.SetFlags(s, 8)    // FL_ONGROUND
		e1.SetNextThink(s, 15.0)
	}

	state := g.CaptureDumpState(100)

	if state.Frame != 100 {
		t.Fatalf("expected Frame=100, got %d", state.Frame)
	}
	if state.SvTime != 12.34 {
		t.Fatalf("expected SvTime=12.34, got %f", state.SvTime)
	}
	if state.Gravity != 800 {
		t.Fatalf("expected Gravity=800, got %f", state.Gravity)
	}
	if state.MaxClients != 4 {
		t.Fatalf("expected MaxClients=4, got %d", state.MaxClients)
	}

	if len(state.Visedicts) == 0 {
		t.Fatalf("expected non-empty Visedicts, got 0")
	}

	var foundE1 *DumpEdict
	for i := range state.Visedicts {
		if state.Visedicts[i].Number == 1 {
			foundE1 = &state.Visedicts[i]
			break
		}
	}
	if foundE1 == nil {
		t.Fatalf("edict 1 not found in dump")
	}

	if foundE1.Origin != [3]float32{100, 200, 300} {
		t.Fatalf("unexpected Origin: got %v", foundE1.Origin)
	}
	if foundE1.Angles != [3]float32{10, 20, 30} {
		t.Fatalf("unexpected Angles: got %v", foundE1.Angles)
	}
	if foundE1.Velocity != [3]float32{50, 0, -100} {
		t.Fatalf("unexpected Velocity: got %v", foundE1.Velocity)
	}
	if foundE1.ModelIndex != 5 {
		t.Fatalf("unexpected ModelIndex: got %d", foundE1.ModelIndex)
	}
	if foundE1.Frame != 2 {
		t.Fatalf("unexpected Frame: got %d", foundE1.Frame)
	}
	if foundE1.Skin != 1 {
		t.Fatalf("unexpected Skin: got %d", foundE1.Skin)
	}
	if foundE1.Effects != 8 {
		t.Fatalf("unexpected Effects: got %d", foundE1.Effects)
	}
	if foundE1.Solid != 3 {
		t.Fatalf("unexpected Solid: got %d", foundE1.Solid)
	}
	if foundE1.Movetype != 3 {
		t.Fatalf("unexpected Movetype: got %d", foundE1.Movetype)
	}
	if foundE1.Flags != 8 {
		t.Fatalf("unexpected Flags: got %d", foundE1.Flags)
	}
	if foundE1.NextThink != 15.0 {
		t.Fatalf("unexpected NextThink: got %f", foundE1.NextThink)
	}

	// JSON serialization / deserialization round-trip
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var parsed RefFrameState
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("json.Unmarshal into RefFrameState failed: %v", err)
	}

	if parsed.Frame != 100 || len(parsed.Visedicts) == 0 {
		t.Fatalf("parsed RefFrameState mismatch: %#v", parsed)
	}
	var pE1 *RefEdict
	for i := range parsed.Visedicts {
		if parsed.Visedicts[i].Number == 1 {
			pE1 = &parsed.Visedicts[i]
			break
		}
	}
	if pE1 == nil {
		t.Fatalf("edict 1 not found in parsed dump")
	}
	if pE1.Number != 1 || pE1.Solid != 3 || pE1.Movetype != 3 || pE1.Flags != 8 {
		t.Fatalf("parsed RefEdict mismatch: %#v", pE1)
	}
}
