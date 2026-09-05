package qbsp

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// childRef is a resolved BSP child: either a node index or a leaf index in
// the global (appended) node/leaf tables.
type childRef struct {
	isLeaf bool
	idx    int
}

// renumberLeaves reorders leaves so non-solid leaves come first: the
// engine sizes PVS rows from Models[0].VisLeafs (the non-solid count), and
// every solid leaf must sit at index >= VisLeafs so PVS bit indices line
// up with leaf numbers. Node children and portal leaf numbers are remapped
// accordingly.
func renumberLeaves(nodes []outNode, leafs []outLeaf) ([]outNode, []outLeaf, map[int]int) {
	remap := make(map[int]int, len(leafs))
	ordered := make([]outLeaf, 0, len(leafs))
	for i, l := range leafs {
		if l.content != bsp.ContentsSolid {
			remap[i] = len(ordered)
			ordered = append(ordered, l)
		}
	}
	for i, l := range leafs {
		if l.content == bsp.ContentsSolid {
			remap[i] = len(ordered)
			ordered = append(ordered, l)
		}
	}
	renumNodes := make([]outNode, len(nodes))
	copy(renumNodes, nodes)
	for i := range renumNodes {
		for c := 0; c < 2; c++ {
			ch := renumNodes[i].children[c]
			if ch.isLeaf {
				ch.idx = remap[ch.idx]
				renumNodes[i].children[c] = ch
			}
		}
	}
	return renumNodes, ordered, remap
}

// visLeafs counts non-solid leaves (the world model's visleafs field).
func visLeafs(leafs []outLeaf) int32 {
	n := int32(0)
	for _, l := range leafs {
		if l.content != bsp.ContentsSolid {
			n++
		}
	}
	return n
}