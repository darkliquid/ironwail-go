package qbsp

import (
	"fmt"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// PortalFile returns the PRT1 portal file (Map ↔ leaf topology) for a
// compiled map: one record per shared facet between non-solid leaves, in
// the format vis consumes (and ericw-tools' vis reads): "PRT1",
// <numleafs-invis> <numportals>, then per portal:
// <numpoints> <leaf0> <leaf1> and the points as "( x y z )".
type PortalFile struct {
	LeafCount int
	Portals   []Portal
}

// Portal is one shared facet between two non-solid leaves.
type Portal struct {
	Leafs  [2]int
	Points []Point
}

// gatherPortals extracts the portals from the arrangement: every facet
// shared by two non-solid cells becomes a portal between their (renumbered)
// leaves. The polygon is the shared facet of the first cell, oriented
// arbitrarily; the leaf order is deterministic.
func (c *compiler) gatherPortals(arr *arrangement, cellLeaf map[int]int, leafRenumber map[int]int) *PortalFile {
	pf := &PortalFile{}
	pf.LeafCount = len(leafRenumber)

	seen := map[[2]int]bool{}
	for ci := range arr.cells {
		if cellLeaf == nil {
			break
		}
		cell := arr.cells[ci]
		if cell.content == bsp.ContentsSolid {
			continue
		}
		for _, h := range cell.hs {
			pi := h.plane
			nj := arr.neighborPlane(ci, pi)
			if nj < 0 {
				continue
			}
			nbr := arr.cells[nj]
			if nbr.content == bsp.ContentsSolid {
				continue
			}
			// Both sides non-solid: a vis portal.
			la := cellLeaf[ci]
			lb := cellLeaf[nj]
			if la == lb {
				continue
			}
			key := [2]int{la, lb}
			keyInv := [2]int{lb, la}
			if seen[key] || seen[keyInv] {
				continue
			}
			// The shared facet polygon (facet of cell ci on plane pi).
			poly := c.facetOf(arr, ci, pi)
			if poly == nil {
				continue
			}
			poly = windingRemoveColinear(poly)
			if len(poly) < 3 {
				continue
			}
			seen[key] = true
			// Remap to renumbered (non-solid-first) leaf indices.
			if r, ok := leafRenumber[la]; ok {
				la = r
			}
			if r, ok := leafRenumber[lb]; ok {
				lb = r
			}
			pf.Portals = append(pf.Portals, Portal{
				Leafs:  [2]int{la, lb},
				Points: []Point(poly),
			})
		}
	}
	return pf
}

// Serialize renders the PRT1 text (readable by ericw-tools vis and ours).
func (pf *PortalFile) Serialize() []byte {
	var b strings.Builder
	if pf.LeafCount == 0 {
		return nil
	}
	fmt.Fprintf(&b, "%s\n", "PRT1")
	fmt.Fprintf(&b, "%d %d\n", pf.LeafCount, len(pf.Portals))
	for _, p := range pf.Portals {
		fmt.Fprintf(&b, "%d %d %d\n", len(p.Points), p.Leafs[0], p.Leafs[1])
		for _, pt := range p.Points {
			fmt.Fprintf(&b, "( %g %g %g )\n", pt[0], pt[1], pt[2])
		}
	}
	return []byte(b.String())
}
