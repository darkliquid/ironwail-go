//go:build gogpu && !cgo
// +build gogpu,!cgo

package gogpu

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/pkg/types"
)

func TestDecalUniformBytes(t *testing.T) {
	vp := types.Mat4{
		1, 2, 3, 4,
		5, 6, 7, 8,
		9, 10, 11, 12,
		13, 14, 15, 16,
	}
	alpha := float32(0.6)

	data := DecalUniformBytes(vp, alpha)
	if len(data) != DecalUniformBufferSize {
		t.Fatalf("len(data) = %d, want %d", len(data), DecalUniformBufferSize)
	}
	matrixBytes := types.Mat4ToBytes(vp)
	if !bytes.Equal(data[:64], matrixBytes[:]) {
		t.Fatal("matrix bytes mismatch")
	}
	if got := math.Float32frombits(binary.LittleEndian.Uint32(data[64:68])); got != alpha {
		t.Fatalf("alpha = %v, want %v", got, alpha)
	}
	for i := 68; i < len(data); i++ {
		if data[i] != 0 {
			t.Fatalf("padding byte %d = %d, want 0", i, data[i])
		}
	}
}

func TestBuildDecalVertices(t *testing.T) {
	corners := [4][3]float32{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
		{10, 11, 12},
	}
	color := [4]float32{0.1, 0.2, 0.3, 0.4}

	got := BuildDecalVertices(corners, color, 3)
	if len(got) != 6 {
		t.Fatalf("len(got) = %d, want 6", len(got))
	}
	if got[0].Position != corners[0] || got[1].Position != corners[1] || got[2].Position != corners[2] {
		t.Fatal("first triangle positions mismatch")
	}
	if got[3].Position != corners[0] || got[4].Position != corners[2] || got[5].Position != corners[3] {
		t.Fatal("second triangle positions mismatch")
	}
	wantUVs := [][2]float32{
		{0.5, 0.5}, {1.0, 0.5}, {1.0, 1.0},
		{0.5, 0.5}, {1.0, 1.0}, {0.5, 1.0},
	}
	for i, want := range wantUVs {
		if got[i].TexCoord != want {
			t.Fatalf("TexCoord[%d] = %v, want %v", i, got[i].TexCoord, want)
		}
		if got[i].Color != color {
			t.Fatalf("Color[%d] = %v, want %v", i, got[i].Color, color)
		}
	}
}

func TestDecalVertexBytes(t *testing.T) {
	vertices := []DecalVertex{{
		Position: [3]float32{1, 2, 3},
		TexCoord: [2]float32{0.25, 0.75},
		Color:    [4]float32{0.1, 0.2, 0.3, 0.4},
	}}

	data := DecalVertexBytes(vertices)
	if len(data) != 36 {
		t.Fatalf("len(data) = %d, want 36", len(data))
	}

	check := func(offset int, want float32, label string) {
		got := math.Float32frombits(binary.LittleEndian.Uint32(data[offset : offset+4]))
		if got != want {
			t.Fatalf("%s = %v, want %v", label, got, want)
		}
	}

	check(0, 1, "position.x")
	check(4, 2, "position.y")
	check(8, 3, "position.z")
	check(12, 0.25, "texcoord.s")
	check(16, 0.75, "texcoord.t")
	check(20, 0.1, "color.r")
	check(24, 0.2, "color.g")
	check(28, 0.3, "color.b")
	check(32, 0.4, "color.a")
}

func TestPrepareDecalDraw(t *testing.T) {
	corners := [4][3]float32{
		{1, 2, 3},
		{4, 5, 6},
		{7, 8, 9},
		{10, 11, 12},
	}
	color := [4]float32{0.1, 0.2, 0.3, 0.4}

	got := PrepareDecalDraw(DecalDrawParams{
		Corners: corners,
		Color:   color,
		Variant: 1,
	})
	if got.VertexCount != 6 {
		t.Fatalf("VertexCount = %d, want 6", got.VertexCount)
	}
	if len(got.VertexBytes) != 6*36 {
		t.Fatalf("len(VertexBytes) = %d, want %d", len(got.VertexBytes), 6*36)
	}
	firstU := math.Float32frombits(binary.LittleEndian.Uint32(got.VertexBytes[12:16]))
	firstV := math.Float32frombits(binary.LittleEndian.Uint32(got.VertexBytes[16:20]))
	if firstU != 0.5 || firstV != 0 {
		t.Fatalf("first uv = (%v,%v), want (0.5,0)", firstU, firstV)
	}
}

