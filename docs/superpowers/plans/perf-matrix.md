# qbsp Perf Matrix — Baselines (bead ironwail-go-60t)

Fixed 6-map matrix for the ironwail-go-ros research. Dataset:
`dataset/bspdec/paired` (main checkout, gitignored; override with
`QWBSP_PERF_DATA`). Baselines captured 2026-09-11, go1.26.6, amd64,
6-core host, single-threaded compile.

## Commands

```
# benchmarks (canaries full; large maps with -benchtime=1x -timeout 30m)
TMPDIR=$PWD/.tmp CGO_ENABLED=0 go test ./internal/qbsp \
  -bench=BenchmarkCompile -benchtime=1x -benchmem -run='^$' -timeout 30m \
  | tee .tmp/bench/<label>.txt

# profiles (large maps: deadline dump, compile does not finish)
go build -o .tmp/qbspprof ./tools/qbspprof
./.tmp/qbspprof -cpuprofile .tmp/bench/<map>.prof -memprofile .tmp/bench/<map>.mem \
  -deadline 90s <map>.map

# compare after a change
benchstat .tmp/bench/old.txt .tmp/bench/new.txt
go tool pprof -top .tmp/qbspprof .tmp/bench/<map>.prof
go tool pprof -top -sample_index=alloc_space .tmp/qbspprof .tmp/bench/<map>.mem
```

## Baseline (pre-fix, 2026-09-11)

### Canaries (complete compile)

| map | size | wall | total-alloc | GC cycles |
|---|---|---|---|---|
| Dam100.map | 78 KB | 198 ms | 197 MiB | 80 |
| end.map | 202 KB | 3.97 s | 3539 MiB | 165 |
| e1m1.map | 674 KB | 6.88 s | 4857 MiB | 138 |

Note: even the id maps show the pathology — ericw-tools compiles e1m1 in
about a second with a fraction of the allocations.

### Large maps (production timeout: 5 min)

| map | size | wall (90s profile window) | total-alloc | GC cycles |
|---|---|---|---|---|
| jjj22_dfl.map | 13.9 MB | unfinished (>16 min foreground) | 4952 MiB/90s | 81 |
| sm190_nait.map | 9.2 MB | unfinished (90s window) | 1940 MiB/90s | 52 |
| jam6_necros_v2.map | 5.6 MB | **32.0 s (completes)** | 43617 MiB/32s (1.36 GiB/s) | 967 |

ericw-tools reference: jjj22_dfl 12.6 s (>24x gap).

### Large-map hot paths (90s windows)

jjj22_dfl and sm190_nait share the superlinear shape — split-selection
scoring dominates:

| flat | cum | function |
|---|---|---|
| 44.3% / 41.7% | 94.6% / 94.5% | classifyBrush (jjj22_dfl / sm190_nait) |
| 42.4% / 39.2% | — | types.Vec3T[float64].Dot (inlined) |
| 6.8% / 7.6% | — | planeEqualOriented |

jam6_necros_v2 completes but is churn-bound; its CPU spreads into the
float plane-comparison machinery that P1a removes:

| flat | cum | function |
|---|---|---|
| 17.8% | 22.2% | planeEqualNear |
| — | 28.4% | compiler.addPlaneIndex (linear plane-table scan) |
| 10.5% | — | planeEqualOriented |
| 4.9% | 15.8% | clipWinding |

jam6 allocation sites (43.6 GiB total): clipWinding 57.5%, splitBrush
30.2%, windingFromBoxPlane 1.7% flat (14.2% cum), tjuncGroup 2.4%,
windingRemoveColinear 1.5%.

### Canary bench baselines (.tmp/bench/old.txt, -count=5, -benchtime=1x)

| map | ns/op (median) | B/op | allocs/op |
|---|---|---|---|
| Dam100 | ~182 ms | 205.8 MB | 1.34 M |
| end | ~3.84 s | 3.71 GB | 21.2 M |
| e1m1 | ~6.81 s | 5.08 GB | 28.6 M |

Note: do NOT run jjj22_dfl/sm190_nait under `go test -bench` (multi-minute
single compiles; a background run was SIGTERMed at 267s by something
external). Use qbspprof -deadline windows for the large maps.

### CPU hot path (jjj22_dfl, 90.49s samples)

| flat | cum | function |
|---|---|---|
| 44.3% | 94.6% | classifyBrush (brush.go:180) |
| 42.4% | 42.4% | types.Vec3T[float64].Dot (inlined) |
| 6.8% | 6.8% | planeEqualOriented |
| — | 95.9% | selectSplitPlane (solidbsp.go:223) via treeBuild.build |

### Allocation sites (jjj22_dfl, 4.95 GiB in 90s)

| share | function |
|---|---|
| 45.1% | clipWinding (poly.go:29) |
| 28.0% | splitBrush (brush.go:198) |
| 17.0% | chopBrushes |
| 4.9% | windingFromBoxPlane |

## Result log

Append one row per measured change (benchstat new vs old + wall on large maps).

| label | date | change | canary wall | jam6 | jjj22_dfl | GC cycles jam6 |
|---|---|---|---|---|---|---|
| baseline | 2026-09-11 | — | 198ms / 3.97s / 6.88s | 32.0s | unfinished (>16 min) | 967 |
| AUTO+onnode | 2026-09-11 | ericw midsplit budget + onnode | 149ms / 2.31s / 1.56s | 24.7s | unfinished (churn 233GiB/90s) | 1004 |
| +arena | 2026-09-11 | compiler-wide winding arena, AABB fast-reject | 145ms / 2.29s / 1.60s | 25.1s | unfinished (RSS ~4GB, bounded) | 45 |

Notes:
- AUTO+onnode without the straddle guard OOM'd at 12.7GB (qbsp.test): a
  volume-mid cut nothing straddles recurses on an unchanged brush list
  (non-axial cuts do not shrink childBounds). The guard now rejects
  no-progress mid splits (arena-rolled-back) and falls back to precise.
- The compiler-wide arena cut total allocations 43%->24GiB on jam6 and
  GC cycles 967->45; GC pause totals 79ms -> 1ms. Large-map RSS is now
  bounded (~4GB jjj22) and -memlimit gives an operational ceiling.
- REMAINING WALL: jjj22_dfl/sm190_nait still exceed 5 min in the deep
  sub-1024 precise scoring path (50+ recursion frames at near-identical
  bounds in sliver regions). Next step is the ericw-faithful
  positive-paired plane table (planenum & ~1 identity, spatial-hash
  interning) + facing-side tested dedup in the precise loop — see beads
  ironwail-go-ysm. ericw reference on the same map: 12.44s, 14,690
  brushes, 78,042 sides.
