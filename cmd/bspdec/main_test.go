package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	"github.com/darkliquid/ironwail-go/internal/qbsp"
)

// box is a minimal QuakeEd box brush (winding pattern compile-proven in the
// qbsp suite's slabBrush helper).
func box(x0, y0, z0, x1, y1, z1 float64, tex string) string {
	f := func(p1, p2, p3 [3]float64) string {
		return fmt.Sprintf("( %g %g %g ) ( %g %g %g ) ( %g %g %g ) %s 0 0 0 1 1\n",
			p1[0], p1[1], p1[2], p2[0], p2[1], p2[2], p3[0], p3[1], p3[2], tex)
	}
	mi := [3]float64{x0, y0, z0}
	ma := [3]float64{x1, y1, z1}
	return "{\n" +
		f([3]float64{ma[0], mi[1], mi[2]}, [3]float64{ma[0], mi[1], ma[2]}, [3]float64{ma[0], ma[1], mi[2]}) +
		f([3]float64{mi[0], ma[1], mi[2]}, [3]float64{mi[0], ma[1], ma[2]}, [3]float64{mi[0], mi[1], ma[2]}) +
		f([3]float64{mi[0], ma[1], mi[2]}, [3]float64{ma[0], ma[1], mi[2]}, [3]float64{mi[0], ma[1], ma[2]}) +
		f([3]float64{mi[0], mi[1], mi[2]}, [3]float64{mi[0], mi[1], ma[2]}, [3]float64{ma[0], mi[1], mi[2]}) +
		f([3]float64{mi[0], mi[1], ma[2]}, [3]float64{mi[0], ma[1], ma[2]}, [3]float64{ma[0], mi[1], ma[2]}) +
		f([3]float64{mi[0], mi[1], mi[2]}, [3]float64{ma[0], mi[1], mi[2]}, [3]float64{mi[0], ma[1], mi[2]}) +
		"}\n"
}

// writeTestBSP compiles a sealed 256-cube room and writes it to dir.
func writeTestBSP(t *testing.T, dir string) string {
	t.Helper()
	src := "{\n\"classname\" \"worldspawn\"\n" +
		box(-64, -64, -64, 0, 320, 256, "wwall") +
		box(256, -64, -64, 320, 320, 256, "wwall") +
		box(0, -64, -64, 256, 0, 256, "wwall") +
		box(0, 256, -64, 256, 320, 256, "wwall") +
		box(0, 0, -64, 256, 256, 0, "ffloor") +
		box(0, 0, 192, 256, 256, 256, "cceil") +
		"}\n{\n\"classname\" \"info_player_start\"\n\"origin\" \"128 128 64\"\n}\n"
	m, err := qbsp.ParseMap(strings.NewReader(src))
	if err != nil {
		t.Fatalf("ParseMap: %v", err)
	}
	res, err := qbsp.Compile(m, qbsp.Options{})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if res.Leaked {
		t.Fatal("test room leaks")
	}
	p := filepath.Join(dir, "room.bsp")
	if err := os.WriteFile(p, res.Data, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return p
}

func TestRunEndToEnd(t *testing.T) {
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	outPath := filepath.Join(dir, "out.map")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-o", outPath, bspPath}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run exit = %d, stderr: %s", code, stderr.String())
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "worldspawn") {
		t.Fatalf("output missing worldspawn:\n%s", data)
	}
	// default output path: <stem>.bspdec.map
	var so, se bytes.Buffer
	if code := run([]string{bspPath}, &so, &se); code != 0 {
		t.Fatalf("default-output run exit = %d: %s", code, se.String())
	}
	def := filepath.Join(dir, "room.bspdec.map")
	if _, err := os.Stat(def); err != nil {
		t.Fatalf("default output missing: %v", err)
	}
}

