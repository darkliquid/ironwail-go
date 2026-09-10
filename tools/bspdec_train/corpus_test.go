package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// writeCorpusFixture fabricates a scanable corpus: two packages split into
// train/test, each map with a sibling labels.json carrying one seam record.
func writeCorpusFixture(t *testing.T, dataDir string) {
	t.Helper()
	mk := func(pkg, stem string) {
		t.Helper()
		dir := filepath.Join(dataDir, "labeled", pkg)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		rec := labelRecord{
			MapID: stem + ".map",
			Seams: []bspdec.SeamLabel{{
				SideBrush: 0,
				Edge:      [2]mapfile.Vec3{{X: 0, Y: 0, Z: 0}, {X: 8, Y: 0, Z: 0}},
				Seam:      true,
			}},
			Stats: map[string]int{"seams": 1},
		}
		b, _ := json.Marshal(&rec)
		if err := os.WriteFile(filepath.Join(dir, stem+".labels.json"), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mk("pkga", "a1")
	mk("pkga", "a2")
	mk("pkgb", "b1")
	manifest := []byte(
		`{"pkg_id":"pkga","license_note":"x","map_files":["maps/a1.map","maps/a2.map"]}` + "\n" +
			`{"pkg_id":"pkgb","license_note":"x","map_files":["maps/b1.map"]}` + "\n")
	if err := os.MkdirAll(filepath.Join(dataDir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "raw", "manifest.jsonl"), manifest, 0o644); err != nil {
		t.Fatal(err)
	}
	splits := map[string]any{
		"train": []string{"pkga"}, "val": []string{}, "test": []string{"pkgb"}, "classic_holdout": []string{},
	}
	b, _ := json.Marshal(splits)
	if err := os.WriteFile(filepath.Join(dataDir, "splits.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanCorpusAndSplits(t *testing.T) {
	dataDir := t.TempDir()
	writeCorpusFixture(t, dataDir)
	recs, sha1, err := scanCorpus(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("records = %d, want 3", len(recs))
	}
	byID := map[string]string{}
	for _, r := range recs {
		byID[r.PkgID+"-"+r.MapID] = r.Split
		if len(r.Labels.Seams) != 1 {
			t.Fatalf("%s/%s seams = %d, want 1", r.PkgID, r.MapID, len(r.Labels.Seams))
		}
	}
	if byID["pkga-a1"] != "train" || byID["pkga-a2"] != "train" || byID["pkgb-b1"] != "test" {
		t.Fatalf("split assignment wrong: %+v", byID)
	}
	if len(sha1) != 64 {
		t.Fatalf("dataset sha malformed: %q", sha1)
	}
	_, sha2, err := scanCorpus(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	if sha1 != sha2 {
		t.Fatalf("dataset sha unstable: %s vs %s", sha1, sha2)
	}
}

func TestScanCorpusSkipsUnlabeled(t *testing.T) {
	dataDir := t.TempDir()
	writeCorpusFixture(t, dataDir)
	// an entry whose label file is missing must not appear
	if err := os.WriteFile(filepath.Join(dataDir, "raw", "manifest.jsonl"),
		[]byte(`{"pkg_id":"pkga","license_note":"x","map_files":["maps/a1.map","maps/a2.map"]}`+"\n"+
			`{"pkg_id":"nofile","license_note":"x","map_files":["maps/z.map"]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	recs, _, err := scanCorpus(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		if r.PkgID == "nofile" {
			t.Fatalf("unlabeled package surfaced: %+v", r)
		}
	}
}

func TestConfigHashDeterministicAndDistinct(t *testing.T) {
	a1 := configHash("a", 42, featureSchema)
	a2 := configHash("a", 42, featureSchema)
	if a1 != a2 {
		t.Fatalf("config hash unstable: %s vs %s", a1, a2)
	}
	if configHash("a", 43, featureSchema) == a1 {
		t.Fatal("seed change did not change the config hash")
	}
	if configHash("b", 42, featureSchema) == a1 {
		t.Fatal("route change did not change the config hash")
	}
	if configHash("a", 42, "v2") == a1 {
		t.Fatal("schema bump did not change the config hash")
	}
}

func TestValidateGuardFiresOnRegression(t *testing.T) {
	out := t.TempDir()
	mk := func(n int, seam bool) []Sample {
		var out []Sample
		for i := 0; i < n; i++ {
			f := make([]float64, nFeatures)
			if seam {
				f[0], f[4], f[5] = 8, 0.5, 2
			} else {
				f[0], f[4], f[5] = 1, 1.25, 0
			}
			out = append(out, Sample{Features: f, Seam: seam})
		}
		return out
	}
	train := append(mk(200, true), mk(200, false)...)
	val := append(mk(80, true), mk(80, false)...)
	mean, std := featureStats(train)
	model := trainLogistic(train, mean, std)
	if _, err := exportRouteA(out, model, mean, std, metrics{ValAUC: model.auc(val, mean, std)}, "cafef00d", 0x5EED); err != nil {
		t.Fatal(err)
	}
	sm := &bspdec.SeamModel{Weights: model.Weights, Bias: model.Bias, Mean: mean, Std: std}
	if err := guardCheck(sm, val, model.auc(val, mean, std)); err != nil {
		t.Fatalf("guard must pass for the trained model: %v", err)
	}
	bad := &bspdec.SeamModel{Weights: make([]float64, nFeatures), Bias: 0, Mean: mean, Std: std}
	if err := guardCheck(bad, val, 1.0); err == nil {
		t.Fatal("guard must fire on weight corruption (AUC collapses to 0.5)")
	}
	if err := validateModel(nil, out, "deadbeef"); err == nil {
		t.Fatal("guard must fire on dataset SHA mismatch")
	}
}
