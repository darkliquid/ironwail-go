package main

import (
	"bytes"
	"os"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// Sample is one Route A classification target: a collinear segment along a
// merged coplanar face, labelled seam (positive, from original-brush truth)
// or interior (negative, a face-outline run).
type Sample struct {
	Features []float64
	Seam     bool
}

// nFeatures must match the training schema (route-a-features-v1); both
// train and inference share bspdec.SeamCandidates' feature math.
const nFeatures = bspdec.SeamFeatureCount

// samplesForMap decompiles one map with the label-derivation options and
// extracts its Route A samples from the shared candidate extractor, using
// the original map's brush planes as ground truth.
func samplesForMap(rec *mapRecord) ([]Sample, error) {
	data, err := os.ReadFile(rec.BSPPath)
	if err != nil {
		return nil, err
	}
	outMap, _, err := bspdec.Decompile(data, bspdec.Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		return nil, err
	}
	cells, err := bspdec.FacePolygons(outMap)
	if err != nil {
		return nil, err
	}
	origData, err := os.ReadFile(rec.MapPath)
	if err != nil {
		return nil, err
	}
	orig, err := mapfile.Parse(bytes.NewReader(origData))
	if err != nil {
		return nil, err
	}
	cands := bspdec.SeamCandidates(cells, bspdec.OriginalBrushPlanes(orig))
	samples := make([]Sample, len(cands))
	for i, c := range cands {
		samples[i] = Sample{Features: c.Features, Seam: c.Seam}
	}
	return samples, nil
}
