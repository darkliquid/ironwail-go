package renderer

import (
	"encoding/binary"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
)

func TestWorldFaceTextureIndexRemapsMissingTextureToDummySlots(t *testing.T) {
	textureData := make([]byte, 4+4*4)
	binary.LittleEndian.PutUint32(textureData[:4], 4)
	for i := 0; i < 4; i++ {
		binary.LittleEndian.PutUint32(textureData[4+i*4:], uint32(0xffffffff))
	}

	tests := []struct {
		name  string
		flags int32
		want  int32
	}{
		{name: "lightmapped", flags: bsp.TexMissing, want: 4},
		{name: "special", flags: bsp.TexMissing | bsp.TexSpecial, want: 5},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := &bsp.Tree{
				TextureData: textureData,
				Texinfo: []bsp.Texinfo{
					{Miptex: 3, Flags: tc.flags},
				},
			}
			face := &bsp.TreeFace{Texinfo: 0}

			if got := worldFaceTextureIndex(tree, face); got != tc.want {
				t.Fatalf("worldFaceTextureIndex() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestWorldFaceFlagsTreatsMissingTextureAsOpaqueDummy(t *testing.T) {
	textureData := make([]byte, 8)
	binary.LittleEndian.PutUint32(textureData[:4], 1)
	binary.LittleEndian.PutUint32(textureData[4:], uint32(0xffffffff))
	tree := &bsp.Tree{
		TextureData: textureData,
		Texinfo: []bsp.Texinfo{
			{Miptex: 0, Flags: bsp.TexSpecial},
		},
	}
	face := &bsp.TreeFace{Texinfo: 0}
	textureMeta := []worldTextureMeta{{Type: model.TexTypeTele}}

	flags := worldFaceFlags(textureMeta, tree, face)
	if flags&model.SurfNoTexture == 0 {
		t.Fatalf("flags = %#x, want SurfNoTexture", flags)
	}
	if flags&model.SurfDrawTurb != 0 {
		t.Fatalf("flags = %#x, missing dummy texture must not be routed as turbulent liquid", flags)
	}
	if !shouldDrawGoGPUOpaqueWorldFace(WorldFace{NumIndices: 3, Flags: flags}) {
		t.Fatalf("missing dummy flags = %#x, want opaque world face", flags)
	}
}

// TestBuildWorldGeometry_NilTree tests handling of nil BSP tree.
func TestBuildWorldGeometry_NilTree(t *testing.T) {
	_, err := BuildWorldGeometry(nil)
	if err == nil {
		t.Fatal("Expected error for nil tree, got nil")
	}
}

// TestBuildWorldGeometry_NoModels tests handling of BSP with no models.
func TestBuildWorldGeometry_NoModels(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{},
	}

	_, err := BuildWorldGeometry(tree)
	if err == nil {
		t.Fatal("Expected error for BSP with no models, got nil")
	}
}

// TestBuildWorldGeometry_SimpleQuad tests geometry extraction for a simple quad face.
func TestBuildWorldGeometry_SimpleQuad(t *testing.T) {
	// Create a minimal BSP with one quad face (4 vertices)
	// This simulates a simple wall or floor polygon

	tree := &bsp.Tree{
		// World model with 1 face
		Models: []bsp.DModel{
			{
				BoundsMin: [3]float32{-100, -100, -100},
				BoundsMax: [3]float32{100, 100, 100},
				FirstFace: 0,
				NumFaces:  1,
			},
		},

		// One face with 4 edges (quad)
		Faces: []bsp.TreeFace{
			{
				PlaneNum:  0,
				Side:      0,
				FirstEdge: 0,
				NumEdges:  4,
				Texinfo:   0,
			},
		},

		// 4 edges forming a quad
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 3}},
			{V: [2]uint32{3, 0}},
		},

		// Surfedges reference the edges
		Surfedges: []int32{0, 1, 2, 3},

		// 4 vertices forming a quad (100x100 units on XY plane)
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{100, 0, 0}},
			{Point: [3]float32{100, 100, 0}},
			{Point: [3]float32{0, 100, 0}},
		},

		// One plane (Z-up)
		Planes: []bsp.DPlane{
			{
				Normal: [3]float32{0, 0, 1},
				Dist:   0,
				Type:   bsp.PlaneZ,
			},
		},
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}

	// Quad should produce 4 vertices
	if len(geom.Vertices) != 4 {
		t.Errorf("Expected 4 vertices, got %d", len(geom.Vertices))
	}

	// Quad should be triangulated into 2 triangles (6 indices)
	expectedIndices := 6
	if len(geom.Indices) != expectedIndices {
		t.Errorf("Expected %d indices (2 triangles), got %d",
			expectedIndices, len(geom.Indices))
	}

	// Should have 1 face metadata entry
	if len(geom.Faces) != 1 {
		t.Errorf("Expected 1 face, got %d", len(geom.Faces))
	}

	// Verify face metadata
	face := geom.Faces[0]
	if face.FirstIndex != 0 {
		t.Errorf("Expected FirstIndex=0, got %d", face.FirstIndex)
	}
	if face.NumIndices != 6 {
		t.Errorf("Expected NumIndices=6, got %d", face.NumIndices)
	}

	// Verify vertex positions match input
	expectedPositions := []([3]float32){
		{0, 0, 0},
		{100, 0, 0},
		{100, 100, 0},
		{0, 100, 0},
	}

	for i, expected := range expectedPositions {
		got := geom.Vertices[i].Position
		if got != expected {
			t.Errorf("Vertex[%d] position = %v, want %v", i, got, expected)
		}
	}

	// Verify normals are set (should be Z-up)
	expectedNormal := [3]float32{0, 0, 1}
	for i, v := range geom.Vertices {
		if v.Normal != expectedNormal {
			t.Errorf("Vertex[%d] normal = %v, want %v", i, v.Normal, expectedNormal)
		}
	}
}

