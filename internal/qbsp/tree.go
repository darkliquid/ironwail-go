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

// outNode is a BSP node in compiler terms.
type outNode struct {
	plane    int
	children [2]childRef
	bounds   [2]vec3
}

// outLeaf is a BSP leaf in compiler terms.
type outLeaf struct {
	content     int32
	mins, maxs  vec3
	marksurface []int // face indices into the global face table
}

// treeBuilder accumulates the final node/leaf tables (world first, then any
// brush-entity submodels).
type treeBuilder struct {
	nodes []outNode
	leafs []outLeaf
	faces map[int][]int // face indices attached per CELL (remapped to leaves)
	// cellLeaf records which leaf index absorbed each arrangement cell.
	cellLeaf map[int]int
}

// build recursively partitions the given arrangement cells into a BSP by
// replaying the arrangement's own split sequence: plane 0, then plane 1,
// and so on. Because the arrangement cells are exactly the intersections of
// these oriented halfspaces, the resulting tree is the arrangement's
// decision tree: every leaf corresponds to exactly one cell, the leaf
// regions tile space without overlap, and a point descends to the cell
// containing it. This is provably consistent (unlike a balancing heuristic,
// which can build overlapping leaf regions).
func (tb *treeBuilder) build(a *arrangement, cells []int) childRef {
	return tb.buildLevel(a, cells, 0)
}

func (tb *treeBuilder) buildLevel(a *arrangement, cells []int, level int) childRef {
	if level >= len(a.planes) {
		return tb.makeLeaf(a, cells)
	}
	pi := level
	var front, back []int
	for _, ci := range cells {
		if hsFront(a.cells[ci], pi) {
			front = append(front, ci)
		} else {
			back = append(back, ci)
		}
	}
	if len(front) == 0 || len(back) == 0 {
		// This plane does not separate the subset; skip it.
		return tb.buildLevel(a, cells, level+1)
	}
	idx := len(tb.nodes)
	tb.nodes = append(tb.nodes, outNode{plane: pi})
	mins, maxs := a.cellBoundsAll(cells)
	tb.nodes[idx].bounds = [2]vec3{mins, maxs}
	tb.nodes[idx].children[0] = tb.buildLevel(a, front, level+1)
	tb.nodes[idx].children[1] = tb.buildLevel(a, back, level+1)
	return childRef{isLeaf: false, idx: idx}
}

// hsFront returns the region's orientation on plane pi (front = +side).
func hsFront(c *cell, pi int) bool {
	for _, h := range c.hs {
		if h.plane == pi {
			return h.front
		}
	}
	return false
}

func (tb *treeBuilder) makeLeaf(a *arrangement, cells []int) childRef {
	content := a.cells[cells[0]].content
	mins, maxs := a.cellBoundsAll(cells)
	idx := len(tb.leafs)
	leaf := outLeaf{content: content, mins: mins, maxs: maxs}
	seen := map[int]bool{}
	for _, ci := range cells {
		if tb.cellLeaf != nil {
			tb.cellLeaf[ci] = idx
		}
		for _, f := range tb.faces[ci] {
			if !seen[f] {
				seen[f] = true
				leaf.marksurface = append(leaf.marksurface, f)
			}
		}
	}
	tb.leafs = append(tb.leafs, leaf)
	return childRef{isLeaf: true, idx: idx}
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

// cellBoundsAll unions the bounds of several cells.
func (a *arrangement) cellBoundsAll(cells []int) (vec3, vec3) {
	if len(cells) == 0 {
		return a.bounds[0], a.bounds[1]
	}
	mins, maxs := a.cellBounds(cells[0])
	for _, ci := range cells[1:] {
		m, x := a.cellBounds(ci)
		for i := 0; i < 3; i++ {
			if m[i] < mins[i] {
				mins[i] = m[i]
			}
			if x[i] > maxs[i] {
				maxs[i] = x[i]
			}
		}
	}
	return mins, maxs
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