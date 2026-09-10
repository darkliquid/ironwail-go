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

// TestFuncDetail_SealingAndEntityStripping verifies that func_detail and func_group
// brushes are merged into the world model (model 0), seal the room against leaks,
// do not allocate separate submodels, and are not emitted into the BSP entity lump.
func TestFuncDetail_SealingAndEntityStripping(t *testing.T) {
	// A room where the floor, 3 walls are worldspawn, 4th wall is func_detail,
	// and the ceiling is func_group.
	x0, y0, z0, x1, y1, z1, th := -128.0, -128.0, -16.0, 128.0, 128.0, 128.0, 16.0
	mapData := "{\n\"classname\" \"worldspawn\"\n" +
		prettySlab(x0, y0, z0, x1, y1, z0+th, "mt_floor") +
		prettySlab(x0, y0, z0+th, x0+th, y1, z1-th, "mt_wall") +
		prettySlab(x1-th, y0, z0+th, x1, y1, z1-th, "mt_wall") +
		prettySlab(x0, y0, z0+th, x1, y0+th, z1-th, "mt_wall") +
		"}\n" +
		"{\n\"classname\" \"func_detail\"\n" +
		prettySlab(x0, y1-th, z0+th, x1, y1, z1-th, "mt_wall") +
		"}\n" +
		"{\n\"classname\" \"func_group\"\n" +
		prettySlab(x0, y0, z1-th, x1, y1, z1, "mt_floor") +
		"}\n" +
		"{\n\"classname\" \"info_player_start\"\n\"origin\" \"0 0 32\"\n}\n"

	m, err := ParseMap(strings.NewReader(mapData))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := Compile(m, Options{Log: t.Logf})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatalf("expected room sealed by func_detail and func_group to not leak, got %d leak points", len(res.LeakPath))
	}

	entStr := string(res.Data)
	if strings.Contains(entStr, "func_detail") {
		t.Errorf("expected func_detail to be stripped from entity lump, but found in BSP")
	}
	if strings.Contains(entStr, "func_group") {
		t.Errorf("expected func_group to be stripped from entity lump, but found in BSP")
	}
	if res.Models != 1 {
		t.Errorf("expected 1 model (world only), got %d models", res.Models)
	}
}

// TestFuncDetail_Illusionary verifies that func_detail_illusionary brushes
// are merged into the world model without becoming solid clipnodes.
func TestFuncDetail_Illusionary(t *testing.T) {
	// A sealed room with an illusionary pillar in the center
	x0, y0, z0, x1, y1, z1, th := -128.0, -128.0, -16.0, 128.0, 128.0, 128.0, 16.0
	mapData := "{\n\"classname\" \"worldspawn\"\n" +
		prettyRoom(x0, y0, z0, x1, y1, z1, th) +
		"}\n" +
		"{\n\"classname\" \"func_detail_illusionary\"\n" +
		prettySlab(-16, -16, 0, 16, 16, 64, "mt_wall") +
		"}\n" +
		"{\n\"classname\" \"info_player_start\"\n\"origin\" \"0 0 32\"\n}\n"

	m, err := ParseMap(strings.NewReader(mapData))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := Compile(m, Options{Log: t.Logf})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatalf("expected room to not leak, got %d leak points", len(res.LeakPath))
	}
	if res.Models != 1 {
		t.Errorf("expected 1 model (world only), got %d models", res.Models)
	}
	entStr := string(res.Data)
	if strings.Contains(entStr, "func_detail_illusionary") {
		t.Errorf("expected func_detail_illusionary to be stripped from entity lump")
	}
}
