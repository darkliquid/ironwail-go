package renderer

// Auxiliary world renderer tests: face vertex extraction, face stats, lightmap helpers.

import (
	"sort"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/renderer/lightmap"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/gogpu/wgpu"
)

func TestExtractFaceVertices_FlippedNormal(t *testing.T) {
	tree := &bsp.Tree{
		Planes: []bsp.DPlane{
			{Normal: [3]float32{1, 0, 0}, Dist: 0},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}},
			{V: [2]uint32{1, 2}},
			{V: [2]uint32{2, 0}},
		},
		Surfedges: []int32{0, 1, 2},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{0, 10, 0}},
			{Point: [3]float32{0, 5, 10}},
		},
	}

	// Front side
	faceFront := &bsp.TreeFace{
		PlaneNum:  0,
		Side:      0, // Front side
		FirstEdge: 0,
		NumEdges:  3,
	}

	vertsFront, _, err := extractFaceVertices(tree, faceFront, nil, nil)
	if err != nil {
		t.Fatalf("extractFaceVertices failed: %v", err)
	}

	expectedNormal := [3]float32{1, 0, 0}
	if vertsFront[0].Normal != expectedNormal {
		t.Errorf("Front face normal = %v, want %v",
			vertsFront[0].Normal, expectedNormal)
	}

	// Back side (should flip normal)
	faceBack := &bsp.TreeFace{
		PlaneNum:  0,
		Side:      1, // Back side
		FirstEdge: 0,
		NumEdges:  3,
	}

	vertsBack, _, err := extractFaceVertices(tree, faceBack, nil, nil)
	if err != nil {
		t.Fatalf("extractFaceVertices failed: %v", err)
	}

	expectedFlippedNormal := [3]float32{-1, 0, 0}
	if vertsBack[0].Normal != expectedFlippedNormal {
		t.Errorf("Back face normal = %v, want %v",
			vertsBack[0].Normal, expectedFlippedNormal)
	}
}

func TestSummarizeGoGPUWorldFaceStats(t *testing.T) {
	liquidAlpha := worldLiquidAlphaSettings{
		water: 0.5,
		lava:  1,
		slime: 1,
		tele:  1,
	}
	faces := []WorldFace{
		{NumIndices: 6, Flags: model.SurfDrawSky, LightmapIndex: -1},
		{NumIndices: 3, Flags: 0, LightmapIndex: 0},
		{NumIndices: 9, Flags: model.SurfDrawFence, LightmapIndex: 1},
		{NumIndices: 12, Flags: model.SurfDrawTurb | model.SurfDrawLava, LightmapIndex: 2},
		{NumIndices: 15, Flags: model.SurfDrawTurb | model.SurfDrawWater, LightmapIndex: -1},
	}

	stats := summarizeGoGPUWorldFaceStats(faces, liquidAlpha)

	if stats.TotalFaces != 5 {
		t.Fatalf("TotalFaces = %d, want 5", stats.TotalFaces)
	}
	if stats.TotalTriangles != 15 {
		t.Fatalf("TotalTriangles = %d, want 15", stats.TotalTriangles)
	}
	if stats.LightmappedFaces != 3 {
		t.Fatalf("LightmappedFaces = %d, want 3", stats.LightmappedFaces)
	}
	if stats.LitWaterFaces != 1 {
		t.Fatalf("LitWaterFaces = %d, want 1", stats.LitWaterFaces)
	}
	if stats.TurbulentFaces != 2 {
		t.Fatalf("TurbulentFaces = %d, want 2", stats.TurbulentFaces)
	}
	if stats.SkyFaces != 1 || stats.SkyTriangles != 2 {
		t.Fatalf("sky stats = (%d faces, %d tris), want (1, 2)", stats.SkyFaces, stats.SkyTriangles)
	}
	if stats.OpaqueFaces != 1 || stats.OpaqueTriangles != 1 {
		t.Fatalf("opaque stats = (%d faces, %d tris), want (1, 1)", stats.OpaqueFaces, stats.OpaqueTriangles)
	}
	if stats.AlphaTestFaces != 1 || stats.AlphaTestTriangles != 3 {
		t.Fatalf("alpha-test stats = (%d faces, %d tris), want (1, 3)", stats.AlphaTestFaces, stats.AlphaTestTriangles)
	}
	if stats.OpaqueLiquidFaces != 1 || stats.OpaqueLiquidTriangles != 4 {
		t.Fatalf("opaque liquid stats = (%d faces, %d tris), want (1, 4)", stats.OpaqueLiquidFaces, stats.OpaqueLiquidTriangles)
	}
	if stats.TranslucentLiquidFaces != 1 || stats.TranslucentLiquidTriangles != 5 {
		t.Fatalf("translucent liquid stats = (%d faces, %d tris), want (1, 5)", stats.TranslucentLiquidFaces, stats.TranslucentLiquidTriangles)
	}
	if stats.UnclassifiedFaces != 0 || stats.UnclassifiedTriangles != 0 {
		t.Fatalf("unclassified stats = (%d faces, %d tris), want 0", stats.UnclassifiedFaces, stats.UnclassifiedTriangles)
	}
}

