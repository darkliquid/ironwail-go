//go:build gogpu && !cgo
// +build gogpu,!cgo

package gogpu

import (
	"reflect"
	"testing"

	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

func TestBuildTranslucentLiquidBrushEntityDraw(t *testing.T) {
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
			{Position: [3]float32{1, 1, 0}},
		},
		Indices: []uint32{0, 1, 2, 1, 3, 2},
		Faces: []worldimpl.WorldFace{
			{FirstIndex: 0, NumIndices: 3, Flags: 1, Center: [3]float32{0.5, 0.5, 0}},
			{FirstIndex: 3, NumIndices: 3, Flags: 2, Center: [3]float32{1, 0.5, 0}},
		},
	}
	entity := BrushEntityParams{
		Alpha:  1,
		Frame:  7,
		Origin: [3]float32{10, 20, 30},
		Scale:  2,
	}

	var planCalls []struct {
		Flags int32
		Alpha float32
	}
	var distanceInputs [][3]float32
	draw := BuildTranslucentLiquidBrushEntityDraw(entity, geom, func(face worldimpl.WorldFace, entityAlpha float32) (float32, bool) {
		planCalls = append(planCalls, struct {
			Flags int32
			Alpha float32
		}{Flags: face.Flags, Alpha: entityAlpha})
		if face.Flags == 1 {
			return 0.4, true
		}
		return 0, false
	}, func(center [3]float32) float32 {
		distanceInputs = append(distanceInputs, center)
		return center[0] + center[1] + center[2]
	})
	if draw == nil {
		t.Fatal("BuildTranslucentLiquidBrushEntityDraw returned nil")
	}
	if draw.Frame != entity.Frame {
		t.Fatalf("Frame = %d, want %d", draw.Frame, entity.Frame)
	}
	if len(planCalls) != len(geom.Faces) {
		t.Fatalf("planCalls = %d, want %d", len(planCalls), len(geom.Faces))
	}
	if planCalls[0].Alpha != 1 || planCalls[1].Alpha != 1 {
		t.Fatalf("planner alpha calls = %+v, want both 1", planCalls)
	}
	if len(draw.Vertices) != len(geom.Vertices) {
		t.Fatalf("len(Vertices) = %d, want %d", len(draw.Vertices), len(geom.Vertices))
	}
	if draw.Vertices[1].Position != ([3]float32{12, 20, 30}) {
		t.Fatalf("Vertices[1].Position = %v, want [12 20 30]", draw.Vertices[1].Position)
	}
	if !reflect.DeepEqual(draw.Indices, []uint32{0, 1, 2}) {
		t.Fatalf("Indices = %v, want [0 1 2]", draw.Indices)
	}
	if len(draw.Faces) != 1 {
		t.Fatalf("len(Faces) = %d, want 1", len(draw.Faces))
	}
	if draw.Faces[0].Face.FirstIndex != 0 || draw.Faces[0].Face.NumIndices != 3 {
		t.Fatalf("face index span = (%d,%d), want (0,3)", draw.Faces[0].Face.FirstIndex, draw.Faces[0].Face.NumIndices)
	}
	wantCenter := [3]float32{11, 21, 30}
	if draw.Faces[0].Center != wantCenter {
		t.Fatalf("face center = %v, want %v", draw.Faces[0].Center, wantCenter)
	}
	if len(distanceInputs) != 1 || distanceInputs[0] != wantCenter {
		t.Fatalf("distance inputs = %v, want [%v]", distanceInputs, wantCenter)
	}
	if draw.Faces[0].Alpha != 0.4 {
		t.Fatalf("face alpha = %v, want 0.4", draw.Faces[0].Alpha)
	}
	if draw.Faces[0].DistanceSq != 62 {
		t.Fatalf("face distance = %v, want 62", draw.Faces[0].DistanceSq)
	}
}

