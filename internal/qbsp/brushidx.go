package qbsp

import "math"

// The solidbsp working set as flat, pointer-free slabs: brush pieces and
// their side records are indexed by int32 IDs into append-only arrays, and
// side windings are arena references (wref). Nothing in the tree-build hot
// path is a heap object with pointers, so the GC marker never scans it —
// mark/scan/sweep over millions of per-split objects was half the compile
// time on large maps (bead ironwail-go-atf).
//
// Lifetime discipline mirrors the winding arena: the brushArena is the
// compiler's (slabs grow monotonically through chopBrushes), and each
// tree-build frame checkpoints on entry and rewinds on exit, so split
// pieces are reused without GC involvement.

// plane-side classification bits (classic qbsp PSIDE_*).
const (
	psideFront  = 1 << 0
	psideBack   = 1 << 1
	psideFacing = 1 << 2
)

// splitEpsilon mirrors ericw's PLANESIDE_EPSILON (brushbsp.c): vertices
// within this distance of the split plane count as on-plane, so razor-thin
// trims and boundary-coincident faces never trigger phantom straddles.
const splitEpsilon = 0.1

// brushRef indexes a brush record in the brushArena; -1 is "no brush".
type brushRef = int32

// sideRec is one side of a working brush: all value types.
type sideRec struct {
	planenum int32
	onnode   bool
	n        vec3
	d        float64
	w        wref
}

// brushRec is a working brush: a contiguous run of sideRecs plus contents.
type brushRec struct {
	sidesStart int32
	sidesCount int32
	content    int32
	sortKey    int64
	bounds     [2]vec3
}

// brushArena owns the slab arrays and the winding arena they reference.
type brushArena struct {
	sides   []sideRec
	brushes []brushRec
	w       *windingArena
}

func newBrushArena(w *windingArena) *brushArena { return &brushArena{w: w} }

// brushMark checkpoints both slabs and the winding arena.
type brushMark struct {
	sides   int
	brushes int
	w       int
}

func (ba *brushArena) mark() brushMark {
	return brushMark{sides: len(ba.sides), brushes: len(ba.brushes), w: ba.w.mark()}
}

func (ba *brushArena) release(m brushMark) {
	ba.sides = ba.sides[:m.sides]
	ba.brushes = ba.brushes[:m.brushes]
	ba.w.release(m.w)
}

// addBrush appends a piece with a contiguous copy of sides and computes its
// bounds from the side planes' windings.
func (ba *brushArena) addBrush(sides []sideRec, content int32, sortKey int64) brushRef {
	id := brushRef(len(ba.brushes))
	ba.brushes = append(ba.brushes, brushRec{
		sidesStart: int32(len(ba.sides)),
		sidesCount: int32(len(sides)),
		content:    content,
		sortKey:    sortKey,
	})
	ba.sides = append(ba.sides, sides...)
	ba.computeBounds(id)
	return id
}

// computeBounds recomputes a brush's AABB from its side windings.
func (ba *brushArena) computeBounds(id brushRef) {
	b := &ba.brushes[id]
	mins, maxs := vec3{X: math.Inf(1), Y: math.Inf(1), Z: math.Inf(1)},
		vec3{X: math.Inf(-1), Y: math.Inf(-1), Z: math.Inf(-1)}
	for _, s := range ba.sidesOf(id) {
		if s.w.count == 0 {
			continue
		}
		for _, v := range ba.w.at(s.w) {
			if v.X < mins.X {
				mins.X = v.X
			}
			if v.X > maxs.X {
				maxs.X = v.X
			}
			if v.Y < mins.Y {
				mins.Y = v.Y
			}
			if v.Y > maxs.Y {
				maxs.Y = v.Y
			}
			if v.Z < mins.Z {
				mins.Z = v.Z
			}
			if v.Z > maxs.Z {
				maxs.Z = v.Z
			}
		}
	}
	b.bounds = [2]vec3{mins, maxs}
}

func (ba *brushArena) sidesOf(id brushRef) []sideRec {
	b := &ba.brushes[id]
	return ba.sides[b.sidesStart : b.sidesStart+b.sidesCount]
}

// sideView resolves a side's winding vertices.
func (ba *brushArena) sideView(s *sideRec) winding {
	return ba.w.at(s.w)
}

// sidePlane returns the side's oriented plane.
func sidePlaneOf(s *sideRec) plane {
	return plane{Normal: s.n, Dist: s.d}
}

// splitBrushSides clips every side of b to both sides of the split plane,
// appending the surviving records (with onnode propagation) to the
// front/back record buffers passed in.
func (ba *brushArena) splitBrushSides(id brushRef, p plane, fs *[]sideRec, bs *[]sideRec) {
	for _, s := range ba.sidesOf(id) {
		if fw, ok := clipWindingRef(ba.w, s.w, p); ok && fw.count >= 3 {
			*fs = append(*fs, sideRec{planenum: s.planenum, onnode: s.onnode, n: s.n, d: s.d, w: fw})
		}
		if bw, ok := clipWindingRef(ba.w, s.w, negPlane(p)); ok && bw.count >= 3 {
			*bs = append(*bs, sideRec{planenum: s.planenum, onnode: s.onnode, n: s.n, d: s.d, w: bw})
		}
	}
}

// brushCrossSectionRef returns the polygon of plane p inside the brush
// (seeded from the brush AABB, clipped by every side on the interior).
func (ba *brushArena) brushCrossSectionRef(id brushRef, p plane) wref {
	b := &ba.brushes[id]
	seed := windingFromBoxPlaneRef(ba.w, p, b.bounds[0], b.bounds[1])
	if seed.count == 0 {
		return wref{}
	}
	for _, s := range ba.sidesOf(id) {
		clipped, ok := clipWindingRef(ba.w, seed, negPlane(sidePlaneOf(&s)))
		if !ok {
			return wref{}
		}
		seed = clipped
	}
	return windingRemoveColinearRef(ba.w, seed)
}
