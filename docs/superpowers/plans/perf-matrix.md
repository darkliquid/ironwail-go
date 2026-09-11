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
| jjj22_dfl.map | 13.9 MB | >16 min foreground, unfinished | 4952 MiB/90s | 81 |
| sm190_nait.map | 9.2 MB | (capture below) | — | — |
| jam6_necros_v2.map | 5.6 MB | (capture below) | — | — |

ericw-tools reference: jjj22_dfl 12.6 s (>24x gap).

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

| label | date | change | canary wall | jjj22_dfl | allocs |
|---|---|---|---|---|---|
| baseline | 2026-09-11 | — | 198ms / 3.97s / 6.88s | unfinished | see above |