func TestGoGPUWorldMaterialBindState_Update(t *testing.T) {
	textureA := new(wgpu.BindGroup)
	textureB := new(wgpu.BindGroup)
	lightmapA := new(wgpu.BindGroup)
	fullbrightA := new(wgpu.BindGroup)

	var state gogpuWorldMaterialBindState
	setTexture, setLightmap, setFullbright := state.update(textureA, lightmapA, fullbrightA)
	if !setTexture || !setLightmap || !setFullbright {
		t.Fatalf("first material bind update = (%v, %v, %v), want all true", setTexture, setLightmap, setFullbright)
	}

	setTexture, setLightmap, setFullbright = state.update(textureA, lightmapA, fullbrightA)
	if setTexture || setLightmap || setFullbright {
		t.Fatalf("identical material bind update = (%v, %v, %v), want all false", setTexture, setLightmap, setFullbright)
	}

	setTexture, setLightmap, setFullbright = state.update(textureB, lightmapA, fullbrightA)
	if !setTexture || setLightmap || setFullbright {
		t.Fatalf("texture-only change update = (%v, %v, %v), want (true, false, false)", setTexture, setLightmap, setFullbright)
	}

	state.invalidate()
	setTexture, setLightmap, setFullbright = state.update(textureB, lightmapA, fullbrightA)
	if !setTexture || !setLightmap || !setFullbright {
		t.Fatalf("post-invalidate update = (%v, %v, %v), want all true", setTexture, setLightmap, setFullbright)
	}
}

