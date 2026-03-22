package server

import "testing"

func TestParseTextSaveGamePreservesTitle(t *testing.T) {
	data := "6\nid1\nA descriptive save title\n"
	for i := 0; i < NumSpawnParms; i++ {
		data += "0\n"
	}
	data += "2\n"
	data += "e2m2\n"
	data += "123.5\n"
	for i := 0; i < 64; i++ {
		data += "m\n"
	}
	data += "{\n\"serverflags\" \"0\"\n}\n"
	data += "{\n\"classname\" \"worldspawn\"\n}\n"

	state, err := ParseTextSaveGame([]byte(data))
	if err != nil {
		t.Fatalf("ParseTextSaveGame failed: %v", err)
	}
	if got := state.Title; got != "A descriptive save title" {
		t.Fatalf("Title = %q, want %q", got, "A descriptive save title")
	}
	if got := state.MapName; got != "e2m2" {
		t.Fatalf("MapName = %q, want %q", got, "e2m2")
	}
}
