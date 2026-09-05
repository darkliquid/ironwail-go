package vis

import (
	"math"
)

// bitCount is the number of PVS leaves (bits per row). Leaves are numbered
// 0..bitCount-1, which our qbsp guarantees equals the non-solid leaves (the
// engine sizes rows from the world model's visleafs).
type bitSet struct {
	bits []uint64
}

func newBitSet(n int) *bitSet {
	return &bitSet{bits: make([]uint64, (n+63)/64)}
}

func (b *bitSet) set(i int)      { b.bits[i/64] |= 1 << uint(i%64) }
func (b *bitSet) has(i int) bool { return b.bits[i/64]&(1<<uint(i%64)) != 0 }

// toBytes returns the row as a little-endian byte slice of n bits.
func (b *bitSet) toBytes(n int) []byte {
	out := make([]byte, (n+7)/8)
	for i := 0; i < n; i++ {
		if b.has(i) {
			out[i/8] |= 1 << uint(i%8)
		}
	}
	return out
}

// computePVS returns one uncompressed PVS row (bitCount bits) per leaf
// 0..bitCount-1, computed by portal flow: from each seed leaf, the flood
// spreads through a portal when the portal's polygon is visible through the
// window polygon (the last crossed portal), marking the far leaf and
// recursing. Windows narrow to the crossed portal's polygon; each portal is
// expanded at most once per run, which bounds the recursion.
func computePVS(bitCount int, portals []Portal) [][]byte {
	byLeaf := make([][]int, bitCount)
	for pi, p := range portals {
		if p.Leafs[0] >= 0 && p.Leafs[0] < bitCount {
			byLeaf[p.Leafs[0]] = append(byLeaf[p.Leafs[0]], pi)
		}
		if p.Leafs[1] >= 0 && p.Leafs[1] < bitCount {
			byLeaf[p.Leafs[1]] = append(byLeaf[p.Leafs[1]], pi)
		}
	}

	rows := make([][]byte, bitCount)
	for seed := 0; seed < bitCount; seed++ {
		bits := newBitSet(bitCount)
		bits.set(seed)
		visited := map[int]bool{}

		var expand func(via int, leaf int, window portalPoly)
		expand = func(via int, leaf int, window portalPoly) {
			for _, q := range byLeaf[leaf] {
				if q == via || visited[q] {
					continue
				}
				p := portals[q]
				far := p.Leafs[1]
				if p.Leafs[1] == leaf {
					far = p.Leafs[0]
				}
				if far < 0 || far >= bitCount || far == leaf {
					continue
				}
				if !windowSees(window, p.Points) {
					continue
				}
				visited[q] = true
				bits.set(far)
				expand(q, far, portalPoly{p.Points})
			}
		}

		// Seed: the seed leaf sees through each of its own portals without
		// any window restriction (convex leaf ⇒ full view).
		for _, p := range byLeaf[seed] {
			portal := portals[p]
			far := portal.Leafs[1]
			if portal.Leafs[1] == seed {
				far = portal.Leafs[0]
			}
			if far < 0 || far >= bitCount {
				continue
			}
			if !visited[p] {
				visited[p] = true
				bits.set(far)
				expand(p, far, portalPoly{portal.Points})
			}
		}
		rows[seed] = bits.toBytes(bitCount)
	}
	return rows
}

// portalPoly is a convex portal polygon for clipping tests.
type portalPoly struct {
	pts [][3]float64
}

// normal computes the (unnormalised) facet normal of the polygon.
func (w portalPoly) normal() [3]float64 {
	n := [3]float64{}
	for i := range w.pts {
		a := w.pts[i]
		b := w.pts[(i+1)%len(w.pts)]
		n[0] += (a[1] - b[1]) * (a[2] + b[2])
		n[1] += (a[2] - b[2]) * (a[0] + b[0])
		n[2] += (a[0] - b[0]) * (a[1] + b[1])
	}
	return n
}

// windowSees reports whether the window polygon sees any part of the target
// polygon: the target is orthogonally projected onto the window's plane and
// clipped against the window's 2D footprint (Sutherland-Hodgman). This is
// the classic portal-to-portal visibility test.
func windowSees(w portalPoly, target [][3]float64) bool {
	if len(w.pts) < 3 || len(target) < 3 {
		return false
	}
	// 2D projection basis: dominant axis of the window normal.
	n := w.normal()
	dom := 0
	if math.Abs(n[1]) > math.Abs(n[dom]) {
		dom = 1
	}
	if math.Abs(n[2]) > math.Abs(n[dom]) {
		dom = 2
	}
	u := (dom + 1) % 3
	v := (dom + 2) % 3
	to2 := func(p [3]float64) [2]float64 { return [2]float64{p[u], p[v]} }

	// Window polygon in 2D, wound counter-clockwise (portal windings from
	// the arrangement are arbitrary; the clip test requires a consistent
	// orientation).
	win := make([][2]float64, 0, len(w.pts))
	for _, p := range w.pts {
		win = append(win, to2(p))
	}
	if signedArea2D(win) < 0 {
		rev := make([][2]float64, len(win))
		for i := range win {
			rev[i] = win[len(win)-1-i]
		}
		win = rev
	}

	clip := make([][2]float64, 0, len(target))
	for _, p := range target {
		clip = append(clip, to2(p))
	}
	// Sutherland-Hodgman clip by w's edges.
	wn := len(win)
	for i := 0; i < wn && len(clip) > 0; i++ {
		a := win[i]
		b := win[(i+1)%wn]
		// inside = left of edge a->b (ccw winding expected)
		inside := func(p [2]float64) bool {
			return (b[0]-a[0])*(p[1]-a[1])-(b[1]-a[1])*(p[0]-a[0]) > -1e-7
		}
		var next [][2]float64
		m := len(clip)
		for j := 0; j < m; j++ {
			cur := clip[j]
			nxt := clip[(j+1)%m]
			curIn := inside(cur)
			nxtIn := inside(nxt)
			if curIn {
				next = append(next, cur)
			}
			if curIn != nxtIn {
				// intersection of segment cur->nxt with the edge line
				dx := nxt[0] - cur[0]
				dy := nxt[1] - cur[1]
				den := (b[0]-a[0])*dy - (b[1]-a[1])*dx
				if math.Abs(den) > 1e-12 {
					t := ((b[0]-a[0])*(a[1]-cur[1]) - (b[1]-a[1])*(a[0]-cur[0])) / den
					next = append(next, [2]float64{cur[0] + t*dx, cur[1] + t*dy})
				}
			}
		}
		clip = next
	}
	return len(clip) >= 3
}

// signedArea2D returns twice the signed area of a 2D polygon (positive for
// counter-clockwise winding).
func signedArea2D(p [][2]float64) float64 {
	sum := 0.0
	for i := range p {
		a := p[i]
		b := p[(i+1)%len(p)]
		sum += a[0]*b[1] - b[0]*a[1]
	}
	return sum}
