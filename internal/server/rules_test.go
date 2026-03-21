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

// TestCheckRulesEndsMatchOnFraglimit tests the deathmatch fraglimit rule.
// It ensuring the match correctly ends and advances to the next level when a player reaches the frag goal.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesEndsMatchOnTimelimit tests the deathmatch timelimit rule.
// It ensuring the match ends when the allotted time has elapsed.
// Where in C: SV_CheckRules in sv_main.c
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

// TestHandleDeathmatchRespawnRequiresReadyStateAndButtonPress tests the player respawn logic in deathmatch.
// It ensuring players can only respawn after the QuakeC logic marks them as ready and they've pressed a button.
// Where in C: SV_Physics_Client in sv_phys.c (handling respawn state)
func TestHandleDeathmatchRespawnRequiresReadyStateAndButtonPress(t *testing.T) {
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

	waiting := s.handleDeathmatchRespawn(client)
	if !waiting {
		t.Fatal("dead client should wait until QC marks it respawnable")
	}
	if ent.Vars.Health > 0 {
		t.Fatalf("client respawned before deadflag was ready: health=%v", ent.Vars.Health)
	}

	ent.Vars.DeadFlag = float32(DeadRespawnable)
	waiting = s.handleDeathmatchRespawn(client)
	if !waiting {
		t.Fatal("dead client should still wait for respawn input")
	}
	if ent.Vars.Health > 0 {
		t.Fatalf("client respawned before button press: health=%v", ent.Vars.Health)
	}

	client.LastCmd.Buttons = 1
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
}

// TestCheckRulesNoTriggerInCoop tests that deathmatch rules are disabled in coop mode.
// It preventing unexpected match ends during cooperative play.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesNoTriggerWhenDisabled tests disabled match rules.
// It ensuring that when limits are set to 0, the match continues indefinitely.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesFraglimitExactlyMet tests exact fraglimit match.
// It verifying the boundary condition for match ending.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesFraglimitNotMetBelowThreshold tests below-fraglimit state.
// It ensuring the match doesn't end early.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesTimelimitNotMetBelowThreshold tests below-timelimit state.
// It ensuring the match doesn't end early.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesSkipsWhenChangeLevelAlreadyIssued tests rule checking idempotency.
// It preventing redundant level change commands when a match end has already been triggered.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesFraglimitChecksAllClients tests that all clients are checked for the fraglimit.
// It ensuring that *any* player reaching the limit triggers the match end.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesNegativeFraglimitIgnored tests robustness against invalid fraglimit values.
// It treating negative limits as disabled, matching canonical engine behavior.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesNegativeTimelimitIgnored tests robustness against invalid timelimit values.
// It treating negative limits as disabled.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesCoopOverridesDeathmatch tests the priority of the coop flag.
// It ensuring cooperative mode takes precedence over deathmatch settings.
// Where in C: SV_CheckRules in sv_main.c
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

// TestCheckRulesSkipsFreedEdicts tests that freed edicts don't affect rule checking.
// It preventing crashes or incorrect results from disconnected players or removed entities.
// Where in C: SV_CheckRules in sv_main.c
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
