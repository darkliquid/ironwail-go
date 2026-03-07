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
