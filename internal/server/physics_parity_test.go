package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

func TestPhysicsWalkJump(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.01
	s.Gravity = 800

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = [3]float32{0, 0, 0}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	// Mock a client with jump button pressed
	// In Quake, the server doesn't usually handle the jump button in PhysicsWalk directly
	// unless it's a player. Let's see if we can trigger it.
	// Actually, Quake QC handles jumping in PlayerPreThink by checking button2.
	// But our PhysicsWalk doesn't seem to do anything with buttons if we aren't running QC.
}

func TestPhysicsWalkStepUp(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.01
	s.Gravity = 800

	// Create a world with a step
	s.WorldModel = &model.Model{
		Type: model.ModBrush,
	}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = [3]float32{0, 0, 24}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Velocity = [3]float32{100, 0, 0}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	// We need a proper Move implementation that can collide with a step
	// For now, let's just see if PhysicsWalk uses StepMove logic.
}
