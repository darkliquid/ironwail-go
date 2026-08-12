// synthetic_map.go — a deterministic, asset-free map for no-assets boots.
//
// The browser walkthrough (plan 22) and any wasm/no-data session need a
// spawnable world even when pak0.pak / maps/*.bsp are absent. This builder
// produces a tiny box room (floor + four walls + ceiling) as real BSP-shaped
// structs (bsp.Tree for the renderer + bsp.File for collision), with a single
// flat-color miptex, a constant lightmap, and one info_player_start entity.
//
// Where in C: no direct lineage (synthetic); the lumps mirror the standard
// Quake BSP29 layout that bsp.LoadTree / bsp.Load / SV_LoadMap expect.

package server

import (
	"fmt"
	"math"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// SyntheticMapName is the map name used for no-assets boots. It is also
// written into the worldspawn/player-start entities so a demo can identify it.
const SyntheticMapName = "synthetic"

const (
	syntheticRoomMin  = -256
	syntheticRoomMax  = 256
	syntheticGroundZ  = 0
	syntheticCeilingZ = 192
	syntheticTexSize  = 64 // miptex width/height (power of two)
)

// faceQuad describes one quad face of the room as four corner points plus the
// texture axes (s/t) and the plane normal/distance.
type faceQuad struct {
	verts   [4][3]float32
	normal  [3]float32
	dist    float32
	planeTy int32
	// texS/T map world units to texture pixels; the room is axis-aligned so
	// each face has unit-scale axes along two world axes.
	texS [4]float32 // [axis(0=x,1=y,2=z), unused, unused, offset]
	texT [4]float32
}

// BuildSyntheticMap returns a full in-memory BSP (Tree for rendering, File
// for collision) for a 512x512x192 box room. nil, nil, err on construction
// failure; the two structs share identical lump data.
func BuildSyntheticMap() (*bsp.Tree, *bsp.File, error) {
	room := syntheticRoomFaces()

	var verts []bsp.DVertex
	vertIndex := map[[3]float32]int{}
	vertexOf := func(p [3]float32) int {
		if i, ok := vertIndex[p]; ok {
			return i
		}
		i := len(verts)
		verts = append(verts, bsp.DVertex{Point: p})
		vertIndex[p] = i
		return i
	}

	// 6 faces x 4 edges. Each face's edges wind counter-clockwise when
	// viewed from the front so the surface normal points outward.
	var faces []bsp.TreeFace
	var surfedges []int32
	for fi, f := range room {
		firstEdge := int32(len(surfedges))
		for vi := 0; vi < 4; vi++ {
			a := vertexOf(f.verts[vi])
			b := vertexOf(f.verts[(vi+1)%4])
			surfedges = append(surfedges, int32(len(surfedges)))
			// Register a unique edge per directed pair to keep the edge list
			// dense; the renderer reads edges via surfedge indices.
			_ = a
			_ = b
		}
		faces = append(faces, bsp.TreeFace{
			PlaneNum:  int32(fi),
			Side:      0,
			FirstEdge: firstEdge,
			NumEdges:  4,
			Texinfo:   int32(fi),
			Styles:    [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
			LightOfs:  0, // fixed below with the sequential lump layout
		})
	}

	// Build a dense edge list in the order the surfedges reference them.
	edges := make([]bsp.TreeEdge, len(surfedges))
	for si, se := range surfedges {
		fi := int(se) / 4
		vi := int(se) % 4
		a := vertexOf(room[fi].verts[vi])
		b := vertexOf(room[fi].verts[(vi+1)%4])
		edges[si] = bsp.TreeEdge{V: [2]uint32{uint32(a), uint32(b)}}
	}

	planes := make([]bsp.DPlane, len(room))
	texinfo := make([]bsp.Texinfo, len(room))
	for i, f := range room {
		planes[i] = bsp.DPlane{Normal: f.normal, Dist: f.dist, Type: f.planeTy}
		texinfo[i] = bsp.Texinfo{
			Vecs:   [2][4]float32{f.texS, f.texT},
			Miptex: 0,
			Flags:  0,
		}
	}

	// A two-node binary tree rooted at the floor split:
	//   root (floor plane, +Z): d = z. z<0 => Children[1] = solid leaf 1.
	//       z>=0 => descend into ceiling node.
	//   ceiling node (-Z): d = -z + ceil. z<=ceil => Children[0] = interior
	//       leaf 2; z>ceil => Children[1] = solid leaf 3.
	type treeLeaf struct {
		contents int32
		mins     [3]float32
		maxs     [3]float32
	}
	roomBoundsMin := [3]float32{syntheticRoomMin, syntheticRoomMin, syntheticGroundZ}
	roomBoundsMax := [3]float32{syntheticRoomMax, syntheticRoomMax, syntheticCeilingZ}
	leaves := []treeLeaf{
		{contents: bsp.ContentsSolid, mins: roomBoundsMin, maxs: roomBoundsMax}, // leaf 0 = solid sentinel
		{contents: bsp.ContentsSolid, mins: roomBoundsMin, maxs: roomBoundsMax}, // leaf 1 = below floor
		{contents: bsp.ContentsEmpty, mins: roomBoundsMin, maxs: roomBoundsMax}, // leaf 2 = interior
		{contents: bsp.ContentsSolid, mins: roomBoundsMin, maxs: roomBoundsMax}, // leaf 3 = above ceiling
	}
	nodes := []bsp.TreeNode{
		{
			PlaneNum:  floorPlaneIndex(planes),
			BoundsMin: roomBoundsMin,
			BoundsMax: roomBoundsMax,
			// d=z>=0 -> continue to ceiling node (index 1); below floor -> solid
			Children: [2]bsp.TreeChild{{IsLeaf: false, Index: 1}, {IsLeaf: true, Index: 1}},
		},
		{
			PlaneNum:  ceilingPlaneIndex(planes),
			BoundsMin: roomBoundsMin,
			BoundsMax: roomBoundsMax,
			// d=z-192: z>192 (above ceiling) -> Children[0] = solid;
			// z<=192 (inside) -> Children[1] = interior.
			Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 3}, {IsLeaf: true, Index: 2}},
		},
	}

	treeLeafs := make([]bsp.TreeLeaf, len(leaves))
	for i, l := range leaves {
		treeLeafs[i] = bsp.TreeLeaf{
			Contents:  l.contents,
			VisOfs:    -1,
			BoundsMin: l.mins,
			BoundsMax: l.maxs,
		}
	}

	// Clipnode graph mirrors the node graph (players collide with the room).
	// DSClipNode children: negative = contents (-1 = solid, -2 = empty),
	// non-negative = child node index.
	clipNodes := []bsp.DSClipNode{
		{PlaneNum: floorPlaneIndex(planes), Children: [2]int32{1, -1}}, // above floor -> ceiling node; below -> solid
		{PlaneNum: ceilingPlaneIndex(planes), Children: [2]int32{-1, -2}}, // z>192 solid; z<=192 empty
	}

	miptex := flatColorMiptex(syntheticTexSize, syntheticTexSize, 0x8f, 0x8f, 0x9f)
	textureData := miptexLump(miptex)

	// Lightmap: the renderer computes each face's lightmap extent from its
	// UV min/max (lightmap grid = 16 world units, style 0 = 1 byte/sample in
	// the monochrome lump). The room spans -256..256 on the in-plane axes, so
	// every face needs a 33x33 sample block. LightOfs was set to 0 above;
	// fill the real sequential offsets in now that the layout is known.
	// buildSyntheticLightmap returns the raw monochrome lump and the per-face
	// offsets.
	lightmap, lightmapOffsets := buildSyntheticLightmap(len(faces), 200)
	for fi := range faces {
		faces[fi].LightOfs = int32(lightmapOffsets[fi])
	}

	entities := syntheticEntities()

	models := []bsp.DModel{
		{
			BoundsMin: roomBoundsMin,
			BoundsMax: roomBoundsMax,
			HeadNode:  [bsp.MaxMapHulls]int32{0, 0, 0},
			VisLeafs:  2,
			FirstFace: 0,
			NumFaces:  int32(len(faces)),
		},
	}

	tree := &bsp.Tree{
		Version:     29,
		Entities:    []byte(entities),
		TextureData: textureData,
		Lighting:    lightmap,
		Planes:      planes,
		Vertexes:    verts,
		Texinfo:     texinfo,
		Edges:       edges,
		Surfedges:   surfedges,
		Faces:       faces,
		Leafs:       treeLeafs,
		Nodes:       nodes,
		Models:      models,
		NumTextures: 1,
	}

	file := &bsp.File{
		Version:    29,
		Entities:   []byte(entities),
		Planes:     planes,
		Vertexes:   verts,
		Texinfo:    texinfo,
		Visibility: nil,
		Lighting:   lightmap,
		Nodes:      nodesAsDSNode(nodes),
		Clipnodes:  clipNodes,
		Leafs:      leafsAsDSLeaf(treeLeafs),
		Faces:      facesAsDSFace(faces),
		Edges:      edgesAsDSEdge(edges),
		Surfedges:  surfedges,
		Models:     models,
		NumTextures: 1,
		TextureData: textureData,
	}

	if err := validateSyntheticTree(tree); err != nil {
		return nil, nil, fmt.Errorf("synthetic map invalid: %w", err)
	}
	return tree, file, nil
}

