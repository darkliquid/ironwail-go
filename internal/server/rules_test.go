package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

func withRuleCVars(t *testing.T, values map[string]string) {
	t.Helper()
	original := make(map[string]string, len(values))
	for name := range values {
		if cvar.Get(name) == nil {
			cvar.Register(name, "0", cvar.FlagServerInfo, "")
		}
		original[name] = cvar.StringValue(name)
	}
	for name, value := range values {
		cvar.Set(name, value)
	}
	t.Cleanup(func() {
		for name, value := range original {
			cvar.Set(name, value)
		}
	})
}

func TestCheckRulesEndsMatchOnFraglimit(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "10",
		"timelimit":  "0",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 10

	s.CheckRules()
	if !s.Static.ChangeLevelIssued {
		t.Fatal("fraglimit did not trigger match end")
	}
}

func TestCheckRulesEndsMatchOnTimelimit(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "0",
		"timelimit":  "2",
	})

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Time = 120

	s.CheckRules()
	if !s.Static.ChangeLevelIssued {
		t.Fatal("timelimit did not trigger match end")
	}
}

func TestHandleDeathmatchRespawnDelay(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
	})

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Active = true
	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = true
	client.Name = "respawn-tester"

	ent := client.Edict
	ent.Free = false
	ent.Vars.Health = 0
	ent.Vars.DeadFlag = float32(DeadDead)

	s.Time = 10
	waiting := s.handleDeathmatchRespawn(client)
	if !waiting {
		t.Fatal("dead client should wait for respawn delay")
	}
	if got, want := client.RespawnTime, float32(12); got != want {
		t.Fatalf("respawn time = %v, want %v", got, want)
	}
	if ent.Vars.Health > 0 {
		t.Fatalf("client respawned too early: health=%v", ent.Vars.Health)
	}

	s.Time = 11.5
	waiting = s.handleDeathmatchRespawn(client)
	if !waiting {
		t.Fatal("client should still be waiting before delay expiry")
	}

	s.Time = 12
	waiting = s.handleDeathmatchRespawn(client)
	if waiting {
		t.Fatal("client should no longer be blocked after respawn")
	}
	if ent.Vars.Health <= 0 {
		t.Fatalf("client health = %v, want respawned health", ent.Vars.Health)
	}
	if got, want := DeadFlag(ent.Vars.DeadFlag), DeadNo; got != want {
		t.Fatalf("deadflag = %v, want %v", got, want)
	}
	if client.RespawnTime != 0 {
		t.Fatalf("respawn time not cleared: %v", client.RespawnTime)
	}
}
