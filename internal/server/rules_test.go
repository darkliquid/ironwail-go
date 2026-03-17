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

func TestCheckRulesNoTriggerInCoop(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "1",
		"deathmatch": "0",
		"fraglimit":  "10",
		"timelimit":  "2",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 20
	s.Time = 300

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("fraglimit/timelimit should not trigger in coop mode")
	}
}

func TestCheckRulesNoTriggerWhenDisabled(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "0",
		"timelimit":  "0",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 999
	s.Time = 99999

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("zero fraglimit/timelimit should not trigger match end")
	}
}

func TestCheckRulesFraglimitExactlyMet(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "20",
		"timelimit":  "0",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 20

	s.CheckRules()
	if !s.Static.ChangeLevelIssued {
		t.Fatal("fraglimit should trigger when frags exactly equal limit")
	}
}

func TestCheckRulesFraglimitNotMetBelowThreshold(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "20",
		"timelimit":  "0",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 19

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("fraglimit should not trigger when frags are below limit")
	}
}

func TestCheckRulesTimelimitNotMetBelowThreshold(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "0",
		"timelimit":  "5",
	})

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Time = 299 // 5 minutes = 300 seconds

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("timelimit should not trigger when time is below limit")
	}
}

func TestCheckRulesSkipsWhenChangeLevelAlreadyIssued(t *testing.T) {
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

	s.Static.ChangeLevelIssued = true
	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 20

	// Should be a no-op since ChangeLevelIssued is already true.
	s.CheckRules()
	// No crash or panic means success; the guard clause prevented re-entry.
}

func TestCheckRulesFraglimitChecksAllClients(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "10",
		"timelimit":  "0",
	})

	s := NewServer()
	if err := s.Init(4); err != nil {
		t.Fatalf("init server: %v", err)
	}

	// Client 0: below limit.
	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 5

	// Client 1: inactive.
	s.Static.Clients[1].Active = false

	// Client 2: at limit.
	s.Static.Clients[2].Active = true
	s.Static.Clients[2].Spawned = true
	s.Static.Clients[2].Edict.Vars.Frags = 10

	s.CheckRules()
	if !s.Static.ChangeLevelIssued {
		t.Fatal("fraglimit should trigger when any client reaches limit")
	}
}

func TestCheckRulesNegativeFraglimitIgnored(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "-5",
		"timelimit":  "0",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 50

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("negative fraglimit should be treated as disabled")
	}
}

func TestCheckRulesNegativeTimelimitIgnored(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "0",
		"deathmatch": "1",
		"fraglimit":  "0",
		"timelimit":  "-10",
	})

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.Time = 99999

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("negative timelimit should be treated as disabled")
	}
}

func TestCheckRulesCoopOverridesDeathmatch(t *testing.T) {
	withRuleCVars(t, map[string]string{
		"coop":       "1",
		"deathmatch": "1",
		"fraglimit":  "10",
		"timelimit":  "2",
	})

	s := NewServer()
	if err := s.Init(2); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Static.Clients[0].Active = true
	s.Static.Clients[0].Spawned = true
	s.Static.Clients[0].Edict.Vars.Frags = 20
	s.Time = 300

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("coop flag should override deathmatch; rules should not trigger")
	}
}

func TestCheckRulesSkipsFreedEdicts(t *testing.T) {
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
	s.Static.Clients[0].Edict.Vars.Frags = 20
	s.Static.Clients[0].Edict.Free = true // Freed edict should be skipped.

	s.CheckRules()
	if s.Static.ChangeLevelIssued {
		t.Fatal("freed edicts should be skipped in fraglimit check")
	}
}
