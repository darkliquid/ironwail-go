package renderer

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
)

// TestBuildWorldGeometry_SyntheticRoomShape mirrors the topology BuildSyntheticMap
// produces (server package): a 6-quad box room, one flat miptex, constant
// lightmap, node/leaf tree. This proves the renderer can build geometry from
// that shape — the plan 22 no-assets walkthrough gate.
func TestBuildWorldGeometry_SyntheticRoomShape(t *testing.T) {
	// 8 corners of the room (-256..256 x, -256..256 y, 0..192 z)
	c := func(x, y, z float32) [3]float32 { return [3]float32{x, y, z} }
	verts := []bsp.DVertex{
		{Point: c(-256, -256, 0)}, {Point: c(256, -256, 0)},
		{Point: c(256, 256, 0)}, {Point: c(-256, 256, 0)},
		{Point: c(-256, -256, 192)}, {Point: c(256, -256, 192)},
		{Point: c(256, 256, 192)}, {Point: c(-256, 256, 192)},
	}
	// 6 quads (floor, ceiling, -X, +X, -Y, +Y) with shared edges deduped.
	quads := [][4]int{
		{0, 1, 2, 3}, // floor
		{4, 7, 6, 5}, // ceiling
		{0, 3, 7, 4}, // -X wall
		{1, 5, 6, 2}, // +X wall
		{3, 2, 6, 7}, // +Y wall
		{0, 4, 5, 1}, // -Y wall
	}
	var edges []bsp.TreeEdge
	var surfedges []int32
	faces := make([]bsp.TreeFace, 6)
	for qi, quad := range quads {
		firstEdge := int32(len(edges))
		for vi := 0; vi < 4; vi++ {
			a := quad[vi]
			b := quad[(vi+1)%4]
			// Register a unique edge per directed pair (dense list).
			edges = append(edges, bsp.TreeEdge{V: [2]uint32{uint32(a), uint32(b)}})
			surfedges = append(surfedges, firstEdge+int32(vi))
		}
		faces[qi] = bsp.TreeFace{
			PlaneNum:  int32(qi),
			FirstEdge: firstEdge,
			NumEdges:  4,
			Texinfo:   int32(qi),
			Styles:    [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
			LightOfs:  0,
		}
	}

	// One flat miptex lump (1 texture, offset 8, 64x64 solid color).
	miptex := make([]byte, 40+64*64)
	copy(miptex[:16], []byte("synthetic"))
	miptex[16], miptex[20] = 64, 64
	putTexU32(miptex[24:28], 40)
	putTexU32(miptex[28:32], 40+64*64)
	for i := 40; i < len(miptex); i++ {
		miptex[i] = 0x8f
	}
	textureData := make([]byte, 8)
	putTexU32(textureData[:4], 1)
	putTexU32(textureData[4:8], 8)
	textureData = append(textureData, miptex...)

	// 6 planes + texinfos (axis-aligned).
	planes := []bsp.DPlane{
		{Normal: [3]float32{0, 0, 1}, Dist: 0, Type: 2},
		{Normal: [3]float32{0, 0, -1}, Dist: 192, Type: 2},
		{Normal: [3]float32{-1, 0, 0}, Dist: 256, Type: 1},
		{Normal: [3]float32{1, 0, 0}, Dist: 256, Type: 1},
		{Normal: [3]float32{0, -1, 0}, Dist: 256, Type: 0},
		{Normal: [3]float32{0, 1, 0}, Dist: 256, Type: 0},
	}
	texinfo := make([]bsp.Texinfo, 6)
	for i := range texinfo {
		texinfo[i] = bsp.Texinfo{Miptex: 0}
	}

	tree := &bsp.Tree{
		Version:     29,
		Entities:    []byte(syntheticTestEntities()),
		TextureData: textureData,
		Lighting:    make([]byte, 6*4*4*3), // tiny constant lightmap
		Planes:      planes,
		Vertexes:    verts,
		Texinfo:     texinfo,
		Edges:       edges,
		Surfedges:   surfedges,
		Faces:       faces,
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid, VisOfs: -1},
			{Contents: bsp.ContentsSolid, VisOfs: -1},
			{Contents: bsp.ContentsEmpty, VisOfs: -1},
			{Contents: bsp.ContentsSolid, VisOfs: -1},
		},
		Nodes: []bsp.TreeNode{
			{PlaneNum: 0, Children: [2]bsp.TreeChild{{IsLeaf: false, Index: 1}, {IsLeaf: true, Index: 1}}},
			{PlaneNum: 1, Children: [2]bsp.TreeChild{{IsLeaf: true, Index: 3}, {IsLeaf: true, Index: 2}}},
		},
		Models: []bsp.DModel{
			{BoundsMin: [3]float32{-256, -256, 0}, BoundsMax: [3]float32{256, 256, 192}, HeadNode: [bsp.MaxMapHulls]int32{0, 0, 0}, FirstFace: 0, NumFaces: 6},
		},
		NumTextures: 1,
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry(synthetic room): %v", err)
	}
	if geom == nil {
		t.Fatal("nil geometry")
	}
	if len(geom.Faces) != 6 {
		t.Fatalf("expected 6 render faces, got %d", len(geom.Faces))
	}
	if len(geom.Vertices) < 6*3 {
		t.Fatalf("expected >=18 vertices (6 quads triangulated), got %d", len(geom.Vertices))
	}
	if len(geom.Indices) < 6*6 {
		t.Fatalf("expected >=36 indices (6 quads x 2 tris x 3), got %d", len(geom.Indices))
	}
}

func syntheticTestEntities() string {
	return `{
"classname" "worldspawn"
}
{
"classname" "info_player_start"
"origin" "0 0 24"
"angle" "0"
}
`
}

func putTexU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}