func TestBuildWorldGeometry_DerivesFaceMetadataAndTexcoords(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{FirstFace: 0, NumFaces: 1},
		},
		Faces: []bsp.TreeFace{
			{
				PlaneNum:  0,
				FirstEdge: 0,
				NumEdges:  4,
				Texinfo:   0,
				LightOfs:  64,
				Styles:    [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
			},
		},
		Texinfo: []bsp.Texinfo{
			{
				Vecs: [2][4]float32{
					{1, 0, 0, 0},
					{0, 1, 0, 0},
				},
				Miptex: 3,
				Flags:  bsp.TexSpecial | bsp.TexMissing,
			},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 3}},
			{V: [2]uint32{3, 0}},
		},
		Surfedges: []int32{0, 1, 2, 3},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{16, 0, 0}},
			{Point: [3]float32{16, 16, 0}},
			{Point: [3]float32{0, 16, 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}},
		},
		Lighting: append(make([]byte, 64), 128, 128, 128, 128),
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}
	if len(geom.Faces) != 1 {
		t.Fatalf("Expected 1 face, got %d", len(geom.Faces))
	}
	face := geom.Faces[0]
	if face.TextureIndex != 3 {
		t.Fatalf("TextureIndex = %d, want 3", face.TextureIndex)
	}
	if face.LightmapIndex != 0 {
		t.Fatalf("LightmapIndex = %d, want 0", face.LightmapIndex)
	}
	wantFlags := deriveWorldFaceFlags(classifyWorldTextureName(""), bsp.TexSpecial|bsp.TexMissing)
	if face.Flags != wantFlags {
		t.Fatalf("Flags = %#x, want %#x", face.Flags, wantFlags)
	}

	if geom.Vertices[1].TexCoord != ([2]float32{16, 0}) {
		t.Fatalf("TexCoord[1] = %v, want [16 0]", geom.Vertices[1].TexCoord)
	}
	wantLightmapCoord := [2]float32{1.5 / worldLightmapPageSize, 0.5 / worldLightmapPageSize}
	gotLightmapCoord := geom.Vertices[1].LightmapCoord
	if gotLightmapCoord[0] != wantLightmapCoord[0] || gotLightmapCoord[1] != wantLightmapCoord[1] {
		t.Fatalf("LightmapCoord[1] = %v, want %v", gotLightmapCoord, wantLightmapCoord)
	}
}

func TestBuildWorldGeometry_PopulatesLeafFacesFromMarkSurfaces(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{FirstFace: 0, NumFaces: 1},
		},
		Faces: []bsp.TreeFace{
			{PlaneNum: 0, FirstEdge: 0, NumEdges: 3, Texinfo: 0},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 0}},
		},
		Surfedges: []int32{0, 1, 2},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{1, 0, 0}},
			{Point: [3]float32{0, 1, 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}, Type: bsp.PlaneZ},
		},
		Leafs: []bsp.TreeLeaf{
			{},
			{FirstMarkSurface: 0, NumMarkSurfaces: 1},
		},
		MarkSurfaces: []int{0},
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}
	if len(geom.LeafFaces) != len(tree.Leafs) {
		t.Fatalf("LeafFaces len = %d, want %d", len(geom.LeafFaces), len(tree.Leafs))
	}
	if len(geom.LeafFaces[1]) != 1 || geom.LeafFaces[1][0] != 0 {
		t.Fatalf("LeafFaces[1] = %v, want [0]", geom.LeafFaces[1])
	}
}

func TestExtractFaceVertices_UsesHighPrecisionLightmapExtents(t *testing.T) {
	tree := &bsp.Tree{
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}},
		},
		Texinfo: []bsp.Texinfo{
			{
				Miptex: 0,
				Vecs: [2][4]float32{
					{0.57220185, -0.10834341, 1.6845309, 1.3638687},
					{0, 0, 0, 0},
				},
			},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 0}},
		},
		Surfedges: []int32{0, 1, 2},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{4026.5183, 32887.621, 131090.9}},
			{Point: [3]float32{4080.9375, 32887.621, 131090.9}},
			{Point: [3]float32{4026.5183, 32937.996, 131090.9}},
		},
		Lighting: make([]byte, 8),
	}
	face := &bsp.TreeFace{
		PlaneNum:  0,
		FirstEdge: 0,
		NumEdges:  3,
		Texinfo:   0,
		LightOfs:  0,
		Styles:    [4]uint8{0, 255, 255, 255},
	}

	allocator, err := surfacepkg.NewLightmapAllocator(worldLightmapPageSize, worldLightmapPageSize, false)
	if err != nil {
		t.Fatalf("surfacepkg.NewLightmapAllocator failed: %v", err)
	}
	var pages []WorldLightmapPage
	_, surface, err := extractFaceVertices(tree, face, allocator, &pages)
	if err != nil {
		t.Fatalf("extractFaceVertices failed: %v", err)
	}
	if surface == nil {
		t.Fatal("expected lightmap surface")
	}
	if len(pages) != 1 || len(pages[0].Surfaces) != 1 {
		t.Fatalf("unexpected lightmap pages: %+v", pages)
	}
	if got := pages[0].Surfaces[0].Width; got != 5 {
		t.Fatalf("lightmap width = %d, want 5; float32 precision would incorrectly shrink this face to 4 texels", got)
	}
}

