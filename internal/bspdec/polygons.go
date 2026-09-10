package bspdec

import (
	"fmt"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

// FaceGeom is one worldspawn cell face: its outward plane and full polygon
// ring (the same winding the decompiler rebuilds internally), so M2 feature
// extraction can enumerate collinear-vertex-runs along merged coplanar faces
// without re-deriving the BSP.
type FaceGeom struct {
	Plane mapfile.Plane
	Ring  []mapfile.Vec3
}

// FacePolygons reconstructs every worldspawn face polygon of a decompiled
// map from the emitted face planes, in cell order (CellLabel.Cell /
// SeamLabel.SideBrush indices line up). Sides whose clip degenerates are
// omitted per face, mirroring the emission path.
func FacePolygons(m *mapfile.Map) ([][]FaceGeom, error) {
	if len(m.Entities) == 0 {
		return nil, fmt.Errorf("bspdec: no worldspawn entity")
	}
	cells := m.Entities[0].Brushes
	out := make([][]FaceGeom, len(cells))
	for ci := range cells {
		b := &Brush{}
		for _, f := range cells[ci].Faces {
			p, length := mapfile.PlaneFromPoints(f.Points[0], f.Points[1], f.Points[2])
			if length < 0.01 {
				return nil, fmt.Errorf("bspdec: cell %d face degenerate", ci)
			}
			b.Sides = append(b.Sides, &Side{Plane: p})
		}
		if len(b.Sides) < 4 {
			return nil, fmt.Errorf("bspdec: cell %d has %d faces", ci, len(b.Sides))
		}
		rebuildWindings(b)
		for _, s := range b.Sides {
			if s.Winding == nil {
				continue
			}
			out[ci] = append(out[ci], FaceGeom{Plane: s.Plane, Ring: s.Winding.Points})
		}
	}
	return out, nil
}

// Centroid returns the average of the face ring points.
func (fg FaceGeom) Centroid() mapfile.Vec3 {
	w := &Winding{Points: fg.Ring}
	return w.Centroid()
}

// Area returns the face ring polygon area.
func (fg FaceGeom) Area() float64 {
	w := &Winding{Points: fg.Ring}
	return w.Area()
}

// CellAdjacency lists the coplanar-adjacent cell pairs of a decompiled
// world (unmerged cells): two cells are adjacent when opposite-direction
// coplanar faces overlap, and the returned area is that contact region
// (Route B grouping task, spec section 7.2).
func CellAdjacency(cellsGeom [][]FaceGeom) (pairs [][2]int, areas []float64, err error) {
	cells := make([]*Brush, len(cellsGeom))
	for ci, fgs := range cellsGeom {
		b := &Brush{}
		for _, fg := range fgs {
			b.Sides = append(b.Sides, &Side{Plane: fg.Plane})
		}
		rebuildWindings(b)
		cells[ci] = b
	}
	for i := range cells {
		for j := i + 1; j < len(cells); j++ {
			for _, fi := range cells[i].Sides {
				if fi.Winding == nil {
					continue
				}
				for _, fj := range cells[j].Sides {
					if fj.Winding == nil {
						continue
					}
					if !planesOpposite(fi.Plane, fj.Plane) {
						continue
					}
					w := clipToBrush(fj.Winding, cells[i], fi.Plane)
					if w == nil || len(w.Points) < 3 {
						continue
					}
					a := w.Area()
					if a < minSliverArea {
						continue
					}
					pairs = append(pairs, [2]int{i, j})
					areas = append(areas, a)
					break
				}
			}
		}
	}
	return pairs, areas, nil
}