// syntheticRoomFaces builds the six room faces in outward-facing winding.
func syntheticRoomFaces() []faceQuad {
	mn, mx := float32(syntheticRoomMin), float32(syntheticRoomMax)
	ground, ceil := float32(syntheticGroundZ), float32(syntheticCeilingZ)
	return []faceQuad{
		// Floor (normal +Z)
		{
			verts:   [4][3]float32{{mn, mn, ground}, {mx, mn, ground}, {mx, mx, ground}, {mn, mx, ground}},
			normal:  [3]float32{0, 0, 1}, dist: ground, planeTy: 2,
			texS: [4]float32{1, 0, 0, 0}, texT: [4]float32{0, 1, 0, 0},
		},
		// Ceiling (normal -Z). Quake's axis-aligned plane fast path computes
		// d = p[axis] - dist, so dist is the positive coordinate (192), and
		// the child selection (not the dist sign) encodes the wall side.
		{
			verts:   [4][3]float32{{mn, mn, ceil}, {mn, mx, ceil}, {mx, mx, ceil}, {mx, mn, ceil}},
			normal:  [3]float32{0, 0, -1}, dist: ceil, planeTy: 2,
			texS: [4]float32{1, 0, 0, 0}, texT: [4]float32{0, 1, 0, 0},
		},
		// Wall -X (normal -X)
		{
			verts:   [4][3]float32{{mn, mn, ground}, {mn, mx, ground}, {mn, mx, ceil}, {mn, mn, ceil}},
			normal:  [3]float32{-1, 0, 0}, dist: -mn, planeTy: 1,
			texS: [4]float32{0, 1, 0, 0}, texT: [4]float32{0, 0, 1, 0},
		},
		// Wall +X (normal +X)
		{
			verts:   [4][3]float32{{mx, mn, ground}, {mx, mn, ceil}, {mx, mx, ceil}, {mx, mx, ground}},
			normal:  [3]float32{1, 0, 0}, dist: mx, planeTy: 1,
			texS: [4]float32{0, 1, 0, 0}, texT: [4]float32{0, 0, 1, 0},
		},
		// Wall -Y (normal -Y)
		{
			verts:   [4][3]float32{{mn, mn, ground}, {mn, mn, ceil}, {mx, mn, ceil}, {mx, mn, ground}},
			normal:  [3]float32{0, -1, 0}, dist: -mn, planeTy: 0,
			texS: [4]float32{1, 0, 0, 0}, texT: [4]float32{0, 0, 1, 0},
		},
		// Wall +Y (normal +Y)
		{
			verts:   [4][3]float32{{mn, mx, ground}, {mx, mx, ground}, {mx, mx, ceil}, {mn, mx, ceil}},
			normal:  [3]float32{0, 1, 0}, dist: mx, planeTy: 0,
			texS: [4]float32{1, 0, 0, 0}, texT: [4]float32{0, 0, 1, 0},
		},
	}
}