func TestSelectVisibleWorldFaces_UsesLeafPVS(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{{VisLeafs: 3}},
		Leafs: []bsp.TreeLeaf{
			{},
			{Contents: bsp.ContentsEmpty, VisOfs: 0},
			{Contents: bsp.ContentsEmpty, VisOfs: 1},
			{Contents: bsp.ContentsEmpty, VisOfs: 2},
		},
		Visibility: []byte{
			0x03,
			0x06,
			0x04,
		},
		Nodes: []bsp.TreeNode{
			{
				PlaneNum: 0,
				Children: [2]bsp.TreeChild{
					{IsLeaf: true, Index: 1},
					{IsLeaf: true, Index: 2},
				},
			},
		},
		Planes: []bsp.DPlane{
			{Type: bsp.PlaneX, Dist: 0},
		},
	}
	allFaces := []WorldFace{
		{FirstIndex: 0, NumIndices: 3},
		{FirstIndex: 3, NumIndices: 3},
		{FirstIndex: 6, NumIndices: 3},
	}
	leafFaces := [][]int{
		nil,
		{0},
		{1},
		{2},
	}

	visible := selectVisibleWorldFaces(tree, allFaces, leafFaces, [3]float32{1, 0, 0})
	if len(visible) != 2 {
		t.Fatalf("visible len = %d, want 2", len(visible))
	}
	if visible[0].FirstIndex != 0 || visible[1].FirstIndex != 3 {
		t.Fatalf("visible faces = %+v, want first two faces", visible)
	}

	visible = selectVisibleWorldFaces(tree, allFaces, leafFaces, [3]float32{-1, 0, 0})
	if len(visible) != 2 {
		t.Fatalf("visible len from leaf2 = %d, want 2", len(visible))
	}
	if visible[0].FirstIndex != 3 || visible[1].FirstIndex != 6 {
		t.Fatalf("visible faces from leaf2 = %+v, want faces 1 and 2", visible)
	}
}

func TestWorldVisibilityScratch_ReusesStorageAndPreservesOrder(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{{VisLeafs: 3}},
		Leafs: []bsp.TreeLeaf{
			{},
			{Contents: bsp.ContentsEmpty, VisOfs: 0},
			{Contents: bsp.ContentsEmpty, VisOfs: 1},
			{Contents: bsp.ContentsEmpty, VisOfs: 2},
		},
		Visibility: []byte{
			0x03,
			0x06,
			0x04,
		},
		Nodes: []bsp.TreeNode{{
			PlaneNum: 0,
			Children: [2]bsp.TreeChild{
				{IsLeaf: true, Index: 1},
				{IsLeaf: true, Index: 2},
			},
		}},
		Planes: []bsp.DPlane{{Type: bsp.PlaneX, Dist: 0}},
	}
	allFaces := []WorldFace{
		{FirstIndex: 0, NumIndices: 3},
		{FirstIndex: 3, NumIndices: 3},
		{FirstIndex: 6, NumIndices: 3},
	}
	leafFaces := [][]int{
		nil,
		{1, 0},
		{1},
		{2},
	}

	var scratch worldVisibilityScratch
	first := scratch.selectVisibleWorldFaces(tree, allFaces, leafFaces, [3]float32{1, 0, 0})
	if len(first) != 2 {
		t.Fatalf("first visible len = %d, want 2", len(first))
	}
	if first[0].FirstIndex != 0 || first[1].FirstIndex != 3 {
		t.Fatalf("first visible faces = %+v, want first two faces in face-index order", first)
	}
	marksPtr := &scratch.marks[0]
	facesPtr := &scratch.faces[:cap(scratch.faces)][0]

	second := scratch.selectVisibleWorldFaces(tree, allFaces, leafFaces, [3]float32{-1, 0, 0})
	if len(second) != 2 {
		t.Fatalf("second visible len = %d, want 2", len(second))
	}
	if second[0].FirstIndex != 3 || second[1].FirstIndex != 6 {
		t.Fatalf("second visible faces = %+v, want faces 1 and 2 in face-index order", second)
	}
	if &scratch.marks[0] != marksPtr {
		t.Fatal("scratch marks backing array was replaced instead of reused")
	}
	if &scratch.faces[:cap(scratch.faces)][0] != facesPtr {
		t.Fatal("scratch face backing array was replaced instead of reused")
	}
}

