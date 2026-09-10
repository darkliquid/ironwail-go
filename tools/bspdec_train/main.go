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
		if err := runRouteA(recs, cacheDir, *out, datasetSHA, *seed); err != nil {
			fatal(err.Error())
		}
	}
}

// runRouteA extracts Route A features, trains the seam classifier on the
// train split, reports validation metrics, and packages the versioned model
// with metadata.json (spec section 7.4).
func runRouteA(recs []mapRecord, cacheDir, out, datasetSHA string, seed int64) error {
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
		return fmt.Errorf("no training samples")
	}
	mean, std := featureStats(trainSet)
	model := trainLogistic(trainSet, mean, std)
	valP, valR, valF1 := model.evaluate(valSet, mean, std)
	testP, testR, testF1 := model.evaluate(testSet, mean, std)
	slog.Info("bspdec-train: route-a trained",
		"train_samples", len(trainSet), "val_samples", len(valSet), "test_samples", len(testSet),
		"val_f1", fmt.Sprintf("%.3f", valF1), "val_p/r", fmt.Sprintf("%.3f/%.3f", valP, valR),
		"test_f1", fmt.Sprintf("%.3f", testF1), "test_p/r", fmt.Sprintf("%.3f/%.3f", testP, testR),
		"cache", cacheDir)
	if _, err := exportRouteA(out, model, mean, std, metrics{ValF1: valF1, ValP: valP, ValR: valR}, datasetSHA, seed); err != nil {
		return err
	}
	slog.Info("bspdec-train: packaged", "dir", fmt.Sprintf("%s/route-a-v%s", out, routeAVersion))
	return nil
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "bspdec-train:", msg)
	os.Exit(1)
}