func TestBuildTranslucentBrushEntityDrawSplitsFacePasses(t *testing.T) {
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
		Alpha:  0.5,
		Frame:  3,
		Origin: [3]float32{5, 10, 0},
		Scale:  2,
	}

	var distanceInputs [][3]float32
	draw := BuildTranslucentBrushEntityDraw(entity, geom, func(face worldimpl.WorldFace, entityAlpha float32) (TranslucentFacePlan, bool) {
		if entityAlpha != 0.5 {
			t.Fatalf("entityAlpha = %v, want 0.5", entityAlpha)
		}
		switch face.Flags {
		case 11:
			return TranslucentFacePlan{Pass: TranslucentFacePassAlphaTest, Alpha: 0.25}, true
		case 22:
			return TranslucentFacePlan{Pass: TranslucentFacePassTranslucent, Alpha: 0.35}, true
		case 33:
			return TranslucentFacePlan{Pass: TranslucentFacePassTranslucent, Alpha: 0.45, Liquid: true}, true
		default:
			return TranslucentFacePlan{}, false
		}
	}, func(center [3]float32) float32 {
		distanceInputs = append(distanceInputs, center)
		return center[0]*10 + center[1]
	})
	if draw == nil {
		t.Fatal("BuildTranslucentBrushEntityDraw returned nil")
	}
	if draw.Frame != entity.Frame {
		t.Fatalf("Frame = %d, want %d", draw.Frame, entity.Frame)
	}
	if !reflect.DeepEqual(draw.Indices, []uint32{0, 1, 2, 1, 3, 2, 1, 4, 3}) {
		t.Fatalf("Indices = %v", draw.Indices)
	}
	if len(draw.AlphaTestFaces) != 1 || len(draw.AlphaTestCenters) != 1 {
		t.Fatalf("alpha test buckets = %d faces, %d centers, want 1 and 1", len(draw.AlphaTestFaces), len(draw.AlphaTestCenters))
	}
	if draw.AlphaTestFaces[0].FirstIndex != 0 || draw.AlphaTestFaces[0].NumIndices != 3 {
		t.Fatalf("alpha test face span = (%d,%d), want (0,3)", draw.AlphaTestFaces[0].FirstIndex, draw.AlphaTestFaces[0].NumIndices)
	}
	if draw.AlphaTestCenters[0] != ([3]float32{6, 11, 0}) {
		t.Fatalf("alpha test center = %v, want [6 11 0]", draw.AlphaTestCenters[0])
	}
	if len(draw.TranslucentFaces) != 1 {
		t.Fatalf("len(TranslucentFaces) = %d, want 1", len(draw.TranslucentFaces))
	}
	if draw.TranslucentFaces[0].Face.FirstIndex != 3 || draw.TranslucentFaces[0].Face.NumIndices != 3 {
		t.Fatalf("translucent face span = (%d,%d), want (3,3)", draw.TranslucentFaces[0].Face.FirstIndex, draw.TranslucentFaces[0].Face.NumIndices)
	}
	if draw.TranslucentFaces[0].Center != ([3]float32{7, 11, 0}) {
		t.Fatalf("translucent center = %v, want [7 11 0]", draw.TranslucentFaces[0].Center)
	}
	if draw.TranslucentFaces[0].Alpha != 0.35 {
		t.Fatalf("translucent alpha = %v, want 0.35", draw.TranslucentFaces[0].Alpha)
	}
	if len(draw.LiquidFaces) != 1 {
		t.Fatalf("len(LiquidFaces) = %d, want 1", len(draw.LiquidFaces))
	}
	if draw.LiquidFaces[0].Face.FirstIndex != 6 || draw.LiquidFaces[0].Face.NumIndices != 3 {
		t.Fatalf("liquid face span = (%d,%d), want (6,3)", draw.LiquidFaces[0].Face.FirstIndex, draw.LiquidFaces[0].Face.NumIndices)
	}
	if draw.LiquidFaces[0].Center != ([3]float32{8, 11, 0}) {
		t.Fatalf("liquid center = %v, want [8 11 0]", draw.LiquidFaces[0].Center)
	}
	if draw.LiquidFaces[0].Alpha != 0.45 {
		t.Fatalf("liquid alpha = %v, want 0.45", draw.LiquidFaces[0].Alpha)
	}
	if !reflect.DeepEqual(distanceInputs, [][3]float32{{7, 11, 0}, {8, 11, 0}}) {
		t.Fatalf("distance inputs = %v, want [[7 11 0] [8 11 0]]", distanceInputs)
	}
}

