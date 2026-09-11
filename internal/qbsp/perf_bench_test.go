package qbsp

import (
	"os"
	"path/filepath"
	"testing"
)

// Performance benchmark matrix for bead ironwail-go-ros. The dataset lives
// outside git (dataset/bspdec/paired in the main checkout); benchmarks skip
// when it is unavailable. Override the location with QWBSP_PERF_DATA.
//
// Usage (match mise env, TMPDIR must be a repo-local scratch dir):
//
//	TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/qbsp -bench=BenchmarkCompile -benchtime=1x -benchmem -run=^$
//
// The three canaries compile fully in seconds; the three large maps
// time out in production (5 min) pre-fix — benchmark them with
// -benchtime=1x and a generous -timeout, or profile them with
// tools/qbspprof -deadline instead.
var perfMatrix = []struct {
	name string
	rel  string // relative to dataset/bspdec/paired
}{
	{"Dam100", "quaddicted-100brush/Dam100.map"},
	{"end", "end/end.map"},
	{"e1m1", "e1m1/e1m1.map"},
	{"jjj22_dfl", "quaddicted-jjj2/jjj22_dfl.map"},
	{"sm190_nait", "quaddicted-sm190_pack/sm190_nait.map"},
	{"jam6_necros_v2", "quaddicted-mapjam6/jam6_necros_v2.map"},
}

func locatePerfDataset(b *testing.B) string {
	if env := os.Getenv("QWBSP_PERF_DATA"); env != "" {
		return env
	}
	candidates := []string{
		"../../dataset/bspdec",                   // main checkout, package dir
		"../../../../ironwail-go/dataset/bspdec", // worktree (sibling of main checkout)
		"../../../dataset/bspdec",                // worktree at repo-root depth
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "paired")); err == nil && st.IsDir() {
			return c
		}
	}
	b.Skipf("perf dataset not found (set QWBSP_PERF_DATA to dataset/bspdec)")
	return ""
}

func BenchmarkCompile(b *testing.B) {
	root := locatePerfDataset(b)
	for _, m := range perfMatrix {
		b.Run(m.name, func(b *testing.B) {
			path := filepath.Join(root, "paired", m.rel)
			f, err := os.Open(path)
			if err != nil {
				b.Skipf("map not available: %s", path)
			}
			parsed, err := ParseMap(f)
			_ = f.Close()
			if err != nil {
				b.Fatalf("parse %s: %v", path, err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Compile(parsed, Options{}); err != nil {
					b.Fatalf("compile %s: %v", path, err)
				}
			}
		})
	}
}
