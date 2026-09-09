package eval

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/bspdec"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// PairResult is the tier-1/2 score for one paired map (spec section 8).
type PairResult struct {
	PkgID      string    `json:"pkg_id"`
	MapID      string    `json:"map_id"`
	OrigLump   LumpStats `json:"orig_lump"`
	RecompLump LumpStats `json:"recomp_lump"`
	VoxelIoU   float64   `json:"voxel_iou"`
	BrushDelta int       `json:"brush_delta"` // recompiled-oracle minus original oracle
	Warnings   int       `json:"warnings"`
	Error      string    `json:"error,omitempty"`
}

// EvaluatePair decompiles the original bsp, recompiles the emitted map with
// the pinned qbsp, and compares lump stats + voxel occupancy.
func EvaluatePair(pairDir, pkgID, mapID string) (PairResult, error) {
	res := PairResult{PkgID: pkgID, MapID: mapID}
	bspPath := filepath.Join(pairDir, mapID+".bsp")
	origData, err := os.ReadFile(bspPath)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	res.OrigLump, err = BSPStats(origData)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	// decompile to an in-memory map
	outMap, stats, err := bspdec.Decompile(origData, bspdec.Options{MergeConvex: true, GridSnap: 8, TextureFallback: "nearest"})
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	for _, s := range stats {
		if s.Model == 0 {
			res.Warnings = s.Warnings
		}
	}
	var buf bytes.Buffer
	if err := mapfile.Write(&buf, outMap, mapfile.WriteOptions{GridSnap: 8}); err != nil {
		res.Error = err.Error()
		return res, err
	}
	if err := SelfCheckParse(&buf); err != nil {
		res.Error = "self-check: " + err.Error()
		return res, err
	}
	// recompile in a scratch dir
	scratch, err := os.MkdirTemp("", "bspdec-eval-*")
	if err != nil {
		return res, err
	}
	defer func() { _ = os.RemoveAll(scratch) }()
	mapFile := filepath.Join(scratch, mapID+".map")
	if err := os.WriteFile(mapFile, buf.Bytes(), 0o644); err != nil {
		return res, err
	}
	recomp, err := CompileMapPair(mapFile, scratch)
	if err != nil {
		res.Error = err.Error()
		return res, err
	}
	reData, err := os.ReadFile(recomp.BSPPath)
	if err != nil {
		return res, err
	}
	res.RecompLump, err = BSPStats(reData)
	if err != nil {
		return res, err
	}
	// voxel IoU over the original model's bounds (model 0)
	oTree, err := bsp.LoadTree(bytes.NewReader(origData))
	if err != nil {
		return res, err
	}
	m0 := oTree.Models[0]
	omin := mapfile.Vec3{X: float64(m0.BoundsMin.X), Y: float64(m0.BoundsMin.Y), Z: float64(m0.BoundsMin.Z)}
	omax := mapfile.Vec3{X: float64(m0.BoundsMax.X), Y: float64(m0.BoundsMax.Y), Z: float64(m0.BoundsMax.Z)}
	occOrig, err := OccupancyMap(origData, 8, omin, omax)
	if err != nil {
		return res, err
	}
	occRe, err := OccupancyMap(reData, 8, omin, omax)
	if err != nil {
		return res, err
	}
	res.VoxelIoU = VoxelIoU(occOrig, occRe)
	return res, nil
}

// EvaluateCorpus sweeps paired/<pkg_id>/ dirs for *.map / *.bsp pairs.
func EvaluateCorpus(pairedDir string) ([]PairResult, error) {
	if _, err := os.Stat(pairedDir); err != nil {
		return nil, err
	}
	pkgs, err := os.ReadDir(pairedDir)
	if err != nil {
		return nil, err
	}
	var results []PairResult
	for _, p := range pkgs {
		if !p.IsDir() {
			continue
		}
		dir := filepath.Join(pairedDir, p.Name())
		maps, err := filepath.Glob(filepath.Join(dir, "*.map"))
		if err != nil {
			return nil, err
		}
		for _, m := range maps {
			id := strings.TrimSuffix(filepath.Base(m), ".map")
			if _, err := os.Stat(filepath.Join(dir, id+".bsp")); err != nil {
				continue
			}
			r, err := EvaluatePair(dir, p.Name(), id)
			if err != nil && r.Error == "" {
				r.Error = err.Error()
			}
			results = append(results, r)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].PkgID != results[j].PkgID {
			return results[i].PkgID < results[j].PkgID
		}
		return results[i].MapID < results[j].MapID
	})
	return results, nil
}

// SelfCheckParse re-parses emitted bytes and runs bspdec's brush validation.
func SelfCheckParse(buf *bytes.Buffer) error {
	m, err := mapfile.Parse(bytes.NewReader(buf.Bytes()))
	if err != nil {
		return err
	}
	return bspdec.SelfCheck(m)
}

