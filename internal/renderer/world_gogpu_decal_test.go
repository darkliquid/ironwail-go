//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"encoding/binary"
	"math"
	"testing"

	worldgogpu "github.com/ironwail/ironwail-go/internal/renderer/world/gogpu"
)

func TestGoGPUDecalMarkParamsPreserveGeometry(t *testing.T) {
	mark := DecalMarkEntity{
		Origin:   [3]float32{1, 2, 3},
		Normal:   [3]float32{0, 0, 1},
		Size:     16,
		Rotation: 0.25,
		Variant:  DecalVariantScorch,
	}

	got := gogpuDecalMarkParams(mark)
	want := worldgogpu.DecalMarkParams{
		Origin:   mark.Origin,
		Normal:   mark.Normal,
		Size:     mark.Size,
		Rotation: mark.Rotation,
		Variant:  int(mark.Variant),
	}
	if got != want {
		t.Fatalf("gogpuDecalMarkParams() = %+v, want %+v", got, want)
	}
}

func TestGoGPUPreparedDecalMarkClampsFinalColorAndAlpha(t *testing.T) {
	draw := decalDraw{
		mark: DecalMarkEntity{
			Origin:   [3]float32{1, 2, 3},
			Normal:   [3]float32{0, 0, 1},
			Size:     12,
			Rotation: 0.5,
			Alpha:    1.5,
			Color:    [3]float32{-0.25, 0.4, 2.0},
			Variant:  DecalVariantMagic,
		},
	}

	got, ok := gogpuDecalPreparedMark(draw)
	if !ok {
		t.Fatal("gogpuDecalPreparedMark() = !ok, want ok")
	}
	if got.Params != gogpuDecalMarkParams(draw.mark) {
		t.Fatalf("Params = %+v, want %+v", got.Params, gogpuDecalMarkParams(draw.mark))
	}
	wantColor := [4]float32{0, 0.4, 1, 1}
	if got.Color != wantColor {
		t.Fatalf("Color = %v, want %v", got.Color, wantColor)
	}
}

func TestGoGPUDecalQuadUsesRootBuilder(t *testing.T) {
	params := worldgogpu.DecalMarkParams{
		Origin:   [3]float32{100, 200, 300},
		Normal:   [3]float32{0, 0, 1},
		Size:     16,
		Rotation: 0.75,
		Variant:  int(DecalVariantChip),
	}

	got, gotOK := gogpuDecalQuad(params)
	want, wantOK := buildDecalQuad(DecalMarkEntity{
		Origin:   params.Origin,
		Normal:   params.Normal,
		Size:     params.Size,
		Rotation: params.Rotation,
	})
	if gotOK != wantOK {
		t.Fatalf("ok = %v, want %v", gotOK, wantOK)
	}
	if got != want {
		t.Fatalf("quad = %v, want %v", got, want)
	}
}

func TestPrepareGoGPUDecalHALDrawsUsesRootAdapterSeam(t *testing.T) {
	draws := []decalDraw{{
		mark: DecalMarkEntity{
			Origin:   [3]float32{10, 20, 30},
			Normal:   [3]float32{0, 0, 1},
			Size:     8,
			Rotation: 0.5,
			Alpha:    1.5,
			Color:    [3]float32{-0.25, 0.4, 2.0},
			Variant:  DecalVariantScorch,
		},
	}}

	got := prepareGoGPUDecalHALDraws(draws)
	if len(got) != 1 {
		t.Fatalf("len(prepareGoGPUDecalHALDraws()) = %d, want 1", len(got))
	}
	if got[0].VertexCount != 6 {
		t.Fatalf("VertexCount = %d, want 6", got[0].VertexCount)
	}
	if len(got[0].VertexBytes) != 6*36 {
		t.Fatalf("len(VertexBytes) = %d, want %d", len(got[0].VertexBytes), 6*36)
	}

	firstU := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[12:16]))
	firstV := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[16:20]))
	if firstU != 0 || firstV != 0.5 {
		t.Fatalf("first uv = (%v,%v), want (0,0.5)", firstU, firstV)
	}

	firstR := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[20:24]))
	firstG := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[24:28]))
	firstB := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[28:32]))
	firstA := math.Float32frombits(binary.LittleEndian.Uint32(got[0].VertexBytes[32:36]))
	if firstR != 0 || firstG != 0.4 || firstB != 1 || firstA != 1 {
		t.Fatalf("first color = (%v,%v,%v,%v), want (0,0.4,1,1)", firstR, firstG, firstB, firstA)
	}
}
