package main

import (
	"math"
	"os"

	"github.com/darkliquid/ironwail-go/internal/bspdec"
)

// groupSchema is Route B's feature vector schema (route-b-features-v1).
const groupSchema = "route-b-features-v1"

// groupFeatures counts the Route B feature vector length.
const groupFeatures = 6

// groupPair is one cell-pair training target: should two decompiled world
// cells join back into one brush? (spec section 7.2, bead xxy.7). Truth
// comes from the label record's original-brush assignment.
type groupPair struct {
	CellA, CellB int
	Features     []float64
	Merge        bool
}

// groupCandidate is the per-map extraction result: candidate pairs and the
// deterministic baseline / true edge counts over those candidates.
type groupCandidate struct {
	Pairs         []groupPair
	BaselineEdges int
	TruthEdges    int
}

// groupSamplesForMap derives Route B candidates for a map: every
// coplanar-adjacent pair of the unmerged cells, labelled by whether the two
// cells descend from the same original brush. Features (deterministic):
//
//	0 contact area / min(cell surface area)  1 either cell is "multi"
//	2 both cells "multi"                     3 |log surface-area ratio|
//	4 the --merge-convex baseline merged them 5 |face-count ratio|
//
// The baseline edge set (merge-convex grouping) is read from the merged
// decompile: cells whose centroids land in the same merged brush.
func groupSamplesForMap(cells [][]bspdec.FaceGeom, merged [][]bspdec.FaceGeom, labels []bspdec.CellLabel) *groupCandidate {
	// CellLabel.Cell indexes the FINAL (merged) cells; carry its truth down
	// to each unmerged cell through the merged brush containing its
	// centroid.
	finOrig := make([]int, len(merged))
	finConf := make([]bool, len(merged))
	for i := range finOrig {
		finOrig[i] = -1
	}
	for _, l := range labels {
		if l.Cell < 0 || l.Cell >= len(merged) {
			continue
		}
		finOrig[l.Cell] = l.OriginalBrush
		finConf[l.Cell] = l.Confidence == "multi"
	}
	orig := make([]int, len(cells))
	conf := make([]bool, len(cells))
	member := make([]int, len(cells))
	for ci := range cells {
		member[ci] = -1
		cent := cellCentroidOf(cells[ci])
		for mi := range merged {
			if pointInCell(cent, merged[mi]) {
				member[ci] = mi
				orig[ci] = finOrig[mi]
				conf[ci] = finConf[mi]
				break
			}
		}
	}
	adj, areas, _ := bspdec.CellAdjacency(cells)
	out := &groupCandidate{}
	areaIdx := 0
	for k := range adj {
		i, j := adj[k][0], adj[k][1]
		surfI := cellSurfaceArea(cells[i])
		surfJ := cellSurfaceArea(cells[j])
		contact := areas[areaIdx]
		areaIdx++
		baseline := member[i] >= 0 && member[i] == member[j]
		merge := orig[i] >= 0 && orig[i] == orig[j]
		f := []float64{
			contact / math.Max(math.Min(surfI, surfJ), 1),
			b2f64(conf[i] || conf[j]),
			b2f64(conf[i] && conf[j]),
			math.Log(math.Max(surfI, surfJ) / math.Max(math.Min(surfI, surfJ), 1)),
			b2f64(baseline),
			math.Abs(float64(len(cells[i]))-float64(len(cells[j]))) / math.Max(float64(len(cells[i])), 1),
		}
		out.Pairs = append(out.Pairs, groupPair{CellA: i, CellB: j, Features: f, Merge: merge})
		if baseline {
			out.BaselineEdges++
		}
		if merge {
			out.TruthEdges++
		}
	}
	return out
}

