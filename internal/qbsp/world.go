package qbsp

import (
	"sort"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// piece is one fragment of a facet after splitting by a subtree.
type piece struct {
	leaf int
	w    winding
}

// splitByTree partitions a facet winding by the subtree rooted at ref,
// yielding one piece per leaf it covers.
func splitByTree(nodes []outNode, ref childRef, w winding) []piece {
	if ref.isLeaf {
		if len(w) < 3 {
			return nil
		}
		return []piece{{leaf: ref.idx, w: w}}
	}
	nd := &nodes[ref.idx]
	p := plane{Normal: nd.splitN, Dist: nd.splitD}
	fw, fok := clipWinding(w, p)
	bw, bok := clipWinding(w, negPlane(p))
	var out []piece
	if fok {
		out = append(out, splitByTree(nodes, nd.children[0], fw)...)
	}
	if bok {
		out = append(out, splitByTree(nodes, nd.children[1], bw)...)
	}
	return out
}

// siblingAtPlane returns the subtree on the OTHER side of the interface at
// the deepest path node whose split plane is pi, for the leaf described by
// path. Returns ok=false when pi is a root-box plane (no interface node).
func siblingAtPlane(nodes []outNode, path []pathStep, pi int) (childRef, bool) {
	for i := len(path) - 1; i >= 0; i-- {
		st := path[i]
		if nodes[st.node].planenum == pi {
			return nodes[st.node].children[1-st.side], true
		}
	}
	return childRef{}, false
}

// planeAtBoxFace finds a table plane coincident with an oriented box face
// (used to attribute outer wall faces to the brush planes at the map
// boundary). Returns -1 when the box face is not coincident with a brush
// plane.
func planeAtBoxFace(planes []plane, boxFace plane) int {
	for i, p := range planes {
		if planeEqualOriented(p, boxFace) {
			return i
		}
	}
	return -1
}

// buildWorldSurfaces computes the world model's faces (per-leaf attachment),
// the inter-leaf portals (PRT1), the leak flood state, and node face spans.
//
// Returned attach is indexed by leaf (post-renumber); the caller copies it
// into leafs[].marksurface. Nodes gain firstface/numfaces spans (faces are
// returned sorted by planenum for the span lookup).
func (c *compiler) buildWorldSurfaces(bounds [2]vec3, root childRef, nodes []outNode, leafs []outLeaf, paths [][]pathStep, m *Map) ([]outFace, [][]int, *PortalFile, []vec3, bool) {
	var faces []outFace
	attach := make([][]int, len(leafs))
	pf := &PortalFile{LeafCount: len(leafs)}
	seenPortal := map[[2]int]bool{}

	// leak flood adjacency + BFS
	n := len(leafs)
	adj := make([][]int, n)
	floodParent := make([]int, n)
	for i := range floodParent {
		floodParent[i] = -2 // unvisited
	}

	// 1. Per-leaf facet enumeration and surface/portal generation.
	voidLeaf := make([]bool, n)
	for li := range leafs {
		L := &leafs[li]
		if L.content == bsp.ContentsSolid {
			continue // solid leaves: no portals, no flood seeds
		}
		for _, f := range L.region.facets(bounds) {
			if f.pi < 0 {
				// void facet: this leaf touches the outside of the map.
				voidLeaf[li] = true
				continue
			}
			sib, ok := siblingAtPlane(nodes, paths[li], f.pi)
			if !ok {
				continue
			}
			neigh := splitByTree(nodes, sib, f.w)
			for _, pc := range neigh {
				if pc.leaf == li {
					continue
				}
				if leafs[pc.leaf].content != bsp.ContentsSolid {
					// portal between two non-solid leaves.
					key := [2]int{li, pc.leaf}
					keyInv := [2]int{pc.leaf, li}
					if !seenPortal[key] && !seenPortal[keyInv] {
						seenPortal[key] = true
						pf.Portals = append(pf.Portals, Portal{
							Leafs:  [2]int{li, pc.leaf},
							Points: []Point(windingRemoveColinear(pc.w)),
						})
					}
					adj[li] = append(adj[li], pc.leaf)
					adj[pc.leaf] = append(adj[pc.leaf], li)
				}
			}
		}
	}

	// 2. Faces: every leaf (solid included) emits its boundary facets where
	// the neighbour has different contents; the face is oriented outward
	// from the denser side and attached to the lighter leaf.
	for li := range leafs {
		L := &leafs[li]
		for _, f := range L.region.facets(bounds) {
			var pi int
			outward := f.p.Normal
			if f.pi < 0 {
				// Void facet: attribute to a coincident table plane (the
				// outer wall brush face), else no geometry here.
				pi = planeAtBoxFace(c.planes, f.p)
				if pi < 0 {
					continue
				}
				// Only the solid side facing the void emits a face.
				if L.content == bsp.ContentsSolid {
					gi := len(faces)
					faces = append(faces, outFace{
						planenum: pi,
						side:     sideBit(c.planes[pi].Normal, outward),
						texinfo:  c.texInfoOrZero(pi),
						poly:     windingOrientTo(f.w, outward),
					})
					attach[li] = append(attach[li], gi)
				}
				continue
			}
			sib, ok := siblingAtPlane(nodes, paths[li], f.pi)
			if !ok {
				continue
			}
			neigh := splitByTree(nodes, sib, f.w)
			for _, pc := range neigh {
				if pc.leaf == li {
					continue
				}
				nc := leafs[pc.leaf].content
				if nc == L.content {
					continue
				}
				dense := denseCell(L.content, nc)
				if dense == 0 {
					// L is the denser side: emit here.
					gi := len(faces)
					faces = append(faces, outFace{
						planenum: f.pi,
						side:     sideBit(c.planes[f.pi].Normal, outward),
						texinfo:  c.texInfoOrZero(f.pi),
						poly:     windingOrientTo(pc.w, outward),
					})
					attach[pc.leaf] = append(attach[pc.leaf], gi)
				}
			}
		}
	}

	// 3. Leak flood from the void ring over non-solid adjacency.
	var queue []int
	for li := range voidLeaf {
		if voidLeaf[li] {
			floodParent[li] = -1
			queue = append(queue, li)
		}
	}
	sort.Ints(queue)
	for qi := 0; qi < len(queue); qi++ {
		cur := queue[qi]
		for _, nb := range adj[cur] {
			if floodParent[nb] == -2 {
				floodParent[nb] = cur
				queue = append(queue, nb)
			}
		}
	}

	// 4. Entity origins in flooded leaves leak.
	var leakPath []vec3
	leaked := false
	for _, ent := range m.Entities[1:] {
		originStr, ok := ent.Value("origin")
		if !ok {
			continue
		}
		origin, err := parseOrigin(originStr)
		if err != nil {
			continue
		}
		leafIdx, inside := pointInLeaf(nodes, root, origin)
		if !inside || leafIdx < 0 || leafIdx >= n {
			return faces, attach, pf, []vec3{origin}, true
		}
		if floodParent[leafIdx] == -2 {
			continue
		}
		// Walk the parent chain back to a ring leaf.
		var trail []vec3
		cur := leafIdx
		for cur >= 0 {
			trail = append([]vec3{c.leafCentroid(bounds, &leafs[cur])}, trail...)
			if floodParent[cur] < 0 {
				break
			}
			cur = floodParent[cur]
		}
		leaked = true
		leakPath = trail
		break
	}

	// 5. Order faces by planenum and set node spans.
	orderFacesByPlane(&faces, leafs)
	setNodeFaceSpans(nodes, faces)

	return faces, attach, pf, leakPath, leaked
}

// centroid returns a representative interior point of a leaf (the facet
// vertex average, which lies strictly inside a convex region).
func (c *compiler) leafCentroid(bounds [2]vec3, L *outLeaf) vec3 {
	var sum vec3
	count := 0
	for _, f := range L.region.facets(bounds) {
		for _, p := range f.w {
			sum[0] += p[0]
			sum[1] += p[1]
			sum[2] += p[2]
			count++
		}
	}
	if count == 0 {
		return vec3{(L.mins[0] + L.maxs[0]) / 2, (L.mins[1] + L.maxs[1]) / 2, (L.mins[2] + L.maxs[2]) / 2}
	}
	return vec3{sum[0] / float64(count), sum[1] / float64(count), sum[2] / float64(count)}
}

// sideBit is 1 when the face normal opposes the table plane's normal.
func sideBit(tableN, outward vec3) int8 {
	if v3Dot(tableN, outward) < 0 {
		return 1
	}
	return 0
}

// texInfoOrZero returns the texinfo registered for a plane (0 fallback).
func (c *compiler) texInfoOrZero(pi int) int {
	if ti, ok := c.texByPlane[pi]; ok {
		return ti
	}
	return 0
}

// orderFacesByPlane stably sorts faces by planenum, remapping leaf
// marksurface indices.
func orderFacesByPlane(faces *[]outFace, leafs []outLeaf) {
	n := len(*faces)
	if n == 0 {
		return
	}
	perm := make([]int, n)
	for i := range perm {
		perm[i] = i
	}
	fs := *faces
	sort.SliceStable(perm, func(a, b int) bool { return fs[perm[a]].planenum < fs[perm[b]].planenum })
	inv := make([]int, n)
	for i, oi := range perm {
		inv[oi] = i
	}
	nf := make([]outFace, n)
	for i, oi := range perm {
		nf[i] = fs[oi]
	}
	for li := range leafs {
		for k, fi := range leafs[li].marksurface {
			leafs[li].marksurface[k] = inv[fi]
		}
	}
	*faces = nf
}

// setNodeFaceSpans fills each node's firstface/numfaces from the
// (planes-ordered) face list.
func setNodeFaceSpans(nodes []outNode, faces []outFace) {
	span := map[int][2]int{}
	for i, f := range faces {
		s := span[f.planenum]
		s[1] = i + 1
		if _, ok := span[f.planenum]; !ok {
			s[0] = i
		}
		span[f.planenum] = s
	}
	for i := range nodes {
		if s, ok := span[nodes[i].planenum]; ok {
			nodes[i].firstface = s[0]
			nodes[i].numfaces = s[1] - s[0]
		}
	}
}

// modelPaths builds model-local root-to-leaf chains from the pre-renumber
// tree, reindexed by the renumber map (used before offsets are applied).
func modelPaths(nodes []outNode, leafs []outLeaf, remap map[int]int) [][]pathStep {
	out := make([][]pathStep, len(leafs))
	for li := range leafs {
		// Re-map the leaf index into the renumbered ordering; node indices
		// are unchanged by renumbering, so the chain stays valid.
		out[remap[li]] = pathToLeaf(nodes, leafs, li)
	}
	return out
}

// buildModelSurfaces computes faces and leaf marksurface attachment for a
// brush-entity submodel tree (no portals, no leak flood: submodels are not
// part of the world's visibility or leak topology in v1).
func (c *compiler) buildModelSurfaces(bounds [2]vec3, root childRef, nodes []outNode, leafs []outLeaf, paths [][]pathStep) ([]outFace, [][]int) {
	var faces []outFace
	attach := make([][]int, len(leafs))
	for li := range leafs {
		L := &leafs[li]
		for _, f := range L.region.facets(bounds) {
			if f.pi < 0 {
				// Root-box face: the outer surface of a solid leaf that
				// fills its region (e.g. a crate). Attribute to a
				// coincident table plane (the brush face at the boundary).
				pi := planeAtBoxFace(c.planes, f.p)
				if pi < 0 || L.content != bsp.ContentsSolid {
					continue
				}
				outward := f.p.Normal
				gi := len(faces)
				faces = append(faces, outFace{
					planenum: pi,
					side:     sideBit(c.planes[pi].Normal, outward),
					texinfo:  c.texInfoOrZero(pi),
					poly:     windingOrientTo(f.w, outward),
				})
				attach[li] = append(attach[li], gi)
				continue
			}
			sib, ok := siblingAtPlane(nodes, paths[li], f.pi)
			if !ok {
				continue
			}
			outPage := splitByTree(nodes, sib, f.w)
			for _, pc := range outPage {
				if pc.leaf == li {
					continue
				}
				nc := leafs[pc.leaf].content
				if nc == L.content {
					continue
				}
				dense := denseCell(L.content, nc)
				if dense == 0 {
					// L is the denser side: emit here, facing outward.
					outward := f.p.Normal
					gi := len(faces)
					faces = append(faces, outFace{
						planenum: f.pi,
						side:     sideBit(c.planes[f.pi].Normal, outward),
						texinfo:  c.texInfoOrZero(f.pi),
						poly:     windingOrientTo(pc.w, outward),
					})
					attach[pc.leaf] = append(attach[pc.leaf], gi)
				}
			}
		}
	}
	return faces, attach
}
