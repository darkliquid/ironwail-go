package bspdec

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// SeamFeatureCount is the Route A feature vector length; it must match the
// training schema "route-a-features-v1" (tools/bspdec_train).
const SeamFeatureCount = 7

// SeamModel is the packaged Route A logistic-regression classifier
// (model.json under models/bspdec/route-a-v*/, spec section 7.4). The
// same artifact trains (tools/bspdec_train) and infers (--ml seams).
type SeamModel struct {
	Schema  string    `json:"schema"`
	Weights []float64 `json:"weights"`
	Bias    float64   `json:"bias"`
	Mean    []float64 `json:"mean"`
	Std     []float64 `json:"std"`
	Classes []string  `json:"classes"`
	Version string    `json:"version,omitempty"`
}

// LoadSeamModel loads the highest-version model.json under modelDir
// (expected <modelDir>/route-a-v*/model.json). Absent or corrupt artifacts
// produce a clear error (spec section 11: -ml without provisioned models).
func LoadSeamModel(modelDir string) (*SeamModel, error) {
	versions, err := filepath.Glob(filepath.Join(modelDir, "route-a-v*"))
	if err != nil {
		return nil, err
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("bspdec: no Route A model under %s (run mise run bspdec-train)", modelDir)
	}
	sort.Slice(versions, func(i, j int) bool {
		return versionOf(versions[i]) > versionOf(versions[j])
	})
	raw, err := os.ReadFile(filepath.Join(versions[0], "model.json"))
	if err != nil {
		return nil, err
	}
	var m SeamModel
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("bspdec: corrupt model %s: %w", versions[0], err)
	}
	if m.Schema != "route-a-features-v1" || len(m.Weights) != SeamFeatureCount {
		return nil, fmt.Errorf("bspdec: model %s schema/weights mismatch (schema=%q weights=%d)", versions[0], m.Schema, len(m.Weights))
	}
	m.Version = filepath.Base(versions[0])
	return &m, nil
}

func versionOf(dir string) int {
	base := filepath.Base(dir)
	rest := strings.TrimPrefix(base, "route-a-v")
	if f, err := strconv.ParseFloat(rest, 64); err == nil {
		return int(f * 1000)
	}
	return -1
}

// Score normalizes the features with the stored quantizer and returns the
// P(seam) logit.
func (m *SeamModel) Score(features []float64) float64 {
	if len(features) != SeamFeatureCount {
		return 0
	}
	z := m.Bias
	for i, w := range m.Weights {
		z += w * (features[i] - m.Mean[i]) / m.Std[i]
	}
	return 1 / (1 + math.Exp(-z))
}

// SeamCandidate is one collinear segment along a merged coplanar face: the
// Route A classification target with its deterministic features, ground
// truth (Seam), and model score (Prob).
type SeamCandidate struct {
	Cell     int
	Edge     [2]mapfile.Vec3
	Features []float64
	Seam     bool
	Prob     float64
}

// OriginalBrushPlanes returns per-brush outward plane sets for the
// worldspawn brushes of an original map (training ground truth; at
// inference the BRUSHLIST lump supplies them via BrushListFromBSP).
func OriginalBrushPlanes(m *mapfile.Map) [][]mapfile.Plane {
	if len(m.Entities) == 0 {
		return nil
	}
	return brushPlaneSets(m.Entities[0].Brushes)
}

// SeamCandidates derives the Route A sample set for a decompiled map's
// world cells: SeamTruth candidates labelled from the original brush
// planes, each with its 7 deterministic features (identical math in train
// and inference). Candidates come only from the sealed-oracle corpus path;
// single-face sides contribute outline negatives like training.
func SeamCandidates(cellsGeom [][]FaceGeom, origPlanes [][]mapfile.Plane) []SeamCandidate {
	cells := make([]*Brush, len(cellsGeom))
	for ci, fgs := range cellsGeom {
		b := &Brush{}
		for _, fg := range fgs {
			b.Sides = append(b.Sides, &Side{Plane: fg.Plane})
		}
		rebuildWindings(b)
		cells[ci] = b
	}
	seams := SeamTruth(&decompiler{}, cells, origPlanes)

	seamPtsBySide := map[int][]mapfile.Vec3{}
	for _, s := range seams {
		seamPtsBySide[s.SideBrush] = append(seamPtsBySide[s.SideBrush], s.Edge[0], s.Edge[1])
	}

	var out []SeamCandidate
	for ci, fgs := range cellsGeom {
		seamPts := seamPtsBySide[ci]
		seamEdges := len(seamPts) / 2
		matched := map[int]bool{}
		for _, fg := range fgs {
			cent := fg.Centroid()
			sq := math.Max(fg.Area(), 1)
			for e := range fg.Ring {
				end := [2]mapfile.Vec3{fg.Ring[e], fg.Ring[(e+1)%len(fg.Ring)]}
				feats := seamFeatures(fg.Plane, cent, sq, seamEdges, seamPts, end)
				if si := seamMatch(seamPts, end); si >= 0 {
					matched[si] = true
					out = append(out, SeamCandidate{Cell: ci, Edge: end, Features: feats, Seam: true})
					continue
				}
				if seamOnLine(seamPts, end[0], end[1]) {
					continue
				}
				out = append(out, SeamCandidate{Cell: ci, Edge: end, Features: feats, Seam: false})
			}
		}
		for si := 0; si+1 < len(seamPts); si += 2 {
			if matched[si/2] {
				continue
			}
			end := [2]mapfile.Vec3{seamPts[si], seamPts[si+1]}
			plane, cent, sq := seamFaceContextGeom(fgs, end)
			out = append(out, SeamCandidate{Cell: ci, Edge: end, Features: seamFeatures(plane, cent, sq, seamEdges, seamPts, end), Seam: true})
		}
	}
	return out
}