// cellCentroidOf averages all face-ring points of a cell.
func cellCentroidOf(fgs []bspdec.FaceGeom) [3]float64 {
	var acc [3]float64
	n := 0
	for _, fg := range fgs {
		for _, p := range fg.Ring {
			acc[0] += p.X
			acc[1] += p.Y
			acc[2] += p.Z
			n++
		}
	}
	if n == 0 {
		return acc
	}
	return [3]float64{acc[0] / float64(n), acc[1] / float64(n), acc[2] / float64(n)}
}

// pointInCell reports whether the point is inside the convex cell (behind
// every outward face plane).
func pointInCell(p [3]float64, fgs []bspdec.FaceGeom) bool {
	for _, fg := range fgs {
		if p[0]*fg.Plane.Normal.X+p[1]*fg.Plane.Normal.Y+p[2]*fg.Plane.Normal.Z-fg.Plane.Dist > 0.01 {
			return false
		}
	}
	return true
}

// cellSurfaceArea sums the face-ring polygon areas of a cell.
func cellSurfaceArea(fgs []bspdec.FaceGeom) float64 {
	total := 0.0
	for _, fg := range fgs {
		total += fg.Area()
	}
	return total
}

func b2f64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// edgeF1 scores an edge classifier (pair predictions) against the true edge
// set with precision/recall/F1.
func edgeF1(pairs []groupPair, predict func(p groupPair) bool) (precision, recall, f float64) {
	tp, fp, fn := 0, 0, 0
	for _, p := range pairs {
		pr := predict(p)
		switch {
		case pr && p.Merge:
			tp++
		case pr && !p.Merge:
			fp++
		case !pr && p.Merge:
			fn++
		}
	}
	precision = float64(tp) / maxi(tp+fp)
	recall = float64(tp) / maxi(tp+fn)
	f = 2 * precision * recall / maxf(precision+recall)
	return precision, recall, f
}

// groupSamplesForRecord decompiles a map twice (unmerged cells for the
// candidates, merged map for the --merge-convex baseline) and extracts its
// Route B pair candidates.
func groupSamplesForRecord(rec *mapRecord) (*groupCandidate, error) {
	data, err := os.ReadFile(rec.BSPPath)
	if err != nil {
		return nil, err
	}
	outNoMerge, _, err := bspdec.Decompile(data, bspdec.Options{MergeConvex: false, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		return nil, err
	}
	cells, err := bspdec.FacePolygons(outNoMerge)
	if err != nil {
		return nil, err
	}
	outMerged, _, err := bspdec.Decompile(data, bspdec.Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		return nil, err
	}
	merged, err := bspdec.FacePolygons(outMerged)
	if err != nil {
		return nil, err
	}
	return groupSamplesForMap(cells, merged, rec.Labels.Cells), nil
}

// routeBStats carries the Route B gate inputs: held-out vs-baseline edge
// F1 results for the ML grouping model.
type routeBStats struct {
	MLF1         float64
	BaselineF1   float64
	MLR          float64
	BaselineR    float64
	TruthEdges   int
	Inconclusive bool
}

// groupGateVerdict reports whether the learned grouping beats the
// deterministic --merge-convex baseline on held-out data (bead xxy.7).
func groupGateVerdict(mlF1, baselineF1 float64) bool { return mlF1 > baselineF1 }

// routeBGate folds the inconclusive case (no residual truth signal on the
// held-out slice) into a conservative failure: no demonstrated gain.
func routeBGate(s *routeBStats) bool {
	if s.Inconclusive {
		return false
	}
	return groupGateVerdict(s.MLF1, s.BaselineF1)
}

func groupGateNote(passed bool) string {
	if passed {
		return "pass: learned grouping F1 beats the --merge-convex baseline on held-out data"
	}
	return "fail/inconclusive: no demonstrated grouping gain over --merge-convex on the held-out slice (residual same-brush pairs ~= 0; the deterministic merger already recovers synthetic-slice grouping). Per spec 13: Route C descoped, Route D primary"
}

// predictivePairs returns a copy of pairs with each Merge flag replaced by
// the classifier's prediction at the given threshold, so edgeF1 can score
// any predictor uniformly.
