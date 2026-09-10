package eval

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditMapPair_Sealed(t *testing.T) {
	dir := roomPairDir(t)
	mapPath := filepath.Join(dir, "room.map")
	bspPath := filepath.Join(dir, "room.bsp")

	res, err := AuditMapPair(mapPath, bspPath, "testpkg", "room", AuditOptions{GridSnap: 8})
	if err != nil {
		t.Fatalf("AuditMapPair: %v", err)
	}

	if res.Forward.Verdict != ForwardClean {
		t.Errorf("forward verdict = %s, want %s (error: %s)", res.Forward.Verdict, ForwardClean, res.Forward.Error)
	}
	if res.Forward.GoLeaked {
		t.Errorf("expected Go qbsp to not leak on sealed room")
	}
	if res.Forward.RefLeaked {
		t.Errorf("expected ref qbsp to not leak on sealed room")
	}

	if res.Decomp.Verdict != DecompClean {
		t.Errorf("decomp verdict = %s, want %s (error: %s)", res.Decomp.Verdict, DecompClean, res.Decomp.Error)
	}
	if res.Decomp.GoLeaked {
		t.Errorf("expected Go qbsp to not leak on decompiled room")
	}
	if res.Decomp.RefLeaked {
		t.Errorf("expected ref qbsp to not leak on decompiled room")
	}
}

func TestAuditMapPair_CategoryA(t *testing.T) {
	// Leaky map: box with missing ceiling so inside leaks to outside
	leakyMapContent := "{\n\"classname\" \"worldspawn\"\n" +
		"{\n( 0 0 0 ) ( 0 256 0 ) ( 256 256 0 ) floor 0 0 0 1 1\n" +
		"( 0 0 -16 ) ( 256 0 -16 ) ( 256 256 -16 ) floor 0 0 0 1 1\n" +
		"( 0 0 0 ) ( 0 0 -16 ) ( 0 256 -16 ) floor 0 0 0 1 1\n" +
		"( 256 0 0 ) ( 256 256 0 ) ( 256 256 -16 ) floor 0 0 0 1 1\n" +
		"( 0 256 0 ) ( 0 256 -16 ) ( 256 256 -16 ) floor 0 0 0 1 1\n" +
		"( 0 0 0 ) ( 256 0 0 ) ( 256 0 -16 ) floor 0 0 0 1 1\n}\n" +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"128 128 64\"\n}\n"
	tmpMap := filepath.Join(t.TempDir(), "leaky.map")
	if err := os.WriteFile(tmpMap, []byte(leakyMapContent), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := AuditMapPair(tmpMap, "", "testpkg", "leaky", AuditOptions{SkipDecomp: true})
	if err != nil {
		t.Fatalf("AuditMapPair: %v", err)
	}

	if res.Forward.Verdict != ForwardCategoryA {
		t.Errorf("forward verdict = %s, want %s", res.Forward.Verdict, ForwardCategoryA)
	}
	if !res.Forward.GoLeaked {
		t.Errorf("expected Go qbsp to report leak on leaky map")
	}
	if res.Forward.RefAvailable && !res.Forward.RefLeaked {
		t.Errorf("expected ericw-tools qbsp to report leak on leaky map")
	}
}

func TestAuditMapPair_Options(t *testing.T) {
	dir := roomPairDir(t)
	mapPath := filepath.Join(dir, "room.map")
	bspPath := filepath.Join(dir, "room.bsp")

	// Skip forward compile
	res1, err := AuditMapPair(mapPath, bspPath, "testpkg", "room", AuditOptions{SkipForward: true})
	if err != nil {
		t.Fatal(err)
	}
	if res1.Forward.Verdict != ForwardSkipped {
		t.Errorf("expected ForwardSkipped, got %s", res1.Forward.Verdict)
	}

	// Skip decompilation
	res2, err := AuditMapPair(mapPath, bspPath, "testpkg", "room", AuditOptions{SkipDecomp: true})
	if err != nil {
		t.Fatal(err)
	}
	if res2.Decomp.Verdict != DecompSkipped {
		t.Errorf("expected DecompSkipped, got %s", res2.Decomp.Verdict)
	}
}

func TestFormatAuditReport(t *testing.T) {
	results := []AuditResult{
		{
			PkgID: "pkg1", MapID: "clean_map",
			Forward: ForwardCompileStats{Verdict: ForwardClean},
			Decomp:  DecompCompileStats{Verdict: DecompClean},
		},
		{
			PkgID: "pkg2", MapID: "inherent_void",
			Forward: ForwardCompileStats{Verdict: ForwardCategoryA, GoLeaked: true, RefAvailable: true, RefLeaked: true},
			Decomp:  DecompCompileStats{Verdict: DecompSkipped},
		},
		{
			PkgID: "pkg3", MapID: "compiler_disparity",
			Forward: ForwardCompileStats{Verdict: ForwardCategoryB, GoLeaked: true, GoTrailLen: 8, RefAvailable: true, RefLeaked: false},
			Decomp:  DecompCompileStats{Verdict: DecompSkipped},
		},
		{
			PkgID: "pkg4", MapID: "decomp_void",
			Forward: ForwardCompileStats{Verdict: ForwardClean},
			Decomp:  DecompCompileStats{Verdict: DecompCategoryCBoth, GoLeaked: true, GoTrailLen: 5, RefAvailable: true, RefLeaked: true},
		},
	}

	report := FormatAuditReport(results)
	if !strings.Contains(report, "Category A: Inherent Geometry Voids") {
		t.Errorf("missing Category A section")
	}
	if !strings.Contains(report, "Category B: Go Compiler Disparities") {
		t.Errorf("missing Category B section")
	}
	if !strings.Contains(report, "Category C: Decompiled Map Voids") {
		t.Errorf("missing Category C section")
	}
	if !strings.Contains(report, "inherent_void") {
		t.Errorf("missing inherent_void in report")
	}
	if !strings.Contains(report, "compiler_disparity") {
		t.Errorf("missing compiler_disparity in report")
	}
	if !strings.Contains(report, "decomp_void") {
		t.Errorf("missing decomp_void in report")
	}
}
