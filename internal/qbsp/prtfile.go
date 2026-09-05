package qbsp

import (
	"fmt"
	"strings"
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