func TestPrepareDecalDrawFromMark(t *testing.T) {
	params := DecalMarkParams{
		Origin:   [3]float32{1, 2, 3},
		Normal:   [3]float32{0, 0, 1},
		Size:     16,
		Rotation: 0.5,
		Variant:  2,
	}
	color := [4]float32{0.2, 0.3, 0.4, 0.5}
	var gotParams DecalMarkParams
	got := PrepareDecalDrawFromMark(params, color, func(in DecalMarkParams) ([4][3]float32, bool) {
		gotParams = in
		return [4][3]float32{
			{1, 2, 3},
			{4, 5, 6},
			{7, 8, 9},
			{10, 11, 12},
		}, true
	})
	if gotParams != params {
		t.Fatalf("buildQuad params = %+v, want %+v", gotParams, params)
	}
	if got.VertexCount != 6 {
		t.Fatalf("VertexCount = %d, want 6", got.VertexCount)
	}
	if len(got.VertexBytes) != 6*36 {
		t.Fatalf("len(VertexBytes) = %d, want %d", len(got.VertexBytes), 6*36)
	}
}

func TestPrepareDecalDrawFromMarkRejectsMissingQuadBuilderOrQuad(t *testing.T) {
	params := DecalMarkParams{Variant: 1}
	color := [4]float32{1, 1, 1, 1}
	if got := PrepareDecalDrawFromMark(params, color, nil); got.VertexCount != 0 || len(got.VertexBytes) != 0 {
		t.Fatal("PrepareDecalDrawFromMark should reject nil builders")
	}
	if got := PrepareDecalDrawFromMark(params, color, func(DecalMarkParams) ([4][3]float32, bool) {
		return [4][3]float32{}, false
	}); got.VertexCount != 0 || len(got.VertexBytes) != 0 {
		t.Fatal("PrepareDecalDrawFromMark should reject failed quad builds")
	}
}

func TestPrepareDecalDraws(t *testing.T) {
	marks := []DecalPreparedMark{
		{
			Params: DecalMarkParams{
				Origin:   [3]float32{1, 2, 3},
				Normal:   [3]float32{0, 0, 1},
				Size:     16,
				Rotation: 0.5,
				Variant:  1,
			},
			Color: [4]float32{0.1, 0.2, 0.3, 0.4},
		},
		{
			Params: DecalMarkParams{
				Origin:   [3]float32{4, 5, 6},
				Normal:   [3]float32{0, 1, 0},
				Size:     24,
				Rotation: 0.25,
				Variant:  2,
			},
			Color: [4]float32{0.5, 0.6, 0.7, 0.8},
		},
		{
			Params: DecalMarkParams{
				Origin:  [3]float32{7, 8, 9},
				Normal:  [3]float32{1, 0, 0},
				Size:    12,
				Variant: 3,
			},
			Color: [4]float32{0.9, 1.0, 0.1, 0.2},
		},
	}
	gotParams := make([]DecalMarkParams, 0, len(marks))
	draws := PrepareDecalDraws(marks, func(in DecalMarkParams) ([4][3]float32, bool) {
		gotParams = append(gotParams, in)
		switch len(gotParams) {
		case 1:
			return [4][3]float32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10, 11, 12}}, true
		case 2:
			return [4][3]float32{}, false
		default:
			return [4][3]float32{{2, 3, 4}, {5, 6, 7}, {8, 9, 10}, {11, 12, 13}}, true
		}
	})
	if len(gotParams) != len(marks) {
		t.Fatalf("builder calls = %d, want %d", len(gotParams), len(marks))
	}
	for i, want := range marks {
		if gotParams[i] != want.Params {
			t.Fatalf("builder params[%d] = %+v, want %+v", i, gotParams[i], want.Params)
		}
	}
	if len(draws) != 2 {
		t.Fatalf("len(draws) = %d, want 2", len(draws))
	}
	if draws[0].VertexCount != 6 || len(draws[0].VertexBytes) != 6*36 {
		t.Fatalf("first draw = %+v, want 6 vertices and bytes", draws[0])
	}
	if draws[1].VertexCount != 6 || len(draws[1].VertexBytes) != 6*36 {
		t.Fatalf("second draw = %+v, want 6 vertices and bytes", draws[1])
	}
	firstU := math.Float32frombits(binary.LittleEndian.Uint32(draws[0].VertexBytes[12:16]))
	firstV := math.Float32frombits(binary.LittleEndian.Uint32(draws[0].VertexBytes[16:20]))
	if firstU != 0.5 || firstV != 0 {
		t.Fatalf("first draw uv = (%v,%v), want (0.5,0)", firstU, firstV)
	}
	secondColorA := math.Float32frombits(binary.LittleEndian.Uint32(draws[1].VertexBytes[32:36]))
	if secondColorA != marks[2].Color[3] {
		t.Fatalf("second draw alpha = %v, want %v", secondColorA, marks[2].Color[3])
	}
}

func TestPrepareDecalDrawsRejectsMissingBuilderOrMarks(t *testing.T) {
	mark := DecalPreparedMark{
		Params: DecalMarkParams{Variant: 1},
		Color:  [4]float32{1, 1, 1, 1},
	}
	if got := PrepareDecalDraws(nil, func(DecalMarkParams) ([4][3]float32, bool) {
		t.Fatal("builder should not be called for nil marks")
		return [4][3]float32{}, false
	}); got != nil {
		t.Fatalf("PrepareDecalDraws(nil, builder) = %v, want nil", got)
	}
	if got := PrepareDecalDraws([]DecalPreparedMark{mark}, nil); got != nil {
		t.Fatalf("PrepareDecalDraws(mark, nil) = %v, want nil", got)
	}
}