func TestWorldDepthAttachmentForViewNil(t *testing.T) {
	if got := worldDepthAttachmentForView(nil); got != nil {
		t.Fatalf("worldDepthAttachmentForView(nil) = %#v, want nil", got)
	}
}

// TestBuildWorldGeometry_Triangle tests triangulation for a triangle face.
func TestBuildWorldGeometry_Triangle(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{
				FirstFace: 0,
				NumFaces:  1,
			},
		},
		Faces: []bsp.TreeFace{
			{
				PlaneNum:  0,
				Side:      0,
				FirstEdge: 0,
				NumEdges:  3, // Triangle
				Texinfo:   0,
			},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 0}},
		},
		Surfedges: []int32{0, 1, 2},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{10, 0, 0}},
			{Point: [3]float32{5, 10, 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 0},
		},
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}

	// Triangle: 3 vertices, 3 indices (1 triangle)
	if len(geom.Vertices) != 3 {
		t.Errorf("Expected 3 vertices, got %d", len(geom.Vertices))
	}
	if len(geom.Indices) != 3 {
		t.Errorf("Expected 3 indices, got %d", len(geom.Indices))
	}
}

// TestBuildWorldGeometry_Hexagon tests fan triangulation for a 6-sided polygon.
func TestBuildWorldGeometry_Hexagon(t *testing.T) {
	// Hexagon (6 vertices) should triangulate into 4 triangles (12 indices)
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{
				FirstFace: 0,
				NumFaces:  1,
			},
		},
		Faces: []bsp.TreeFace{
			{
				PlaneNum:  0,
				Side:      0,
				FirstEdge: 0,
				NumEdges:  6, // Hexagon
				Texinfo:   0,
			},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 3}},
			{V: [2]uint32{3, 4}},
			{V: [2]uint32{4, 5}},
			{V: [2]uint32{5, 0}},
		},
		Surfedges: []int32{0, 1, 2, 3, 4, 5},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{10, 0, 0}},
			{Point: [3]float32{5, 8, 0}},
			{Point: [3]float32{-5, 8, 0}},
			{Point: [3]float32{-10, 0, 0}},
			{Point: [3]float32{-5, -8, 0}},
			{Point: [3]float32{5, -8, 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 0},
		},
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}

	// Hexagon: 6 vertices
	if len(geom.Vertices) != 6 {
		t.Errorf("Expected 6 vertices, got %d", len(geom.Vertices))
	}

	// Hexagon: (6-2) = 4 triangles = 12 indices
	expectedIndices := 12
	if len(geom.Indices) != expectedIndices {
		t.Errorf("Expected %d indices (4 triangles), got %d",
			expectedIndices, len(geom.Indices))
	}
}

