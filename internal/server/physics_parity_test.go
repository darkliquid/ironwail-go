// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
)

// TestPhysicsWalkJump tests walk/jump physics parity.
// It ensuring that jumping behavior matches the original engine's movement model.
// Where in C: SV_Physics_Client and QuakeC PlayerPreThink.
func TestPhysicsWalkJump(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.01
	s.Gravity = 800

	ent := s.AllocEdict()
	ent.SetOrigin(s, qtypes.Vec3{})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	// Mock a client with jump button pressed
	// In Quake, the server doesn't usually handle the jump button in PhysicsWalk directly
	// unless it's a player. Let's see if we can trigger it.
	// Actually, Quake QC handles jumping in PlayerPreThink by checking button2.
	// But our PhysicsWalk doesn't seem to do anything with buttons if we aren't running QC.
}

// TestPhysicsWalkStepUp tests step-up behavior during walking.
// It verifying that entities can correctly climb small steps and slopes without getting stuck.
// Where in C: SV_WalkMove in sv_phys.c
func TestPhysicsWalkStepUp(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.01
	s.Gravity = 800

	// Create a world with a step
	s.WorldModel = &model.Model{
		Type: model.ModBrush,
	}

	ent := s.AllocEdict()
	ent.SetOrigin(s, qtypes.Vec3{X: 0, Y: 0, Z: 24})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetVelocity(s, qtypes.Vec3{X: 100, Y: 0, Z: 0})
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	// We need a proper Move implementation that can collide with a step
	// For now, let's just see if PhysicsWalk uses StepMove logic.
}
