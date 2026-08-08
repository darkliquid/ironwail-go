package sim

import (
	"testing"

	"quake"
	"quake/engine"
)

// TestWorldSpawnFireThink demonstrates the core In-Go loop: spawn entities,
// wire a think closure (as door_fire / SUB_CalcMove would), fire it through
// the World, and assert the resulting field mutations.
func TestWorldSpawnFireThink(t *testing.T) {
	w := New()
	defer engine.ResetBackend()

	door := w.Spawn("func_door")
	door.Origin = quake.MakeVec3(0, 0, 0)
	door.AbsMin = quake.MakeVec3(-32, -32, 0)
	door.AbsMax = quake.MakeVec3(32, 32, 72)

	// The mod's think closure, as SUB_CalcMove would set up: schedule the
	// door to move by setting velocity + nextthink, and play a sound via the
	// engine builtin (which routes to World's Backend recorder).
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

	w.Step()
	if got := w.Time; got != 0.1 {
		t.Fatalf("time after step = %v, want 0.1", got)
	}
}

// TestWorldFindByTargetname exercises the find() builtin (via the engine
// builtin routing to World's Backend) against the world registry.
func TestWorldFindByTargetname(t *testing.T) {
	w := New()
	defer engine.ResetBackend()

	master := w.Spawn("func_door")
	master.TargetName = "door_a"
	chained := w.Spawn("func_door")
	chained.TargetName = ""

	// SUB_UseTargets-style lookup: find(targetname, self.target).
	found := engine.Find(nil, "targetname", "door_a")
	if found != master {
		t.Fatalf("find(targetname=door_a) = %p, want master %p", found, master)
	}
	if engine.Find(nil, "targetname", "missing") != nil {
		t.Fatal("find(missing) should return nil")
	}
	_ = chained
}
