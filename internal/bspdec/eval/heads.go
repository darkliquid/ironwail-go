package eval

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// HeadroomRow compares the treewalk baseline against the BRUSHLIST-direct
// ceiling for one paired map (spec section 13 M1 gate): voxel IoU under each
// route, decompiled world brush counts, the oracle brush count, and the
// fidelity gain the direct path buys.
type HeadroomRow struct {
	PkgID           string
	MapID           string
	BaselineIoU     float64
	CeilingIoU      float64
	BaselineBrushes int
	CeilingBrushes  int
	OracleBrushes   int
	Gain            float64
	Error           string `json:"error,omitempty"`
}

// ComposeHeadroomReport fills per-row gain (ceiling minus baseline IoU) for
// error-free rows and returns the rows plus their mean gain. brushlistRun
// records which route the caller labelled as the ceiling (report metadata).
func ComposeHeadroomReport(rows []HeadroomRow, brushlistRun bool) ([]HeadroomRow, float64) {
	var gains []float64
	for i := range rows {
		if rows[i].Error == "" {
			rows[i].Gain = rows[i].CeilingIoU - rows[i].BaselineIoU
			gains = append(gains, rows[i].Gain)
		}
	}
	mean := 0.0
	if len(gains) > 0 {
		for _, g := range gains {
			mean += g
		}
		mean /= float64(len(gains))
	}
	return rows, mean
}

// GoVerdict applies the M1 gate: ML routes A/B proceed only when the
// BRUSHLIST ceiling lifts fidelity meaningfully above the treewalk baseline
// (spec section 13 "After M1").
func GoVerdict(gain, threshold float64) string {
	if gain >= threshold {
		return "go"
	}
	return "no-go"
}

// EvaluateHeadroom scores one paired map under a chosen decompile route: the
// treewalk baseline (useBrushlist=false) or the BRUSHLIST direct ceiling
// (useBrushlist=true). It mirrors EvaluatePair's metric pipeline (decompile
// with MergeConvex+GridSnap, self-check, recompile, voxel IoU over model 0)
// and additionally returns the decompiled model-0 brush count.
func EvaluateHeadroom(pairDir, pkgID, mapID string, useBrushlist bool) (float64, int, error) {
	bspPath := filepath.Join(pairDir, mapID+".bsp")
	origData, err := os.ReadFile(bspPath)
	if err != nil {
		return 0, 0, err
	}
	outMap, stats, err := bspdec.Decompile(origData, bspdec.Options{
		MergeConvex:     true,
		GridSnap:        8,
		TextureFallback: "nearest",
		NoBrushlist:     !useBrushlist,
	})
	if err != nil {
		return 0, 0, err
	}
	worldBrushes := 0
	for _, s := range stats {
		if s.Model == 0 {
			worldBrushes = s.Brushes
		}
	}
	var buf bytes.Buffer
	if err := mapfile.Write(&buf, outMap, mapfile.WriteOptions{GridSnap: 8}); err != nil {
		return 0, 0, err
	}
	if err := SelfCheckParse(&buf); err != nil {
		return 0, 0, err
	}
	scratch, err := os.MkdirTemp("", "bspdec-headroom-*")
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	mapFile := filepath.Join(scratch, mapID+".map")
	if err := os.WriteFile(mapFile, buf.Bytes(), 0o644); err != nil {
		return 0, 0, err
	}
	recomp, err := CompileMapPair(mapFile, scratch)
	if err != nil {
		return 0, 0, err
	}
	reData, err := os.ReadFile(recomp.BSPPath)
	if err != nil {
		return 0, 0, err
	}
	oTree, err := bsp.LoadTree(bytes.NewReader(origData))
	if err != nil {
		return 0, 0, err
	}
	m0 := oTree.Models[0]
	omin := mapfile.Vec3{X: float64(m0.BoundsMin.X), Y: float64(m0.BoundsMin.Y), Z: float64(m0.BoundsMin.Z)}
	omax := mapfile.Vec3{X: float64(m0.BoundsMax.X), Y: float64(m0.BoundsMax.Y), Z: float64(m0.BoundsMax.Z)}
	occOrig, err := OccupancyMap(origData, 8, omin, omax)
	if err != nil {
		return 0, 0, err
	}
	occRe, err := OccupancyMap(reData, 8, omin, omax)
	if err != nil {
		return 0, 0, err
	}
	return VoxelIoU(occOrig, occRe), worldBrushes, nil
}