func TestPrepareDecalDrawsWithAdapter(t *testing.T) {
	type rootDecalDraw struct {
		mark   DecalMarkParams
		color  [4]float32
		drop   bool
		called int
	}

	input := []rootDecalDraw{
		{
			mark:  DecalMarkParams{Origin: [3]float32{1, 2, 3}, Normal: [3]float32{0, 0, 1}, Size: 16, Rotation: 0.5, Variant: 1},
			color: [4]float32{0.1, 0.2, 0.3, 0.4},
		},
		{
			mark:  DecalMarkParams{Origin: [3]float32{4, 5, 6}, Normal: [3]float32{0, 1, 0}, Size: 12, Rotation: 0.25, Variant: 2},
			color: [4]float32{0.5, 0.6, 0.7, 0.8},
			drop:  true,
		},
		{
			mark:  DecalMarkParams{Origin: [3]float32{7, 8, 9}, Normal: [3]float32{1, 0, 0}, Size: 20, Variant: 3},
			color: [4]float32{0.9, 0.8, 0.7, 0.6},
		},
	}

	var adapted []DecalPreparedMark
	var built []DecalMarkParams
	draws := PrepareDecalDrawsWithAdapter(input, func(mark rootDecalDraw) (DecalPreparedMark, bool) {
		if mark.drop {
			return DecalPreparedMark{}, false
		}
		prepared := DecalPreparedMark{Params: mark.mark, Color: mark.color}
		adapted = append(adapted, prepared)
		return prepared, true
	}, func(params DecalMarkParams) ([4][3]float32, bool) {
		built = append(built, params)
		switch len(built) {
		case 1:
			return [4][3]float32{{1, 2, 3}, {4, 5, 6}, {7, 8, 9}, {10, 11, 12}}, true
		default:
			return [4][3]float32{{2, 3, 4}, {5, 6, 7}, {8, 9, 10}, {11, 12, 13}}, true
		}
	})

	if len(adapted) != 2 {
		t.Fatalf("adapted len = %d, want 2", len(adapted))
	}
	if len(built) != 2 {
		t.Fatalf("built len = %d, want 2", len(built))
	}
	if built[0] != input[0].mark {
		t.Fatalf("built[0] = %+v, want %+v", built[0], input[0].mark)
	}
	if built[1] != input[2].mark {
		t.Fatalf("built[1] = %+v, want %+v", built[1], input[2].mark)
	}
	if len(draws) != 2 {
		t.Fatalf("len(draws) = %d, want 2", len(draws))
	}
	if draws[0].VertexCount != 6 || draws[1].VertexCount != 6 {
		t.Fatalf("vertex counts = %d, %d, want 6, 6", draws[0].VertexCount, draws[1].VertexCount)
	}
	firstAlpha := math.Float32frombits(binary.LittleEndian.Uint32(draws[0].VertexBytes[32:36]))
	if firstAlpha != input[0].color[3] {
		t.Fatalf("first alpha = %v, want %v", firstAlpha, input[0].color[3])
	}
	secondU := math.Float32frombits(binary.LittleEndian.Uint32(draws[1].VertexBytes[12:16]))
	secondV := math.Float32frombits(binary.LittleEndian.Uint32(draws[1].VertexBytes[16:20]))
	if secondU != 0.5 || secondV != 0.5 {
		t.Fatalf("second draw uv = (%v,%v), want (0.5,0.5)", secondU, secondV)
	}
}

func TestPrepareDecalDrawsWithAdapterRejectsMissingInputs(t *testing.T) {
	type rootDecalDraw struct{ mark DecalMarkParams }

	builder := func(DecalMarkParams) ([4][3]float32, bool) {
		t.Fatal("builder should not be called")
		return [4][3]float32{}, false
	}
	adapter := func(rootDecalDraw) (DecalPreparedMark, bool) {
		t.Fatal("adapter should not be called")
		return DecalPreparedMark{}, false
	}

	if got := PrepareDecalDrawsWithAdapter([]rootDecalDraw(nil), adapter, builder); got != nil {
		t.Fatalf("PrepareDecalDrawsWithAdapter(nil, adapter, builder) = %v, want nil", got)
	}
	if got := PrepareDecalDrawsWithAdapter([]rootDecalDraw{{mark: DecalMarkParams{Variant: 1}}}, nil, builder); got != nil {
		t.Fatalf("PrepareDecalDrawsWithAdapter(mark, nil, builder) = %v, want nil", got)
	}
	if got := PrepareDecalDrawsWithAdapter([]rootDecalDraw{{mark: DecalMarkParams{Variant: 1}}}, adapter, nil); got != nil {
		t.Fatalf("PrepareDecalDrawsWithAdapter(mark, adapter, nil) = %v, want nil", got)
	}
}
