package qbsp

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

const (
	planeX = types.PlaneX
	planeY = types.PlaneY
	planeZ = types.PlaneZ
)

// classifyPlane returns the axial type of n, or non-axial dominant axis.
func classifyPlane(n vec3) int32 {
	return types.ClassifyPlaneType64(n)
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
		if p.Normal.X < 0 {
			p.Normal = v3(1, 0, 0)
			p.Dist = -p.Dist
		}
	case planeY:
		if p.Normal.Y < 0 {
			p.Normal = v3(0, 1, 0)
			p.Dist = -p.Dist
		}
	case planeZ:
		if p.Normal.Z < 0 {
			p.Normal = v3(0, 0, 1)
			p.Dist = -p.Dist
		}
	}
	return p
}

// snapPlaneDist rounds an axial plane distance to an integer when it is within
// 0.01 of one (avoiding micro-splits from float rounding on integer map grids),
// while using a strict 1e-5 tolerance for non-axial planes to prevent irrational
// diagonal plane distances (e.g. diagonal 45-degree trims) from being corrupted.
func snapPlaneDist(n vec3, d float64) float64 {
	tol := 1e-5
	if isAxial(n) {
		tol = 0.01
	}
	r := math.Round(d)
	if math.Abs(d-r) < tol {
		return r
	}
	return d
}

// planeEqualNear reports planewise equality within the qbsp tolerance used
// for merging coincident planes (tighter than the duplicate-face check,
// because the compiler depends on exact dedup).
func planeEqualNear(a, b plane) bool {
	if math.Abs(a.Normal.Dot(b.Normal)) < 1-1e-4 {
		return false
	}
	// Compare distances under the same normal direction; if normals are
	// opposite, the distance sign flips.
	d := a.Dist - b.Dist
	if a.Normal.Dot(b.Normal) < 0 {
		d = a.Dist + b.Dist
	}
	return math.Abs(d) < 0.01
}

// contentsForBrush determines a brush's contents from its texture names,
// matching Quake conventions: *waterN/*slimeN/*lavaN liquids, sky, and
// skip/hint (non-solid). Returns the BSP contents value and whether the
// brush contributes geometry.
//
// QUARANTINED (bead ironwail-go-<skip-landing>): keeping brushes that mix
// real textures with skip faces (ericw Brush_GetContents style) misseals
// large DECOMPILED maps — the bspdec corpus emits skip-annotated void
// brushes whose first real face misattributes contents (water/sky on void
// brushes), opening leaks and exploding the tree on jam6_necros_v2. The
// old rule (any skip/hint face drops the brush) sealed those but opened
// e3_mh-style authored terrain patches. The mixed-brush rule needs a
// smarter contents decision before re-landing; tests are skip-gated in
// leak_disparity_test.go and skipface_test.go.
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
