package main

import (
	"encoding/json"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func TestTrainDeterministicAndSeparable(t *testing.T) {
	// two fully separable clusters: long interior seams (feat 4 = 0.5, feat
	// 5 high) vs short outline edges (feat 4 = 1.25, feat 5 = 0)
	mk := func(n int, long bool) []Sample {
		var out []Sample
		for i := 0; i < n; i++ {
			f := make([]float64, nFeatures)
			if long {
				f[0], f[4], f[5] = 8, 0.5, 2
			} else {
				f[0], f[4], f[5] = 1, 1.25, 0
			}
			out = append(out, Sample{Features: f, Seam: long})
		}
		return out
	}
	train := append(mk(200, true), mk(200, false)...)
	val := append(mk(50, true), mk(50, false)...)
	mean, std := featureStats(train)
	m1 := trainLogistic(train, mean, std)
	m2 := trainLogistic(train, mean, std)
	for i := range m1.Weights {
		if m1.Weights[i] != m2.Weights[i] {
			t.Fatalf("training not deterministic: weight %d: %v vs %v", i, m1.Weights[i], m2.Weights[i])
		}
	}
	if auc := m1.auc(val, mean, std); auc < 0.99 {
		t.Fatalf("val AUC = %v, want ~1.0 on separable data", auc)
	}
	bad := &Model{Weights: make([]float64, nFeatures), Bias: 0}
	if auc := bad.auc(val, mean, std); math.Abs(auc-0.5) > 0.01 {
		t.Fatalf("constant predictor AUC = %v, want 0.5", auc)
	}
}

func TestExportRouteAPackages(t *testing.T) {
	out := t.TempDir()
	m := &Model{Weights: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}, Bias: -0.25}
	mean := []float64{1, 1, 1, 1, 1, 1, 1}
	std := []float64{1, 1, 1, 1, 1, 1, 1}
	dir, err := exportRouteA(out, m, mean, std, metrics{ValAUC: 0.9, ValP: 0.91, ValR: 0.89}, "deadbeef", 0x5EED)
	if err != nil {
		t.Fatal(err)
	}
	_ = math.Abs
	var art modelArtifact
	b, err := os.ReadFile(filepath.Join(dir, "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &art); err != nil {
		t.Fatal(err)
	}
	if len(art.Weights) != nFeatures || art.Bias != -0.25 || art.Schema != featureSchema {
		t.Fatalf("model.json payload wrong: %+v", art)
	}
	var meta modelMetadata
	b, err = os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Route != "a" || meta.DatasetSHA != "deadbeef" || meta.Metrics.ValAUC != 0.9 || len(meta.ClassLabels) != 2 {
		t.Fatalf("metadata wrong: %+v", meta)
	}
	if meta.Quantizer != "z-score" {
		t.Fatalf("quantizer = %q", meta.Quantizer)
	}
}

func TestHeuristicF1AndGate(t *testing.T) {
	// separable fixture: interior seams (f4 small, f5>0) vs outline runs
	mk := func(n int, seam bool) []Sample {
		var out []Sample
		for i := 0; i < n; i++ {
			f := make([]float64, nFeatures)
			f[3] = 1
			if seam {
				f[0], f[4], f[5] = 8, 0.25, 2
			} else {
				f[0], f[4], f[5] = 1, 1.0, 0
			}
			out = append(out, Sample{Features: f, Seam: seam})
		}
		return out
	}
	// heuristic on perfectly separable data: perfect on this fixture
	_, _, hf := heuristicModel{}.f1(append(mk(50, true), mk(50, false)...))
	if hf < 0.99 {
		t.Fatalf("heuristic F1 = %v, want ~1.0 on separable fixture", hf)
	}
	// gate logic
	if !gateVerdict(0.90, 0.80) {
		t.Fatal("gate must pass when ML beats the heuristic")
	}
	if gateVerdict(0.80, 0.90) {
		t.Fatal("gate must fail when ML trails the heuristic")
	}
	if gateVerdict(0.80, 0.80) {
		t.Fatal("gate must fail on a tie (strictly greater required)")
	}
}

// TestGateMLBeatsHeuristicOnCorpusDistribution: on data shaped like the
// corpus (positive-skewed, mild noise), the trained model clears the rule.
func TestGateMLBeatsHeuristicOnCorpusDistribution(t *testing.T) {
	rng := newRand(7)
	mk := func(n int, seam bool) []Sample {
		var out []Sample
		for i := 0; i < n; i++ {
			f := make([]float64, nFeatures)
			f[3] = 1
			if seam {
				// interior join: small centroid distance, on a seam-rich face
				f[0] = 4 + rng.Float64()*4
				f[4] = 0.1 + rng.Float64()*0.5 // overlaps the heuristic boundary
				f[5] = 0.7 + rng.Float64()*0.9
			} else {
				// outline run: farther from centroid, fewer face seams
				f[0] = 0.5 + rng.Float64()*2
				f[4] = 0.2 + rng.Float64()*0.8 // overlaps the positive range
				f[5] = rng.Float64() * 0.6
			}
			out = append(out, Sample{Features: f, Seam: seam})
		}
		return out
	}
	train := append(mk(400, true), mk(200, false)...)
	test := append(mk(200, true), mk(100, false)...)
	mean, std := featureStats(train)
	model := trainLogistic(train, mean, std)
	mlF1 := modelBestF1(model, test, mean, std)
	_, _, heurF1 := heuristicModel{}.f1(test)
	if !gateVerdict(mlF1, heurF1) {
		t.Fatalf("gate failed: ml_f1=%.3f heuristic_f1=%.3f on separable-with-noise data", mlF1, heurF1)
	}
}

// newRand returns a deterministic rand source for synthetic fixtures.
func newRand(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}
