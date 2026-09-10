package main

import (
	"encoding/json"
	"math"
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
	p, r, f := m1.evaluate(val, mean, std)
	if f < 0.99 {
		t.Fatalf("val F1 = %v (p=%.2f r=%.2f), want ~1.0 on separable data", f, p, r)
	}
	if math.IsNaN(f) {
		t.Fatal("F1 is NaN")
	}
}

func TestExportRouteAPackages(t *testing.T) {
	out := t.TempDir()
	m := &Model{Weights: []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7}, Bias: -0.25}
	mean := []float64{1, 1, 1, 1, 1, 1, 1}
	std := []float64{1, 1, 1, 1, 1, 1, 1}
	dir, err := exportRouteA(out, m, mean, std, metrics{ValF1: 0.9, ValP: 0.91, ValR: 0.89}, "deadbeef", 0x5EED)
	if err != nil {
		t.Fatal(err)
	}
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
	if meta.Route != "a" || meta.DatasetSHA != "deadbeef" || meta.Metrics.ValF1 != 0.9 || len(meta.ClassLabels) != 2 {
		t.Fatalf("metadata wrong: %+v", meta)
	}
	if meta.Quantizer != "z-score" {
		t.Fatalf("quantizer = %q", meta.Quantizer)
	}
}
