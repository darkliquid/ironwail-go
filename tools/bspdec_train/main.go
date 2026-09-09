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
	flag.Parse()

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
	// Task 4 lands the Route A feature extraction and train loop here; the
	// CLI contract (scan -> train -> package -> metadata.json) is exercised
	// end-to-end by the acceptance run once it does.
	slog.Info("bspdec-train: pipeline ready; train loop lands with the feature extraction task (plan Task 3/4)")
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "bspdec-train:", msg)
	os.Exit(1)
}
