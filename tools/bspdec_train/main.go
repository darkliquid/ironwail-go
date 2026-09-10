// Command bspdec_train is the M2 training pipeline (spec section 10): it
// scans the labeled corpus, derives per-route training samples, runs the
// seed-pinned train loops, and packages versioned models with metadata.json
// under models/bspdec/route-a-vX/. Every step caches under
// <data>/.train-cache/<dataset-sha>/<config-hash>/.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// featureSchema bumps whenever Route A's feature vector layout changes;
// it is part of the cache and model keys.
const featureSchema = "route-a-features-v1"

func main() {
	dataDir := flag.String("data", "dataset/bspdec", "corpus root")
	route := flag.String("route", "a", "route to train: a (seams) | all")
	seed := flag.Int64("seed", 0x5EED, "training seed (pinned)")
	out := flag.String("out", "models/bspdec", "model registry root")
	validate := flag.Bool("validate", false, "run the regression guard on the packaged model and exit 1 on regression")
	gate := flag.Bool("gate", false, "run the Route A held-out gate (ML F1 vs deterministic split heuristic) and exit 1 on failure")
	flag.Parse()

	if *validate {
		recs, datasetSHA, err := scanCorpus(*dataDir)
		if err != nil {
			fatal("scan: " + err.Error())
		}
		if err := validateModel(recs, *out, datasetSHA); err != nil {
			fatal(err.Error())
		}
		slog.Info("bspdec-train: validation passed", "model", "route-a-v"+routeAVersion)
		return
	}

	if *route != "a" && *route != "b" && *route != "all" {
		fatal(fmt.Sprintf("unknown route %q (want a|b|all)", *route))
	}

	recs, datasetSHA, err := scanCorpus(*dataDir)
	if err != nil {
		fatal("scan: " + err.Error())
	}
	var train, val, test, seams int
	for _, r := range recs {
		switch r.Split {
		case "train":
			train++
		case "val":
			val++
		case "test":
			test++
		}
		seams += len(r.Labels.Seams)
	}
	cfg := configHash(*route, *seed, featureSchema)
	cacheDir := fmt.Sprintf("%s/.train-cache/%s/%s", *dataDir, datasetSHA[:16], cfg[:16])
	slog.Info("bspdec-train: corpus scan",
		"dataset", datasetSHA[:12], "maps", len(recs),
		"train/val/test", fmt.Sprintf("%d/%d/%d", train, val, test),
		"seam_samples", seams, "cache", cacheDir, "out", *out)

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fatal(err.Error())
	}
	switch {
	case *route == "a" || *route == "all":
		stats, err := runRouteA(recs, cacheDir, *out, datasetSHA, *seed)
		if err != nil {
			fatal(err.Error())
		}
		if *gate {
			passed := gateVerdict(stats.MLF1, stats.HeurF1)
			slog.Info("bspdec-train: route-a gate",
				"ml_f1", fmt.Sprintf("%.3f", stats.MLF1),
				"heuristic_f1", fmt.Sprintf("%.3f", stats.HeurF1),
				"test_auc", fmt.Sprintf("%.3f", stats.TestAUC),
				"verdict", gateNote(passed))
			if !passed {
				fatal(gateNote(false))
			}
		}
	case *route == "b":
		stats, err := runRouteB(recs, *out, datasetSHA, *seed)
		if err != nil {
			fatal(err.Error())
		}
		if *gate {
			passed := routeBGate(stats)
			slog.Info("bspdec-train: route-b gate",
				"ml_f1", fmt.Sprintf("%.3f", stats.MLF1),
				"baseline_f1", fmt.Sprintf("%.3f", stats.BaselineF1),
				"truth_edges", stats.TruthEdges,
				"verdict", groupGateNote(passed))
			if !passed {
				fatal(groupGateNote(false))
			}
		}
	}
}