func floorPlaneIndex(planes []bsp.DPlane) int32 {
	for i, p := range planes {
		if p.Normal == [3]float32{0, 0, 1} {
			return int32(i)
		}
	}
	return 0
}

func ceilingPlaneIndex(planes []bsp.DPlane) int32 {
	for i, p := range planes {
		if p.Normal == [3]float32{0, 0, -1} {
			return int32(i)
		}
	}
	if len(planes) > 1 {
		return 1
	}
	return 0
}

// flatColorMiptex builds a single mip-level, solid-color miptex blob.
func flatColorMiptex(w, h int, r, g, b byte) []byte {
	// 16-byte name + 4-byte width + 4-byte height + 4 offsets (x4) = 40
	// header, then 4 mip levels at w*h, (w/2)², (w/4)², (w/8)² bytes.
	buf := make([]byte, 40+ (w*h + (w/2)*(h/2) + (w/4)*(h/4) + (w/8)*(h/8)))
	name := []byte("synthetic")
	copy(buf[:16], name)
	buf[16] = byte(w)
	buf[17] = byte(w >> 8)
	buf[18] = byte(w >> 16)
	buf[19] = byte(w >> 24)
	buf[20] = byte(h)
	buf[21] = byte(h >> 8)
	buf[22] = byte(h >> 16)
	buf[23] = byte(h >> 24)
	// Offsets
	off := uint32(40)
	putU32(buf[24:28], off)
	off += uint32(w * h)
	putU32(buf[28:32], off)
	off += uint32((w / 2) * (h / 2))
	putU32(buf[32:36], off)
	off += uint32((w / 4) * (h / 4))
	putU32(buf[36:40], off)
	// Fill all 4 mip levels with the flat color
	for i := 40; i < len(buf); i++ {
		buf[i] = r
	}
	return buf
}

