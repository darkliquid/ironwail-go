package qbsp

import (
	"strings"
	"testing"
)

// TestLeakDetection_SealedBox verifies that a fully enclosed box compiles with Leaked == false.
func TestLeakDetection_SealedBox(t *testing.T) {
	mapData := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(-128, -128, -16, 128, 128, 128, 16) +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"0 0 32\"\n}\n"

	m, err := ParseMap(strings.NewReader(mapData))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := Compile(m, Options{Log: t.Logf})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatalf("expected sealed box to not leak, got leak trail of %d points", len(res.LeakPath))
	}
}

// TestLeakDetection_SubgridPlates verifies that thin sub-grid plates (e.g. 4-unit plates)
// do not cause spurious leak false-positives.
func TestLeakDetection_SubgridPlates(t *testing.T) {
	// A sealed box with a 4-unit thin trim plate on the floor
	mapData := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(-128, -128, -16, 128, 128, 128, 16) +
		prettySlab(-64, -64, 0, 64, 64, 4, "mt_trim") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"0 0 32\"\n}\n"

	m, err := ParseMap(strings.NewReader(mapData))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := Compile(m, Options{Log: t.Logf})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatalf("expected sealed box with subgrid plate to not leak, got leak trail of %d points", len(res.LeakPath))
	}
}
