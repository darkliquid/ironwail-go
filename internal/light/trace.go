package light

import (
	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// TreeTracer returns a shadow-trace function that reports whether the
// segment from→to crosses a solid leaf in the BSP tree (the classic
// ray-vs-BSP walk: descend the nearer side of each split plane first).
func TreeTracer(tree *bsp.Tree) func(from, to [3]float64) bool {
	return func(from, to [3]float64) bool {
		if tree == nil || len(tree.Nodes) == 0 {
			return false
		}
		return traceBlocked(tree, from, to, 0)
	}
}

func traceBlocked(tree *bsp.Tree, from, to [3]float64, nodeIdx int) bool {
	if nodeIdx < 0 || nodeIdx >= len(tree.Nodes) {
		return false
	}
	node := &tree.Nodes[nodeIdx]
	if node.PlaneNum < 0 || int(node.PlaneNum) >= len(tree.Planes) {
		return false
	}
	pl := &tree.Planes[node.PlaneNum]
	df := float64(pl.Normal.X*float32(from[0])+pl.Normal.Y*float32(from[1])+pl.Normal.Z*float32(from[2])) - float64(pl.Dist)
	dt := float64(pl.Normal.X*float32(to[0])+pl.Normal.Y*float32(to[1])+pl.Normal.Z*float32(to[2])) - float64(pl.Dist)
	if df >= 0 && dt >= 0 {
		return traceChild(tree, from, to, node.Children[0])
	}
	if df < 0 && dt < 0 {
		return traceChild(tree, from, to, node.Children[1])
	}
	// The segment crosses the plane: descend the front part, then the back.
	t := df / (df - dt)
	mid := [3]float64{
		from[0] + (to[0]-from[0])*t,
		from[1] + (to[1]-from[1])*t,
		from[2] + (to[2]-from[2])*t,
	}
	if traceChild(tree, from, mid, node.Children[0]) {
		return true
	}
	return traceChild(tree, mid, to, node.Children[1])
}

func traceChild(tree *bsp.Tree, from, to [3]float64, child bsp.TreeChild) bool {
	if child.IsLeaf {
		if child.Index >= 0 && child.Index < len(tree.Leafs) {
			return tree.Leafs[child.Index].Contents == bsp.ContentsSolid
		}
		return false
	}
	return traceBlocked(tree, from, to, child.Index)
}
