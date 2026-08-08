package server

import (
	"testing"
)

// TestSaveLoadRoundTripPreservesFrameEvolution is the H5 save/load parity
// gate: a session that evolves for N frames must produce the SAME edict-field
// stream after a save→restore cycle as one that never saved.
//
// Where in C: SV_Savegame_f / SV_Loadgame_f in sv_main.c.
//
// It is the missing piece over the existing round-trip tests: those restore
// static state (light styles, health/ammo), but do not prove that a load
// leaves the *simulation* bit-identical to an uninterrupted run. This test
// runs two parallel servers from the same seed, saves/restores one mid-run,
// and compares per-frame edict streams.
func TestSaveLoadRoundTripPreservesFrameEvolution(t *testing.T) {
	newStreamServer := func() *Server {
		s := newPhysicsTestServer()
		s.Active = true
		s.Name = "start"
		s.Time = 0
		s.FrameTime = 0.1
		s.WorldModel = CreateSyntheticWorldModel()
		s.Edicts[0].SetSolid(s, float32(SolidBSP))
		s.ClearWorld()

		// A walking player and a pusher platform, both evolved by physics.
		player := allocPhysicsTestEdict(s)
		player.SetMoveType(s, float32(MoveTypeWalk))
		player.SetSolid(s, float32(SolidSlideBox))
		player.SetOrigin(s, [3]float32{0, 0, 24})
		player.SetMins(s, [3]float32{-16, -16, -24})
		player.SetMaxs(s, [3]float32{16, 16, 32})
		player.SetSize(s, [3]float32{32, 32, 56})
		player.SetVelocity(s, [3]float32{0, 0, 0})
		player.SetFlags(s, float32(FlagOnGround))

		pusher := allocPhysicsTestEdict(s)
		pusher.SetMoveType(s, float32(MoveTypePush))
		pusher.SetSolid(s, float32(SolidBSP))
		pusher.SetOrigin(s, [3]float32{64, 0, 0})
		pusher.SetMins(s, [3]float32{-16, -16, -8})
		pusher.SetMaxs(s, [3]float32{16, 16, 8})
		pusher.SetVelocity(s, [3]float32{0, 0, 20})
		s.LinkEdict(pusher, false)

		return s
	}

	const frames = 120

	// Control: uninterrupted run.
	control := newStreamServer()
	for i := 0; i < frames; i++ {
		control.Physics()
	}

	// Experiment: run half, save, restore, run the rest.
	subject := newStreamServer()
	for i := 0; i < frames/2; i++ {
		subject.Physics()
	}
	state, err := subject.CaptureSaveGameState()
	if err != nil {
		t.Fatalf("CaptureSaveGameState: %v", err)
	}
	subject2 := newStreamServer()
	if err := subject2.RestoreSaveGameState(state); err != nil {
		t.Fatalf("RestoreSaveGameState: %v", err)
	}
	for i := frames / 2; i < frames; i++ {
		subject2.Physics()
	}

	// Compare the FULL edict stream (not just static state): every edict's key
	// field, after the same number of frames, must be identical.
	for i := 0; i < subject2.NumEdicts; i++ {
		ce := control.EdictNum(i)
		se := subject2.EdictNum(i)
		if (ce == nil) != (se == nil) {
			t.Fatalf("edict %d presence differs: control has %v, subject has %v", i, ce != nil, se != nil)
		}
		if ce == nil {
			continue
		}
		compareEdictStreamFields(t, i, control, subject2, ce, se)
	}
}

func compareEdictStreamFields(t *testing.T, num int, ctrl, subj *Server, ce, se *Edict) {
	t.Helper()
	type fieldPair struct {
		name string
		a, b any
	}
	pairs := []fieldPair{
		{"origin", ce.Origin(ctrl), se.Origin(subj)},
		{"velocity", ce.Velocity(ctrl), se.Velocity(subj)},
		{"angles", ce.Angles(ctrl), se.Angles(subj)},
		{"ltime", ce.LTime(ctrl), se.LTime(subj)},
		{"nextthink", ce.NextThink(ctrl), se.NextThink(subj)},
		{"health", ce.Health(ctrl), se.Health(subj)},
		{"waterlevel", ce.WaterLevel(ctrl), se.WaterLevel(subj)},
		{"flags", ce.Flags(ctrl), se.Flags(subj)},
		{"groundentity", ce.GroundEntity(ctrl), se.GroundEntity(subj)},
	}
	for _, p := range pairs {
		if p.a != p.b {
			t.Fatalf("edict %d field %s diverged after save/load: control=%v subject=%v", num, p.name, p.a, p.b)
		}
	}
}
