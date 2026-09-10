package main

import (
	"encoding/json"

	"os"
	"path/filepath"
	"time"
)

// version is the packaged Route A model version (mise run bspdec-models
// bumps it deliberately; spec section 7.4 route-a-v<semver>).
const routeAVersion = "0.1.0"

// metrics are the recorded validation numbers carried in metadata.json.
type metrics struct {
	ValAUC float64 `json:"val_auc"`
	ValP   float64 `json:"val_p"`
	ValR   float64 `json:"val_r"`
}

// modelArtifact is model.json: schema + weights + quantizer params, the
// payload the --ml loader consumes.
type modelArtifact struct {
	Schema  string    `json:"schema"` // route-a-features-v1
	Weights []float64 `json:"weights"`
	Bias    float64   `json:"bias"`
	Mean    []float64 `json:"mean"`
	Std     []float64 `json:"std"`
	Classes []string  `json:"classes"` // [interior, seam]
}

// modelMetadata is metadata.json (spec section 7.4 fields): inputs, classes,
// quantizer params, corpus provenance, pinned compiler, seed, metrics.
type modelMetadata struct {
	Route       string    `json:"route"`
	Version     string    `json:"version"`
	InputSchema string    `json:"input_schema"`
	ClassLabels []string  `json:"class_labels"`
	Quantizer   string    `json:"quantizer"` // z-score per feature
	FeatureMean []float64 `json:"feature_mean,omitempty"`
	FeatureStd  []float64 `json:"feature_std,omitempty"`
	DatasetSHA  string    `json:"dataset_sha"`
	QbspPin     string    `json:"qbsp_pin"`
	Seed        int64     `json:"seed"`
	Metrics     metrics   `json:"metrics"`
	CreatedAt   string    `json:"created_at"`
}

// qbspPin identifies the compiler the corpus was built with; sourced from
// the module revision the pipeline runs under (git describe, best effort).
var qbspPin = func() string {
	if b, err := os.ReadFile("vcs.revision"); err == nil {
		return string(b)
	}
	return "local"
}()

// exportRoute packages a trained model under <out>/<route>-v<version>/ with
// model.json + metadata.json and returns the artifact directory. Re-runs
// overwrite in place (the corpus SHA in metadata distinguishes versions).
func exportRoute(out, route, version, schema string, m *Model, mean, std []float64, met metrics, datasetSHA string, seed int64) (string, error) {
	dir := filepath.Join(out, "route-"+route+"-v"+version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	art := modelArtifact{
		Schema:  schema,
		Weights: m.Weights,
		Bias:    m.Bias,
		Mean:    mean,
		Std:     std,
		Classes: []string{"interior", "seam"},
	}
	raw, err := json.MarshalIndent(art, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "model.json"), raw, 0o644); err != nil {
		return "", err
	}
	meta := modelMetadata{
		Route:       "a",
		Version:     routeAVersion,
		InputSchema: featureSchema,
		ClassLabels: []string{"interior", "seam"},
		Quantizer:   "z-score",
		DatasetSHA:  datasetSHA,
		QbspPin:     qbspPin,
		Seed:        seed,
		Metrics:     met,
		CreatedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	raw, err = json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.json"), raw, 0o644); err != nil {
		return "", err
	}
	return dir, nil
}
