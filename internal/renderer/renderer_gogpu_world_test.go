package renderer

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/testutil"
	"github.com/gogpu/wgpu"

	surfacepkg "github.com/darkliquid/ironwail-go/internal/renderer/surface"
	"github.com/darkliquid/ironwail-go/pkg/types"
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
				BoundsMin: types.Vec3{X: -100, Y: -100, Z: -100},
				BoundsMax: types.Vec3{X: 100, Y: 100, Z: 100},
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
			{Point: types.Vec3{X: 0, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 100, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 100, Y: 100, Z: 0}},
			{Point: types.Vec3{X: 0, Y: 100, Z: 0}},
		},

		// One plane (Z-up)
		Planes: []bsp.DPlane{
			{
				Normal: types.Vec3{X: 0, Y: 0, Z: 1},
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
	expectedPositions := []types.Vec3{
		{X: 0, Y: 0, Z: 0},
		{X: 100, Y: 0, Z: 0},
		{X: 100, Y: 100, Z: 0},
		{X: 0, Y: 100, Z: 0},
	}

	for i, expected := range expectedPositions {
		got := geom.Vertices[i].Position
		if got != expected {
			t.Errorf("Vertex[%d] position = %v, want %v", i, got, expected)
		}
	}

	// Verify normals are set (should be Z-up)
	expectedNormal := types.Vec3{X: 0, Y: 0, Z: 1}
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
			{Point: types.Vec3{X: 0, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 16, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 16, Y: 16, Z: 0}},
			{Point: types.Vec3{X: 0, Y: 16, Z: 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
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
	// Lightmap V coordinate is rescaled for the vertically-stacked lightmap
	// texture with 1px padding. With 1 page: totalHeight = (1024 + 2) = 1026.
	// V = (0.5 / 1024) * (1024 / 1026) = 0.5 / 1026.
	// U is unchanged (width is not padded).
	wantLightmapCoord := [2]float32{1.5 / worldLightmapPageSize, 0.5 / float32(worldLightmapPageSize+2)}
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
			{Point: types.Vec3{X: 0, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 1, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 0, Y: 1, Z: 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Type: bsp.PlaneZ},
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
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}},
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
			{Point: types.Vec3{X: 4026.5183, Y: 32887.621, Z: 131090.9}},
			{Point: types.Vec3{X: 4080.9375, Y: 32887.621, Z: 131090.9}},
			{Point: types.Vec3{X: 4026.5183, Y: 32937.996, Z: 131090.9}},
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

	visible := selectVisibleWorldFaces(tree, allFaces, leafFaces, types.Vec3{X: 1, Y: 0, Z: 0})
	if len(visible) != 2 {
		t.Fatalf("visible len = %d, want 2", len(visible))
	}
	if visible[0].FirstIndex != 0 || visible[1].FirstIndex != 3 {
		t.Fatalf("visible faces = %+v, want first two faces", visible)
	}

	visible = selectVisibleWorldFaces(tree, allFaces, leafFaces, types.Vec3{X: -1, Y: 0, Z: 0})
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
	first := scratch.selectVisibleWorldFaces(tree, allFaces, leafFaces, types.Vec3{X: 1, Y: 0, Z: 0})
	if len(first) != 2 {
		t.Fatalf("first visible len = %d, want 2", len(first))
	}
	if first[0].FirstIndex != 0 || first[1].FirstIndex != 3 {
		t.Fatalf("first visible faces = %+v, want first two faces in face-index order", first)
	}
	marksPtr := &scratch.marks[0]
	facesPtr := &scratch.faces[:cap(scratch.faces)][0]

	second := scratch.selectVisibleWorldFaces(tree, allFaces, leafFaces, types.Vec3{X: -1, Y: 0, Z: 0})
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
			{Point: types.Vec3{X: 0, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 10, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 5, Y: 10, Z: 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0},
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
			{Point: types.Vec3{X: 10, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 5, Y: 8, Z: 0}},
			{Point: types.Vec3{X: -5, Y: 8, Z: 0}},
			{Point: types.Vec3{X: -10, Y: 0, Z: 0}},
			{Point: types.Vec3{X: -5, Y: -8, Z: 0}},
			{Point: types.Vec3{X: 5, Y: -8, Z: 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0},
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

// TestBuildWorldGeometry_MultipleFaces tests processing multiple faces.
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
			{Point: types.Vec3{X: 0, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 10, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 10, Y: 10, Z: 0}},
			{Point: types.Vec3{X: 0, Y: 10, Z: 0}},
			// Quad 2
			{Point: types.Vec3{X: 20, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 30, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 30, Y: 10, Z: 0}},
			{Point: types.Vec3{X: 20, Y: 10, Z: 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0},
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0},
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
			{Point: types.Vec3{X: 0, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 10, Y: 0, Z: 0}},
			{Point: types.Vec3{X: 5, Y: 10, Z: 0}},
		},
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0},
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

func TestGogpuWorldLightmapArrayBindGroupForFaceLitWaterFallback(t *testing.T) {
	fallbackBG := &wgpu.BindGroup{}
	liquidFaceNoLightmap := WorldFace{
		LightmapIndex: -1,
		Flags:         model.SurfDrawTurb | model.SurfDrawWater,
	}

	bg, litWater := gogpuWorldLightmapArrayBindGroupForFace(liquidFaceNoLightmap, nil, fallbackBG, true)
	if bg != fallbackBG {
		t.Errorf("got bind group %v, want fallbackBG %v", bg, fallbackBG)
	}
	if litWater != 1 {
		t.Errorf("got litWater = %v, want 1 for lit water liquid face without lightmap", litWater)
	}

	bgDisabled, litWaterDisabled := gogpuWorldLightmapArrayBindGroupForFace(liquidFaceNoLightmap, nil, fallbackBG, false)
	if bgDisabled != fallbackBG {
		t.Errorf("got bind group %v, want fallbackBG %v", bgDisabled, fallbackBG)
	}
	if litWaterDisabled != 0 {
		t.Errorf("got litWater = %v, want 0 when hasLitWater is false", litWaterDisabled)
	}
}

func TestQbj2StartWaterVisibility(t *testing.T) {
	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil || quakeDir == "" {
		t.Skip("QUAKE_DIR not set")
	}
	bspPath := filepath.Join(quakeDir, "qbj2", "maps", "start.bsp")
	f, err := os.Open(bspPath)
	if err != nil {
		t.Skipf("qbj2 start.bsp not found: %v", err)
	}
	defer func() { _ = f.Close() }()

	tree, err := bsp.LoadTree(f)
	testutil.AssertNoError(t, err)

	geom, err := BuildWorldGeometry(tree)
	testutil.AssertNoError(t, err)

	spawnPos := types.Vec3{X: -256.0, Y: -2576.0, Z: -2120.0}
	visibleFaces := selectVisibleWorldFaces(geom.Tree, geom.Faces, geom.LeafFaces, spawnPos)
	t.Logf("Total faces: %d, Visible faces at spawn %v: %d", len(geom.Faces), spawnPos, len(visibleFaces))

	liquidAlpha := worldLiquidAlphaSettingsForGeometry(geom)
	t.Logf("Liquid alpha settings: water=%.2f lava=%.2f slime=%.2f tele=%.2f", liquidAlpha.water, liquidAlpha.lava, liquidAlpha.slime, liquidAlpha.tele)

	var skyCount, translucentLiquidCount, opaqueCount, opaqueLiquidCount, alphaTestCount int
	for _, face := range visibleFaces {
		switch {
		case shouldDrawGoGPUSkyWorldFace(face):
			skyCount++
		case shouldDrawGoGPUTranslucentLiquidFace(face, liquidAlpha):
			translucentLiquidCount++
		case shouldDrawGoGPUOpaqueWorldFace(face):
			opaqueCount++
		case shouldDrawGoGPUAlphaTestWorldFace(face):
			alphaTestCount++
		case shouldDrawGoGPUOpaqueLiquidFace(face, liquidAlpha):
			opaqueLiquidCount++
		}
	}
	t.Logf("Classification: sky=%d, translucentLiquid=%d, opaque=%d, opaqueLiquid=%d, alphaTest=%d",
		skyCount, translucentLiquidCount, opaqueCount, opaqueLiquidCount, alphaTestCount)
}

func TestGoGPUWorldBatchCacheImmutability(t *testing.T) {
	r := &Renderer{}
	liquidAlpha := worldLiquidAlphaSettings{water: 0.5, lava: 1, slime: 1, tele: 1}

	batchedIndices := []uint32{0, 1, 2, 3, 4, 5}
	opaqueBatches := []gogpuWorldFaceBatch{{firstIndex: 0, numIndices: 3}}
	alphaBatches := []gogpuWorldFaceBatch{{firstIndex: 3, numIndices: 3}}

	r.storeGoGPUWorldBatchCacheEntry(1, liquidAlpha, 2, nil, nil, batchedIndices, opaqueBatches, alphaBatches, nil)

	entry := r.gogpuWorldBatchCacheEntry(1, liquidAlpha)
	if entry == nil {
		t.Fatal("expected cache entry to be found")
	}
	if len(entry.indices) != 6 {
		t.Fatalf("expected entry.indices length 6, got %d", len(entry.indices))
	}
	for i, idx := range batchedIndices {
		if entry.indices[i] != idx {
			t.Errorf("entry.indices[%d] = %d, want %d", i, entry.indices[i], idx)
		}
	}
}

func TestWaterPVSLeafExpansion(t *testing.T) {
	// Build a minimal BSP tree with 3 leaves:
	// Leaf 0: Solid (unused)
	// Leaf 1: Empty room containing ground and water surface (face 0: ground, face 1: water)
	// Leaf 2: Underwater pool containing submerged floor (face 2: pool floor, face 1: water)
	// Visibility: Leaf 1 PVS only sees Leaf 1 (simulating opaque vis where Leaf 2 is not in Leaf 1 PVS).
	// With water PVS expansion, selecting visible faces from Leaf 1 must also include face 2 (pool floor).
	tree := &bsp.Tree{
		Version: bsp.BSPVersion,
		Planes: []bsp.DPlane{
			{Normal: types.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0, Type: bsp.PlaneZ},
		},
		Leafs: []bsp.TreeLeaf{
			{Contents: bsp.ContentsSolid},
			{Contents: bsp.ContentsEmpty, VisOfs: 0},
			{Contents: bsp.ContentsWater, VisOfs: 1},
		},
		Nodes: []bsp.TreeNode{
			{PlaneNum: 0, Children: [2]bsp.TreeChild{
				{IsLeaf: true, Index: 1},
				{IsLeaf: true, Index: 2},
			}},
		},
		Visibility: []byte{
			0b00000001, // Leaf 1 sees only Leaf 1 (bit 0)
			0b00000010, // Leaf 2 sees only Leaf 2 (bit 1)
		},
	}

	allFaces := []WorldFace{
		{FirstIndex: 0, NumIndices: 6, TextureIndex: 0, Flags: 0},                                         // 0: Ground (opaque)
		{FirstIndex: 6, NumIndices: 6, TextureIndex: 1, Flags: model.SurfDrawTurb | model.SurfDrawWater}, // 1: Water surface (turbulent)
		{FirstIndex: 12, NumIndices: 6, TextureIndex: 0, Flags: 0},                                        // 2: Submerged floor (opaque)
	}

	leafFaces := [][]int{
		{},     // Leaf 0 (solid)
		{0, 1}, // Leaf 1 (empty room): ground + water surface
		{1, 2}, // Leaf 2 (underwater): water surface + submerged floor
	}

	// Camera is in Leaf 1 (Z = 50 > 0)
	cameraPos := types.Vec3{X: 0, Y: 0, Z: 50}
	visible := selectVisibleWorldFaces(tree, allFaces, leafFaces, cameraPos)

	// Verify that all 3 faces are visible (ground, water surface, and submerged floor)
	if len(visible) != 3 {
		t.Fatalf("selectVisibleWorldFaces returned %d faces, want 3 (including submerged floor)", len(visible))
	}
}

func TestStartBSPWaterAndSlimeParityDiagnosis(t *testing.T) {
	quakeDir, err := testutil.LocateQuakeDir()
	if err != nil || quakeDir == "" {
		t.Skip("QUAKE_DIR not set")
	}
	vfs := fs.NewFileSystem()
	err = vfs.Init(quakeDir, "id1")
	if err != nil {
		t.Skipf("vfs.Init failed: %v", err)
	}
	defer vfs.Close()

	data, err := vfs.LoadFile("maps/start.bsp")
	if err != nil {
		t.Skipf("maps/start.bsp not found: %v", err)
	}

	tree, err := bsp.LoadTree(bytes.NewReader(data))
	testutil.AssertNoError(t, err)

	geom, err := BuildWorldGeometry(tree)
	testutil.AssertNoError(t, err)

	t.Logf("start.bsp has %d leafs, %d faces, %d marksurfaces", len(tree.Leafs), len(geom.Faces), len(tree.MarkSurfaces))

	// Find water and slime leaves
	var waterLeafs, slimeLeafs, emptyLeafs []int
	for i := 1; i < len(tree.Leafs); i++ {
		switch tree.Leafs[i].Contents {
		case bsp.ContentsWater:
			waterLeafs = append(waterLeafs, i)
		case bsp.ContentsSlime:
			slimeLeafs = append(slimeLeafs, i)
		case bsp.ContentsEmpty:
			emptyLeafs = append(emptyLeafs, i)
		}
	}
	t.Logf("start.bsp leafs by content: %d water, %d slime, %d empty", len(waterLeafs), len(slimeLeafs), len(emptyLeafs))

	for _, leafIdx := range waterLeafs {
		leaf := &tree.Leafs[leafIdx]
		leafPVS := tree.LeafPVS(leaf)
		visEmpty := 0
		for _, eIdx := range emptyLeafs {
			if leafVisibleInMask(leafPVS, eIdx-1) {
				visEmpty++
			}
		}
		t.Logf("Water leaf %d (bounds %v..%v): sees %d/%d empty leaves, visOfs=%d",
			leafIdx, leaf.BoundsMin, leaf.BoundsMax, visEmpty, len(emptyLeafs), leaf.VisOfs)
	}

	for _, leafIdx := range slimeLeafs {
		leaf := &tree.Leafs[leafIdx]
		leafPVS := tree.LeafPVS(leaf)
		visEmpty := 0
		for _, eIdx := range emptyLeafs {
			if leafVisibleInMask(leafPVS, eIdx-1) {
				visEmpty++
			}
		}
		t.Logf("Slime leaf %d (bounds %v..%v): sees %d/%d empty leaves, visOfs=%d",
			leafIdx, leaf.BoundsMin, leaf.BoundsMax, visEmpty, len(emptyLeafs), leaf.VisOfs)
	}
}