// seamFeatures builds the route-a-features-v1 vector for one segment.
func seamFeatures(plane mapfile.Plane, cent mapfile.Vec3, sqArea float64, seamEdges int, seamPts []mapfile.Vec3, end [2]mapfile.Vec3) []float64 {
	length := segLen(end[0], end[1])
	mid := segMid(end[0], end[1])
	return []float64{
		length / 8,
		math.Abs(plane.Normal.X),
		math.Abs(plane.Normal.Y),
		math.Abs(plane.Normal.Z),
		pointDist(mid, cent) / math.Sqrt(sqArea),
		math.Log1p(float64(seamEdges)),
		float64(seamSharedEndpoints(seamPts, end)),
	}
}

func seamMatch(seamPts []mapfile.Vec3, end [2]mapfile.Vec3) int {
	for i := 0; i+1 < len(seamPts); i += 2 {
		if seamSegsClose(seamPts[i], seamPts[i+1], end[0], end[1]) {
			return i / 2
		}
	}
	return -1
}

func seamOnLine(seamPts []mapfile.Vec3, p0, p1 mapfile.Vec3) bool {
	for i := 0; i+1 < len(seamPts); i += 2 {
		if segDist(segMid(p0, p1), seamPts[i], seamPts[i+1]) < 0.5 && seamCollinear(p0, p1, seamPts[i], seamPts[i+1]) {
			return true
		}
	}
	return false
}

func seamSharedEndpoints(seamPts []mapfile.Vec3, end [2]mapfile.Vec3) int {
	n := 0
	for i := 0; i+1 < len(seamPts); i += 2 {
		if seamSegsClose(end[0], end[1], seamPts[i], seamPts[i+1]) {
			continue
		}
		if seamNear(seamPts[i], end[0]) || seamNear(seamPts[i], end[1]) ||
			seamNear(seamPts[i+1], end[0]) || seamNear(seamPts[i+1], end[1]) {
			n++
		}
	}
	return n
}

func seamFaceContextGeom(fgs []FaceGeom, end [2]mapfile.Vec3) (mapfile.Plane, mapfile.Vec3, float64) {
	mid := segMid(end[0], end[1])
	bestPlane := mapfile.Plane{}
	bestCent := mapfile.Vec3{}
	bestSq := 1.0
	bestD := math.Inf(1)
	for _, fg := range fgs {
		d := math.Abs(mid.X*fg.Plane.Normal.X + mid.Y*fg.Plane.Normal.Y + mid.Z*fg.Plane.Normal.Z - fg.Plane.Dist)
		if d < bestD {
			bestD = d
			bestPlane = fg.Plane
			bestCent = fg.Centroid()
			bestSq = math.Max(fg.Area(), 1)
		}
	}
	return bestPlane, bestCent, bestSq
}

func seamSegsClose(a0, a1, b0, b1 mapfile.Vec3) bool {
	const tol = 0.5
	return seamNear(a0, b0) && seamNear(a1, b1) || seamNear(a0, b1) && seamNear(a1, b0)
}

func seamNear(a, b mapfile.Vec3) bool {
	return math.Abs(a.X-b.X) < 0.5 && math.Abs(a.Y-b.Y) < 0.5 && math.Abs(a.Z-b.Z) < 0.5
}

func seamCollinear(a0, a1, b0, b1 mapfile.Vec3) bool {
	d := v3Cross(seamSub(a1, a0), seamSub(b1, b0))
	la := segLen(a0, a1)
	lb := segLen(b0, b1)
	if la < 1e-9 || lb < 1e-9 {
		return false
	}
	return v3Len(d)/(la*lb) < 1e-2
}

func segMid(a, b mapfile.Vec3) mapfile.Vec3 {
	return mapfile.Vec3{X: (a.X + b.X) / 2, Y: (a.Y + b.Y) / 2, Z: (a.Z + b.Z) / 2}
}

func segLen(a, b mapfile.Vec3) float64 {
	d := seamSub(b, a)
	return math.Sqrt(d.X*d.X + d.Y*d.Y + d.Z*d.Z)
}

func pointDist(a, b mapfile.Vec3) float64 {
	return segLen(a, b)
}

func segDist(p, a, b mapfile.Vec3) float64 {
	ab := seamSub(b, a)
	den := ab.X*ab.X + ab.Y*ab.Y + ab.Z*ab.Z
	t := 0.0
	if den > 1e-12 {
		t = ((p.X-a.X)*ab.X + (p.Y-a.Y)*ab.Y + (p.Z-a.Z)*ab.Z) / den
	}
	t = math.Max(0, math.Min(1, t))
	return segLen(p, mapfile.Vec3{X: a.X + t*ab.X, Y: a.Y + t*ab.Y, Z: a.Z + t*ab.Z})
}

func seamSub(a, b mapfile.Vec3) mapfile.Vec3 {
	return mapfile.Vec3{X: a.X - b.X, Y: a.Y - b.Y, Z: a.Z - b.Z}
}
