package server

import (
	"math/rand"
	"sort"
	"testing"
)

// BenchmarkEntitySendSortOnManyCandidates measures the plan 27 O3 concern:
// per-client-per-frame stable sort of all entity-send candidates followed by
// an early-cutoff write loop. At qbj3 scale the candidate list is ~1-2k
// entities but only the first ~dozens actually fit the message budget; the
// sort is the per-frame cost that stays even when few entities are sent.
func BenchmarkEntitySendSortOnManyCandidates(b *testing.B) {
	const numCandidates = 1500
	rng := rand.New(rand.NewSource(7))
	type cand struct {
		key    int
		entNum int
		_      [32]byte // mimic padding so the struct is representative
	}
	cands := make([]cand, numCandidates)
	for i := range cands {
		cands[i] = cand{key: rng.Intn(numCandidates), entNum: i}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := append([]cand(nil), cands...)
		sort.SliceStable(buf, func(a, z int) bool {
			if buf[a].key != buf[z].key {
				return buf[a].key < buf[z].key
			}
			return buf[a].entNum < buf[z].entNum
		})
		// Early cutoff: only the first ~50 are written.
		written := 0
		for _, c := range buf {
			if written >= 50 {
				break
			}
			written++
			_ = c
		}
	}
}

// BenchmarkEntitySendSortSmall mirrors the actual small-client case (~200
// visedicts) to size the real per-frame cost in a normal room.
func BenchmarkEntitySendSortSmall(b *testing.B) {
	const numCandidates = 200
	rng := rand.New(rand.NewSource(7))
	type cand struct {
		key    int
		entNum int
	}
	cands := make([]cand, numCandidates)
	for i := range cands {
		cands[i] = cand{key: rng.Intn(numCandidates), entNum: i}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := append([]cand(nil), cands...)
		sort.SliceStable(buf, func(a, z int) bool {
			if buf[a].key != buf[z].key {
				return buf[a].key < buf[z].key
			}
			return buf[a].entNum < buf[z].entNum
		})
	}
}