// TestBuildWorldGeometry_MultipleF aces tests processing multiple faces.
func TestBuildWorldGeometry_MultipleFaces(t *testing.T) {
	// Two quads (8 vertices total, 12 indices)
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{
				FirstFace: 0,
				NumFaces:  2,
			},
		},
		Faces: []bsp.TreeFace{
			// Face 0: quad
			{
				PlaneNum:  0,
				Side:      0,
				FirstEdge: 0,
				NumEdges:  4,
				Texinfo:   0,
			},
			// Face 1: quad
			{
				PlaneNum:  1,
				Side:      0,
				FirstEdge: 4,
				NumEdges:  4,
				Texinfo:   0,
			},
		},
		Edges: []bsp.TreeEdge{
			// Quad 1
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 3}},
			{V: [2]uint32{3, 0}},
			// Quad 2
			{V: [2]uint32{4, 5}},
			{V: [2]uint32{5, 6}},
			{V: [2]uint32{6, 7}},
			{V: [2]uint32{7, 4}},
		},
		Surfedges: []int32{0, 1, 2, 3, 4, 5, 6, 7},
		Vertexes: []bsp.DVertex{
			// Quad 1
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{10, 0, 0}},
			{Point: [3]float32{10, 10, 0}},
			{Point: [3]float32{0, 10, 0}},
			// Quad 2
			{Point: [3]float32{20, 0, 0}},
			{Point: [3]float32{30, 0, 0}},
			{Point: [3]float32{30, 10, 0}},
			{Point: [3]float32{20, 10, 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 0},
			{Normal: [3]float32{0, 0, 1}, Dist: 0},
		},
	}

	geom, err := BuildWorldGeometry(tree)
	if err != nil {
		t.Fatalf("BuildWorldGeometry failed: %v", err)
	}

	// 2 quads = 8 vertices
	if len(geom.Vertices) != 8 {
		t.Errorf("Expected 8 vertices, got %d", len(geom.Vertices))
	}

	// 2 quads = 4 triangles = 12 indices
	if len(geom.Indices) != 12 {
		t.Errorf("Expected 12 indices, got %d", len(geom.Indices))
	}

	// 2 face metadata entries
	if len(geom.Faces) != 2 {
		t.Errorf("Expected 2 faces, got %d", len(geom.Faces))
	}
}

// TestUploadWorld tests world geometry upload to renderer.
func TestUploadWorld(t *testing.T) {
	// Create a test renderer (headless)
	cfg := DefaultConfig()
	cfg.Width = 800
	cfg.Height = 600
	// Note: Cannot create headless renderer with gogpu backend currently
	// This test may fail if GPU is not available

	r, err := NewWithConfig(cfg)
	if err != nil {
		t.Skipf("Skipping test: cannot create renderer: %v", err)
	}
	defer r.Shutdown()

	// Create minimal BSP
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{
				FirstFace: 0,
				NumFaces:  1,
			},
		},
		Faces: []bsp.TreeFace{
			{
				PlaneNum:  0,
				Side:      0,
				FirstEdge: 0,
				NumEdges:  3,
				Texinfo:   0,
			},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 0}},
		},
		Surfedges: []int32{0, 1, 2},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{10, 0, 0}},
			{Point: [3]float32{5, 10, 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: [3]float32{0, 0, 1}, Dist: 0},
		},
	}

	// Upload world data
	err = r.UploadWorld(tree)
	if err != nil {
		t.Fatalf("UploadWorld failed: %v", err)
	}

	// Verify world data is stored
	worldData := r.WorldData()
	if worldData == nil {
		t.Fatal("World data not stored after upload")
	}

	if worldData.TotalVertices != 3 {
		t.Errorf("Expected 3 vertices, got %d", worldData.TotalVertices)
	}

	if worldData.TotalIndices != 3 {
		t.Errorf("Expected 3 indices, got %d", worldData.TotalIndices)
	}

	if worldData.TotalFaces != 1 {
		t.Errorf("Expected 1 face, got %d", worldData.TotalFaces)
	}

	// Test ClearWorld
	r.ClearWorld()
	worldData = r.WorldData()
	if worldData != nil {
		t.Error("World data not cleared")
	}
}