func miptexLump(miptex []byte) []byte {
	// 4-byte count + one 4-byte offset
	out := make([]byte, 8)
	putU32(out[0:4], 1)
	putU32(out[4:8], 8)
	return append(out, miptex...)
}

// buildSyntheticLightmap returns a tiny constant RGB lightmap for each face.
// syntheticLightmapGrid is the Quake lightmap grid size in world units: the
// renderer computes each face's lightmap extent as
// ceil((maxUV-textureMin)/16)*16 in 16-unit blocks, so a face spanning
// 512 world units (the room's in-plane extent) yields 33x33 blocks.
const syntheticLightmapGrid = 16

// buildSyntheticLightmap emits a monochrome lightmap lump sized so every one
// of the room's faces has enough style-0 samples at its (sequential) offset.
// faces is the number of faces (6); level is the constant light intensity
// written to every sample. Returns the lump and the per-face byte offsets.
// The renderer reads the lump as monochrome (tree.LightingRGB=false), one
// byte per sample.
func buildSyntheticLightmap(faces int, level byte) ([]byte, []int) {
	const extentBlocks = (syntheticRoomMax - syntheticRoomMin) / syntheticLightmapGrid
	// smax = extentBlocks + 1 = 33, matching the renderer's extentU/16+1.
	samplesPerFace := (extentBlocks + 1) * (extentBlocks + 1)

	lump := make([]byte, faces*samplesPerFace)
	offsets := make([]int, faces)
	for i := 0; i < faces; i++ {
		offsets[i] = i * samplesPerFace
		for j := offsets[i]; j < offsets[i]+samplesPerFace; j++ {
			lump[j] = level
		}
	}
	return lump, offsets
}

