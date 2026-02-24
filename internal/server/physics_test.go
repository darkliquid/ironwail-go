package server

import (
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func newPhysicsTestServer() *Server {
	s := &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		FrameTime:   0.1,
		Edicts:      []*Edict{{Vars: &EntVars{}}},
		NumEdicts:   1,
	}
	return s
}

func TestClipVelocity(t *testing.T) {
	in := [3]float32{100, 0.05, -5}
	normal := [3]float32{0, 0, 1}
	out, blocked := ClipVelocity(in, normal, 1)

	if blocked != 1 {
		t.Fatalf("blocked = %d, want 1", blocked)
	}
	if out[2] != 0 {
		t.Fatalf("out[2] = %v, want 0", out[2])
	}
}

func TestPhysicsNoClipMovesOriginAndAngles(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Velocity = [3]float32{10, -5, 2}
	ent.Vars.AVelocity = [3]float32{0, 90, 0}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	s.PhysicsNoClip(ent)

	if ent.Vars.Origin != [3]float32{1, -0.5, 0.2} {
		t.Fatalf("origin = %v", ent.Vars.Origin)
	}
	if ent.Vars.Angles != [3]float32{0, 9, 0} {
		t.Fatalf("angles = %v", ent.Vars.Angles)
	}
}

func TestPhysicsPusherAdvancesLocalTimeWhenIdle(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypePush)
	ent.Vars.LTime = 3
	ent.Vars.NextThink = 10
	s.PhysicsPusher(ent)

	if diff := ent.Vars.LTime - 3.1; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("ltime = %v, want 3.1", ent.Vars.LTime)
	}
}

func TestPhysicsTossOnGroundDoesNotMove(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Origin = [3]float32{1, 2, 3}
	ent.Vars.Velocity = [3]float32{50, 60, 70}

	s.PhysicsToss(ent)

	if ent.Vars.Origin != [3]float32{1, 2, 3} {
		t.Fatalf("origin changed on ground toss: %v", ent.Vars.Origin)
	}
}

func TestPhysicsStepOnGroundSkipsFreefall(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Velocity = [3]float32{0, 0, 42}

	s.PhysicsStep(ent)

	if ent.Vars.Velocity[2] != 42 {
		t.Fatalf("z velocity changed: %v", ent.Vars.Velocity[2])
	}
}

func TestPhysicsFrameOnSpawnedMap(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	before := s.Time
	s.Physics()
	if s.Time <= before {
		t.Fatalf("time did not advance: before=%v after=%v", before, s.Time)
	}
}
