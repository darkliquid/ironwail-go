package bspdec

import (
	"encoding/binary"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// buildMiptexLump builds a minimal miptex lump: count, offset table, then
// name records (name[16] + width + height; bspdec reads only the name).
func buildMiptexLump(names ...string) []byte {
	out := make([]byte, 4+4*len(names))
	binary.LittleEndian.PutUint32(out[0:], uint32(len(names)))
	for i, n := range names {
		binary.LittleEndian.PutUint32(out[4+i*4:], uint32(len(out)))
		rec := make([]byte, 24)
		copy(rec[:16], n)
		out = append(out, rec...)
	}
	return out
}

// twoTextureTopTree builds a minimal tree whose only faces are two coplanar
// quads on z=64: "left" covering x in [0,32], "right" covering x in [32,64].
func twoTextureTopTree() *bsp.Tree {
	mkQuad := func(x0, x1 float32) []bsp.DVertex {
		pts := [4][3]float32{{x0, 0, 64}, {x1, 0, 64}, {x1, 64, 64}, {x0, 64, 64}}
		verts := make([]bsp.DVertex, 4)
		for i := range pts {
			verts[i] = bsp.DVertex{Point: types.Vec3{X: pts[i][0], Y: pts[i][1], Z: pts[i][2]}}
		}
		return verts
	}
	tree := &bsp.Tree{
		Planes:      []bsp.DPlane{{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 64, Type: bsp.PlaneZ}},
		TextureData: buildMiptexLump("left", "right"),
		Texinfo: []bsp.Texinfo{
			{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, -1, 0, 0}}, Miptex: 0},
			{Vecs: [2][4]float32{{1, 0, 0, 0}, {0, -1, 0, 0}}, Miptex: 1},
		},
	}
	for _, q := range [2][]bsp.DVertex{mkQuad(0, 32), mkQuad(32, 64)} {
		base := uint32(len(tree.Vertexes))
		tree.Vertexes = append(tree.Vertexes, q...)
		edgeBase := len(tree.Edges)
		for e := 0; e < 4; e++ {
			tree.Edges = append(tree.Edges, bsp.TreeEdge{V: [2]uint32{base + uint32(e), base + uint32((e+1)%4)}})
			tree.Surfedges = append(tree.Surfedges, int32(edgeBase+e))
		}
		tree.Faces = append(tree.Faces, bsp.TreeFace{
			PlaneNum: 0, Side: 0,
			FirstEdge: int32(edgeBase), NumEdges: 4,
			Texinfo: int32(len(tree.Faces)),
		})
	}
	return tree
}

func TestSplitDifferentTextures(t *testing.T) {
	tree := twoTextureTopTree()
	d := newDecompiler(tree, Options{TextureFallback: "nearest"})
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	d.textureBrush(b)
	parts := d.splitDifferentTextures(b)
	if len(parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(parts))
	}
	top := mapfile.Plane{Normal: vc(0, 0, 1), Dist: 64}
	tops := map[string]bool{}
	for _, p := range parts {
		for _, s := range p.Sides {
			if planesMatch(s.Plane, top) {
				tops[s.TexName] = true
			}
		}
	}
	if !tops["left"] || !tops["right"] {
		t.Fatalf("top textures after split = %v, want left+right", tops)
	}
}

func TestSplitKeepsUniformSideAlone(t *testing.T) {
	tree := twoTextureTopTree()
	// drop the "right" face: only one texinfo remains
	tree.Faces = tree.Faces[:1]
	d := newDecompiler(tree, Options{TextureFallback: "nearest"})
	b := boxBrush(vc(0, 0, 0), vc(64, 64, 64))
	d.textureBrush(b)
	if parts := d.splitDifferentTextures(b); len(parts) != 1 {
		t.Fatalf("parts = %d, want 1 (no split)", len(parts))
	}
}