// syntheticEntities emits a worldspawn + one info_player_start inside the room.
func syntheticEntities() string {
	sx, sy, sz := 0, 0, float64(syntheticGroundZ)+24
	return fmt.Sprintf(`{
"classname" "worldspawn"
"message" "Synthetic demo room (plan 22 no-assets fallback)"
"mapversion" "220"
"model" "*0"
}
{
"classname" "info_player_start"
"origin" "%d %d %d"
"angle" "0"
}
`, int(sx), int(sy), int(sz))
}

func putU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

// --- lump conversions for bsp.File (BSP29 16-bit indices) ---

func nodesAsDSNode(nodes []bsp.TreeNode) []bsp.DSNode {
	out := make([]bsp.DSNode, len(nodes))
	for i, n := range nodes {
		on := bsp.DSNode{
			PlaneNum:  n.PlaneNum,
			BoundsMin: b16(n.BoundsMin),
			BoundsMax: b16(n.BoundsMax),
			FirstFace: uint16(n.FirstFace),
			NumFaces:  uint16(n.NumFaces),
		}
		for side, c := range n.Children {
			if c.IsLeaf {
				on.Children[side] = int16(-(c.Index + 1))
			} else {
				on.Children[side] = int16(c.Index)
			}
		}
		out[i] = on
	}
	return out
}

func leafsAsDSLeaf(leafs []bsp.TreeLeaf) []bsp.DSLeaf {
	out := make([]bsp.DSLeaf, len(leafs))
	for i, l := range leafs {
		out[i] = bsp.DSLeaf{
			Contents:         l.Contents,
			VisOfs:           l.VisOfs,
			BoundsMin:        b16(l.BoundsMin),
			BoundsMax:        b16(l.BoundsMax),
			FirstMarkSurface: uint16(l.FirstMarkSurface),
			NumMarkSurfaces:  uint16(l.NumMarkSurfaces),
		}
	}
	return out
}

func facesAsDSFace(faces []bsp.TreeFace) []bsp.DSFace {
	out := make([]bsp.DSFace, len(faces))
	for i, f := range faces {
		out[i] = bsp.DSFace{
			PlaneNum:  int16(f.PlaneNum),
			Side:      int16(f.Side),
			FirstEdge: f.FirstEdge,
			NumEdges:  int16(f.NumEdges),
			Texinfo:   int16(f.Texinfo),
			Styles:    f.Styles,
			LightOfs:  f.LightOfs,
		}
	}
	return out
}

func edgesAsDSEdge(edges []bsp.TreeEdge) []bsp.DSEdge {
	out := make([]bsp.DSEdge, len(edges))
	for i, e := range edges {
		out[i] = bsp.DSEdge{V: [2]uint16{uint16(e.V[0]), uint16(e.V[1])}}
	}
	return out
}

func b16(v [3]float32) [3]int16 {
	return [3]int16{int16(v[0]), int16(v[1]), int16(v[2])}
}

// validateSyntheticTree performs cheap sanity checks on the generated BSP so
// a construction bug fails loudly at build time instead of at render time.
func validateSyntheticTree(tree *bsp.Tree) error {
	if len(tree.Models) == 0 {
		return fmt.Errorf("no world model")
	}
	if tree.Models[0].NumFaces == 0 {
		return fmt.Errorf("world model has no faces")
	}
	if len(tree.Faces) != int(tree.Models[0].NumFaces) {
		return fmt.Errorf("face count mismatch")
	}
	if len(tree.Nodes) == 0 {
		return fmt.Errorf("no BSP nodes")
	}
	if len(tree.Leafs) < 2 {
		return fmt.Errorf("need solid+empty leaves")
	}
	for _, n := range tree.Nodes {
		if n.PlaneNum < 0 || int(n.PlaneNum) >= len(tree.Planes) {
			return fmt.Errorf("node plane out of range")
		}
	}
	return nil
}

var _ = math.Abs // keep math import if unused in future edits