func TestBuildTranslucentBrushEntityDrawRejectsMissingCallbacks(t *testing.T) {
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{{Position: [3]float32{0, 0, 0}}},
		Indices:  []uint32{0},
		Faces:    []worldimpl.WorldFace{{FirstIndex: 0, NumIndices: 1}},
	}
	entity := BrushEntityParams{Alpha: 0.5}

	if draw := BuildTranslucentLiquidBrushEntityDraw(entity, geom, nil, func([3]float32) float32 { return 0 }); draw != nil {
		t.Fatal("BuildTranslucentLiquidBrushEntityDraw should reject nil face planners")
	}
	if draw := BuildTranslucentLiquidBrushEntityDraw(entity, geom, func(worldimpl.WorldFace, float32) (float32, bool) { return 1, true }, nil); draw != nil {
		t.Fatal("BuildTranslucentLiquidBrushEntityDraw should reject nil distance callbacks")
	}
	if draw := BuildTranslucentBrushEntityDraw(entity, geom, nil, func([3]float32) float32 { return 0 }); draw != nil {
		t.Fatal("BuildTranslucentBrushEntityDraw should reject nil face planners")
	}
	if draw := BuildTranslucentBrushEntityDraw(entity, geom, func(worldimpl.WorldFace, float32) (TranslucentFacePlan, bool) {
		return TranslucentFacePlan{Pass: TranslucentFacePassAlphaTest}, true
	}, nil); draw != nil {
		t.Fatal("BuildTranslucentBrushEntityDraw should reject nil distance callbacks")
	}
}

func TestTranslucentBrushBuildersRejectAlphaBeforePlanning(t *testing.T) {
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
		},
		Indices: []uint32{0, 1, 2},
		Faces:   []worldimpl.WorldFace{{FirstIndex: 0, NumIndices: 3}},
	}

	liquidPlannerCalled := false
	liquidDistanceCalled := false
	if draw := BuildTranslucentLiquidBrushEntityDraw(BrushEntityParams{Alpha: 0.5}, geom, func(worldimpl.WorldFace, float32) (float32, bool) {
		liquidPlannerCalled = true
		return 1, true
	}, func([3]float32) float32 {
		liquidDistanceCalled = true
		return 0
	}); draw != nil {
		t.Fatalf("BuildTranslucentLiquidBrushEntityDraw returned %+v, want nil", draw)
	}
	if liquidPlannerCalled || liquidDistanceCalled {
		t.Fatalf("liquid planner called=%v distance called=%v, want both false", liquidPlannerCalled, liquidDistanceCalled)
	}

	brushPlannerCalled := false
	brushDistanceCalled := false
	if draw := BuildTranslucentBrushEntityDraw(BrushEntityParams{Alpha: 1}, geom, func(worldimpl.WorldFace, float32) (TranslucentFacePlan, bool) {
		brushPlannerCalled = true
		return TranslucentFacePlan{Pass: TranslucentFacePassAlphaTest}, true
	}, func([3]float32) float32 {
		brushDistanceCalled = true
		return 0
	}); draw != nil {
		t.Fatalf("BuildTranslucentBrushEntityDraw returned %+v, want nil", draw)
	}
	if brushPlannerCalled || brushDistanceCalled {
		t.Fatalf("brush planner called=%v distance called=%v, want both false", brushPlannerCalled, brushDistanceCalled)
	}
}

func TestBuildTranslucentBrushEntityDrawSkipsPassSkipPlans(t *testing.T) {
	geom := &worldimpl.WorldGeometry{
		Vertices: []worldimpl.WorldVertex{
			{Position: [3]float32{0, 0, 0}},
			{Position: [3]float32{1, 0, 0}},
			{Position: [3]float32{0, 1, 0}},
		},
		Indices: []uint32{0, 1, 2},
		Faces:   []worldimpl.WorldFace{{FirstIndex: 0, NumIndices: 3}},
	}
	entity := BrushEntityParams{Alpha: 0.5}

	draw := BuildTranslucentBrushEntityDraw(entity, geom, func(worldimpl.WorldFace, float32) (TranslucentFacePlan, bool) {
		return TranslucentFacePlan{Pass: TranslucentFacePassSkip}, true
	}, func([3]float32) float32 {
		t.Fatal("distance callback should not be called for skipped faces")
		return 0
	})
	if draw != nil {
		t.Fatalf("BuildTranslucentBrushEntityDraw returned %+v, want nil", draw)
	}
}
