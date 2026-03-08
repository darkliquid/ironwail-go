package server

import "testing"

func TestSaveGameStateRoundTripsLightStyles(t *testing.T) {
	srv := NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init: %v", err)
	}
	srv.Active = true
	srv.Name = "start"
	srv.LightStyles[2] = "abc"
	srv.LightStyles[5] = "z"

	state, err := srv.CaptureSaveGameState()
	if err != nil {
		t.Fatalf("CaptureSaveGameState: %v", err)
	}

	restored := NewServer()
	if err := restored.Init(1); err != nil {
		t.Fatalf("Init restored: %v", err)
	}
	restored.Name = "start"
	if err := restored.RestoreSaveGameState(state); err != nil {
		t.Fatalf("RestoreSaveGameState: %v", err)
	}

	if got := restored.LightStyles[2]; got != "abc" {
		t.Fatalf("lightstyle 2 = %q, want %q", got, "abc")
	}
	if got := restored.LightStyles[5]; got != "z" {
		t.Fatalf("lightstyle 5 = %q, want %q", got, "z")
	}
	if got := restored.LightStyles[0]; got != "m" {
		t.Fatalf("default lightstyle = %q, want %q", got, "m")
	}
}

func TestSaveGameStateRoundTripsGameplayState(t *testing.T) {
	srv := NewServer()
	if err := srv.Init(1); err != nil {
		t.Fatalf("Init: %v", err)
	}
	srv.Active = true
	srv.Name = "start"
	srv.Time = 321.5
	srv.Paused = true
	srv.Static.ServerFlags = 0x24

	player := srv.EdictNum(1)
	player.Vars.Health = 87
	player.Vars.CurrentAmmo = 19
	player.Vars.AmmoShells = 31
	player.Vars.AmmoNails = 42
	player.Vars.AmmoRockets = 5
	player.Vars.AmmoCells = 63
	player.Vars.Weapon = 7
	player.Vars.Items = 0x0001 | 0x0004 | 0x0100
	player.Vars.ArmorType = 0.8
	player.Vars.ArmorValue = 145

	srv.Static.Clients[0].Name = "Ranger"
	srv.Static.Clients[0].Color = 12
	srv.Static.Clients[0].SpawnParms[0] = 100
	srv.Static.Clients[0].SpawnParms[1] = 250
	srv.Static.Clients[0].SpawnParms[2] = 3

	state, err := srv.CaptureSaveGameState()
	if err != nil {
		t.Fatalf("CaptureSaveGameState: %v", err)
	}

	restored := NewServer()
	if err := restored.Init(1); err != nil {
		t.Fatalf("Init restored: %v", err)
	}
	restored.Name = "start"
	if err := restored.RestoreSaveGameState(state); err != nil {
		t.Fatalf("RestoreSaveGameState: %v", err)
	}

	restoredPlayer := restored.EdictNum(1)
	if restoredPlayer == nil || restoredPlayer.Vars == nil {
		t.Fatal("restored player edict missing")
	}
	if got := restoredPlayer.Vars.Health; got != 87 {
		t.Fatalf("restored health = %v, want 87", got)
	}
	if got := restoredPlayer.Vars.CurrentAmmo; got != 19 {
		t.Fatalf("restored current ammo = %v, want 19", got)
	}
	if got := restoredPlayer.Vars.AmmoShells; got != 31 {
		t.Fatalf("restored shells = %v, want 31", got)
	}
	if got := restoredPlayer.Vars.AmmoNails; got != 42 {
		t.Fatalf("restored nails = %v, want 42", got)
	}
	if got := restoredPlayer.Vars.AmmoRockets; got != 5 {
		t.Fatalf("restored rockets = %v, want 5", got)
	}
	if got := restoredPlayer.Vars.AmmoCells; got != 63 {
		t.Fatalf("restored cells = %v, want 63", got)
	}
	if got := restoredPlayer.Vars.Weapon; got != 7 {
		t.Fatalf("restored weapon = %v, want 7", got)
	}
	if got := restoredPlayer.Vars.Items; got != (0x0001 | 0x0004 | 0x0100) {
		t.Fatalf("restored items = %v, want %v", got, float32(0x0001|0x0004|0x0100))
	}
	if got := restoredPlayer.Vars.ArmorType; got != 0.8 {
		t.Fatalf("restored armor type = %v, want 0.8", got)
	}
	if got := restoredPlayer.Vars.ArmorValue; got != 145 {
		t.Fatalf("restored armor value = %v, want 145", got)
	}

	if got := restored.Static.ServerFlags; got != 0x24 {
		t.Fatalf("restored server flags = %d, want 0x24", got)
	}
	if got := restored.Time; got != 321.5 {
		t.Fatalf("restored time = %v, want 321.5", got)
	}
	if !restored.Paused {
		t.Fatal("restored paused = false, want true")
	}
	if got := restored.Static.Clients[0].Name; got != "Ranger" {
		t.Fatalf("restored client name = %q, want Ranger", got)
	}
	if got := restored.Static.Clients[0].Color; got != 12 {
		t.Fatalf("restored client color = %d, want 12", got)
	}
	if got := restored.Static.Clients[0].SpawnParms[0]; got != 100 {
		t.Fatalf("restored spawn parm1 = %v, want 100", got)
	}
	if got := restored.Static.Clients[0].SpawnParms[1]; got != 250 {
		t.Fatalf("restored spawn parm2 = %v, want 250", got)
	}
	if got := restored.Static.Clients[0].SpawnParms[2]; got != 3 {
		t.Fatalf("restored spawn parm3 = %v, want 3", got)
	}
	if restored.Static.Clients[0].Edict != restoredPlayer {
		t.Fatal("restored client edict not rebound to player edict")
	}
}
