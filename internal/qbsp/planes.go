package qbsp

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// planeType mirrors the Quake plane classification (PlaneX/Y/Z for axial
// planes; PlaneNonAxial otherwise), used to speed up point-vs-plane tests
// and required by the BSP plane lump entries.
type planeType int8

const (
	planeX        = planeType(0)
	planeY        = planeType(1)
	planeZ        = planeType(2)
	planeNonAxial = planeType(3)
)

// classifyPlane returns the axial type of n, or non-axial.
func classifyPlane(n vec3) planeType {
	if n[0] == 1 || n[0] == -1 {
		return planeX
	}
	if n[1] == 1 || n[1] == -1 {
		return planeY
	}
	if n[2] == 1 || n[2] == -1 {
		return planeZ
	}
	return planeNonAxial
}

// normalizePlane flips axial planes to POSITIVE normals, the Quake BSP
// convention the engine's plane fast-path relies on (PointInLeaf computes
// d = p.X - dist for planeX, assuming normal (1,0,0); a negative-normal
// axial plane would misroute the descent). Flipping keeps the same
// geometric plane; callers must interpret front/back relative to the
// normalized plane.
func normalizePlane(p plane) plane {
	switch classifyPlane(p.Normal) {
	case planeX:
		if p.Normal[0] < 0 {
			p.Normal = v3(1, 0, 0)
			p.Dist = -p.Dist
		}
	case planeY:
		if p.Normal[1] < 0 {
			p.Normal = v3(0, 1, 0)
			p.Dist = -p.Dist
		}
	case planeZ:
		if p.Normal[2] < 0 {
			p.Normal = v3(0, 0, 1)
			p.Dist = -p.Dist
		}
	}
	return p
}

// snapPlaneDist rounds a plane distance to an integer when it is very close
// to one, mirroring the classic qbsp behaviour that keeps coincident
// geometry from creating micro-splits.
func snapPlaneDist(d float64) float64 {
	r := math.Round(d)
	if math.Abs(d-r) < 0.01 {
		return r
	}
	return d
}
// planeEqualNear reports planewise equality within the qbsp tolerance used
// for merging coincident planes (tighter than the duplicate-face check,
// because the compiler depends on exact dedup).
func planeEqualNear(a, b plane) bool {
	if math.Abs(v3Dot(a.Normal, b.Normal)) < 1-1e-4 {
		return false
	}
	// Compare distances under the same normal direction; if normals are
	// opposite, the distance sign flips.
	d := a.Dist - b.Dist
	if v3Dot(a.Normal, b.Normal) < 0 {
		d = a.Dist + b.Dist
	}
	return math.Abs(d) < 0.01
}

// contentsForBrush determines a brush's contents from its texture names,
// matching Quake conventions: *waterN/*slimeN/*lavaN liquids, sky, and
// skip/hint (non-solid). Returns the BSP contents value and whether the
// brush contributes geometry.
func contentsForBrush(faces []MapFace) (int32, bool) {
	if len(faces) == 0 {
		return bsp.ContentsSolid, true
	}
	for _, f := range faces {
		name := f.TexName
		switch {
		case name == "skip" || name == "hint":
			return bsp.ContentsSolid, false
		case len(name) >= 6 && name[:6] == "*water":
			return bsp.ContentsWater, true
		case len(name) >= 6 && name[:6] == "*slime":
			return bsp.ContentsSlime, true
		case len(name) >= 5 && name[:5] == "*lava":
			return bsp.ContentsLava, true
		case len(name) >= 3 && name[:3] == "sky":
			return bsp.ContentsSky, true
		}
	}
	return bsp.ContentsSolid, true
}

// contentsKindForTexture returns the classic contents-name convention for
// a texture (used by texture table classification downstream).
func contentsKindForTexture(name string) int32 {
	c, _ := contentsForBrush([]MapFace{{TexName: name}})
	return c
}