// runRouteB trains the cell-grouping classifier (Route B, spec 7.2): pair
// candidates from coplanar adjacency of the unmerged cells, trained on the
// train split, evaluated as group-recovery edge F1 versus the
// --merge-convex baseline on the held-out val split. Packages
// route-b-v0.1.0 with metadata.
func runRouteB(recs []mapRecord, out, datasetSHA string, seed int64) (*routeBStats, error) {
	var trainPairs, valPairs []groupPair
	for _, r := range recs {
		// the gate evaluates on the held-out test split (route A uses it too)
		if r.Split != "train" && r.Split != "test" {
			continue
		}
		gc, err := groupSamplesForRecord(&r)
		if err != nil {
			slog.Warn("bspdec-train: route-b extraction failed", "map", r.PkgID+"/"+r.MapID, "err", err)
			continue
		}
		slog.Info("bspdec-train: route-b pairs", "map", r.PkgID+"/"+r.MapID,
			"pairs", len(gc.Pairs), "truth", gc.TruthEdges, "baseline", gc.BaselineEdges)
		if r.Split == "train" {
			trainPairs = append(trainPairs, gc.Pairs...)
		} else {
			valPairs = append(valPairs, gc.Pairs...)
		}
	}
	if len(trainPairs) == 0 {
		return nil, fmt.Errorf("no route-b training pairs")
	}
	samples := make([]Sample, len(trainPairs))
	for i, p := range trainPairs {
		samples[i] = Sample{Features: p.Features, Seam: p.Merge}
	}
	mean, std := featureStats(samples)
	model := trainLogistic(samples, mean, std)
	valSamples := make([]Sample, len(valPairs))
	for i, p := range valPairs {
		valSamples[i] = Sample{Features: p.Features, Seam: p.Merge}
	}
	// ML grouping = the --merge-convex baseline union the learned additions;
	// score both against the same true-edge set at each threshold.
	base := func(p groupPair) bool { return p.Features[4] == 1 }
	mlAt := func(th float64) func(p groupPair) bool {
		return func(p groupPair) bool {
			return base(p) || model.predict(normalizeFeatures(p.Features, mean, std)) >= th
		}
	}
	mlBest := 0.0
	for th := 0.05; th <= 0.95; th += 0.05 {
		_, _, f := edgeF1(valPairs, mlAt(th))
		if f > mlBest {
			mlBest = f
		}
	}
	mlF1 := mlBest
	_, _, baselineF1 := edgeF1(valPairs, base)
	_, _, mlR := edgeF1(valPairs, mlAt(0.5))
	_, _, baseR := edgeF1(valPairs, base)
	truthTest := 0
	for _, p := range valPairs {
		if p.Merge {
			truthTest++
		}
	}
	if _, err := exportRoute(out, "b", "0.1.0", groupSchema, model, mean, std, metrics{ValAUC: model.auc(valSamples, mean, std)}, datasetSHA, seed); err != nil {
		return nil, err
	}
	slog.Info("bspdec-train: route-b trained",
		"train_pairs", len(trainPairs), "test_pairs", len(valPairs),
		"ml_f1", fmt.Sprintf("%.3f", mlF1), "baseline_f1", fmt.Sprintf("%.3f", baselineF1),
		"ml_recall@0.5", fmt.Sprintf("%.3f", mlR), "baseline_recall", fmt.Sprintf("%.3f", baseR))
	slog.Info("bspdec-train: packaged", "dir", fmt.Sprintf("%s/route-b-v0.1.0", out))
	return &routeBStats{MLF1: mlF1, BaselineF1: baselineF1, MLR: mlR, BaselineR: baseR, TruthEdges: truthTest, Inconclusive: truthTest < 10}, nil
}

// runRouteA extracts Route A features, trains the seam classifier on the
// train split, reports validation metrics, and packages the versioned model
// with metadata.json (spec section 7.4).
// routeAStats carries the gate inputs for the Route A run: held-out F1 for
// the ML model and the deterministic split heuristic, plus test AUC.
type routeAStats struct {
	MLF1    float64
	HeurF1  float64
	TestAUC float64
}

// gateVerdict reports whether the ML classifier beats the deterministic
// split heuristic on held-out data (bead xxy.6 acceptance).
func gateVerdict(mlF1, heurF1 float64) bool { return mlF1 > heurF1 }

