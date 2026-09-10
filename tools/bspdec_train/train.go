package main

import (
	"math"
	"sort"
)

// Model is a trained Route A logistic-regression classifier over the
// route-a-features-v1 vector (spec section 7.1, v0 backend: pure-stdlib BGD).
type Model struct {
	Weights []float64 `json:"weights"` // length nFeatures
	Bias    float64   `json:"bias"`
}

// trainParams pin the optimizer; batch gradient descent with no shuffling
// keeps every run bit-identical for a given corpus and config.
const (
	trainEpochs    = 300
	trainLearnRate = 0.05
	trainThresh    = 0.5
)

// featureStats returns per-feature mean and std over the samples (train
// split only), used to normalize before training and at inference. The
// feature count is taken from the samples.
func featureStats(samples []Sample) (mean, std []float64) {
	n := len(samples[0].Features)
	mean = make([]float64, n)
	std = make([]float64, n)
	count := float64(len(samples))
	for _, s := range samples {
		for i, v := range s.Features {
			mean[i] += v
		}
	}
	for i := range mean {
		mean[i] /= count
	}
	for _, s := range samples {
		for i, v := range s.Features {
			d := v - mean[i]
			std[i] += d * d
		}
	}
	for i := range std {
		std[i] = math.Sqrt(std[i] / count)
		if std[i] < 1e-9 {
			std[i] = 1
		}
	}
	return mean, std
}

func normalizeFeatures(f []float64, mean, std []float64) []float64 {
	x := make([]float64, len(f))
	for i := range f {
		x[i] = (f[i] - mean[i]) / std[i]
	}
	return x
}

// trainLogistic fits weights with batch gradient descent (deterministic:
// zero init, fixed epochs and rate, no shuffling).
func trainLogistic(samples []Sample, mean, std []float64) *Model {
	nFeatures := len(samples[0].Features)
	if nFeatures == 0 {
		return &Model{Weights: []float64{}}
	}
	m := &Model{Weights: make([]float64, nFeatures)}
	nsamples := float64(len(samples))
	for epoch := 0; epoch < trainEpochs; epoch++ {
		g := make([]float64, nFeatures)
		gb := 0.0
		for _, s := range samples {
			x := normalizeFeatures(s.Features, mean, std)
			p := m.predict(x)
			e := p - b2f(s.Seam)
			for i := range g {
				g[i] += e * x[i]
			}
			gb += e
		}
		for i := range m.Weights {
			m.Weights[i] -= trainLearnRate * g[i] / nsamples
		}
		m.Bias -= trainLearnRate * gb / nsamples
	}
	return m
}

func (m *Model) predict(x []float64) float64 {
	z := m.Bias
	for i, w := range m.Weights {
		z += w * x[i]
	}
	return 1 / (1 + math.Exp(-z))
}

// evaluate returns precision, recall, and F1 on the samples at the pinned
// decision threshold, with the model's own normalization.
func (m *Model) evaluate(samples []Sample, mean, std []float64) (precision, recall, f float64) {
	return f1At(m, samples, mean, std, trainThresh)
}

// auc is the rank-based ROC AUC the regression guard records and checks:
// it is threshold-free and a degenerate constant predictor scores 0.5 even
// on heavily positive-skewed sample sets.
func (m *Model) auc(samples []Sample, mean, std []float64) float64 {
	type scored struct {
		pr   float64
		seam bool
	}
	sc := make([]scored, 0, len(samples))
	for _, s := range samples {
		sc = append(sc, scored{pr: m.predict(normalizeFeatures(s.Features, mean, std)), seam: s.Seam})
	}
	sort.Slice(sc, func(i, j int) bool { return sc[i].pr < sc[j].pr })
	pos, neg := 0, 0
	for _, s := range sc {
		if s.seam {
			pos++
		} else {
			neg++
		}
	}
	if pos == 0 || neg == 0 {
		return 0.5
	}
	// Mann-Whitney U with average ranks for tied scores: a constant
	// predictor (corrupted weights) lands exactly at 0.5.
	rankSum := 0.0
	for i := 0; i < len(sc); {
		j := i
		for j < len(sc) && sc[j].pr == sc[i].pr {
			j++
		}
		avgRank := (float64(i) + 1 + float64(j)) / 2 // 1-based mean rank
		for k := i; k < j; k++ {
			if sc[k].seam {
				rankSum += avgRank
			}
		}
		i = j
	}
	return (rankSum - float64(pos)*(float64(pos)+1)/2) / (float64(pos) * float64(neg))
}

func f1At(m *Model, samples []Sample, mean, std []float64, th float64) (precision, recall, f float64) {
	tp, fp, fn := 0, 0, 0
	for _, s := range samples {
		x := normalizeFeatures(s.Features, mean, std)
		pr := m.predict(x) >= th
		switch {
		case pr && s.Seam:
			tp++
		case pr && !s.Seam:
			fp++
		case !pr && s.Seam:
			fn++
		}
	}
	precision = float64(tp) / math.Max(float64(tp+fp), 1)
	recall = float64(tp) / math.Max(float64(tp+fn), 1)
	f = 2 * precision * recall / math.Max(precision+recall, 1e-12)
	return precision, recall, f
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
