// Command bspdec_train is the M2 training pipeline (spec section 10): it
// scans the labeled corpus, derives per-route training samples, runs the
// seed-pinned train loops, and packages versioned models with metadata.json
// under models/bspdec/route-a-vX/. Every step caches under
// <data>/.train-cache/<dataset-sha>/<config-hash>/.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
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

	if *route != "a" && *route != "all" {
		fatal(fmt.Sprintf("unknown route %q (want a|all)", *route))
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
	if *route == "a" || *route == "all" {
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
	}
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
	var trainSet, valSet, testSet []Sample
	for _, r := range recs {
		samples, err := samplesForMap(&r)
		if err != nil {
			slog.Warn("bspdec-train: feature extraction failed", "map", r.PkgID+"/"+r.MapID, "err", err)
			continue
		}
		slog.Info("bspdec-train: features", "map", r.PkgID+"/"+r.MapID, "samples", len(samples))
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
	if _, err := exportRouteA(out, model, mean, std, metrics{ValAUC: valAUC, ValP: valP, ValR: valR}, datasetSHA, seed); err != nil {
		return nil, err
	}
	slog.Info("bspdec-train: packaged", "dir", fmt.Sprintf("%s/route-a-v%s", out, routeAVersion))
	return &routeAStats{MLF1: mlF1, HeurF1: heurF1, TestAUC: testAUC}, nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "bspdec-train:", msg)
	os.Exit(1)
}
