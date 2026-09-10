package main

// heuristicModel is the deterministic split heuristic Route A must beat on
// the held-out corpus (spec section 7.1 gate, beads xxy.6 acceptance): a
// hand-crafted rule over the same route-a-features-v1 vector, with no
// learning and no tuning on the data.
//
// Rule: a segment is a seam when it sits interior to its face (distance to
// the face centroid under half the area-scaler) AND the face already yields
// at least one recorded seam. Outline runs of large faces and isolated
// single-seam faces are classified interior.
type heuristicModel struct{}

func (heuristicModel) name() string { return "deterministic split heuristic" }

// predict applies the rule to a route-a-features-v1 vector.
func (heuristicModel) predict(f []float64) bool {
	// f[4] = dist(seg, face centroid) / sqrt(area); outline midpoints of a
	// square face sit at 0.5, interior joins at 0.
	// f[5] = log1p(number of recorded seams on the face).
	return f[4] <= 0.5 && f[5] > 0
}

// f1 evaluates the heuristic against the samples (per-sample, no
// normalization needed: the features are already in their raw scale).
func (heuristicModel) f1(samples []Sample) (precision, recall, f float64) {
	tp, fp, fn := 0, 0, 0
	for _, s := range samples {
		pr := heuristicModel{}.predict(s.Features)
		switch {
		case pr && s.Seam:
			tp++
		case pr && !s.Seam:
			fp++
		case !pr && s.Seam:
			fn++
		}
	}
	precision = float64(tp) / maxi(tp+fp)
	recall = float64(tp) / maxi(tp+fn)
	f = 2 * precision * recall / maxf(precision+recall)
	return precision, recall, f
}

// bestF1 returns the ML model's best F1 over decision thresholds (the
// threshold-free comparison point against the heuristic).
func modelBestF1(m *Model, samples []Sample, mean, std []float64) float64 {
	best := 0.0
	for th := 0.05; th <= 0.95; th += 0.05 {
		_, _, f := f1At(m, samples, mean, std, th)
		if f > best {
			best = f
		}
	}
	return best
}

func maxi(v int) float64 {
	if v < 1 {
		return 1
	}
	return float64(v)
}

func maxf(v float64) float64 {
	if v < 1e-12 {
		return 1e-12
	}
	return v
}
