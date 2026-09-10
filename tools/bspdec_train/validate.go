package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
)

// regressionTol is how far below the recorded best val F1 the re-validation
// may drop before the guard fires (spec section 10 regression guard).
const regressionTol = 0.05

// validateModel re-scores the packaged Route A model on the corpus' val
// split and compares against the recorded metadata: (1) the corpus must be
// the one the model was trained on (dataset SHA match), (2) val F1 must not
// regress beyond the tolerance. Any violation is the guard firing.
func validateModel(recs []mapRecord, modelDir string, datasetSHA string) error {
	metaPath := filepath.Join(modelDir, "route-a-v"+routeAVersion, "metadata.json")
	raw, err := os.ReadFile(metaPath)
	if err != nil {
		return fmt.Errorf("regression guard: metadata: %w", err)
	}
	var meta modelMetadata
	if err := json.Unmarshal(raw, &meta); err != nil {
		return fmt.Errorf("regression guard: corrupt metadata: %w", err)
	}
	if meta.DatasetSHA != datasetSHA {
		return fmt.Errorf("regression guard fired: corpus drift (model trained on dataset %s, current %s)", shortSHA(meta.DatasetSHA), shortSHA(datasetSHA))
	}
	model, err := bspdec.LoadSeamModel(modelDir)
	if err != nil {
		return err
	}
	var valSet []Sample
	for _, r := range recs {
		if r.Split != "val" {
			continue
		}
		samples, err := samplesForMap(&r)
		if err != nil {
			continue
		}
		valSet = append(valSet, samples...)
	}
	m := &Model{Weights: model.Weights, Bias: model.Bias}
	_, _, f := m.evaluate(valSet, model.Mean, model.Std)
	if f < meta.Metrics.ValF1-regressionTol {
		return fmt.Errorf("regression guard fired: val F1 %.3f < recorded %.3f (tol %.2f)", f, meta.Metrics.ValF1, regressionTol)
	}
	return nil
}
func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
