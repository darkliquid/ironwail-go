// Package eval measures decompiler fidelity: BSP lump statistics and
// voxelized occupancy IoU between an original BSP and a recompiled map.
// It also owns the corpus manifest format (spec section 9.1).
package eval

import (
	"bytes"
	"errors"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// LumpStats counts the raw lump records of a BSP (tier-1 coarse diff).
type LumpStats struct {
	Planes    int
	Nodes     int
	Leafs     int
	Faces     int
	Edges     int
	Clipnodes int
}

// BSPStats loads data and returns lump record counts.
func BSPStats(data []byte) (LumpStats, error) {
	f, err := bsp.Load(bytes.NewReader(data))
	if err != nil {
		return LumpStats{}, err
	}
	return LumpStats{
		Planes:    len(f.Planes),
		Nodes:     nodeCount(f),
		Leafs:     leafCount(f),
		Faces:     faceCount(f),
		Edges:     edgeCount(f),
		Clipnodes: clipnodeCount(f),
	}, nil
}

func nodeCount(f *bsp.File) int {
	switch n := f.Nodes.(type) {
	case []bsp.DSNode:
		return len(n)
	case []bsp.DL1Node:
		return len(n)
	case []bsp.DL2Node:
		return len(n)
	}
	return 0
}

func leafCount(f *bsp.File) int {
	switch n := f.Leafs.(type) {
	case []bsp.DSLeaf:
		return len(n)
	case []bsp.DL1Leaf:
		return len(n)
	case []bsp.DL2Leaf:
		return len(n)
	}
	return 0
}

func faceCount(f *bsp.File) int {
	switch n := f.Faces.(type) {
	case []bsp.DSFace:
		return len(n)
	case []bsp.DLFace:
		return len(n)
	}
	return 0
}

func edgeCount(f *bsp.File) int {
	switch n := f.Edges.(type) {
	case []bsp.DSEdge:
		return len(n)
	case []bsp.DLEdge:
		return len(n)
	}
	return 0
}

func clipnodeCount(f *bsp.File) int {
	switch n := f.Clipnodes.(type) {
	case []bsp.DSClipNode:
		return len(n)
	case []bsp.DLClipNode:
		return len(n)
	}
	return 0
}

// PointInSolid classifies a point against the render tree: solid iff the
// reached leaf is non-empty.
//
// Where in C: SV_PointContents-style BSP descent in sv_main.c.
func PointInSolid(tree *bsp.Tree, p mapfile.Vec3) bool {
	idx := 0
	for {
		if idx < 0 || idx >= len(tree.Nodes) {
			return true // fell off the tree: treat as solid (Quake convention)
		}
		node := &tree.Nodes[idx]
		pl := tree.Planes[node.PlaneNum]
		d := float64(p.X)*float64(pl.Normal.X) + float64(p.Y)*float64(pl.Normal.Y) + float64(p.Z)*float64(pl.Normal.Z) - float64(pl.Dist)
		child := &node.Children[0]
		if d < 0 {
			child = &node.Children[1]
		}
		if child.IsLeaf {
			return tree.Leafs[child.Index].Contents != bsp.ContentsEmpty
		}
		idx = child.Index
	}
}

// OccupancyMap rasterizes the solid region of a BSP model 0: lattice cell
// centers inside solid leaves, over the clamped region.
func OccupancyMap(data []byte, cell float64, cmins, cmaxs mapfile.Vec3) (map[[3]int]struct{}, error) {
	tree, err := bsp.LoadTree(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if len(tree.Models) == 0 {
		return nil, errNoModels
	}
	m := tree.Models[0]
	x0 := math.Floor(math.Max(float64(m.BoundsMin.X), cmins.X)/cell) * cell
	y0 := math.Floor(math.Max(float64(m.BoundsMin.Y), cmins.Y)/cell) * cell
	z0 := math.Floor(math.Max(float64(m.BoundsMin.Z), cmins.Z)/cell) * cell
	x1 := math.Min(float64(m.BoundsMax.X), cmaxs.X)
	y1 := math.Min(float64(m.BoundsMax.Y), cmaxs.Y)
	z1 := math.Min(float64(m.BoundsMax.Z), cmaxs.Z)
	occ := map[[3]int]struct{}{}
	for x := x0; x < x1; x += cell {
		for y := y0; y < y1; y += cell {
			for z := z0; z < z1; z += cell {
				if PointInSolid(tree, mapfile.Vec3{X: x + cell/2, Y: y + cell/2, Z: z + cell/2}) {
					occ[[3]int{int(x / cell), int(y / cell), int(z / cell)}] = struct{}{}
				}
			}
		}
	}
	return occ, nil
}

// VoxelIoU returns |A∩B| / |A∪B|.
func VoxelIoU(a, b map[[3]int]struct{}) float64 {
	inter := 0
	for k := range a {
		if _, ok := b[k]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

var errNoModels = errors.New("no models in BSP")