func TestRunJSONSummarySchema(t *testing.T) {
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	outPath := filepath.Join(dir, "out.map")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-json", "-o", outPath, bspPath}, &stdout, &stderr); code != 0 {
		t.Fatalf("run exit: stderr %s", stderr.String())
	}
	var doc map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	for _, k := range []string{"input", "output", "bspx_brushlist", "models", "ml", "exit"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing key %q in %v", k, doc)
		}
	}
	if doc["exit"].(float64) != 0 {
		t.Fatalf("exit field = %v", doc["exit"])
	}
	models := doc["models"].([]any)
	if len(models) == 0 {
		t.Fatal("no models in summary")
	}
	m0 := models[0].(map[string]any)
	for _, k := range []string{"model", "brushes", "leaves_solid", "planes_used", "warnings"} {
		if _, ok := m0[k]; !ok {
			t.Fatalf("missing model key %q in %v", k, m0)
		}
	}
	if m0["brushes"].(float64) < 6 {
		t.Fatalf("model 0 brushes = %v, want >= 6", m0["brushes"])
	}
	ml := doc["ml"].(map[string]any)
	if ml["stage"] != "none" || ml["model"] != nil {
		t.Fatalf("ml = %v, want {none, null}", ml)
	}
}

func TestRunExitCodes(t *testing.T) {
	var so, se bytes.Buffer
	if code := run([]string{"/nonexistent.bsp"}, &so, &se); code != 1 {
		t.Fatalf("missing input exit = %d, want 1", code)
	}
	if code := run([]string{}, &so, &se); code != 1 {
		t.Fatalf("no-args exit = %d, want 1", code)
	}
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	if code := run([]string{"-ml", "seams", bspPath}, &so, &se); code != 1 {
		t.Fatalf("--ml exit = %d, want 1 (no provisioned model)", code)
	}
	if code := run([]string{"-texture-fallback", "bogus", bspPath}, &so, &se); code != 1 {
		t.Fatalf("bad fallback exit = %d, want 1", code)
	}
	if code := run([]string{"-format", "obj", bspPath}, &so, &se); code != 1 {
		t.Fatalf("bad format exit = %d, want 1", code)
	}
}

func TestHasBSPXBrushlist(t *testing.T) {
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	data, err := os.ReadFile(bspPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !hasBSPXBrushlist(data) {
		t.Fatal("compiled fixture should carry a BRUSHLIST lump")
	}
	if hasBSPXBrushlist([]byte("no magic here")) {
		t.Fatal("junk data reported as BRUSHLIST")
	}
}
func TestRunMLSeams(t *testing.T) {
	dir := t.TempDir()
	bspPath := writeTestBSP(t, dir)
	modelDir := filepath.Join(dir, "models", "route-a-v0.1.0")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	art := bspdec.SeamModel{
		Schema:  "route-a-features-v1",
		Weights: make([]float64, bspdec.SeamFeatureCount),
		Bias:    0,
		Mean:    make([]float64, bspdec.SeamFeatureCount),
		Std:     make([]float64, bspdec.SeamFeatureCount),
	}
	for i := range art.Std {
		art.Std[i] = 1
	}
	b, _ := json.Marshal(art)
	if err := os.WriteFile(filepath.Join(modelDir, "model.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.map")
	var so, se bytes.Buffer
	code := run([]string{"-ml", "seams", "-model-dir", filepath.Join(dir, "models"), "-o", out, "-json", bspPath}, &so, &se)
	if code != 0 {
		t.Fatalf("--ml seams exit = %d, stderr: %s", code, se.String())
	}
	if _, err := os.Stat(out + ".seams.json"); err != nil {
		t.Fatalf("sidecar missing: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(so.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	ml := doc["ml"].(map[string]any)
	if ml["stage"] != "seams" || ml["model"] != "route-a-v0.1.0" {
		t.Fatalf("ml = %v, want {seams, route-a-v0.1.0}", ml)
	}
	if doc["scored"].(float64) < 6 {
		t.Fatalf("scored = %v, want >= 6 candidates", doc["scored"])
	}
}