func TestSortGoGPUWorldFaceDrawsByMaterial(t *testing.T) {
	texA := &wgpu.BindGroup{}
	texB := &wgpu.BindGroup{}
	lightA := &wgpu.BindGroup{}
	lightB := &wgpu.BindGroup{}
	fullA := &wgpu.BindGroup{}
	fullB := &wgpu.BindGroup{}
	draws := []gogpuWorldFaceDraw{
		{
			face:                WorldFace{FirstIndex: 30},
			textureBindGroup:    texB,
			lightmapBindGroup:   lightB,
			fullbrightBindGroup: fullA,
		},
		{
			face:                WorldFace{FirstIndex: 10},
			textureBindGroup:    texA,
			lightmapBindGroup:   lightB,
			fullbrightBindGroup: fullB,
		},
		{
			face:                WorldFace{FirstIndex: 20},
			textureBindGroup:    texA,
			lightmapBindGroup:   lightA,
			fullbrightBindGroup: fullA,
		},
	}

	sort.Slice(draws, func(i, j int) bool {
		return gogpuWorldFaceBatchKeyLess(gogpuWorldFaceBatchKeyForDraw(draws[i]), gogpuWorldFaceBatchKeyForDraw(draws[j]))
	})

	if draws[0].face.FirstIndex != 20 {
		t.Fatalf("draws[0].FirstIndex = %d, want 20", draws[0].face.FirstIndex)
	}
	if draws[1].face.FirstIndex != 10 {
		t.Fatalf("draws[1].FirstIndex = %d, want 10", draws[1].face.FirstIndex)
	}
	if draws[2].face.FirstIndex != 30 {
		t.Fatalf("draws[2].FirstIndex = %d, want 30", draws[2].face.FirstIndex)
	}
}

func TestAppendGoGPUOpaqueWorldFaceBatches(t *testing.T) {
	texA := &wgpu.BindGroup{}
	texB := &wgpu.BindGroup{}
	lightA := &wgpu.BindGroup{}
	fullA := &wgpu.BindGroup{}
	draws := []gogpuWorldFaceDraw{
		{
			face:                WorldFace{FirstIndex: 0, NumIndices: 3},
			textureBindGroup:    texA,
			lightmapBindGroup:   lightA,
			fullbrightBindGroup: fullA,
		},
		{
			face:                WorldFace{FirstIndex: 3, NumIndices: 3},
			textureBindGroup:    texA,
			lightmapBindGroup:   lightA,
			fullbrightBindGroup: fullA,
		},
		{
			face:                WorldFace{FirstIndex: 6, NumIndices: 3},
			textureBindGroup:    texB,
			lightmapBindGroup:   lightA,
			fullbrightBindGroup: fullA,
		},
	}
	worldIndices := []uint32{10, 11, 12, 20, 21, 22, 30, 31, 32}

	indices, batches := appendGoGPUOpaqueWorldFaceBatches(nil, nil, draws, worldIndices)

	if len(indices) != len(worldIndices) {
		t.Fatalf("len(indices) = %d, want %d", len(indices), len(worldIndices))
	}
	if len(batches) != 2 {
		t.Fatalf("len(batches) = %d, want 2", len(batches))
	}
	if batches[0].firstIndex != 0 || batches[0].numIndices != 6 {
		t.Fatalf("batches[0] = %+v, want firstIndex=0 numIndices=6", batches[0])
	}
	if batches[1].firstIndex != 6 || batches[1].numIndices != 3 {
		t.Fatalf("batches[1] = %+v, want firstIndex=6 numIndices=3", batches[1])
	}
}

