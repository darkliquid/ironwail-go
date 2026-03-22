//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
)

func TestBuildModelGeometry_SubmodelUsesRequestedModelFaces(t *testing.T) {
	tree := &bsp.Tree{
		Models: []bsp.DModel{
			{FirstFace: 0, NumFaces: 1},
			{FirstFace: 1, NumFaces: 1},
		},
		Faces: []bsp.TreeFace{
			{PlaneNum: 0, FirstEdge: 0, NumEdges: 3, Texinfo: 0},
			{PlaneNum: 0, FirstEdge: 3, NumEdges: 3, Texinfo: 0},
		},
		Edges: []bsp.TreeEdge{
			{V: [2]uint32{0, 1}}, {V: [2]uint32{1, 2}}, {V: [2]uint32{2, 0}},
			{V: [2]uint32{3, 4}}, {V: [2]uint32{4, 5}}, {V: [2]uint32{5, 3}},
		},
		Surfedges: []int32{0, 1, 2, 3, 4, 5},
		Vertexes: []bsp.DVertex{
			{Point: [3]float32{0, 0, 0}},
			{Point: [3]float32{1, 0, 0}},
			{Point: [3]float32{0, 1, 0}},
			{Point: [3]float32{10, 0, 0}},
			{Point: [3]float32{11, 0, 0}},
			{Point: [3]float32{10, 1, 0}},
		},
		Planes: []bsp.DPlane{{Normal: [3]float32{0, 0, 1}, Type: bsp.PlaneZ}},
	}

	geom, err := BuildModelGeometry(tree, 1)
	if err != nil {
		t.Fatalf("BuildModelGeometry failed: %v", err)
	}
	if len(geom.Vertices) != 3 || len(geom.Indices) != 3 || len(geom.Faces) != 1 {
		t.Fatalf("submodel geometry = %d verts, %d indices, %d faces; want 3/3/1", len(geom.Vertices), len(geom.Indices), len(geom.Faces))
	}
	if got := geom.Vertices[0].Position; got != [3]float32{10, 0, 0} {
		t.Fatalf("first submodel vertex = %v, want [10 0 0]", got)
	}
}

func TestProjectBrushMarkersProjectsOpaqueVertices(t *testing.T) {
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{0.5, 0.5, 0}},
		},
		Indices: []uint32{0, 1},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 2},
		},
	}
	renderer := &Renderer{
		brushModelGeometry: map[int]*WorldGeometry{1: geom},
	}
	entities := []BrushEntity{{SubmodelIndex: 1, Origin: [3]float32{}, Angles: [3]float32{}, Scale: 1, Alpha: 1}}

	markers := renderer.projectBrushMarkers(entities, types.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}, 101, 101, true, false)

	if len(markers) != 2 {
		t.Fatalf("marker count = %d, want 2", len(markers))
	}
	if markers[0].color != gogpuBrushMarkerColor || markers[0].size != gogpuBrushMarkerSize {
		t.Fatalf("first marker = %#v, want brush marker color/size", markers[0])
	}
	if markers[0].x != 50 || markers[0].y != 50 {
		t.Fatalf("first marker screen pos = (%d,%d), want (50,50)", markers[0].x, markers[0].y)
	}
}

func TestProjectBrushMarkersRespectsFacePasses(t *testing.T) {
	geom := &WorldGeometry{
		Vertices: []WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{0.25, 0, 0}},
			{Position: [3]float32{0.5, 0, 0}},
		},
		Indices: []uint32{0, 1, 1, 2, 2, 0},
		Faces: []WorldFace{
			{FirstIndex: 0, NumIndices: 2, Flags: 0},
			{FirstIndex: 2, NumIndices: 2, Flags: model.SurfDrawSky},
			{FirstIndex: 4, NumIndices: 2, Flags: model.SurfDrawWater},
		},
	}
	renderer := &Renderer{
		brushModelGeometry: map[int]*WorldGeometry{1: geom},
	}
	entities := []BrushEntity{{SubmodelIndex: 1, Origin: [3]float32{}, Angles: [3]float32{}, Scale: 1, Alpha: 1}}
	vp := types.Mat4{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}

	opaqueMarkers := renderer.projectBrushMarkers(entities, vp, 101, 101, true, false)
	if len(opaqueMarkers) != 2 {
		t.Fatalf("opaque marker count = %d, want 2 from opaque non-sky face only", len(opaqueMarkers))
	}

	skyMarkers := renderer.projectBrushMarkers(entities, vp, 101, 101, true, true)
	if len(skyMarkers) != 2 {
		t.Fatalf("sky marker count = %d, want 2 from sky face only", len(skyMarkers))
	}

	translucentMarkers := renderer.projectBrushMarkers(entities, vp, 101, 101, false, false)
	if len(translucentMarkers) != 2 {
		t.Fatalf("translucent marker count = %d, want 2 from liquid face only", len(translucentMarkers))
	}
	for _, marker := range translucentMarkers {
		if marker.alpha != 1 {
			t.Fatalf("translucent marker alpha = %v, want 1", marker.alpha)
		}
	}
}

func TestVisibleSkyBrushEntities(t *testing.T) {
	entities := []BrushEntity{
		{SubmodelIndex: 1, Alpha: 1},
		{SubmodelIndex: 2, Alpha: 0.5},
		{SubmodelIndex: 3, Alpha: 0},
	}

	sky := visibleSkyBrushEntities(entities)
	if len(sky) != 2 || sky[0].SubmodelIndex != 1 || sky[1].SubmodelIndex != 2 {
		t.Fatalf("sky = %#v, want all visible brush entities", sky)
	}
}
