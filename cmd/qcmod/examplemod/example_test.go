// Package examplemod is a runnable mod that uses the qcmod In-Go simulator
// (quad/sim) to test QuakeGo-style gameplay logic without the engine.
//
// It accompanies cmd/qcmod's usage docs: run
//
//	go run ./cmd/qcmod test ./cmd/qcmod/examplemod
//
// from the repo root. The test defines a tiny door entity whose think
// schedules a move (like door_fire / SUB_CalcMove) and asserts the resulting
// velocity/nextthink, then confirms the chain finds a target by targetname.
package examplemod

import (
	"testing"

	"quake"
	"quake/engine"
	"quake/sim"
)

// TestDoorSchedulesMove is the canonical In-Go mod test: a door's think sets
// velocity + nextthink (SUB_CalcMove), and the sim records the sound.
func TestDoorSchedulesMove(t *testing.T) {
	w := sim.New()
	defer engine.ResetBackend()

	door := w.Spawn("func_door")
	door.Origin = quake.MakeVec3(0, 0, 0)

	door.Think = func() {
		door.Velocity = quake.MakeVec3(0, 0, 100)
		door.NextThink = w.Time + 1.0
		engine.Sound(door, 0, "doors/door1.wav", 1, 1)
	}
	if err := w.Fire(door, nil, door.Think); err != nil {
		t.Fatalf("Fire: %v", err)
	}

	if door.Velocity != (quake.Vec3{0, 0, 100}) {
		t.Fatalf("velocity = %v, want [0 0 100]", door.Velocity)
	}
	if got := door.NextThink - w.Time; got != 1.0 {
		t.Fatalf("nextthink delta = %v, want 1.0", got)
	}
	if len(w.Sounds) != 1 || w.Sounds[0] != "doors/door1.wav" {
		t.Fatalf("sounds = %v, want [doors/door1.wav]", w.Sounds)
	}
}

// TestChainFindsLinkedTarget is SUB_UseTargets-style: find(targetname, X)
// locates the chained door even when its targetname was folded onto the
// owner, so a relay can fire it.
func TestChainFindsLinkedTarget(t *testing.T) {
	w := sim.New()
	defer engine.ResetBackend()

	owner := w.Spawn("func_door")
	owner.TargetName = "west_door_up"
	half := w.Spawn("func_door")
	half.TargetName = "" // folded by door_link in real mods

	if found := engine.Find(nil, "targetname", "west_door_up"); found != owner {
		t.Fatalf("find(targetname=west_door_up) = %p, want owner %p", found, owner)
	}
	if engine.Find(nil, "targetname", "missing") != nil {
		t.Fatal("find(missing) should be nil")
	}
	_ = half
}