func TestBuildWorldLightmapPageRGBA_DefaultsUntouchedTexelsToBlack(t *testing.T) {
	page := &worldimpl.WorldLightmapPage{Width: 2, Height: 1}
	got := lightmap.CompositePageRGBA(page, lightmap.DefaultStyleValues())
	want := []byte{
		0, 0, 0, 255,
		0, 0, 0, 255,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d (full rgba=%v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildWorldLightmapPageRGBA_CompositesSurfaceIntoBlackPage(t *testing.T) {
	page := &worldimpl.WorldLightmapPage{
		Width:  2,
		Height: 1,
		Surfaces: []worldimpl.WorldLightmapSurface{{
			X:       0,
			Y:       0,
			Width:   1,
			Height:  1,
			Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
			Samples: []byte{128, 64, 32},
		}},
	}
	got := lightmap.CompositePageRGBA(page, lightmap.DefaultStyleValues())
	want := []byte{
		128, 64, 32, 255,
		0, 0, 0, 255,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d (full rgba=%v)", i, got[i], want[i], got)
		}
	}
}

func TestBuildWorldLightmapPageRGBA_ReusesCachedBufferForDirtySurfaces(t *testing.T) {
	page := &WorldLightmapPage{
		Width:  2,
		Height: 1,
		Surfaces: []WorldLightmapSurface{
			{
				X:       0,
				Y:       0,
				Width:   1,
				Height:  1,
				Styles:  [bsp.MaxLightmaps]uint8{0, 255, 255, 255},
				Samples: []byte{200, 0, 0},
			},
			{
				X:       1,
				Y:       0,
				Width:   1,
				Height:  1,
				Styles:  [bsp.MaxLightmaps]uint8{1, 255, 255, 255},
				Samples: []byte{0, 200, 0},
			},
		},
	}
	values := lightmap.DefaultStyleValues()
	values[1] = 1
	first := lightmap.CompositePageRGBA(page, values)
	if len(first) == 0 {
		t.Fatal("expected initial lightmap RGBA")
	}
	firstPixel := first[0]
	cleanPixel := first[4:8]
	cachePtr := &first[0]

	values[0] = 0.5
	page.Dirty = true
	page.Surfaces[0].Dirty = true
	second := lightmap.CompositePageRGBA(page, values)
	if len(second) == 0 {
		t.Fatal("expected recomposited lightmap RGBA")
	}
	if &second[0] != cachePtr {
		t.Fatal("expected dirty recomposite to reuse cached lightmap buffer")
	}
	if second[0] >= firstPixel {
		t.Fatalf("expected dirty surface to darken, got %d want < %d", second[0], firstPixel)
	}
	for i := range cleanPixel {
		if second[4+i] != cleanPixel[i] {
			t.Fatalf("clean surface byte %d changed: got %d want %d", i, second[4+i], cleanPixel[i])
		}
	}
}

func TestExtractLightmapRegionRGBAReusesScratchBuffer(t *testing.T) {
	rgba := []byte{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
	}
	first := lightmap.ExtractRegionRGBA(nil, rgba, 2, 1, 0, 1, 2)
	if len(first) != 8 {
		t.Fatalf("len(first) = %d, want 8", len(first))
	}
	if first[0] != 5 || first[4] != 13 {
		t.Fatalf("unexpected region bytes: %v", first)
	}
	ptr := &first[0]
	second := lightmap.ExtractRegionRGBA(first, rgba, 2, 1, 0, 1, 2)
	if len(second) != 8 {
		t.Fatalf("len(second) = %d, want 8", len(second))
	}
	if &second[0] != ptr {
		t.Fatal("expected region extraction to reuse scratch buffer")
	}
}

func TestWorldLiquidAlphaSettingsForGeometryUsesCachedWorldFacts(t *testing.T) {
	geom := &WorldGeometry{
		LiquidAlphaOverrides: worldimpl.LiquidAlphaOverrides{
			HasWater: true,
			Water:    0.25,
		},
		TransparentWaterSafe: true,
	}
	got := worldLiquidAlphaSettingsForGeometry(geom)
	if got.water != 0.25 {
		t.Fatalf("water = %v, want 0.25 from cached override", got.water)
	}

	geom.TransparentWaterSafe = false
	geom.LiquidAlphaOverrides = worldimpl.LiquidAlphaOverrides{}
	got = worldLiquidAlphaSettingsForGeometry(geom)
	if got.water != 1 || got.lava != 1 || got.slime != 1 || got.tele != 1 {
		t.Fatalf("unsafe transparent-water map without explicit override should force opaque liquids, got %+v", got)
	}
}

func TestRendererHasTranslucentWorldLiquidFacesGoGPUUsesCachedGeometryFacts(t *testing.T) {
	r := &Renderer{
		worldRendererState: worldRendererState{
			worldData: &WorldRenderData{
				Geometry: &WorldGeometry{
					LiquidFaceTypes: model.SurfDrawWater,
					LiquidAlphaOverrides: worldimpl.LiquidAlphaOverrides{
						HasWater: true,
						Water:    0.25,
					},
					TransparentWaterSafe: true,
				},
			},
		},
	}
	if !r.hasTranslucentWorldLiquidFacesGoGPU() {
		t.Fatal("expected cached liquid face types to report translucent world liquids")
	}
}