func gateNote(passed bool) string {
	if passed {
		return "pass: ML seam F1 beats the deterministic split heuristic on held-out data"
	}
	return "fail: ML does not beat the deterministic split heuristic on held-out data (spec 13: descope Route C; Route D becomes the primary research track)"
}

func runRouteA(recs []mapRecord, cacheDir, out, datasetSHA string, seed int64) (*routeAStats, error) {
	var trainSet, valSet, testSet, classicSet []Sample
	for _, r := range recs {
		samples, err := samplesForMap(&r)
		if err != nil {
			slog.Warn("bspdec-train: feature extraction failed", "map", r.PkgID+"/"+r.MapID, "err", err)
			continue
		}
		slog.Info("bspdec-train: features", "map", r.PkgID+"/"+r.MapID,
			"samples", len(samples), "era", r.Era, "split", r.Split)
		// M4: classic (real-map) data stays OUT of the ML train distribution;
		// it becomes the distribution-shift slice the model is scored against.
		if r.Era != "synthetic" && r.Era != "" {
			classicSet = append(classicSet, samples...)
			continue
		}
		switch r.Split {
		case "train":
			trainSet = append(trainSet, samples...)
		case "val":
			valSet = append(valSet, samples...)
		case "test":
			testSet = append(testSet, samples...)
		}
	}
	if len(trainSet) == 0 {
		return nil, fmt.Errorf("no training samples")
	}
	mean, std := featureStats(trainSet)
	model := trainLogistic(trainSet, mean, std)
	valAUC := model.auc(valSet, mean, std)
	testAUC := model.auc(testSet, mean, std)
	mlF1 := modelBestF1(model, testSet, mean, std)
	_, _, heurF1 := heuristicModel{}.f1(testSet)
	valP, valR, _ := model.evaluate(valSet, mean, std)
	slog.Info("bspdec-train: route-a trained",
		"train_samples", len(trainSet), "val_samples", len(valSet), "test_samples", len(testSet),
		"val_auc", fmt.Sprintf("%.3f", valAUC), "val_p/r@0.5", fmt.Sprintf("%.3f/%.3f", valP, valR),
		"test_auc", fmt.Sprintf("%.3f", testAUC), "test_ml_f1", fmt.Sprintf("%.3f", mlF1),
		"test_heuristic_f1", fmt.Sprintf("%.3f", heurF1),
		"cache", cacheDir)
	// distribution-shift slice: model trained purely on synthetic, scored
	// against the classic maps held out of training (M4)
	classicAUC := 0.5
	if len(classicSet) > 0 {
		classicAUC = model.auc(classicSet, mean, std)
		_, rc, f := model.evaluate(classicSet, mean, std)
		rep := shiftReport{Samples: len(classicSet), AUC: classicAUC, F1At05: f, RecallAt05: rc}
		raw, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(filepath.Join(out, "route-a-v"+routeAVersion, "shift.json"), raw, 0o644); err != nil {
			return nil, err
		}
		slog.Info("bspdec-train: distribution shift", "classic_samples", len(classicSet), "classic_auc", fmt.Sprintf("%.3f", classicAUC))
	}
	if _, err := exportRoute(out, "a", routeAVersion, featureSchema, model, mean, std, metrics{ValAUC: valAUC, ValP: valP, ValR: valR}, datasetSHA, seed); err != nil {
		return nil, err
	}
	slog.Info("bspdec-train: packaged", "dir", fmt.Sprintf("%s/route-a-v%s", out, routeAVersion))
	return &routeAStats{MLF1: mlF1, HeurF1: heurF1, TestAUC: testAUC}, nil
}

// shiftReport records the classic-map (out-of-distribution) evaluation:
// how the synthetic-trained model transfers to vintage real maps, written
// as shift.json next to the packaged model (M4 acceptance).
type shiftReport struct {
	Samples    int     `json:"samples"`
	AUC        float64 `json:"auc"`
	F1At05     float64 `json:"f1@0.5"`
	RecallAt05 float64 `json:"recall@0.5"`
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "bspdec-train:", msg)
	os.Exit(1)
}
