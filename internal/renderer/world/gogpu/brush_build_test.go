//go:build gogpu && !cgo
// +build gogpu,!cgo

package gogpu

import (
	"reflect"
	"testing"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

func TestBuildClassifiedBrushEntityDrawSplitsOpaqueAndAlphaTestFaces(t *testing.T) {
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
			{Position: [3]float32{1, 1, 0}},
			{Position: [3]float32{2, 0, 0}},
		},
		Indices: []uint32{0, 1, 2, 1, 3, 2, 1, 4, 3},
		Faces: []worldimpl.WorldFace{
			{FirstIndex: 0, NumIndices: 3, Flags: 11, Center: [3]float32{0.5, 0.5, 0}},
			{FirstIndex: 3, NumIndices: 3, Flags: 22, Center: [3]float32{1, 0.5, 0}},
			{FirstIndex: 6, NumIndices: 3, Flags: 33, Center: [3]float32{1.5, 0.5, 0}},
		},
	}
	entity := BrushEntityParams{
		Alpha:  1,
		Frame:  2,
		Origin: [3]float32{5, 10, 0},
		Scale:  2,
	}

	draw := BuildClassifiedBrushEntityDraw(entity, geom, func(face worldimpl.WorldFace, entityAlpha float32) BrushEntityFaceClass {
		if entityAlpha != 1 {
			t.Fatalf("entityAlpha = %v, want 1", entityAlpha)
		}
		switch face.Flags {
		case 11, 33:
			return BrushEntityFaceClassOpaque
		case 22:
			return BrushEntityFaceClassAlphaTest
		default:
			return BrushEntityFaceClassSkip
		}
	})
	if draw == nil {
		t.Fatal("BuildClassifiedBrushEntityDraw returned nil")
	}
	if draw.Frame != entity.Frame {
		t.Fatalf("Frame = %d, want %d", draw.Frame, entity.Frame)
	}
	if len(draw.Vertices) != len(geom.Vertices) {
		t.Fatalf("len(Vertices) = %d, want %d", len(draw.Vertices), len(geom.Vertices))
	}
	if draw.Vertices[1].Position != ([3]float32{7, 10, 0}) {
		t.Fatalf("Vertices[1].Position = %v, want [7 10 0]", draw.Vertices[1].Position)
	}
	if !reflect.DeepEqual(draw.OpaqueIndices, []uint32{0, 1, 2, 1, 4, 3}) {
		t.Fatalf("OpaqueIndices = %v, want [0 1 2 1 4 3]", draw.OpaqueIndices)
	}
	if !reflect.DeepEqual(draw.AlphaTestIndices, []uint32{1, 3, 2}) {
		t.Fatalf("AlphaTestIndices = %v, want [1 3 2]", draw.AlphaTestIndices)
	}
	if len(draw.OpaqueFaces) != 2 || len(draw.OpaqueCenters) != 2 {
		t.Fatalf("opaque buckets = %d faces, %d centers, want 2 and 2", len(draw.OpaqueFaces), len(draw.OpaqueCenters))
	}
	if draw.OpaqueFaces[0].FirstIndex != 0 || draw.OpaqueFaces[0].NumIndices != 3 {
		t.Fatalf("opaque face 0 span = (%d,%d), want (0,3)", draw.OpaqueFaces[0].FirstIndex, draw.OpaqueFaces[0].NumIndices)
	}
	if draw.OpaqueFaces[1].FirstIndex != 3 || draw.OpaqueFaces[1].NumIndices != 3 {
		t.Fatalf("opaque face 1 span = (%d,%d), want (3,3)", draw.OpaqueFaces[1].FirstIndex, draw.OpaqueFaces[1].NumIndices)
	}
	if draw.AlphaTestFaces[0].FirstIndex != 0 || draw.AlphaTestFaces[0].NumIndices != 3 {
		t.Fatalf("alpha-test face span = (%d,%d), want (0,3)", draw.AlphaTestFaces[0].FirstIndex, draw.AlphaTestFaces[0].NumIndices)
	}
	if draw.OpaqueCenters[0] != ([3]float32{6, 11, 0}) || draw.OpaqueCenters[1] != ([3]float32{8, 11, 0}) {
		t.Fatalf("opaque centers = %v, want [[6 11 0] [8 11 0]]", draw.OpaqueCenters)
	}
	if draw.AlphaTestCenters[0] != ([3]float32{7, 11, 0}) {
		t.Fatalf("alpha-test center = %v, want [7 11 0]", draw.AlphaTestCenters[0])
	}
}

func TestBuildClassifiedBrushEntityDrawRejectsNilClassifierAndZeroAlpha(t *testing.T) {
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{{Position: [3]float32{0, 0, 0}}},
		Indices:  []uint32{0},
		Faces:    []worldimpl.WorldFace{{FirstIndex: 0, NumIndices: 1}},
	}
	if draw := BuildClassifiedBrushEntityDraw(BrushEntityParams{Alpha: 1}, geom, nil); draw != nil {
		t.Fatalf("BuildClassifiedBrushEntityDraw(nil classifier) = %+v, want nil", draw)
	}
	if draw := BuildClassifiedBrushEntityDraw(BrushEntityParams{Alpha: 0}, geom, func(worldimpl.WorldFace, float32) BrushEntityFaceClass {
		return BrushEntityFaceClassOpaque
	}); draw != nil {
		t.Fatalf("BuildClassifiedBrushEntityDraw(alpha 0) = %+v, want nil", draw)
	}
}
